package auth

import (
	"bytes"
	"testing"

	"carryapi/internal/crypto"
	"carryapi/internal/db"
	"carryapi/internal/settings"
	"carryapi/internal/user"
)

func newLoginService(t *testing.T) (*LoginService, *user.Store) {
	t.Helper()
	d, _ := db.Open(":memory:")
	db.Migrate(d)
	t.Cleanup(func() { d.Close() })
	c, _ := crypto.New(bytes.Repeat([]byte{1}, 32))
	us := user.New(d, c)
	ss := NewSessionStore(d)
	st := settings.New(d)
	return NewLoginService(us, ss, st), us
}

func TestLoginSuccessNo2FA(t *testing.T) {
	ls, us := newLoginService(t)
	hash, _ := HashPassword("pw123")
	us.Create("login@x.com", hash, "user")
	sess, requires2FA, err := ls.Login("login@x.com", "pw123")
	if err != nil {
		t.Fatalf("Login: %v", err)
	}
	if requires2FA {
		t.Error("should not require 2FA")
	}
	if sess.Token == "" {
		t.Error("expected session token")
	}
}

func TestLoginWrongPassword(t *testing.T) {
	ls, us := newLoginService(t)
	hash, _ := HashPassword("pw123")
	us.Create("login@x.com", hash, "user")
	_, _, err := ls.Login("login@x.com", "wrong")
	if err != ErrInvalidCredentials {
		t.Errorf("err = %v, want ErrInvalidCredentials", err)
	}
}

func TestLoginNonexistentUser(t *testing.T) {
	ls, _ := newLoginService(t)
	_, _, err := ls.Login("nobody@x.com", "pw")
	if err != ErrInvalidCredentials {
		t.Errorf("err = %v, want ErrInvalidCredentials (no enumeration)", err)
	}
}

func TestLoginDisabledUser(t *testing.T) {
	ls, us := newLoginService(t)
	hash, _ := HashPassword("pw123")
	u, _ := us.Create("dis@x.com", hash, "user")
	us.UpdateStatus(u.ID, "disabled")
	_, _, err := ls.Login("dis@x.com", "pw123")
	if err != ErrUserDisabled {
		t.Errorf("err = %v, want ErrUserDisabled", err)
	}
}

func TestRegister(t *testing.T) {
	ls, _ := newLoginService(t)
	ls.settings.Set("registration_open", "true")
	u, err := ls.Register("new@x.com", "pw123")
	if err != nil || u.Email != "new@x.com" || u.Role != "user" {
		t.Fatalf("Register: u=%+v err=%v", u, err)
	}
}

func TestRegisterClosed(t *testing.T) {
	ls, _ := newLoginService(t)
	ls.settings.Set("registration_open", "false")
	_, err := ls.Register("new@x.com", "pw123")
	if err != ErrRegistrationClosed {
		t.Errorf("err = %v, want ErrRegistrationClosed", err)
	}
}
