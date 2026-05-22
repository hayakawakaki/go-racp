package transport

import (
	"fmt"
	"net/url"

	"github.com/hayakawakaki/go-racp/internal/features/stall/domain"
)

func pageURL(baseURL string, page int, typeName string, itemID int) string {
	values := url.Values{}
	values.Set("page", fmt.Sprintf("%d", page))
	if typeName != "" {
		values.Set("type", typeName)
	}
	if itemID > 0 {
		values.Set("item", fmt.Sprintf("%d", itemID))
	}

	return baseURL + "?" + values.Encode()
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
	switch t {
	case domain.VendorTypeSelling:
		return "inline-flex items-center rounded-full bg-emerald-50 px-2 py-0.5 text-xs font-medium text-emerald-700"
	case domain.VendorTypeBuying:
		return "inline-flex items-center rounded-full bg-blue-50 px-2 py-0.5 text-xs font-medium text-blue-700"
	}

	return "inline-flex items-center rounded-full bg-gray-100 px-2 py-0.5 text-xs font-medium text-gray-700"
}
