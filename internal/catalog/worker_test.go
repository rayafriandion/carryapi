package catalog

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// TestKeyRetryWorkerRecovers 冷却到期后后台自测成功 -> key 恢复 active。
func TestKeyRetryWorkerRecovers(t *testing.T) {
	up := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"data":[]}`))
	}))
	defer up.Close()
	f := newCatalogFixture(t)
	_, _ = f.providers.Create("P", up.URL, "sk-a", "openai_chat")
	k, _ := f.providers.GetKey(1)
	_ = f.providers.DegradeKey(k.ID, "test")
	if _, err := f.db.Exec("UPDATE provider_api_keys SET retry_after = ? WHERE id = ?",
		time.Now().Add(-time.Minute), k.ID); err != nil {
		t.Fatalf("backdate: %v", err)
	}
	due, _ := f.providers.RetryDueKeys()
	if len(due) != 1 {
		t.Fatalf("due = %d, want 1", len(due))
	}
	w := NewKeyRetryWorker(f.providers, nil)
	w.retryKey(context.Background(), due[0])
	got, _ := f.providers.GetKey(k.ID)
	if got.Status != KeyStatusActive {
		t.Errorf("status = %q, want active (recovered after probe ok)", got.Status)
	}
	events, _ := f.providers.KeyEvents(k.ID)
	if events[0].Event != KeyEventRecovered {
		t.Errorf("latest event = %q, want recovered", events[0].Event)
	}
}

// TestKeyRetryWorkerDeletes 冷却到期后后台自测 3 次仍失败 -> key 被删除。
func TestKeyRetryWorkerDeletes(t *testing.T) {
	up := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(500)
		w.Write([]byte(`{"error":{"message":"boom"}}`))
	}))
	defer up.Close()
	f := newCatalogFixture(t)
	_, _ = f.providers.Create("P", up.URL, "sk-a", "openai_chat")
	k, _ := f.providers.GetKey(1)
	_ = f.providers.DegradeKey(k.ID, "test")
	if _, err := f.db.Exec("UPDATE provider_api_keys SET retry_after = ? WHERE id = ?",
		time.Now().Add(-time.Minute), k.ID); err != nil {
		t.Fatalf("backdate: %v", err)
	}
	due, _ := f.providers.RetryDueKeys()
	w := NewKeyRetryWorker(f.providers, nil)
	w.retryKey(context.Background(), due[0])
	got, _ := f.providers.GetKey(k.ID)
	if got.Status != KeyStatusDeleted {
		t.Errorf("status = %q, want deleted after 3 failed probes", got.Status)
	}
	events, _ := f.providers.KeyEvents(k.ID)
	if events[0].Event != KeyEventDeleted {
		t.Errorf("latest event = %q, want deleted", events[0].Event)
	}
}
