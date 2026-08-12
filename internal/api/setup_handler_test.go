package api

import (
	"encoding/json"
	"testing"

	"carryapi/internal/auth"
)

func TestSetupStatusAndCreateAdmin(t *testing.T) {
	f := setupAPI(t)
	sh := NewSetupHandler(f.users)

	// 无 admin -> needs_setup true
	rec := serve(sh.Status, "GET", "/api/setup/status", nil)
	if rec.Code != 200 {
		t.Fatalf("status code=%d", rec.Code)
	}
	var st struct {
		NeedsSetup bool `json:"needs_setup"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &st); err != nil || !st.NeedsSetup {
		t.Fatalf("expected needs_setup=true, got %s", rec.Body.String())
	}

	// 创建 admin
	body, _ := json.Marshal(map[string]string{"email": "admin@x.com", "password": "secret123"})
	rec = serve(sh.CreateAdmin, "POST", "/api/setup/admin", body)
	if rec.Code != 200 {
		t.Fatalf("create admin code=%d body=%s", rec.Code, rec.Body.String())
	}

	// 创建后 needs_setup false
	rec = serve(sh.Status, "GET", "/api/setup/status", nil)
	_ = json.Unmarshal(rec.Body.Bytes(), &st)
	if st.NeedsSetup {
		t.Fatalf("expected needs_setup=false after create, got %s", rec.Body.String())
	}

	// 已存在 admin 再创建 -> 403
	body2, _ := json.Marshal(map[string]string{"email": "admin2@x.com", "password": "secret123"})
	rec = serve(sh.CreateAdmin, "POST", "/api/setup/admin", body2)
	if rec.Code != 403 {
		t.Fatalf("expected 403 when admin exists, got %d body=%s", rec.Code, rec.Body.String())
	}

	// 新 admin 角色正确且可登录
	u, err := f.users.GetByEmail("admin@x.com")
	if err != nil || u.Role != "admin" {
		t.Fatalf("created user role=%q err=%v", u.Role, err)
	}
	if !auth.VerifyPassword("secret123", u.PasswordHash) {
		t.Fatal("created admin password not verifiable")
	}
}

func TestSetupCreateAdminValidation(t *testing.T) {
	f := setupAPI(t)
	sh := NewSetupHandler(f.users)

	// 密码太短 -> 400
	body, _ := json.Marshal(map[string]string{"email": "a@x.com", "password": "short"})
	rec := serve(sh.CreateAdmin, "POST", "/api/setup/admin", body)
	if rec.Code != 400 {
		t.Fatalf("expected 400 for short password, got %d", rec.Code)
	}
	// 空邮箱 -> 400
	body2, _ := json.Marshal(map[string]string{"email": "", "password": "secret123"})
	rec = serve(sh.CreateAdmin, "POST", "/api/setup/admin", body2)
	if rec.Code != 400 {
		t.Fatalf("expected 400 for empty email, got %d", rec.Code)
	}
	// 非法 JSON -> 400
	rec = serve(sh.CreateAdmin, "POST", "/api/setup/admin", []byte(`{bad`))
	if rec.Code != 400 {
		t.Fatalf("expected 400 for bad json, got %d", rec.Code)
	}
}
