package cards

import (
	"fmt"
	"github.com/gscarneiro/eafc-bot/internal/domain"
	"github.com/gscarneiro/eafc-bot/internal/futgg"
	"strconv"
)

// CatalogEntry liga toda carta do clube a um slug estável, mesmo quando a
// análise cara de evolução foi pulada por estar abaixo do corte.
type CatalogEntry struct {
	Slug   string            `json:"slug"`
	Player domain.ClubPlayer `json:"player"`
	Report *CardReport       `json:"report,omitempty"`
}

func BuildCatalog(club domain.Club, reports []CardReport, roles futgg.RolesTable) []CatalogEntry {
	used := map[string]bool{}
	byItem := map[string]*CardReport{}
	byID := map[int64][]*CardReport{}
	for i := range reports {
		r := &reports[i]
		if r.Player.ClubItemID != "" {
			byItem[r.Player.ClubItemID] = r
		}
		byID[r.Player.ID] = append(byID[r.Player.ID], r)
		used[r.Slug] = true
	}
	seenID := map[int64]int{}
	out := make([]CatalogEntry, 0, len(club.Players))
	for _, p := range club.Players {
		var r *CardReport
		if p.ClubItemID != "" {
			r = byItem[p.ClubItemID]
		}
		if r == nil {
			list := byID[p.ID]
			idx := seenID[p.ID]
			if idx < len(list) {
				r = list[idx]
			}
			seenID[p.ID]++
		}
		slug := ""
		if r != nil {
			slug = r.Slug
		}
		if slug == "" {
			slug = catalogSlug(p.Player, used)
			used[slug] = true
		}
		if r == nil {
			rr := &CardReport{Slug: slug, Player: p, ByPosition: positionRoles(p.Player, roles), EvolutionStatus: EvolutionNotChecked}
			r = rr
		}
		out = append(out, CatalogEntry{Slug: slug, Player: p, Report: r})
	}
	return out
}

func catalogSlug(p domain.Player, used map[string]bool) string {
	base := cardSlug(p)
	if !used[base] {
		return base
	}
	for i := 2; ; i++ {
		s := fmt.Sprintf("%s-%s", base, strconv.Itoa(i))
		if !used[s] {
			return s
		}
	}
}

func FindCatalog(entries []CatalogEntry, slug string) (CatalogEntry, bool) {
	for _, e := range entries {
		if e.Slug == slug {
			return e, true
		}
	}
	return CatalogEntry{}, false
}
