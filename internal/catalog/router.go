package catalog

import (
	"errors"
	"math/rand"
	"sort"
	"time"
)

var globalRand = rand.New(rand.NewSource(time.Now().UnixNano()))

var errNoAvailableBinding = errors.New("no available upstream binding")

type HealthCacheReader interface {
	Get(providerID int64, upstreamModel string) string
}

type SelectedBinding struct {
	Provider      Provider
	Binding       ModelBinding
	UpstreamModel string
}

type Router struct {
	providers *ProviderStore
	health    HealthCacheReader
	random    *rand.Rand
}

func NewRouter(providers *ProviderStore, health HealthCacheReader) *Router {
	return &Router{providers: providers, health: health}
}

func (r *Router) Select(model Model, bindings []ModelBinding) (SelectedBinding, []ModelBinding, error) {
	active, err := r.activeBindings(bindings)
	if err != nil {
		return SelectedBinding{}, nil, err
	}

	switch model.RoutingStrategy {
	case RoutingStrategyRandom:
		b := weightedRandom(globalRand, active)
		sel, err := r.makeSelected(b)
		return sel, active, err
	case RoutingStrategyAuto:
		switch model.AutoMode {
		case AutoModeHealth:
			b, candidates := r.healthSelect(active)
			sel, err := r.makeSelected(b)
			return sel, candidates, err
		case AutoModePriority:
			b := priorityRandom(globalRand, active)
			sel, err := r.makeSelected(b)
			return sel, active, err
		default:
			sel, err := r.makeSelected(active[0])
			return sel, active, err
		}
	default:
		sel, err := r.makeSelected(active[0])
		return sel, active, err
	}
}

func (r *Router) Next(prev ModelBinding, candidates []ModelBinding) (SelectedBinding, bool, error) {
	active, err := r.activeBindings(candidates)
	if err != nil {
		return SelectedBinding{}, false, err
	}
	for i, b := range active {
		if b.ID == prev.ID && i+1 < len(active) {
			sel, err := r.makeSelected(active[i+1])
			return sel, true, err
		}
	}
	return SelectedBinding{}, false, nil
}

func (r *Router) activeBindings(bindings []ModelBinding) ([]ModelBinding, error) {
	out := make([]ModelBinding, 0, len(bindings))
	for _, b := range bindings {
		if !b.Enabled {
			continue
		}
		provider, err := r.providers.Get(b.ProviderID)
		if err != nil || provider.Status != "active" {
			continue
		}
		if r.health != nil {
			st := r.health.Get(b.ProviderID, b.UpstreamModel)
			if st == StatusUnhealthy {
				continue
			}
		}
		out = append(out, b)
	}
	if len(out) == 0 {
		return nil, errNoAvailableBinding
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Priority != out[j].Priority {
			return out[i].Priority < out[j].Priority
		}
		return out[i].ID < out[j].ID
	})
	return out, nil
}

func (r *Router) makeSelected(b ModelBinding) (SelectedBinding, error) {
	provider, err := r.providers.Get(b.ProviderID)
	if err != nil {
		return SelectedBinding{}, err
	}
	return SelectedBinding{Provider: provider, Binding: b, UpstreamModel: b.UpstreamModel}, nil
}

func (r *Router) healthSelect(bindings []ModelBinding) (ModelBinding, []ModelBinding) {
	healthy := make([]ModelBinding, 0, len(bindings))
	unhealthy := make([]ModelBinding, 0)
	for _, b := range bindings {
		if r.health == nil || r.health.Get(b.ProviderID, b.UpstreamModel) != StatusUnhealthy {
			healthy = append(healthy, b)
		} else {
			unhealthy = append(unhealthy, b)
		}
	}
	if len(healthy) > 0 {
		return priorityRandom(globalRand, healthy), append(healthy, unhealthy...)
	}
	return priorityRandom(globalRand, bindings), bindings
}

func priorityRandom(r *rand.Rand, bindings []ModelBinding) ModelBinding {
	top := bindings[0].Priority
	group := make([]ModelBinding, 0)
	for _, b := range bindings {
		if b.Priority != top {
			break
		}
		group = append(group, b)
	}
	return weightedRandom(r, group)
}

func weightedRandom(r *rand.Rand, bindings []ModelBinding) ModelBinding {
	total := 0
	for _, b := range bindings {
		total += b.Weight
	}
	if total <= 0 {
		return bindings[0]
	}
	var n int
	if r != nil {
		n = r.Intn(total)
	} else {
		n = rand.Intn(total)
	}
	for _, b := range bindings {
		n -= b.Weight
		if n < 0 {
			return b
		}
	}
	return bindings[len(bindings)-1]
}
