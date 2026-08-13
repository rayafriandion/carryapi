package catalog

import (
	"testing"
	"time"
)

func TestRouterExcludesUnhealthy(t *testing.T) {
	f := newCatalogFixture(t)
	rs := NewRoutingStats(f.db)
	hc := NewHealthCache(f.bindingsStore(), f.providers, rs)

	p1, _ := f.providers.Create("healthy-prov", "https://x.com", "k", "openai_chat")
	p2, _ := f.providers.Create("sick-prov", "https://y.com", "k", "openai_chat")
	// ModelStore.Create inserts the model row (satisfying model_bindings FK) and
	// auto-creates p1's binding (model_id, p1.ID, "gpt-4o", priority=100).
	m, err := f.models.Create("m", p1.ID, "gpt-4o")
	if err != nil {
		t.Fatalf("create model: %v", err)
	}
	// p2 active 但插入失败日志使其 unhealthy
	insertLog(t, f.db, p2.ID, "gpt-4o", 500, "upstream", 0, 0, 0)
	insertLog(t, f.db, p2.ID, "gpt-4o", 500, "upstream", 0, 0, 0)
	// p2 must have a binding in model_bindings so HealthCache.Refresh picks it up
	// (via ListByProvider(p2.ID)) and BindingHealth queries its request_logs (the
	// two 500s above). Without this, Get(p2.ID,"gpt-4o") returns StatusNoData and
	// p2 is skipped by priority rather than by unhealthy exclusion.
	p2b, err := f.bindingsStore().Create(m.ID, p2.ID, "gpt-4o", 50, 1, true)
	if err != nil {
		t.Fatalf("create p2 binding: %v", err)
	}
	hc.Refresh(time.Now())

	bindings := []ModelBinding{
		{ID: 1, ModelID: m.ID, ProviderID: p1.ID, UpstreamModel: "gpt-4o", Priority: 100, Weight: 1, Enabled: true},
		{ID: p2b.ID, ModelID: m.ID, ProviderID: p2.ID, UpstreamModel: "gpt-4o", Priority: 50, Weight: 1, Enabled: true},
	}
	r := NewRouter(f.providers, hc)
	// Force health-based routing so the router consults HealthCache; the model
	// row itself is created with the default priority auto-mode.
	m.RoutingStrategy = RoutingStrategyAuto
	m.AutoMode = AutoModeHealth
	sel, candidates, err := r.Select(m, bindings)
	if err != nil {
		t.Fatalf("Select: %v", err)
	}
	if sel.Provider.ID != p1.ID {
		t.Errorf("expected healthy provider %d, got %d", p1.ID, sel.Provider.ID)
	}
	// activeBindings already excludes unhealthy providers before healthSelect
	// ranks them, so candidates contains only the healthy binding (p1).
	if len(candidates) != 1 {
		t.Errorf("expected 1 candidate (healthy only), got %d", len(candidates))
	}
	if len(candidates) > 0 && candidates[0].ProviderID != p1.ID {
		t.Errorf("expected candidate to be healthy provider %d, got %d", p1.ID, candidates[0].ProviderID)
	}
}
