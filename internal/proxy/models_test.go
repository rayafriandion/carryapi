package proxy

import (
	"encoding/json"
	"net/http/httptest"
	"testing"

	"carryapi/internal/apikey"
	"carryapi/internal/catalog"
	"carryapi/internal/db"
	"carryapi/internal/user"
)

// newModelListFixture 建一个带启用模型 my-gpt4(provider openai_chat)的完整 proxy,
// 并返回用户(供 /v1/models 测试创建 API Key)。
// 注:model_test.go 已有 newModelFixture 返回 (*Proxy, int64),签名不同,故另命名。
func newModelListFixture(t *testing.T) (*Proxy, *user.User) {
	t.Helper()
	d, err := db.Open(":memory:")
	if err != nil {
		t.Fatalf("db: %v", err)
	}
	db.Migrate(d)
	t.Cleanup(func() { d.Close() })
	c := mustCipher(t)
	us := user.New(d, c)
	ks := apikey.New(d)
	ps := catalog.NewProviderStore(d, c)
	ms := catalog.NewModelStore(d)
	pr := catalog.NewPriceStore(d)
	p := NewProxy(Deps{DB: d, Keys: ks, Users: us, Models: ms, Providers: ps, Prices: pr})
	u, err := us.Create("proxy@x.com", "hash", "user")
	if err != nil {
		t.Fatalf("create user: %v", err)
	}
	prov, err := ps.Create("Mock", "http://127.0.0.1:1", "sk-upstream", "openai_chat")
	if err != nil {
		t.Fatalf("create provider: %v", err)
	}
	m, err := ms.Create("my-gpt4", prov.ID, "gpt-4o")
	if err != nil {
		t.Fatalf("create model: %v", err)
	}
	if _, err := pr.Set(m.ID, 5.0, 15.0, nil, nil); err != nil {
		t.Fatalf("set price: %v", err)
	}
	return p, &u
}

func TestHandleModels(t *testing.T) {
	p, u := newModelListFixture(t) // 有 my-gpt4(启用)
	plaintext, _, _ := p.deps.Keys.Create(u.ID, "test")
	req := httptest.NewRequest("GET", "/v1/models", nil)
	req.Header.Set("Authorization", "Bearer "+plaintext)
	rec := httptest.NewRecorder()
	p.ServeHTTP(rec, req)
	if rec.Code != 200 {
		t.Fatalf("code=%d body=%s", rec.Code, rec.Body.String())
	}
	var resp struct {
		Object string `json:"object"`
		Data   []struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	json.Unmarshal(rec.Body.Bytes(), &resp)
	if resp.Object != "list" || len(resp.Data) != 1 || resp.Data[0].ID != "my-gpt4" {
		t.Errorf("models = %+v", resp)
	}
}

func TestHandleModelsAuthRequired(t *testing.T) {
	p, _ := newModelListFixture(t)
	req := httptest.NewRequest("GET", "/v1/models", nil)
	rec := httptest.NewRecorder()
	p.ServeHTTP(rec, req)
	if rec.Code != 401 {
		t.Errorf("code=%d, want 401", rec.Code)
	}
}
