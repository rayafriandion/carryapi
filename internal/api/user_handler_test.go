package api

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"carryapi/internal/middleware"
	"carryapi/internal/user"
)

// authedReq returns a request whose context carries the given user (simulates
// a logged-in admin via SessionMiddleware).
func authedReq(usr *user.User, method, path string, body []byte) *http.Request {
	var rd io.Reader
	if body != nil {
		rd = bytes.NewReader(body)
	}
	req := httptest.NewRequest(method, path, rd)
	req.Header.Set("Content-Type", "application/json")
	ctx := context.WithValue(req.Context(), middleware.UserKey{}, usr)
	return req.WithContext(ctx)
}

func TestUpdateCannotDemoteSelf(t *testing.T) {
	f := setupAPI(t)
	admin := &user.User{ID: 1, Email: "a@x.com", Role: "admin", Status: "active"}

	body, _ := json.Marshal(map[string]string{"role": "user"})
	req := authedReq(admin, "PUT", "/api/users/1", body)
	req = withChiParam(req, "id", "1")
	rec := httptest.NewRecorder()
	f.users.Create(admin.Email, "hash", "admin")
	h := NewUserHandler(f.users, f.sessions)
	h.Update(rec, req)
	if rec.Code != 400 {
		t.Fatalf("expected 400 demoting self, got %d body=%s", rec.Code, rec.Body.String())
	}
	// 角色未被改变
	got, _ := f.users.GetByID(1)
	if got.Role != "admin" {
		t.Fatalf("role changed unexpectedly: %q", got.Role)
	}
}

func TestUpdateCannotDisableSelf(t *testing.T) {
	f := setupAPI(t)
	admin := &user.User{ID: 1, Email: "a@x.com", Role: "admin", Status: "active"}
	f.users.Create(admin.Email, "hash", "admin")

	body, _ := json.Marshal(map[string]string{"status": "disabled"})
	req := authedReq(admin, "PUT", "/api/users/1", body)
	req = withChiParam(req, "id", "1")
	rec := httptest.NewRecorder()
	NewUserHandler(f.users, f.sessions).Update(rec, req)
	if rec.Code != 400 {
		t.Fatalf("expected 400 disabling self, got %d", rec.Code)
	}
}

func TestUpdateCannotDemoteFirstAdmin(t *testing.T) {
	f := setupAPI(t)
	f.users.Create("boot@x.com", "hash", "admin")  // id=1 首个 admin
	f.users.Create("other@x.com", "hash", "admin") // id=2 另一个 admin
	other := &user.User{ID: 2, Email: "other@x.com", Role: "admin", Status: "active"}

	body, _ := json.Marshal(map[string]string{"role": "user"})
	req := authedReq(other, "PUT", "/api/users/1", body)
	req = withChiParam(req, "id", "1")
	rec := httptest.NewRecorder()
	NewUserHandler(f.users, f.sessions).Update(rec, req)
	if rec.Code != 400 {
		t.Fatalf("expected 400 demoting first admin, got %d body=%s", rec.Code, rec.Body.String())
	}
	got, _ := f.users.GetByID(1)
	if got.Role != "admin" {
		t.Fatalf("first admin demoted unexpectedly: %q", got.Role)
	}
}

func TestUpdateCanDemoteOtherNonFirstAdmin(t *testing.T) {
	f := setupAPI(t)
	f.users.Create("boot@x.com", "hash", "admin")  // id=1 首个
	f.users.Create("other@x.com", "hash", "admin") // id=2
	boot := &user.User{ID: 1, Email: "boot@x.com", Role: "admin", Status: "active"}

	body, _ := json.Marshal(map[string]string{"role": "user"})
	req := authedReq(boot, "PUT", "/api/users/2", body)
	req = withChiParam(req, "id", "2")
	rec := httptest.NewRecorder()
	NewUserHandler(f.users, f.sessions).Update(rec, req)
	if rec.Code != 200 {
		t.Fatalf("expected 200 demoting non-first admin, got %d body=%s", rec.Code, rec.Body.String())
	}
	got, _ := f.users.GetByID(2)
	if got.Role != "user" {
		t.Fatalf("expected demoted to user, got %q", got.Role)
	}
}

func TestDeleteCannotDeleteFirstAdmin(t *testing.T) {
	f := setupAPI(t)
	f.users.Create("boot@x.com", "hash", "admin")  // id=1 首个
	f.users.Create("other@x.com", "hash", "admin") // id=2
	other := &user.User{ID: 2, Email: "other@x.com", Role: "admin", Status: "active"}

	req := authedReq(other, "DELETE", "/api/users/1", nil)
	req = withChiParam(req, "id", strconv.Itoa(1))
	rec := httptest.NewRecorder()
	NewUserHandler(f.users, f.sessions).Delete(rec, req)
	if rec.Code != 400 {
		t.Fatalf("expected 400 deleting first admin, got %d", rec.Code)
	}
}
