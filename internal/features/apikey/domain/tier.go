package domain

import "sort"

type Tier struct {
	Name          string
	RatePerMinute int
	Burst         int
}

type TierSet struct {
	byName map[string]Tier
	order  []string
}

func NewTierSet(tiers []Tier) TierSet {
	byName := make(map[string]Tier, len(tiers))
	order := make([]string, 0, len(tiers))
	for _, tier := range tiers {
		byName[tier.Name] = tier
		order = append(order, tier.Name)
	}
	sort.Strings(order)

	return TierSet{byName: byName, order: order}
}

func (s TierSet) Has(name string) bool {
	_, ok := s.byName[name]

	return ok
}

func (s TierSet) Get(name string) (Tier, bool) {
	tier, ok := s.byName[name]

	return tier, ok
}

func (s TierSet) List() []Tier {
	out := make([]Tier, 0, len(s.order))
	for _, name := range s.order {
		out = append(out, s.byName[name])
	}

	return out
}
