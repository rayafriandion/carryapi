package proxy

import (
	"testing"

	"carryapi/internal/catalog"
)

func TestComputeCost(t *testing.T) {
	var cr float64 = 0.5
	var cw float64 = 1.0
	price := &catalog.Price{InputPrice: 5.0, OutputPrice: 15.0, CacheReadPrice: &cr, CacheWritePrice: &cw}
	rc := &requestContext{
		inputTokens: 1000, outputTokens: 2000, cacheRead: 500, cacheCreation: 100,
	}
	// 5*1000/1e6 + 15*2000/1e6 + 0.5*500/1e6 + 1.0*100/1e6
	want := 5.0*1000/1e6 + 15.0*2000/1e6 + 0.5*500/1e6 + 1.0*100/1e6
	got := computeCost(price, rc)
	if got != want {
		t.Errorf("cost = %f, want %f", got, want)
	}
}

func TestComputeCostNoCachePrices(t *testing.T) {
	price := &catalog.Price{InputPrice: 2.0, OutputPrice: 8.0}
	rc := &requestContext{inputTokens: 1000, outputTokens: 500, cacheRead: 100, cacheCreation: 50}
	// 无 cache 价格时:cacheRead 用 input_price,cacheCreation 用 input_price
	want := 2.0*1000/1e6 + 8.0*500/1e6 + 2.0*(100+50)/1e6
	got := computeCost(price, rc)
	if got != want {
		t.Errorf("cost = %f, want %f", got, want)
	}
}
