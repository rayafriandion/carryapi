package auth

import (
	"testing"
	"time"

	"github.com/pquerna/otp/totp"
)

func TestGenerateAndVerifyTOTP(t *testing.T) {
	secret, url, err := GenerateTOTPSecret("alice@example.com")
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	if secret == "" || url == "" {
		t.Fatal("secret or url empty")
	}
	// 当前码应验证通过
	code, err := totp.GenerateCode(secret, time.Now())
	if err != nil {
		t.Fatalf("GenerateCode: %v", err)
	}
	if !VerifyTOTP(code, secret) {
		t.Error("VerifyTOTP should accept current code")
	}
	if VerifyTOTP("000000", secret) {
		t.Error("VerifyTOTP should reject bogus code")
	}
}

func TestBackupCodes(t *testing.T) {
	codes := GenerateBackupCodes()
	if len(codes) != 10 {
		t.Fatalf("expected 10 backup codes, got %d", len(codes))
	}
	hashed := make([]string, len(codes))
	for i, c := range codes {
		hashed[i] = HashBackupCode(c)
	}
	// 第一个码应匹配 index 0
	idx, ok := VerifyBackupCode(codes[0], hashed)
	if !ok || idx != 0 {
		t.Errorf("expected match at 0, got idx=%d ok=%v", idx, ok)
	}
	// 错误码不匹配
	if _, ok := VerifyBackupCode("deadbeef", hashed); ok {
		t.Error("should not match bogus code")
	}
}
