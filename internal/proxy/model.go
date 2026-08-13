package proxy

import (
	"carryapi/internal/catalog"
	"carryapi/internal/ir"
	"carryapi/internal/user"
)

type resolvedModel struct {
	model      *catalog.Model
	provider   *catalog.Provider
	selected   *catalog.SelectedBinding
	candidates []catalog.ModelBinding
	price      *catalog.Price
}

func (p *Proxy) resolveModel(name string) (*resolvedModel, error) {
	model, err := p.deps.Models.GetByName(name)
	if err != nil {
		return nil, ir.NewError("not_found", "model_not_found", "The model '"+name+"' does not exist", 404)
	}
	if !model.Enabled {
		return nil, ir.NewError("not_found", "model_not_found", "The model '"+name+"' is disabled", 404)
	}
	bindings, err := p.deps.Bindings.ListEnabledByModel(model.ID)
	if err != nil {
		return nil, ir.NewError("internal", "bindings_unavailable", "failed to load model bindings", 500)
	}
	selected, candidates, err := p.getRouter().Select(model, bindings)
	if err != nil {
		return nil, ir.NewError("internal", "provider_not_found", "provider not configured", 500)
	}
	price, err := p.deps.Prices.GetCurrent(model.ID)
	if err != nil {
		return nil, ir.NewError("internal", "price_not_configured", "model has no price configured", 500)
	}
	provider := selected.Provider
	return &resolvedModel{
		model:      &model,
		provider:   &provider,
		selected:   &selected,
		candidates: candidates,
		price:      &price,
	}, nil
}

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
