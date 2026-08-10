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
