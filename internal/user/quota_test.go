package user

import (
	"testing"
)

func TestSetAndGetQuota(t *testing.T) {
	s := newStore(t)
	u, _ := s.Create("q@x.com", "h", "user")
	var lim int64 = 100000
	var cost float64 = 5.0
	q, err := s.SetQuota(Quota{Scope: "user", ScopeID: u.ID, Period: "month", LimitTokens: &lim, LimitCost: &cost})
	if err != nil {
		t.Fatalf("SetQuota: %v", err)
	}
	if q.ID == 0 {
		t.Fatal("expected non-zero ID")
	}
	got, err := s.GetQuotas("user", u.ID)
	if err != nil || len(got) != 1 {
		t.Fatalf("GetQuotas: %v len=%d", err, len(got))
	}
	if *got[0].LimitTokens != 100000 || *got[0].LimitCost != 5.0 {
		t.Errorf("quota = %+v", got[0])
	}
}

func TestUpdateAndDeleteQuota(t *testing.T) {
	s := newStore(t)
	u, _ := s.Create("q2@x.com", "h", "user")
	var lim int64 = 1000
	q, _ := s.SetQuota(Quota{Scope: "user", ScopeID: u.ID, Period: "total", LimitTokens: &lim})
	var newLim int64 = 5000
	if err := s.UpdateQuota(q.ID, &newLim, nil); err != nil {
		t.Fatalf("UpdateQuota: %v", err)
	}
	got, _ := s.GetQuota(q.ID)
	if *got.LimitTokens != 5000 {
		t.Errorf("after update: %d", *got.LimitTokens)
	}
	s.DeleteQuota(q.ID)
	if _, err := s.GetQuota(q.ID); err == nil {
		t.Error("expected error after delete")
	}
}

func TestIncrementUsage(t *testing.T) {
	s := newStore(t)
	u, _ := s.Create("q3@x.com", "h", "user")
	var lim int64 = 100000
	s.SetQuota(Quota{Scope: "user", ScopeID: u.ID, Period: "total", LimitTokens: &lim})
	// 累加两次
	s.IncrementUsage("user", u.ID, 100, 0.5)
	s.IncrementUsage("user", u.ID, 50, 0.25)
	got, _ := s.GetQuotas("user", u.ID)
	if got[0].UsedTokens != 150 || got[0].UsedCost != 0.75 {
		t.Errorf("usage = tokens=%d cost=%.2f", got[0].UsedTokens, got[0].UsedCost)
	}
}

func TestModelQuotaUpsert(t *testing.T) {
	s := newStore(t)
	// 未设置时返回零值(ID==0)
	q, err := s.GetModelQuota(42)
	if err != nil {
		t.Fatalf("GetModelQuota(empty): %v", err)
	}
	if q.ID != 0 {
		t.Fatalf("expected zero quota, got %+v", q)
	}
	// 设置 token+费用上限
	var lim int64 = 1_000_000
	var cost float64 = 10.0
	q, err = s.SetModelQuota(42, "month", &lim, &cost)
	if err != nil {
		t.Fatalf("SetModelQuota: %v", err)
	}
	if q.ID == 0 || q.Scope != "model" || q.ScopeID != 42 {
		t.Fatalf("set quota = %+v", q)
	}
	if *q.LimitTokens != lim || *q.LimitCost != cost || q.Period != "month" {
		t.Fatalf("set quota limits = %+v", q)
	}
	// 更新(upsert,仍只有一条记录)
	var newLim int64 = 500_000
	if _, err := s.SetModelQuota(42, "total", &newLim, nil); err != nil {
		t.Fatalf("SetModelQuota(update): %v", err)
	}
	got, _ := s.GetModelQuota(42)
	if got.ID != q.ID || *got.LimitTokens != newLim || got.LimitCost != nil || got.Period != "total" {
		t.Fatalf("updated quota = %+v", got)
	}
	if all, _ := s.GetQuotas("model", 42); len(all) != 1 {
		t.Fatalf("expected 1 model quota row, got %d", len(all))
	}
	// 清空(两个 limit 均为 nil) -> 删除记录
	if _, err := s.SetModelQuota(42, "total", nil, nil); err != nil {
		t.Fatalf("SetModelQuota(clear): %v", err)
	}
	q, _ = s.GetModelQuota(42)
	if q.ID != 0 {
		t.Fatalf("expected quota cleared, got %+v", q)
	}
	// DeleteModelQuota 幂等
	if err := s.DeleteModelQuota(42); err != nil {
		t.Fatalf("DeleteModelQuota: %v", err)
	}
}

func TestKeyQuotaUpsert(t *testing.T) {
	s := newStore(t)
	// 未设置时返回零值(ID==0)
	q, err := s.GetKeyQuota(7)
	if err != nil {
		t.Fatalf("GetKeyQuota(empty): %v", err)
	}
	if q.ID != 0 {
		t.Fatalf("expected zero quota, got %+v", q)
	}
	// 设置 token 上限
	var lim int64 = 500_000
	q, err = s.SetKeyQuota(7, "month", &lim, nil)
	if err != nil {
		t.Fatalf("SetKeyQuota: %v", err)
	}
	if q.ID == 0 || q.Scope != "key" || q.ScopeID != 7 || *q.LimitTokens != lim || q.LimitCost != nil {
		t.Fatalf("set quota = %+v", q)
	}
	// 更新为费用上限(upsert)
	var cost float64 = 5.0
	q, err = s.SetKeyQuota(7, "total", nil, &cost)
	if err != nil {
		t.Fatalf("SetKeyQuota(update): %v", err)
	}
	if q.LimitTokens != nil || q.LimitCost == nil || *q.LimitCost != cost || q.Period != "total" {
		t.Fatalf("updated quota = %+v", q)
	}
	if all, _ := s.GetQuotas("key", 7); len(all) != 1 {
		t.Fatalf("expected 1 key quota row, got %d", len(all))
	}
	// 清空 -> 删除记录
	if _, err := s.SetKeyQuota(7, "total", nil, nil); err != nil {
		t.Fatalf("SetKeyQuota(clear): %v", err)
	}
	q, _ = s.GetKeyQuota(7)
	if q.ID != 0 {
		t.Fatalf("expected quota cleared, got %+v", q)
	}
	// DeleteKeyQuota 幂等
	if err := s.DeleteKeyQuota(7); err != nil {
		t.Fatalf("DeleteKeyQuota: %v", err)
	}
}
