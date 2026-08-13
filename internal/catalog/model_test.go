package catalog

import (
	"bytes"
	"database/sql"
	"testing"

	"carryapi/internal/crypto"
	"carryapi/internal/db"
)

// catalogFixture opens ONE shared :memory: database and wires up all three
// stores against it. newProviderStore(t) (Task 1) opens its own :memory: db,
// so ModelStore/PriceStore tests must not try to share that connection.
type catalogFixture struct {
	db        *sql.DB
	providers *ProviderStore
	models    *ModelStore
	prices    *PriceStore
	bindings  *ModelBindingStore
}

func newCatalogFixture(t *testing.T) *catalogFixture {
	t.Helper()
	d, err := db.Open(":memory:")
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	if err := db.Migrate(d); err != nil {
		t.Fatalf("Migrate: %v", err)
	}
	// :memory: 数据库对每个连接独立,限制单连接确保迁移与被测代码共享同一份数据。
	d.SetMaxOpenConns(1)
	t.Cleanup(func() { d.Close() })
	c, _ := crypto.New(bytes.Repeat([]byte{1}, 32))
	return &catalogFixture{
		db:        d,
		providers: NewProviderStore(d, c),
		models:    NewModelStore(d),
		prices:    NewPriceStore(d),
		bindings:  NewModelBindingStore(d),
	}
}

func (f *catalogFixture) bindingsStore() *ModelBindingStore {
	return f.bindings
}

func TestModelCreateAndGetByName(t *testing.T) {
	f := newCatalogFixture(t)
	p, _ := f.providers.Create("OpenAI", "https://api.openai.com/v1", "k", "openai_chat")
	m, err := f.models.Create("my-gpt4", p.ID, "gpt-4o")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if !m.Enabled {
		t.Error("default enabled should be true")
	}
	got, err := f.models.GetByName("my-gpt4")
	if err != nil || got.ProviderID != p.ID || got.UpstreamModel != "gpt-4o" {
		t.Fatalf("GetByName: %+v err %v", got, err)
	}
}

func TestModelGetByNameNotFound(t *testing.T) {
	f := newCatalogFixture(t)
	if _, err := f.models.GetByName("nope"); err == nil {
		t.Error("expected error for unknown model")
	}
}

func TestModelUpdateDisable(t *testing.T) {
	f := newCatalogFixture(t)
	p, _ := f.providers.Create("OpenAI", "https://api.openai.com/v1", "k", "openai_chat")
	m, _ := f.models.Create("m1", p.ID, "gpt-4o")
	// Update only changes name + enabled; bindings are managed via RoutingView CRUD.
	if err := f.models.Update(m.ID, "m1", false); err != nil {
		t.Fatalf("Update: %v", err)
	}
	got, _ := f.models.Get(m.ID)
	if got.Name != "m1" || got.Enabled {
		t.Errorf("after update: %+v", got)
	}
	// ListEnabled 不应包含禁用模型
	enabled, _ := f.models.ListEnabled()
	if len(enabled) != 0 {
		t.Errorf("ListEnabled = %d, want 0", len(enabled))
	}
}

func TestUpdateDoesNotChangeBindings(t *testing.T) {
	f := newCatalogFixture(t)
	p1, _ := f.providers.Create("p1", "https://x.com", "k", "openai_chat")
	p2, _ := f.providers.Create("p2", "https://y.com", "k", "openai_chat")
	m, _ := f.models.Create("m", p1.ID, "gpt-4o")
	// 加第二条 binding
	_, _ = f.bindings.Create(m.ID, p2.ID, "claude", 200, 1, true)

	// Update 改 name + enabled,binding 不受影响
	err := f.models.Update(m.ID, "m-renamed", false)
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	bindings, _ := f.bindings.ListByModel(m.ID)
	if len(bindings) != 2 {
		t.Fatalf("expected 2 bindings unchanged, got %d", len(bindings))
	}
	// binding 内容不变
	if bindings[0].UpstreamModel != "gpt-4o" || bindings[1].UpstreamModel != "claude" {
		t.Errorf("bindings changed: %+v", bindings)
	}
}
