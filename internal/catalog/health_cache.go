package catalog

import (
	"context"
	"strconv"
	"sync"
	"time"
)

type cachedHealth struct {
	status string
}

type HealthCache struct {
	bindings  *ModelBindingStore
	providers *ProviderStore
	stats     *RoutingStats
	mu        sync.RWMutex
	states    map[string]cachedHealth
}

func NewHealthCache(bindings *ModelBindingStore, providers *ProviderStore, stats *RoutingStats) *HealthCache {
	return &HealthCache{
		bindings:  bindings,
		providers: providers,
		stats:     stats,
		states:    make(map[string]cachedHealth),
	}
}

func healthCacheKey(providerID int64, upstreamModel string) string {
	return strconv.FormatInt(providerID, 10) + ":" + upstreamModel
}

// Get 并发安全读;未缓存返回 no_data。
func (c *HealthCache) Get(providerID int64, upstreamModel string) string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	st, ok := c.states[healthCacheKey(providerID, upstreamModel)]
	if !ok {
		return StatusNoData
	}
	return st.status
}

// Refresh 同步预算一次所有 active provider 的 binding。
func (c *HealthCache) Refresh(now time.Time) {
	providers, err := c.providers.List()
	if err != nil {
		return
	}
	newStates := make(map[string]cachedHealth)
	for _, p := range providers {
		if p.Status != "active" {
			continue
		}
		bindings, err := c.bindings.ListByProvider(p.ID)
		if err != nil {
			continue
		}
		for _, b := range bindings {
			if !b.Enabled {
				continue
			}
			status, err := c.stats.BindingHealth(p.ID, b.UpstreamModel, now)
			if err != nil {
				status = StatusNoData
			}
			newStates[healthCacheKey(p.ID, b.UpstreamModel)] = cachedHealth{status: status}
		}
	}
	c.mu.Lock()
	c.states = newStates
	c.mu.Unlock()
}

// Start 后台循环预算,每 1 分钟一次;ctx 取消时退出。
func (c *HealthCache) Start(ctx context.Context) {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()
	c.Refresh(time.Now())
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			c.Refresh(time.Now())
		}
	}
}
