package core

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

const (
	AccessTTL  = 15 * time.Minute
	RefreshTTL = 60 * 24 * time.Hour
	issuer     = "prompttree"
)

var ErrInvalidToken = errors.New("invalid token")

// Claims — содержимое access-токена. JWT используется только для аутентификации (ТЗ п. 5.3).
type Claims struct {
	DeviceID uuid.UUID `json:"did"`
	jwt.RegisteredClaims
}

type Identity struct {
	UserID   uuid.UUID
	DeviceID uuid.UUID
}

type TokenIssuer struct {
	secret []byte
	now    func() time.Time
}

func NewTokenIssuer(secret []byte) *TokenIssuer {
	return &TokenIssuer{secret: secret, now: time.Now}
}

func (t *TokenIssuer) IssueAccess(id Identity) (string, time.Time, error) {
	now := t.now()
	exp := now.Add(AccessTTL)
	claims := Claims{
		DeviceID: id.DeviceID,
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    issuer,
			Subject:   id.UserID.String(),
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(exp),
		},
	}
	s, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(t.secret)
	return s, exp, err
}

func (t *TokenIssuer) VerifyAccess(token string) (Identity, error) {
	var c Claims
	_, err := jwt.ParseWithClaims(token, &c, func(*jwt.Token) (any, error) { return t.secret, nil },
		jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}),
		jwt.WithIssuer(issuer),
		jwt.WithExpirationRequired(),
		jwt.WithTimeFunc(t.now),
	)
	if err != nil {
		return Identity{}, ErrInvalidToken
	}
	uid, err := uuid.Parse(c.Subject)
	if err != nil || c.DeviceID == uuid.Nil {
		return Identity{}, ErrInvalidToken
	}
	return Identity{UserID: uid, DeviceID: c.DeviceID}, nil
}

// newRefreshToken returns the token for the client and the hash stored in the DB.
func newRefreshToken() (string, []byte, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", nil, err
	}
	tok := base64.RawURLEncoding.EncodeToString(b)
	return tok, hashRefresh(tok), nil
}

func hashRefresh(tok string) []byte {
	h := sha256.Sum256([]byte(tok))
	return h[:]
}
