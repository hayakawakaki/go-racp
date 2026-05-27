package transport

import (
	"net/url"

	itemstate "github.com/hayakawakaki/go-racp/internal/features/item/transport/state"
	"github.com/hayakawakaki/go-racp/internal/platform/ui"
)

var itemTypeOptions = []string{
	"Weapon", "Armor", "Card", "Healing", "Usable",
	"Etc", "Ammo", "PetEgg", "PetArmor", "DelayConsume",
	"ShadowGear", "Cash",
}

func itemTypeSelectOptions() []ui.SelectOption {
	out := make([]ui.SelectOption, 0, len(itemTypeOptions)+1)
	out = append(out, ui.SelectOption{Value: "", Label: "All types"})
	for _, name := range itemTypeOptions {
		out = append(out, ui.SelectOption{Value: name, Label: name})
	}
	return out
}

func pageHrefPattern(state itemstate.ListState) string {
	values := url.Values{}
	values.Set("page", "__PAGE__")
	if state.Query != "" {
		values.Set("q", state.Query)
	}
	if state.Type != "" {
		values.Set("type", state.Type)
	}
	base := state.BaseURL
	if base == "" {
		base = "/items"
	}

	return base + "?" + values.Encode()
}
