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
	pr.Set(m.ID, 5.0, 15.0, &cr, nil, "USD")
	p.deps.Providers = ps
	p.deps.Models = ms
	p.deps.Prices = pr
	return p, prov.ID
}

func TestResolveModel(t *testing.T) {
	p, _ := newModelFixture(t)
	resolved, err := p.resolveModel("my-gpt4")
	if err != nil {
		t.Fatalf("resolveModel: %v", err)
	}
	if resolved.model.UpstreamModel != "gpt-4o" || resolved.provider.BaseURL != "https://api.openai.com/v1" || resolved.price.InputPrice != 5.0 {
		t.Errorf("resolve: model=%+v provider=%+v price=%+v", resolved.model, resolved.provider, resolved.price)
	}
}

func TestResolveModelNotFound(t *testing.T) {
	p, _ := newModelFixture(t)
	if _, err := p.resolveModel("nope"); err == nil {
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
	m, err := p.deps.Models.GetByName("my-gpt4")
	if err != nil {
		t.Fatalf("get model: %v", err)
	}
	// 未设置配额时放行
	if err := p.checkQuota(&u, ak.ID, m.ID); err != nil {
		t.Fatalf("checkQuota with no limits: %v", err)
	}
	// 用户 token 上限并超额 -> 429
	limit := int64(10)
	if _, err := p.deps.Users.SetQuota(user.Quota{Scope: "user", ScopeID: u.ID, Period: "month", LimitTokens: &limit}); err != nil {
		t.Fatalf("set quota: %v", err)
	}
	if err := p.deps.Users.IncrementUsage("user", u.ID, 11, 0); err != nil {
		t.Fatalf("increment usage: %v", err)
	}
	if err := p.checkQuota(&u, ak.ID, m.ID); err == nil {
		t.Fatal("expected quota exceeded error (user)")
	}
	// 删除用户配额后,模型 token 上限超额 -> 429
	userQuotas, err := p.deps.Users.GetQuotas("user", u.ID)
	if err != nil {
		t.Fatalf("get user quotas: %v", err)
	}
	for _, q := range userQuotas {
		_ = p.deps.Users.DeleteQuota(q.ID)
	}
	mlimit := int64(5)
	if _, err := p.deps.Users.SetModelQuota(m.ID, "total", &mlimit, nil); err != nil {
		t.Fatalf("set model quota: %v", err)
	}
	if err := p.deps.Users.IncrementUsage("model", m.ID, 6, 0); err != nil {
		t.Fatalf("increment model usage: %v", err)
	}
	if err := p.checkQuota(&u, ak.ID, m.ID); err == nil {
		t.Fatal("expected quota exceeded error (model)")
	}
	// 删除模型配额后,Key 配额超额 -> 429
	modelQuotas, err := p.deps.Users.GetQuotas("model", m.ID)
	if err != nil {
		t.Fatalf("get model quotas: %v", err)
	}
	for _, q := range modelQuotas {
		_ = p.deps.Users.DeleteQuota(q.ID)
	}
	klimit := int64(3)
	if _, err := p.deps.Users.SetKeyQuota(ak.ID, "total", &klimit, nil); err != nil {
		t.Fatalf("set key quota: %v", err)
	}
	if err := p.deps.Users.IncrementUsage("key", ak.ID, 4, 0); err != nil {
		t.Fatalf("increment key usage: %v", err)
	}
	if err := p.checkQuota(&u, ak.ID, m.ID); err == nil {
		t.Fatal("expected quota exceeded error (key)")
	}
}
