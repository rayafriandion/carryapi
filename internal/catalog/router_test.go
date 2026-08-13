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
	// p2 active 但插入失败日志使其 unhealthy
	insertLog(t, f.db, p2.ID, "gpt-4o", 500, "upstream", 0, 0, 0)
	insertLog(t, f.db, p2.ID, "gpt-4o", 500, "upstream", 0, 0, 0)
	hc.Refresh(time.Now())

	m := Model{ID: 1, RoutingStrategy: RoutingStrategyAuto, AutoMode: AutoModeHealth}
	bindings := []ModelBinding{
		{ID: 1, ModelID: 1, ProviderID: p1.ID, UpstreamModel: "gpt-4o", Priority: 100, Weight: 1, Enabled: true},
		{ID: 2, ModelID: 1, ProviderID: p2.ID, UpstreamModel: "gpt-4o", Priority: 200, Weight: 1, Enabled: true},
	}
	r := NewRouter(f.providers, hc)
	sel, candidates, err := r.Select(m, bindings)
	if err != nil {
		t.Fatalf("Select: %v", err)
	}
	if sel.Provider.ID != p1.ID {
		t.Errorf("expected healthy provider %d, got %d", p1.ID, sel.Provider.ID)
	}
	// candidates 应含全部(healthy 在前)
	if len(candidates) != 2 {
		t.Errorf("expected 2 candidates, got %d", len(candidates))
	}
}
