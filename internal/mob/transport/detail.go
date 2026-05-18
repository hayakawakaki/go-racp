package transport

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"

	"github.com/hayakawakaki/go-racp/internal/httpx"
	"github.com/hayakawakaki/go-racp/internal/mob/domain"
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

	state := DetailState{
		Mob:   mob,
		Stats: buildStats(mob),
		Exp:   buildExp(mob),
	}
	httpx.RenderHTML(w, r, h.logger, detailPage(h.layout(), state))
}

func (h *Handler) renderNotFound(w http.ResponseWriter, r *http.Request, idText string) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusNotFound)
	if err := notFoundPage(h.layout(), idText).Render(r.Context(), w); err != nil {
		h.logger.Error("mob: render not found", "err", err)
	}
}

func buildStats(mob *domain.Mob) []labeledRow {
	rows := []labeledRow{
		{Label: "HP", Value: fmt.Sprintf("%d", mob.HP)},
		{Label: "Attack", Value: fmt.Sprintf("%d - %d", mob.Attack, mob.Attack2)},
		{Label: "Defense", Value: fmt.Sprintf("%d", mob.Defense)},
		{Label: "Magic Defense", Value: fmt.Sprintf("%d", mob.MagicDefense)},
		{Label: "Resistance", Value: fmt.Sprintf("%d", mob.Resistance)},
		{Label: "Magic Resistance", Value: fmt.Sprintf("%d", mob.MagicResistance)},
		{Label: "Attack Range", Value: fmt.Sprintf("%d", mob.AttackRange)},
		{Label: "Skill Range", Value: fmt.Sprintf("%d", mob.SkillRange)},
		{Label: "Str", Value: fmt.Sprintf("%d", mob.Str)},
		{Label: "Agi", Value: fmt.Sprintf("%d", mob.Agi)},
		{Label: "Vit", Value: fmt.Sprintf("%d", mob.Vit)},
		{Label: "Int", Value: fmt.Sprintf("%d", mob.Int)},
		{Label: "Dex", Value: fmt.Sprintf("%d", mob.Dex)},
		{Label: "Luk", Value: fmt.Sprintf("%d", mob.Luk)},
	}

	return rows
}

func buildExp(mob *domain.Mob) []labeledRow {
	if mob.BaseExp == 0 && mob.JobExp == 0 {
		return nil
	}
	rows := []labeledRow{
		{Label: "Base Exp", Value: fmt.Sprintf("%d", mob.BaseExp)},
		{Label: "Job Exp", Value: fmt.Sprintf("%d", mob.JobExp)},
	}
	if mob.MvpExp > 0 {
		rows = append(rows, labeledRow{Label: "MVP Exp", Value: fmt.Sprintf("%d", mob.MvpExp)})
	}

	return rows
}
