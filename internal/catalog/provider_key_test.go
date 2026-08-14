package catalog

import (
	"testing"
	"time"
)

func TestAddAndGetKey(t *testing.T) {
	f := newCatalogFixture(t)
	p, _ := f.providers.Create("OpenAI", "https://api.openai.com/v1", "sk-one", "openai_chat")
	k, err := f.providers.AddKey(p.ID, "sk-two", "team-b")
	if err != nil {
		t.Fatalf("AddKey: %v", err)
	}
	if k.APIKey != "sk-two" || k.Status != KeyStatusActive || k.Label != "team-b" {
		t.Errorf("unexpected key: %+v", k)
	}
	if k.Priority < 100 || k.BasePriority != 100 {
		t.Errorf("priority = %d base=%d, want >=100 / 100", k.Priority, k.BasePriority)
	}
	got, err := f.providers.GetKey(k.ID)
	if err != nil {
		t.Fatalf("GetKey: %v", err)
	}
	if got.APIKey != "sk-two" {
		t.Errorf("round-trip key = %q", got.APIKey)
	}
	// 事件日志应有 created
	events, _ := f.providers.KeyEvents(k.ID)
	if len(events) == 0 || events[0].Event != KeyEventCreated {
		t.Errorf("expected created event, got %+v", events)
	}
}

func TestCreateSeedsKeyPool(t *testing.T) {
	f := newCatalogFixture(t)
	p, _ := f.providers.Create("OpenAI", "https://api.openai.com/v1", "sk-seed", "openai_chat")
	keys, err := f.providers.ActiveKeys(p.ID)
	if err != nil {
		t.Fatalf("ActiveKeys: %v", err)
	}
	if len(keys) != 1 || keys[0].APIKey != "sk-seed" {
		t.Fatalf("expected one seeded key, got %+v", keys)
	}
	// legacy Provider.APIKey 与池第一个 active key 同步
	prov, _ := f.providers.Get(p.ID)
	if prov.APIKey != "sk-seed" {
		t.Errorf("provider legacy api key = %q, want sk-seed", prov.APIKey)
	}
}

func TestSelectKeyUserAffinity(t *testing.T) {
	f := newCatalogFixture(t)
	p, _ := f.providers.Create("OpenAI", "https://api.openai.com/v1", "sk-a", "openai_chat")
	keyB, _ := f.providers.AddKey(p.ID, "sk-b", "b")
	// 建一个真实用户(provider_key_prefs 有外键约束)
	res, err := f.db.Exec("INSERT INTO users(email, role, status) VALUES(?, 'user', 'active')", "u@x.com")
	if err != nil {
		t.Fatalf("create user: %v", err)
	}
	uid, _ := res.LastInsertId()
	// 用户 uid 先用过 keyB(模拟该用户历史上落在 keyB 上,缓存已建立)
	if err := f.providers.MarkUsed(keyB.ID, uid); err != nil {
		t.Fatalf("MarkUsed: %v", err)
	}
	// 即便 keyA 优先级/ID 更靠前,也应优先返回该用户用过的 keyB(缓存命中)
	sel, err := f.providers.SelectKey(p.ID, uid)
	if err != nil {
		t.Fatalf("SelectKey: %v", err)
	}
	if sel.ID != keyB.ID {
		t.Errorf("SelectKey for user = key#%d, want key#%d (user affinity)", sel.ID, keyB.ID)
	}
}

func TestDegradeRecoverDeleteLifecycle(t *testing.T) {
	f := newCatalogFixture(t)
	p, _ := f.providers.Create("OpenAI", "https://api.openai.com/v1", "sk-a", "openai_chat")
	keyA, _ := f.providers.GetKey(1) // seeded key id=1
	keyB, _ := f.providers.AddKey(p.ID, "sk-b", "b")

	// 降级 keyA
	if err := f.providers.DegradeKey(keyA.ID, "http 429: rate limited"); err != nil {
		t.Fatalf("DegradeKey: %v", err)
	}
	got, _ := f.providers.GetKey(keyA.ID)
	if got.Status != KeyStatusCoolingDown {
		t.Errorf("status = %q, want cooling_down", got.Status)
	}
	if got.FailCount != 1 {
		t.Errorf("fail_count = %d, want 1", got.FailCount)
	}
	if got.RetryAfter == nil || !got.RetryAfter.After(time.Now()) {
		t.Errorf("retry_after = %v, want in future", got.RetryAfter)
	}
	if got.Priority <= keyB.Priority {
		t.Errorf("degraded priority = %d, want moved after keyB(%d)", got.Priority, keyB.Priority)
	}
	// 冷却中的 key 不参与选择
	if _, err := f.providers.SelectKey(p.ID, 1); err != nil {
		t.Fatalf("SelectKey with one active key: %v", err)
	}
	// 事件日志应有 degraded
	events, _ := f.providers.KeyEvents(keyA.ID)
	if len(events) == 0 || events[0].Event != KeyEventDegraded {
		t.Errorf("expected degraded event, got %+v", events)
	}

	// 恢复 keyA
	if err := f.providers.RecoverKey(keyA.ID, "probe ok"); err != nil {
		t.Fatalf("RecoverKey: %v", err)
	}
	got, _ = f.providers.GetKey(keyA.ID)
	if got.Status != KeyStatusActive || got.FailCount != 0 || got.Priority != got.BasePriority {
		t.Errorf("after recover: %+v", got)
	}

	// 删除 keyB(手动)
	if err := f.providers.DeleteKey(keyB.ID, true, "manual"); err != nil {
		t.Fatalf("DeleteKey: %v", err)
	}
	got, _ = f.providers.GetKey(keyB.ID)
	if got.Status != KeyStatusDeleted {
		t.Errorf("status = %q, want deleted", got.Status)
	}
	// 已删除 key 不参与选择
	if _, err := f.providers.SelectKey(p.ID, 1); err != nil {
		t.Fatalf("SelectKey: %v", err)
	}
	ev, _ := f.providers.KeyEvents(keyB.ID)
	if ev[0].Event != KeyEventDeletedManual {
		t.Errorf("expected deleted_manual event, got %+v", ev)
	}
}

func TestRetryDueKeys(t *testing.T) {
	f := newCatalogFixture(t)
	_, _ = f.providers.Create("OpenAI", "https://api.openai.com/v1", "sk-a", "openai_chat")
	k, _ := f.providers.GetKey(1)
	_ = f.providers.DegradeKey(k.ID, "test") // retry_after = now + 1h
	// 未到期:不返回
	due, err := f.providers.RetryDueKeys()
	if err != nil {
		t.Fatalf("RetryDueKeys: %v", err)
	}
	if len(due) != 0 {
		t.Errorf("due before cooldown = %d, want 0", len(due))
	}
	// 手动把 retry_after 拨到过去
	_, err = f.db.Exec("UPDATE provider_api_keys SET retry_after = ? WHERE id = ?",
		time.Now().Add(-time.Minute), k.ID)
	if err != nil {
		t.Fatalf("backdate retry_after: %v", err)
	}
	due, _ = f.providers.RetryDueKeys()
	if len(due) != 1 || due[0].ID != k.ID {
		t.Errorf("due after cooldown = %+v, want key#%d", due, k.ID)
	}
}

func TestReplacePrimaryKeyViaUpdate(t *testing.T) {
	f := newCatalogFixture(t)
	p, _ := f.providers.Create("OpenAI", "https://api.openai.com/v1", "sk-a", "openai_chat")
	_, _ = f.providers.AddKey(p.ID, "sk-b", "b")
	// 编辑供应商并给新 api_key -> 替换第一个 active key
	if err := f.providers.Update(p.ID, "OpenAI", "https://api.openai.com/v1", "sk-new", "openai_chat", "active"); err != nil {
		t.Fatalf("Update: %v", err)
	}
	keys, _ := f.providers.ActiveKeys(p.ID)
	if len(keys) != 2 {
		t.Fatalf("active keys = %d, want 2", len(keys))
	}
	if keys[0].APIKey != "sk-new" {
		t.Errorf("primary key = %q, want sk-new (replaced)", keys[0].APIKey)
	}
	if keys[1].APIKey != "sk-b" {
		t.Errorf("second key = %q, want sk-b (preserved)", keys[1].APIKey)
	}
}
