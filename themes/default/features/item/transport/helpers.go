package transport

import (
	"fmt"
	"net/url"

	itemstate "github.com/hayakawakaki/go-racp/internal/features/item/transport/state"
)

const droppedByPerPage = 10

func droppedByAlpineState(count int) string {
	pages := max((count+droppedByPerPage-1)/droppedByPerPage, 1)

	return fmt.Sprintf("{ page: 1, perPage: %d, totalPages: %d }", droppedByPerPage, pages)
}

var itemTypeOptions = []string{
	"Weapon", "Armor", "Card", "Healing", "Usable",
	"Etc", "Ammo", "PetEgg", "PetArmor", "DelayConsume",
	"ShadowGear", "Cash",
}

func pageURL(state itemstate.ListState, page int) string {
	values := url.Values{}
	values.Set("page", fmt.Sprintf("%d", page))
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
