package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/pquerna/otp"
	"github.com/pquerna/otp/totp"
)

func GenerateTOTPSecret(email string) (secret, otpauthURL string, err error) {
	key, err := totp.Generate(totp.GenerateOpts{
		Issuer:      "carryAPI",
		AccountName: email,
	})
	if err != nil {
		return "", "", fmt.Errorf("generate totp key: %w", err)
	}
	return key.Secret(), key.URL(), nil
}

func VerifyTOTP(code, secret string) bool {
	// totp.Validate 默认允许 ±1 时间窗(30s)
	ok, _ := totp.ValidateCustom(code, secret, time.Now().UTC(), totp.ValidateOpts{
		Period:    30,
		Skew:      1,
		Digits:    otp.DigitsSix,
		Algorithm: otp.AlgorithmSHA1,
	})
	return ok
}

func GenerateBackupCodes() []string {
	codes := make([]string, 10)
	for i := range codes {
		b := make([]byte, 4)
		rand.Read(b)
		codes[i] = hex.EncodeToString(b)
	}
	return codes
}

func HashBackupCode(code string) string {
	h := sha256.Sum256([]byte(code))
	return hex.EncodeToString(h[:])
}

func VerifyBackupCode(code string, hashedCodes []string) (int, bool) {
	h := HashBackupCode(code)
	for i, hc := range hashedCodes {
		if h == hc {
			return i, true
		}
	}
	return -1, false
}
