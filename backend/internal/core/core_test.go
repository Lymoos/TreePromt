package core

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	"prompttree/backend/internal/testdb"
)

func TestPasswordHashAndVerify(t *testing.T) {
	h, err := HashPassword("правильный пароль 🔑")
	if err != nil {
		t.Fatal(err)
	}
	if ok, err := VerifyPassword("правильный пароль 🔑", h); err != nil || !ok {
		t.Fatalf("correct password rejected: ok=%v err=%v", ok, err)
	}
	if ok, _ := VerifyPassword("неправильный", h); ok {
		t.Fatal("wrong password accepted")
	}
	h2, _ := HashPassword("правильный пароль 🔑")
	if h == h2 {
		t.Fatal("hashes must use a random salt")
	}
	if _, err := VerifyPassword("x", "$argon2id$garbage"); err == nil {
		t.Fatal("malformed hash must fail")
	}
}

func TestAccessToken(t *testing.T) {
	iss := NewTokenIssuer([]byte("0123456789abcdef0123456789abcdef"))
	id := Identity{UserID: uuid.New(), DeviceID: uuid.New()}
	tok, _, err := iss.IssueAccess(id)
	if err != nil {
		t.Fatal(err)
	}
	got, err := iss.VerifyAccess(tok)
	if err != nil || got != id {
		t.Fatalf("verify: %v %v", got, err)
	}

	other := NewTokenIssuer([]byte("ffffffffffffffffffffffffffffffff"))
	if _, err := other.VerifyAccess(tok); err == nil {
		t.Fatal("token signed with another secret accepted")
	}

	iss.now = func() time.Time { return time.Now().Add(AccessTTL + time.Minute) }
	if _, err := iss.VerifyAccess(tok); err == nil {
		t.Fatal("expired token accepted")
	}
	if _, err := iss.VerifyAccess("not.a.jwt"); err == nil {
		t.Fatal("garbage accepted")
	}
}

func newAuth(t *testing.T, allowRegistration bool) *AuthService {
	pool := testdb.New(t)
	s, err := NewAuthService(pool, NewTokenIssuer([]byte("0123456789abcdef0123456789abcdef")), allowRegistration)
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func TestRegistrationClosedAfterFirstUser(t *testing.T) {
	s := newAuth(t, false)
	ctx := context.Background()
	if _, err := s.Register(ctx, "Max@Example.com", "длинный-пароль-1", "Максим"); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Register(ctx, "other@example.com", "длинный-пароль-2", ""); !errors.Is(err, ErrRegistrationClosed) {
		t.Fatalf("second registration: %v", err)
	}
}

func TestRegisterValidation(t *testing.T) {
	s := newAuth(t, true)
	ctx := context.Background()
	if _, err := s.Register(ctx, "not-an-email", "длинный-пароль-1", ""); !errors.Is(err, ErrInvalidEmail) {
		t.Fatalf("email: %v", err)
	}
	if _, err := s.Register(ctx, "a@example.com", "short", ""); !errors.Is(err, ErrWeakPassword) {
		t.Fatalf("password: %v", err)
	}
	if _, err := s.Register(ctx, "a@example.com", "длинный-пароль-1", ""); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Register(ctx, "A@example.com", "длинный-пароль-1", ""); !errors.Is(err, ErrEmailTaken) {
		t.Fatalf("duplicate (case-insensitive): %v", err)
	}
}

func TestLoginRefreshRotationAndReuse(t *testing.T) {
	s := newAuth(t, true)
	ctx := context.Background()
	if _, err := s.Register(ctx, "max@example.com", "длинный-пароль-1", ""); err != nil {
		t.Fatal(err)
	}
	dev := Device{ID: uuid.New(), Name: "Pixel", Platform: "android"}

	if _, err := s.Login(ctx, "max@example.com", "wrong-password!", dev); !errors.Is(err, ErrInvalidCredentials) {
		t.Fatalf("wrong password: %v", err)
	}
	if _, err := s.Login(ctx, "nobody@example.com", "длинный-пароль-1", dev); !errors.Is(err, ErrInvalidCredentials) {
		t.Fatalf("unknown email: %v", err)
	}

	t1, err := s.Login(ctx, "MAX@example.com", "длинный-пароль-1", dev)
	if err != nil {
		t.Fatal(err)
	}
	t2, err := s.Refresh(ctx, t1.RefreshToken)
	if err != nil {
		t.Fatal(err)
	}
	if t2.RefreshToken == t1.RefreshToken {
		t.Fatal("refresh token must rotate")
	}

	// Повтор старого токена сразу — гонка двух запросов: отказ без отзыва.
	if _, err := s.Refresh(ctx, t1.RefreshToken); !errors.Is(err, ErrStaleRefresh) {
		t.Fatalf("immediate reuse: %v", err)
	}
	// Повтор после окна — кража: отзывается вся семья, включая новый токен.
	s.now = func() time.Time { return time.Now().Add(rotationGrace + time.Second) }
	if _, err := s.Refresh(ctx, t1.RefreshToken); !errors.Is(err, ErrInvalidRefreshToken) {
		t.Fatalf("late reuse: %v", err)
	}
	if _, err := s.Refresh(ctx, t2.RefreshToken); !errors.Is(err, ErrInvalidRefreshToken) {
		t.Fatalf("family must be revoked: %v", err)
	}
}

func TestDeviceCannotBeTakenByAnotherUser(t *testing.T) {
	s := newAuth(t, true)
	ctx := context.Background()
	for _, e := range []string{"a@example.com", "b@example.com"} {
		if _, err := s.Register(ctx, e, "длинный-пароль-1", ""); err != nil {
			t.Fatal(err)
		}
	}
	dev := Device{ID: uuid.New(), Name: "PC", Platform: "windows"}
	if _, err := s.Login(ctx, "a@example.com", "длинный-пароль-1", dev); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Login(ctx, "b@example.com", "длинный-пароль-1", dev); !errors.Is(err, ErrDeviceOwnedByOther) {
		t.Fatalf("got %v", err)
	}
}

func TestLogoutRevokesSession(t *testing.T) {
	s := newAuth(t, true)
	ctx := context.Background()
	if _, err := s.Register(ctx, "a@example.com", "длинный-пароль-1", ""); err != nil {
		t.Fatal(err)
	}
	tok, err := s.Login(ctx, "a@example.com", "длинный-пароль-1", Device{ID: uuid.New()})
	if err != nil {
		t.Fatal(err)
	}
	if err := s.Logout(ctx, tok.RefreshToken); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Refresh(ctx, tok.RefreshToken); !errors.Is(err, ErrInvalidRefreshToken) {
		t.Fatalf("got %v", err)
	}
}
