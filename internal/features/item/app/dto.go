package app

import (
	"github.com/hayakawakaki/go-racp/internal/features/item/domain"
)

const DefaultPerPage = 20

type ItemDTO struct {
	AegisName   string   `json:"aegis_name"`
	Name        string   `json:"name"`
	ClientName  string   `json:"client_name"`
	Image       string   `json:"image"`
	Type        string   `json:"type"`
	SubType     string   `json:"sub_type,omitempty"`
	Description []string `json:"description,omitempty"`
	Weight      float64  `json:"weight"`
	Buy         int      `json:"buy"`
	Sell        int      `json:"sell"`
	ID          int      `json:"id"`
	Slots       int      `json:"slots,omitempty"`
}

type ItemPage struct {
	Items      []*domain.Item
	Total      int
	Page       int
	PerPage    int
	TotalPages int
}

type ServiceStatus struct {
	LastReload  string
	LastError   string
	ItemsLoaded int
}

func ToDTO(item *domain.Item) ItemDTO {
	if item == nil {
		return ItemDTO{}
	}

	return ItemDTO{
		ID:          item.ID,
		AegisName:   item.AegisName,
		Name:        item.Name,
		ClientName:  item.ClientName,
		Image:       item.Image,
		Type:        item.Type.String(),
		SubType:     item.SubType,
		Description: item.Description,
		Weight:      item.Weight,
		Buy:         item.Buy,
		Sell:        item.Sell,
		Slots:       item.Slots,
	}
}
