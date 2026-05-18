package transport

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/hayakawakaki/go-racp/internal/httpx"
	"github.com/hayakawakaki/go-racp/internal/item/domain"
	mobdomain "github.com/hayakawakaki/go-racp/internal/mob/domain"
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
	state := DetailState{
		Item:             item,
		Stats:            buildStats(item),
		DescriptionLines: lines,
		DroppedBy:        droppedBy,
	}
	httpx.RenderHTML(w, r, h.logger, detailPage(h.layout(), state))
}

func (h *Handler) renderNotFound(w http.ResponseWriter, r *http.Request, idText string) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusNotFound)
	if err := notFoundPage(h.layout(), idText).Render(r.Context(), w); err != nil {
		h.logger.Error("item: render not found", "err", err)
	}
}

func buildStats(item *domain.Item) []labeledRow {
	rows := []labeledRow{
		{Label: "Type", Value: item.Type.Display()},
		{Label: "Weight", Value: fmt.Sprintf("%.1f", item.Weight)},
		{Label: "Buy", Value: fmt.Sprintf("%d z", item.Buy)},
		{Label: "Sell", Value: fmt.Sprintf("%d z", item.Sell)},
	}
	switch item.Type {
	case domain.ItemTypeWeapon:
		rows = append(rows,
			labeledRow{Label: "Weapon Level", Value: fmt.Sprintf("%d", item.WeaponLevel)},
			labeledRow{Label: "Attack", Value: fmt.Sprintf("%d", item.Attack)},
			labeledRow{Label: "Range", Value: fmt.Sprintf("%d", item.Range)},
			labeledRow{Label: "Slots", Value: fmt.Sprintf("%d", item.Slots)},
			labeledRow{Label: "Refineable", Value: yesNo(item.Refineable)},
		)
	case domain.ItemTypeArmor:
		rows = append(rows,
			labeledRow{Label: "Armor Level", Value: fmt.Sprintf("%d", item.ArmorLevel)},
			labeledRow{Label: "Defense", Value: fmt.Sprintf("%d", item.Defense)},
			labeledRow{Label: "Slots", Value: fmt.Sprintf("%d", item.Slots)},
			labeledRow{Label: "Refineable", Value: yesNo(item.Refineable)},
		)
	}
	if label := locationLabel(item.Type); label != "" {
		if locations := item.Locations.Display(); len(locations) > 0 {
			rows = append(rows, labeledRow{Label: label, Value: strings.Join(locations, ", ")})
		}
	}
	if item.SubType != "" {
		rows = append(rows, labeledRow{Label: "Subtype", Value: item.SubType})
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

const droppedByPerPage = 10

func droppedByAlpineState(count int) string {
	pages := max((count+droppedByPerPage-1)/droppedByPerPage, 1)

	return fmt.Sprintf("{ page: 1, perPage: %d, totalPages: %d }", droppedByPerPage, pages)
}
