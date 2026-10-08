package core

import (
	"context"
	"errors"
	"fmt"
	"net/mail"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	ErrInvalidCredentials  = errors.New("invalid email or password")
	ErrRegistrationClosed  = errors.New("registration is closed")
	ErrEmailTaken          = errors.New("email already registered")
	ErrWeakPassword        = errors.New("password must be 10 to 1024 characters")
	ErrInvalidEmail        = errors.New("invalid email")
	ErrDeviceOwnedByOther  = errors.New("device belongs to another user")
	ErrInvalidDevice       = errors.New("invalid device")
	ErrStaleRefresh        = errors.New("refresh token was just rotated; use the newest one")
	ErrInvalidRefreshToken = errors.New("invalid refresh token")
)

// Повторное предъявление уже ротированного refresh-токена в течение этого окна
// считаем гонкой двух запросов, а не кражей: семью сессий не отзываем.
const rotationGrace = 30 * time.Second

type Device struct {
	ID       uuid.UUID `json:"id"`
	Name     string    `json:"name"`
	Platform string    `json:"platform"`
}

type Tokens struct {
	AccessToken     string    `json:"access_token"`
	AccessExpiresAt time.Time `json:"access_expires_at"`
	RefreshToken    string    `json:"refresh_token"`
	UserID          uuid.UUID `json:"user_id"`
	DeviceID        uuid.UUID `json:"device_id"`
}

type User struct {
	ID    uuid.UUID `json:"id"`
	Email string    `json:"email"`
	Name  string    `json:"name"`
}

type AuthService struct {
	pool              *pgxpool.Pool
	tokens            *TokenIssuer
	allowRegistration bool
	now               func() time.Time
	dummyHash         string
}

func NewAuthService(pool *pgxpool.Pool, tokens *TokenIssuer, allowRegistration bool) (*AuthService, error) {
	// Хэш-пустышка для выравнивания времени ответа, когда email не найден.
	dummy, err := HashPassword("timing-equalizer-password")
	if err != nil {
		return nil, err
	}
	return &AuthService{pool: pool, tokens: tokens, allowRegistration: allowRegistration, now: time.Now, dummyHash: dummy}, nil
}

func normalizeEmail(s string) (string, error) {
	s = strings.ToLower(strings.TrimSpace(s))
	a, err := mail.ParseAddress(s)
	if err != nil || a.Address != s || len(s) > 254 {
		return "", ErrInvalidEmail
	}
	return s, nil
}

func checkPassword(p string) error {
	if n := utf8.RuneCountInString(p); n < 10 || len(p) > 1024 {
		return ErrWeakPassword
	}
	return nil
}

func (s *AuthService) Register(ctx context.Context, email, password, name string) (User, error) {
	email, err := normalizeEmail(email)
	if err != nil {
		return User{}, err
	}
	if err := checkPassword(password); err != nil {
		return User{}, err
	}
	hash, err := HashPassword(password)
	if err != nil {
		return User{}, err
	}
	u := User{ID: uuid.Must(uuid.NewV7()), Email: email, Name: strings.TrimSpace(name)}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return User{}, err
	}
	defer tx.Rollback(ctx)
	// Блокировка, чтобы два одновременных запроса не создали двух «первых» пользователей.
	if _, err := tx.Exec(ctx, `LOCK TABLE core.users IN SHARE ROW EXCLUSIVE MODE`); err != nil {
		return User{}, err
	}
	if !s.allowRegistration {
		var n int
		if err := tx.QueryRow(ctx, `SELECT count(*) FROM core.users`).Scan(&n); err != nil {
			return User{}, err
		}
		if n > 0 {
			return User{}, ErrRegistrationClosed
		}
	}
	_, err = tx.Exec(ctx, `INSERT INTO core.users (id, email, password_hash, name) VALUES ($1, $2, $3, $4)`,
		u.ID, u.Email, hash, u.Name)
	if isUniqueViolation(err) {
		return User{}, ErrEmailTaken
	}
	if err != nil {
		return User{}, err
	}
	return u, tx.Commit(ctx)
}

func (s *AuthService) Login(ctx context.Context, email, password string, dev Device) (Tokens, error) {
	if dev.ID == uuid.Nil || utf8.RuneCountInString(dev.Name) > 100 || utf8.RuneCountInString(dev.Platform) > 50 {
		return Tokens{}, ErrInvalidDevice
	}
	email, err := normalizeEmail(email)
	if err != nil {
		_, _ = VerifyPassword(password, s.dummyHash)
		return Tokens{}, ErrInvalidCredentials
	}
	var userID uuid.UUID
	var hash string
	err = s.pool.QueryRow(ctx, `SELECT id, password_hash FROM core.users WHERE email = $1`, email).Scan(&userID, &hash)
	if errors.Is(err, pgx.ErrNoRows) {
		_, _ = VerifyPassword(password, s.dummyHash)
		return Tokens{}, ErrInvalidCredentials
	}
	if err != nil {
		return Tokens{}, err
	}
	ok, err := VerifyPassword(password, hash)
	if err != nil {
		return Tokens{}, err
	}
	if !ok {
		return Tokens{}, ErrInvalidCredentials
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return Tokens{}, err
	}
	defer tx.Rollback(ctx)

	var owner uuid.UUID
	err = tx.QueryRow(ctx, `SELECT user_id FROM core.devices WHERE id = $1 FOR UPDATE`, dev.ID).Scan(&owner)
	switch {
	case errors.Is(err, pgx.ErrNoRows):
		_, err = tx.Exec(ctx, `INSERT INTO core.devices (id, user_id, name, platform) VALUES ($1, $2, $3, $4)`,
			dev.ID, userID, dev.Name, dev.Platform)
	case err != nil:
	case owner != userID:
		return Tokens{}, ErrDeviceOwnedByOther
	default:
		// Повторный вход по паролю снимает отзыв устройства.
		_, err = tx.Exec(ctx, `UPDATE core.devices SET name = $2, platform = $3, last_seen_at = now(), revoked_at = NULL WHERE id = $1`,
			dev.ID, dev.Name, dev.Platform)
	}
	if err != nil {
		return Tokens{}, err
	}
	t, err := s.newSession(ctx, tx, Identity{UserID: userID, DeviceID: dev.ID}, uuid.Must(uuid.NewV7()))
	if err != nil {
		return Tokens{}, err
	}
	return t, tx.Commit(ctx)
}

func (s *AuthService) newSession(ctx context.Context, tx pgx.Tx, id Identity, family uuid.UUID) (Tokens, error) {
	refresh, hash, err := newRefreshToken()
	if err != nil {
		return Tokens{}, err
	}
	_, err = tx.Exec(ctx, `INSERT INTO core.sessions (id, user_id, device_id, family_id, refresh_token_hash, expires_at)
		VALUES ($1, $2, $3, $4, $5, $6)`,
		uuid.Must(uuid.NewV7()), id.UserID, id.DeviceID, family, hash, s.now().Add(RefreshTTL))
	if err != nil {
		return Tokens{}, err
	}
	access, exp, err := s.tokens.IssueAccess(id)
	if err != nil {
		return Tokens{}, err
	}
	return Tokens{AccessToken: access, AccessExpiresAt: exp, RefreshToken: refresh, UserID: id.UserID, DeviceID: id.DeviceID}, nil
}

// Refresh rotates the refresh token. Presenting an already-rotated token outside
// the grace window is treated as theft: the whole session family is revoked.
func (s *AuthService) Refresh(ctx context.Context, refresh string) (Tokens, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return Tokens{}, err
	}
	defer tx.Rollback(ctx)

	var (
		sessID, userID, deviceID, family uuid.UUID
		expires                          time.Time
		rotated, revoked, devRevoked     *time.Time
	)
	err = tx.QueryRow(ctx, `SELECT s.id, s.user_id, s.device_id, s.family_id, s.expires_at, s.rotated_at, s.revoked_at, d.revoked_at
		FROM core.sessions s JOIN core.devices d ON d.id = s.device_id
		WHERE s.refresh_token_hash = $1 FOR UPDATE OF s`, hashRefresh(refresh)).
		Scan(&sessID, &userID, &deviceID, &family, &expires, &rotated, &revoked, &devRevoked)
	if errors.Is(err, pgx.ErrNoRows) {
		return Tokens{}, ErrInvalidRefreshToken
	}
	if err != nil {
		return Tokens{}, err
	}
	now := s.now()
	if revoked != nil || devRevoked != nil || now.After(expires) {
		return Tokens{}, ErrInvalidRefreshToken
	}
	if rotated != nil {
		if now.Sub(*rotated) < rotationGrace {
			return Tokens{}, ErrStaleRefresh
		}
		if _, err := tx.Exec(ctx, `UPDATE core.sessions SET revoked_at = now() WHERE family_id = $1 AND revoked_at IS NULL`, family); err != nil {
			return Tokens{}, err
		}
		if err := tx.Commit(ctx); err != nil {
			return Tokens{}, err
		}
		return Tokens{}, ErrInvalidRefreshToken
	}
	if _, err := tx.Exec(ctx, `UPDATE core.sessions SET rotated_at = $2 WHERE id = $1`, sessID, now); err != nil {
		return Tokens{}, err
	}
	if _, err := tx.Exec(ctx, `UPDATE core.devices SET last_seen_at = now() WHERE id = $1`, deviceID); err != nil {
		return Tokens{}, err
	}
	t, err := s.newSession(ctx, tx, Identity{UserID: userID, DeviceID: deviceID}, family)
	if err != nil {
		return Tokens{}, err
	}
	return t, tx.Commit(ctx)
}

// Logout revokes the session family of the given refresh token. Unknown tokens are ignored.
func (s *AuthService) Logout(ctx context.Context, refresh string) error {
	_, err := s.pool.Exec(ctx, `UPDATE core.sessions SET revoked_at = now()
		WHERE revoked_at IS NULL AND family_id = (SELECT family_id FROM core.sessions WHERE refresh_token_hash = $1)`,
		hashRefresh(refresh))
	return err
}

func (s *AuthService) Me(ctx context.Context, userID uuid.UUID) (User, error) {
	var u User
	err := s.pool.QueryRow(ctx, `SELECT id, email, name FROM core.users WHERE id = $1`, userID).Scan(&u.ID, &u.Email, &u.Name)
	if err != nil {
		return User{}, fmt.Errorf("load user: %w", err)
	}
	return u, nil
}

func isUniqueViolation(err error) bool {
	var pg *pgconn.PgError
	return errors.As(err, &pg) && pg.Code == "23505"
}
