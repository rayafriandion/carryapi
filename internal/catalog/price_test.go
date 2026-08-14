package catalog

import (
	"testing"
)

func TestPriceSetAndGetCurrent(t *testing.T) {
	f := newCatalogFixture(t)
	p, _ := f.providers.Create("OpenAI", "https://api.openai.com/v1", "k", "openai_chat")
	m, _ := f.models.Create("m1", p.ID, "gpt-4o")
	var cr float64 = 0.5
	price, err := f.prices.Set(m.ID, 5.0, 15.0, &cr, nil, "USD")
	if err != nil {
		t.Fatalf("Set: %v", err)
	}
	if price.InputPrice != 5.0 || price.OutputPrice != 15.0 || *price.CacheReadPrice != 0.5 {
		t.Errorf("price = %+v", price)
	}
	if price.Currency != "USD" {
		t.Errorf("currency = %q, want USD", price.Currency)
	}
	cur, err := f.prices.GetCurrent(m.ID)
	if err != nil || cur.ID != price.ID {
		t.Fatalf("GetCurrent: %+v err %v", cur, err)
	}
}

func TestPriceHistory(t *testing.T) {
	f := newCatalogFixture(t)
	p, _ := f.providers.Create("OpenAI", "https://api.openai.com/v1", "k", "openai_chat")
	m, _ := f.models.Create("m1", p.ID, "gpt-4o")
	f.prices.Set(m.ID, 1.0, 1.0, nil, nil, "USD")
	f.prices.Set(m.ID, 2.0, 2.0, nil, nil, "CNY") // 涨价 + 切换币种
	cur, _ := f.prices.GetCurrent(m.ID)
	if cur.InputPrice != 2.0 {
		t.Errorf("current = %f, want 2.0", cur.InputPrice)
	}
	if cur.Currency != "CNY" {
		t.Errorf("currency = %q, want CNY", cur.Currency)
	}
	hist, _ := f.prices.List(m.ID)
	if len(hist) != 2 {
		t.Errorf("history = %d, want 2", len(hist))
	}
}

func TestPriceNoPrice(t *testing.T) {
	f := newCatalogFixture(t)
	p, _ := f.providers.Create("OpenAI", "https://api.openai.com/v1", "k", "openai_chat")
	m, _ := f.models.Create("m1", p.ID, "gpt-4o")
	if _, err := f.prices.GetCurrent(m.ID); err != ErrNoPrice {
		t.Errorf("err = %v, want ErrNoPrice", err)
	}
}

func TestPriceCustomAndInvalidCurrency(t *testing.T) {
	f := newCatalogFixture(t)
	p, _ := f.providers.Create("OpenAI", "https://api.openai.com/v1", "k", "openai_chat")
	m, _ := f.models.Create("m1", p.ID, "gpt-4o")
	// 预设之外的自定义币种应可用
	if _, err := f.prices.Set(m.ID, 1.0, 1.0, nil, nil, "CZK"); err != nil {
		t.Fatalf("set custom currency: %v", err)
	}
	cur, _ := f.prices.GetCurrent(m.ID)
	if cur.Currency != "CZK" {
		t.Errorf("currency = %q, want CZK", cur.Currency)
	}
	// 非法币种应报错(超过 8 位)
	if _, err := f.prices.Set(m.ID, 1.0, 1.0, nil, nil, "ZZZZZZZZZ"); err == nil {
		t.Error("expected error for invalid currency")
	}
}
