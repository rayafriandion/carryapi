package proxy

import (
	"carryapi/internal/catalog"
	"carryapi/internal/ir"
	"carryapi/internal/user"
)

func (p *Proxy) resolveModel(name string) (*catalog.Model, *catalog.Provider, *catalog.Price, error) {
	model, err := p.deps.Models.GetByName(name)
	if err != nil {
		return nil, nil, nil, ir.NewError("not_found", "model_not_found", "The model '"+name+"' does not exist", 404)
	}
	if !model.Enabled {
		return nil, nil, nil, ir.NewError("not_found", "model_not_found", "The model '"+name+"' is disabled", 404)
	}
	provider, err := p.deps.Providers.Get(model.ProviderID)
	if err != nil {
		return nil, nil, nil, ir.NewError("internal", "provider_not_found", "provider not configured", 500)
	}
	if provider.Status != "active" {
		return nil, nil, nil, ir.NewError("internal", "provider_disabled", "provider is disabled", 500)
	}
	price, err := p.deps.Prices.GetCurrent(model.ID)
	if err != nil {
		return nil, nil, nil, ir.NewError("internal", "price_not_configured", "model has no price configured", 500)
	}
	return &model, &provider, &price, nil
}

// checkQuota 请求前预检:token/费用上限。
func (p *Proxy) checkQuota(u *user.User, keyID int64) error {
	scopes := []struct {
		scope   string
		scopeID int64
	}{
		{"user", u.ID},
		{"key", keyID},
	}
	for _, s := range scopes {
		quotas, err := p.deps.Users.GetQuotas(s.scope, s.scopeID)
		if err != nil {
			return ir.NewError("internal", "quota_check_failed", "failed to check quota", 500)
		}
		for _, q := range quotas {
			limitTokens := q.LimitTokens
			limitCost := q.LimitCost
			if limitTokens != nil && q.UsedTokens >= *limitTokens {
				return ir.NewError("rate_limit", "quota_exceeded", "quota exceeded (tokens)", 429)
			}
			if limitCost != nil && q.UsedCost >= *limitCost {
				return ir.NewError("rate_limit", "quota_exceeded", "quota exceeded (cost)", 429)
			}
		}
	}
	return nil
}
