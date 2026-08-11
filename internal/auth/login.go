package auth

import (
	"encoding/json"
	"errors"
	"sync"
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
	Err2FALocked          = errors.New("2fa temporarily locked")
)

// failState tracks per-account TOTP verification failures for the brute-force
// lockout (5 failures -> 15 minute lock).
type failState struct {
	failures    int
	lockedUntil time.Time
}

type LoginService struct {
	users    *user.Store
	sessions *SessionStore
	settings *settings.Store

	mu       sync.Mutex
	lockouts map[int64]failState
}

// max2FAFailures and twoFALockDuration implement the plan's global constraint
// "TOTP 验证失败 5 次锁定 15 分钟".
const (
	max2FAFailures    = 5
	twoFALockDuration = 15 * time.Minute
)

func NewLoginService(users *user.Store, sessions *SessionStore, settings *settings.Store) *LoginService {
	return &LoginService{users: users, sessions: sessions, settings: settings, lockouts: make(map[int64]failState)}
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
	// 账户级锁定:锁定期间直接拒绝,不消耗验证机会
	ls.mu.Lock()
	if st, ok := ls.lockouts[u.ID]; ok && time.Now().Before(st.lockedUntil) {
		ls.mu.Unlock()
		return Session{}, Err2FALocked
	}
	ls.mu.Unlock()
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
		ls.record2FAFailure(u.ID)
		return Session{}, Err2FAFailed
	}
	// 先试 TOTP,失败再试备份码
	ok := VerifyTOTP(code, string(totpSecret))
	if !ok && backupMethod != nil {
		var hashedCodes []string
		if json.Unmarshal(backupMethod.Secret, &hashedCodes) == nil {
			if idx, found := VerifyBackupCode(code, hashedCodes); found {
				ok = true
				// 用过的备份码从数组移除(一次性)
				hashedCodes = append(hashedCodes[:idx], hashedCodes[idx+1:]...)
				newJSON, _ := json.Marshal(hashedCodes)
				// 更新:删旧 + 加新
				ls.users.DeleteAuthMethod(backupMethod.ID, u.ID)
				if len(hashedCodes) > 0 {
					ls.users.AddAuthMethod(u.ID, "totp_backup", "", newJSON)
				}
			}
		}
	}
	if !ok {
		ls.record2FAFailure(u.ID)
		return Session{}, Err2FAFailed
	}
	// 验证成功:清除该账户的失败计数
	ls.mu.Lock()
	delete(ls.lockouts, u.ID)
	ls.mu.Unlock()

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

// record2FAFailure increments the per-account TOTP failure counter and locks
// the account (now + 15m) once 5 consecutive failures are reached.
func (ls *LoginService) record2FAFailure(userID int64) {
	ls.mu.Lock()
	defer ls.mu.Unlock()
	st := ls.lockouts[userID]
	if time.Now().Before(st.lockedUntil) {
		return // 已锁定,保持锁定
	}
	st.failures++
	if st.failures >= max2FAFailures {
		st.lockedUntil = time.Now().Add(twoFALockDuration)
		st.failures = 0
	}
	ls.lockouts[userID] = st
}
