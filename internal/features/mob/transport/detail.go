package transport

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"

	"github.com/hayakawakaki/go-racp/internal/features/mob/domain"
	"github.com/hayakawakaki/go-racp/internal/features/mob/transport/state"
	"github.com/hayakawakaki/go-racp/internal/platform/httpx"
)

func (h *Handler) showDetail(w http.ResponseWriter, r *http.Request) {
	idText := r.PathValue("id")
	id, err := strconv.Atoi(idText)
	if err != nil || id < 1 {
		h.renderNotFound(w, r, idText)
		return
	}

	mob, err := h.svc.Get(r.Context(), id)
	if errors.Is(err, domain.ErrNotFound) || errors.Is(err, domain.ErrEmptySnapshot) {
		h.renderNotFound(w, r, idText)
		return
	}
	if err != nil {
		h.logger.Error("mob: detail", "err", err, "id", id)
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	lookup := h.currentItemLookup()
	s := state.DetailState{
		Mob:      mob,
		Stats:    buildStats(mob),
		Exp:      buildExp(mob),
		Drops:    resolveDrops(mob.Drops, lookup),
		MvpDrops: resolveDrops(mob.MvpDrops, lookup),
	}
	httpx.RenderHTML(w, r, h.logger, h.theme.MobDetailPage(h.layout(), s))
}

func resolveDrops(drops []domain.MobDrop, lookup ItemLookup) []state.DropRow {
	if len(drops) == 0 {
		return nil
	}
	out := make([]state.DropRow, 0, len(drops))
	for _, drop := range drops {
		row := state.DropRow{Aegis: drop.ItemAegis, Rate: drop.Rate}
		if lookup != nil {
			if item := lookup.LookupByAegis(drop.ItemAegis); item != nil {
				row.ItemID = item.ID
				row.Image = item.Image
				row.ClientName = item.ClientName
				row.Slots = item.Slots
			}
		}
		out = append(out, row)
	}

	return out
}

func (h *Handler) renderNotFound(w http.ResponseWriter, r *http.Request, idText string) {
	httpx.RenderComponent404(w, r, h.logger, h.theme.MobNotFoundPage(h.layout(), idText))
}

func buildStats(mob *domain.Mob) []state.LabeledRow {
	return []state.LabeledRow{
		intRow("HP", mob.HP),
		{Label: "Attack", Value: fmt.Sprintf("%d - %d", mob.Attack, mob.Attack2)},
		intRow("Defense", mob.Defense),
		intRow("Magic Defense", mob.MagicDefense),
		intRow("Resistance", mob.Resistance),
		intRow("Magic Resistance", mob.MagicResistance),
		intRow("Attack Range", mob.AttackRange),
		intRow("Skill Range", mob.SkillRange),
		intRow("Str", mob.Str),
		intRow("Agi", mob.Agi),
		intRow("Vit", mob.Vit),
		intRow("Int", mob.Int),
		intRow("Dex", mob.Dex),
		intRow("Luk", mob.Luk),
	}
}

func buildExp(mob *domain.Mob) []state.LabeledRow {
	if mob.BaseExp == 0 && mob.JobExp == 0 {
		return nil
	}
	rows := []state.LabeledRow{
		intRow("Base Exp", mob.BaseExp),
		intRow("Job Exp", mob.JobExp),
	}
	if mob.MvpExp > 0 {
		rows = append(rows, intRow("MVP Exp", mob.MvpExp))
	}

	return rows
}

func intRow(label string, value int) state.LabeledRow {
	return state.LabeledRow{Label: label, Value: fmt.Sprintf("%d", value)}
}
