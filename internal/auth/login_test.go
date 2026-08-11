package auth

import (
	"bytes"
	"encoding/json"
	"testing"
	"time"

	"github.com/pquerna/otp/totp"

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

// setup2FAUser creates a user with a real TOTP secret auth_method and returns
// the login service, user, and the plaintext TOTP secret.
func setup2FAUser(t *testing.T, ls *LoginService, us *user.Store, email string) (user.User, string) {
	t.Helper()
	hash, _ := HashPassword("pw123")
	u, err := us.Create(email, hash, "user")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	secret, _, err := GenerateTOTPSecret(email)
	if err != nil {
		t.Fatalf("GenerateTOTPSecret: %v", err)
	}
	if err := us.AddAuthMethod(u.ID, "totp", "", []byte(secret)); err != nil {
		t.Fatalf("AddAuthMethod totp: %v", err)
	}
	return u, secret
}

func TestComplete2FASuccessAndBackupConsumption(t *testing.T) {
	ls, us := newLoginService(t)
	u, secret := setup2FAUser(t, ls, us, "2fa@x.com")

	// TOTP 路径:有效 code 验证成功
	code, err := totp.GenerateCode(secret, time.Now())
	if err != nil {
		t.Fatalf("GenerateCode: %v", err)
	}
	sess, err := ls.Complete2FA("2fa@x.com", code)
	if err != nil {
		t.Fatalf("Complete2FA with TOTP code: %v", err)
	}
	if sess.Token == "" {
		t.Error("expected session token after TOTP login")
	}

	// 备份码路径:加一个 totp_backup auth_method(JSON 数组,含一个哈希备份码)
	backupCode := GenerateBackupCodes()[0]
	hashed := HashBackupCode(backupCode)
	hashesJSON, _ := json.Marshal([]string{hashed})
	if err := us.AddAuthMethod(u.ID, "totp_backup", "", hashesJSON); err != nil {
		t.Fatalf("AddAuthMethod backup: %v", err)
	}

	// 第一次用备份码 -> 成功
	sess, err = ls.Complete2FA("2fa@x.com", backupCode)
	if err != nil {
		t.Fatalf("Complete2FA with backup code (1st): %v", err)
	}
	if sess.Token == "" {
		t.Error("expected session token after backup code login")
	}
	// 备份码应已消耗(一次性):再次用同一备份码 -> 失败
	if _, err := ls.Complete2FA("2fa@x.com", backupCode); err != Err2FAFailed {
		t.Errorf("second use of backup code: err = %v, want Err2FAFailed (consumed)", err)
	}
}

func TestComplete2FAThrottled(t *testing.T) {
	ls, us := newLoginService(t)
	_, secret := setup2FAUser(t, ls, us, "throttle@x.com")

	// 5 次连续错误 -> 账户锁定
	for i := 0; i < 5; i++ {
		if _, err := ls.Complete2FA("throttle@x.com", "000000"); err != Err2FAFailed {
			t.Fatalf("attempt %d: err = %v, want Err2FAFailed", i+1, err)
		}
	}
	// 锁定期间:即使输入一个本来有效的 TOTP code 也被拒绝
	validCode, err := totp.GenerateCode(secret, time.Now())
	if err != nil {
		t.Fatalf("GenerateCode: %v", err)
	}
	if _, err := ls.Complete2FA("throttle@x.com", validCode); err != Err2FALocked {
		t.Errorf("locked attempt: err = %v, want Err2FALocked", err)
	}
}
