package catalog

import (
	"context"
	"testing"
	"time"
)

func TestHealthCacheRefreshAndGet(t *testing.T) {
	f := newCatalogFixture(t)
	rs := NewRoutingStats(f.db)
	hc := NewHealthCache(f.bindingsStore(), f.providers, rs)

	// 插入 binding + 失败日志
	p, _ := f.providers.Create("p", "https://x.com", "k", "openai_chat")
	// ModelStore.Create already inserts an enabled binding for (model_id, p.ID, "gpt-4o"),
	// and the schema enforces UNIQUE(model_id, provider_id, upstream_model), so an
	// explicit bindingsStore().Create would hit a constraint failure. The auto-created
	// binding is exactly what this test exercises.
	f.models.Create("m", p.ID, "gpt-4o")
	insertLog(t, f.db, p.ID, "gpt-4o", 500, "upstream", 1*time.Hour, 0, 0)
	insertLog(t, f.db, p.ID, "gpt-4o", 500, "upstream", 1*time.Hour, 0, 0)

	hc.Refresh(time.Now())
	got := hc.Get(p.ID, "gpt-4o")
	if got != StatusUnhealthy {
		t.Errorf("expected unhealthy, got %s", got)
	}
}

func TestHealthCacheNoDataForUnknown(t *testing.T) {
	f := newCatalogFixture(t)
	rs := NewRoutingStats(f.db)
	hc := NewHealthCache(f.bindingsStore(), f.providers, rs)
	hc.Refresh(time.Now())
	got := hc.Get(999, "unknown")
	if got != StatusNoData {
		t.Errorf("expected no_data, got %s", got)
	}
}

func TestHealthCacheStartStopsOnCancel(t *testing.T) {
	f := newCatalogFixture(t)
	rs := NewRoutingStats(f.db)
	hc := NewHealthCache(f.bindingsStore(), f.providers, rs)
	ctx, cancel := context.WithCancel(context.Background())
	go hc.Start(ctx)
	cancel()
	time.Sleep(100 * time.Millisecond)
	// 不 panic 即通过
}
