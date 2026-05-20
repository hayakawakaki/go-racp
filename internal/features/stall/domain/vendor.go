package domain

import (
	"context"
	"time"
)

type VendorType int

const (
	VendorTypeUnknown VendorType = 0
	VendorTypeSelling VendorType = 1
	VendorTypeBuying  VendorType = 2
)

func VendorTypeFromString(value string) (VendorType, bool) {
	switch value {
	case "selling":
		return VendorTypeSelling, true
	case "buying":
		return VendorTypeBuying, true
	}

	return VendorTypeUnknown, false
}

func (t VendorType) String() string {
	switch t {
	case VendorTypeSelling:
		return "selling"
	case VendorTypeBuying:
		return "buying"
	}

	return ""
}

type VendorKey struct {
	Type VendorType
	ID   int
}

type VendorItem struct {
	ItemID int
	Index  int
	Amount int
	Price  int
}

type Vendor struct {
	StallName   string
	SellerName  string
	VendorMap   string
	Items       []VendorItem
	ID          int
	CharID      int
	X           int
	Y           int
	Autotrade   int
	Type        VendorType
	BudgetLimit int
}

func (v Vendor) Key() VendorKey {
	return VendorKey{Type: v.Type, ID: v.ID}
}

type Snapshot struct {
	LoadedAt time.Time
	ByKey    map[VendorKey]*Vendor
	ByItem   map[int][]VendorKey
	Vendors  []Vendor
}

type ListQuery struct {
	Type    VendorType
	ItemID  int
	Page    int
	PerPage int
}

type Page struct {
	Vendors    []Vendor
	Total      int
	Page       int
	PerPage    int
	TotalPages int
}

type Repository interface {
	LoadAll(ctx context.Context) ([]Vendor, error)
}
