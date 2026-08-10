package auth

import (
	"encoding/json"
	"errors"
	"time"

	"carryapi/internal/settings"
	"carryapi/internal/user"
)

var (
	ErrInvalidCredentials = errors.New("invalid credentials")
	ErrUserDisabled       = errors.New("user disabled")
	ErrRegistrationClosed = errors.New("registration closed")
	Err2FARequired        = errors.New("2fa required")
	Err2FAFailed          = errors.New("2fa verification failed")
)

type LoginService struct {
	users    *user.Store
	sessions *SessionStore
	settings *settings.Store
}

func NewLoginService(users *user.Store, sessions *SessionStore, settings *settings.Store) *LoginService {
	return &LoginService{users: users, sessions: sessions, settings: settings}
}

func (ls *LoginService) Login(email, password string) (Session, bool, error) {
	u, err := ls.users.GetByEmail(email)
	if err != nil {
		return Session{}, false, ErrInvalidCredentials
	}
	if u.Status == "disabled" {
		return Session{}, false, ErrUserDisabled
	}
	if !VerifyPassword(password, u.PasswordHash) {
		return Session{}, false, ErrInvalidCredentials
	}
	// 检查是否启用 2FA
	methods, _ := ls.users.GetAuthMethods(u.ID)
	for _, m := range methods {
		if m.Provider == "totp" {
			return Session{}, true, nil // 需要 2FA,不建 session
		}
	}
	sess, err := ls.sessions.Create(u.ID, 7*24*time.Hour, "", "")
	if err != nil {
		return Session{}, false, err
	}
	return sess, false, nil
}

func (ls *LoginService) Complete2FA(email, code string) (Session, error) {
	u, err := ls.users.GetByEmail(email)
	if err != nil {
		return Session{}, ErrInvalidCredentials
	}
	if u.Status == "disabled" {
		return Session{}, ErrUserDisabled
	}
	// 找 totp secret + 备份码哈希
	methods, _ := ls.users.GetAuthMethods(u.ID)
	var totpSecret []byte
	var backupMethod *user.AuthMethod
	for i, m := range methods {
		switch m.Provider {
		case "totp":
			totpSecret = m.Secret
		case "totp_backup":
			backupMethod = &methods[i]
		}
	}
	if totpSecret == nil {
		return Session{}, Err2FAFailed
	}
	// 先试 TOTP,失败再试备份码
	if !VerifyTOTP(code, string(totpSecret)) {
		if backupMethod == nil {
			return Session{}, Err2FAFailed
		}
		var hashedCodes []string
		if json.Unmarshal(backupMethod.Secret, &hashedCodes) != nil {
			return Session{}, Err2FAFailed
		}
		idx, ok := VerifyBackupCode(code, hashedCodes)
		if !ok {
			return Session{}, Err2FAFailed
		}
		// 用过的备份码从数组移除(一次性)
		hashedCodes = append(hashedCodes[:idx], hashedCodes[idx+1:]...)
		newJSON, _ := json.Marshal(hashedCodes)
		// 更新:删旧 + 加新(简化;实现者也可加 user.UpdateAuthMethodSecret)
		ls.users.DeleteAuthMethod(backupMethod.ID, u.ID)
		if len(hashedCodes) > 0 {
			ls.users.AddAuthMethod(u.ID, "totp_backup", "", newJSON)
		}
	}
	sess, err := ls.sessions.Create(u.ID, 7*24*time.Hour, "", "")
	if err != nil {
		return Session{}, err
	}
	return sess, nil
}

func (ls *LoginService) Register(email, password string) (user.User, error) {
	open, _, _ := ls.settings.Get("registration_open")
	if open == "false" {
		return user.User{}, ErrRegistrationClosed
	}
	hash, err := HashPassword(password)
	if err != nil {
		return user.User{}, err
	}
	return ls.users.Create(email, hash, "user")
}
