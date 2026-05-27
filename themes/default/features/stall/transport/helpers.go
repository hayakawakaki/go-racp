package transport

import (
	"fmt"
	"net/url"

	"github.com/hayakawakaki/go-racp/internal/features/stall/domain"
	stallstate "github.com/hayakawakaki/go-racp/internal/features/stall/transport/state"
)

func vendorsHrefPattern(state stallstate.ListState) string {
	values := url.Values{}
	values.Set("page", "__PAGE__")
	if state.ItemID > 0 {
		values.Set("item", fmt.Sprintf("%d", state.ItemID))
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

func countByType(vendors []domain.Vendor, t domain.VendorType) int {
	count := 0
	for _, v := range vendors {
		if v.Type == t {
			count++
		}
	}
	return count
}

func vendorsColumnBadgeClass(t domain.VendorType) string {
	base := "rounded-md px-2 py-0.5 text-[11px] font-semibold tabular-nums"
	switch t {
	case domain.VendorTypeBuying:
		return base + " bg-sky-100 text-sky-800 dark:bg-sky-950/40 dark:text-sky-300"
	case domain.VendorTypeSelling:
		return base + " bg-emerald-100 text-emerald-800 dark:bg-emerald-950/40 dark:text-emerald-300"
	}
	return base + " bg-accent-wash text-accent-on-surface"
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
