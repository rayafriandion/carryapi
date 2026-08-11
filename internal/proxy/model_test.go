package proxy

import (
	"testing"

	"carryapi/internal/catalog"
	"carryapi/internal/user"
)

func newModelFixture(t *testing.T) (*Proxy, int64) {
	t.Helper()
	p, _, c := newAuthFixture(t) // 复用 db + 共享 cipher
	d := p.deps.DB
	ps := catalog.NewProviderStore(d, c)
	ms := catalog.NewModelStore(d)
	pr := catalog.NewPriceStore(d)
	prov, _ := ps.Create("OpenAI", "https://api.openai.com/v1", "sk-1", "openai_chat")
	m, _ := ms.Create("my-gpt4", prov.ID, "gpt-4o")
	var cr float64 = 0.5
	pr.Set(m.ID, 5.0, 15.0, &cr, nil)
	p.deps.Providers = ps
	p.deps.Models = ms
	p.deps.Prices = pr
	return p, prov.ID
}

func TestResolveModel(t *testing.T) {
	p, _ := newModelFixture(t)
	model, provider, price, err := p.resolveModel("my-gpt4")
	if err != nil {
		t.Fatalf("resolveModel: %v", err)
	}
	if model.UpstreamModel != "gpt-4o" || provider.BaseURL != "https://api.openai.com/v1" || price.InputPrice != 5.0 {
		t.Errorf("resolve: model=%+v provider=%+v price=%+v", model, provider, price)
	}
}

func TestResolveModelNotFound(t *testing.T) {
	p, _ := newModelFixture(t)
	if _, _, _, err := p.resolveModel("nope"); err == nil {
		t.Error("expected error for unknown model")
	}
}

func TestCheckQuota(t *testing.T) {
	p, _ := newModelFixture(t)
	u, err := p.deps.Users.GetByEmail("proxy@x.com")
	if err != nil {
		t.Fatalf("get user: %v", err)
	}
	_, ak, _ := p.deps.Keys.Create(u.ID, "test")
	// 未设置配额时放行
	if err := p.checkQuota(&u, ak.ID); err != nil {
		t.Fatalf("checkQuota with no limits: %v", err)
	}
	// 设置 token 上限并超额 -> 429
	limit := int64(10)
	if _, err := p.deps.Users.SetQuota(user.Quota{Scope: "user", ScopeID: u.ID, Period: "month", LimitTokens: &limit}); err != nil {
		t.Fatalf("set quota: %v", err)
	}
	if err := p.deps.Users.IncrementUsage("user", u.ID, 11, 0); err != nil {
		t.Fatalf("increment usage: %v", err)
	}
	if err := p.checkQuota(&u, ak.ID); err == nil {
		t.Fatal("expected quota exceeded error")
	}
}
