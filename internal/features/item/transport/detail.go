package transport

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/hayakawakaki/go-racp/internal/features/item/domain"
	"github.com/hayakawakaki/go-racp/internal/features/item/transport/state"
	mobdomain "github.com/hayakawakaki/go-racp/internal/features/mob/domain"
	"github.com/hayakawakaki/go-racp/internal/platform/httpx"
)

func (h *Handler) showDetail(w http.ResponseWriter, r *http.Request) {
	idText := r.PathValue("id")
	id, err := strconv.Atoi(idText)
	if err != nil || id < 1 {
		h.renderNotFound(w, r, idText)
		return
	}

	item, err := h.svc.Get(r.Context(), id)
	if errors.Is(err, domain.ErrNotFound) || errors.Is(err, domain.ErrEmptySnapshot) {
		h.renderNotFound(w, r, idText)
		return
	}
	if err != nil {
		h.logger.Error("item: detail", "err", err, "id", id)
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	lines := make([]string, 0, len(item.Description))
	for _, line := range item.Description {
		lines = append(lines, renderDescription([]string{line}))
	}
	var droppedBy []mobdomain.DropOf
	if h.dropLookup != nil {
		droppedBy = h.dropLookup.WhoDrops(item.AegisName)
	}
	s := state.DetailState{
		Item:             item,
		Stats:            buildStats(item),
		DescriptionLines: lines,
		DroppedBy:        droppedBy,
	}
	httpx.RenderHTML(w, r, h.logger, h.theme.ItemDetailPage(h.layout(), s))
}

func (h *Handler) renderNotFound(w http.ResponseWriter, r *http.Request, idText string) {
	httpx.RenderComponent404(w, r, h.logger, h.theme.ItemNotFoundPage(h.layout(), idText))
}

func buildStats(item *domain.Item) []state.LabeledRow {
	rows := []state.LabeledRow{
		{Label: "Type", Value: item.Type.Display()},
		{Label: "Weight", Value: fmt.Sprintf("%.1f", item.Weight)},
		{Label: "Buy", Value: fmt.Sprintf("%d z", item.Buy)},
		{Label: "Sell", Value: fmt.Sprintf("%d z", item.Sell)},
	}
	switch item.Type {
	case domain.ItemTypeWeapon:
		rows = append(rows,
			state.LabeledRow{Label: "Weapon Level", Value: fmt.Sprintf("%d", item.WeaponLevel)},
			state.LabeledRow{Label: "Attack", Value: fmt.Sprintf("%d", item.Attack)},
			state.LabeledRow{Label: "Range", Value: fmt.Sprintf("%d", item.Range)},
			state.LabeledRow{Label: "Slots", Value: fmt.Sprintf("%d", item.Slots)},
			state.LabeledRow{Label: "Refineable", Value: yesNo(item.Refineable)},
		)
	case domain.ItemTypeArmor:
		rows = append(rows,
			state.LabeledRow{Label: "Armor Level", Value: fmt.Sprintf("%d", item.ArmorLevel)},
			state.LabeledRow{Label: "Defense", Value: fmt.Sprintf("%d", item.Defense)},
			state.LabeledRow{Label: "Slots", Value: fmt.Sprintf("%d", item.Slots)},
			state.LabeledRow{Label: "Refineable", Value: yesNo(item.Refineable)},
		)
	}
	if label := locationLabel(item.Type); label != "" {
		if locations := item.Locations.Display(); len(locations) > 0 {
			rows = append(rows, state.LabeledRow{Label: label, Value: strings.Join(locations, ", ")})
		}
	}
	if item.SubType != "" {
		rows = append(rows, state.LabeledRow{Label: "Subtype", Value: item.SubType})
	}

	return rows
}

func locationLabel(itemType domain.ItemType) string {
	switch itemType {
	case domain.ItemTypeWeapon, domain.ItemTypeArmor:
		return "Equip Location"
	case domain.ItemTypeCard:
		return "Slot Location"
	}

	return ""
}

func yesNo(value bool) string {
	if value {
		return "Yes"
	}

	return "No"
}
