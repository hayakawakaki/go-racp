package transport

import (
	"fmt"
	"net/url"
	"strconv"

	"github.com/hayakawakaki/go-racp/internal/features/stall/domain"
	stallstate "github.com/hayakawakaki/go-racp/internal/features/stall/transport/state"
)

func vendorsTotal(state stallstate.ListState) int {
	return state.BuyingPage.Total + state.SellingPage.Total
}

func buyingHrefPattern(state stallstate.ListState) string {
	return vendorsHrefPattern(state, "buying_page", "__PAGE__", "selling_page", state.SellingPage.Page)
}

func sellingHrefPattern(state stallstate.ListState) string {
	return vendorsHrefPattern(state, "selling_page", "__PAGE__", "buying_page", state.BuyingPage.Page)
}

func vendorsHrefPattern(state stallstate.ListState, primaryKey, primaryValue, otherKey string, otherPage int) string {
	values := url.Values{}
	values.Set(primaryKey, primaryValue)
	if otherPage > 1 {
		values.Set(otherKey, strconv.Itoa(otherPage))
	}
	if state.ItemID > 0 {
		values.Set("item", strconv.Itoa(state.ItemID))
	}
	base := state.BaseURL
	if base == "" {
		base = "/vendors"
	}
	return base + "?" + values.Encode()
}

func itemIDValue(id int) string {
	if id <= 0 {
		return ""
	}

	return fmt.Sprintf("%d", id)
}

func rowID(v domain.Vendor) string {
	return fmt.Sprintf("vendor-%s-%d", v.Type.String(), v.ID)
}

func typeLabel(t domain.VendorType) string {
	switch t {
	case domain.VendorTypeSelling:
		return "Selling"
	case domain.VendorTypeBuying:
		return "Buying"
	}

	return ""
}

func typeBadgeClass(t domain.VendorType) string {
	base := "inline-flex items-center rounded-md px-2 py-0.5 text-[11px] font-semibold tracking-wide"
	switch t {
	case domain.VendorTypeSelling:
		return base + " bg-emerald-100 text-emerald-800 dark:bg-emerald-950/40 dark:text-emerald-300"
	case domain.VendorTypeBuying:
		return base + " bg-sky-100 text-sky-800 dark:bg-sky-950/40 dark:text-sky-300"
	}

	return base + " bg-accent-wash text-accent-on-surface"
}
