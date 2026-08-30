package api

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/gscarneiro/eafc-bot/internal/cards"
	"github.com/gscarneiro/eafc-bot/internal/domain"
	"github.com/gscarneiro/eafc-bot/internal/query"
	"github.com/gscarneiro/eafc-bot/internal/store"
)

// evolutionAnalysisMinRating Ã© o contrato da Ã¡rea de EvoluÃ§Ãµes. A opÃ§Ã£o de
// configuraÃ§Ã£o ainda pode baixar o piso para detalhes de carta, mas nunca
// pode esconder desta anÃ¡lise as cartas 88+ que a pessoa pediu para revisar.
const evolutionAnalysisMinRating = 88

type EvolutionPathImpact struct {
	Kind            string             `json:"kind"`
	SlotIndex       int                `json:"slot_index,omitempty"`
	Position        domain.Position    `json:"position,omitempty"`
	Starter         *domain.ClubPlayer `json:"starter,omitempty"`
	StarterGGRating float64            `json:"starter_gg_rating,omitempty"`
	FinalGGRating   float64            `json:"final_gg_rating,omitempty"`
	Gain            float64            `json:"gain,omitempty"`
}

type EvolutionPathCandidate struct {
	ID          string              `json:"id"`
	Potential   cards.EvoPotential  `json:"potential"`
	Impact      EvolutionPathImpact `json:"impact"`
	VersionHash string              `json:"version_hash"`
	Saved       bool                `json:"saved"`
}

// EvolutionPlayerAnalysis Ã© uma linha por CÃ“PIA fÃ­sica. Paths ficam aninhados
// para que "sem path" e "nÃ£o verificada" tambÃ©m apareÃ§am na cobertura.
type EvolutionPlayerAnalysis struct {
	Player            domain.ClubPlayer        `json:"player"`
	CardSlug          string                   `json:"card_slug,omitempty"`
	IdentityComplete  bool                     `json:"identity_complete"`
	Status            cards.EvolutionStatus    `json:"status"`
	Paths             []EvolutionPathCandidate `json:"paths"`
	BestFinalGGRating float64                  `json:"best_final_gg_rating,omitempty"`
	BestXIGain        float64                  `json:"best_xi_gain,omitempty"`
	EntraNoXI         bool                     `json:"entra_no_xi"`
}

type EvolutionPathsSummary struct {
	Players        int `json:"players"`
	Confirmed      int `json:"confirmed"`
	NoPath         int `json:"no_path"`
	NotEligible    int `json:"not_eligible"`
	FetchError     int `json:"fetch_error"`
	NotChecked     int `json:"not_checked"`
	EntraNoXI      int `json:"entra_no_xi"`
	MelhoraTitular int `json:"melhora_titular"`
}

type evolutionPathsCollectionResponse struct {
	query.Page[EvolutionPlayerAnalysis]
	Summary EvolutionPathsSummary `json:"@eafc.summary"`
}

type savedEvolutionPathView struct {
	Saved  domain.SavedEvolutionPath `json:"saved"`
	Status string                    `json:"status"`
}

type savedEvolutionPathsResponse struct {
	Value []savedEvolutionPathView `json:"value"`
	Count int                      `json:"@odata.count"`
}

func evolutionPathsSchema() query.Schema[EvolutionPlayerAnalysis] {
	return query.NewSchema("evolucoes/caminhos", "entra_no_xi desc,best_xi_gain desc,best_final_gg_rating desc,player/common_name asc", 500,
		text("player/common_name", func(v EvolutionPlayerAnalysis) string { return v.Player.CommonName }, false, true),
		text("player/name", func(v EvolutionPlayerAnalysis) string { return v.Player.Name }, false, true),
		text("player/position", func(v EvolutionPlayerAnalysis) string { return string(v.Player.Position) }, true, false),
		integer("player/rating", func(v EvolutionPlayerAnalysis) int { return v.Player.Rating }),
		text("status", func(v EvolutionPlayerAnalysis) string { return string(v.Status) }, true, false),
		number("best_final_gg_rating", func(v EvolutionPlayerAnalysis) float64 { return v.BestFinalGGRating }),
		number("best_xi_gain", func(v EvolutionPlayerAnalysis) float64 { return v.BestXIGain }),
		boolean("entra_no_xi", func(v EvolutionPlayerAnalysis) bool { return v.EntraNoXI }, true),
	)
}

func (s *Server) handleEvolutionPaths(w http.ResponseWriter, r *http.Request) {
	snap, ok := s.load(w, r)
	if !ok {
		return
	}
	rows := buildEvolutionPlayerAnalyses(snap)
	if backend, ok := s.Store.(store.SavedEvolutionPathStore); ok {
		if saved, err := backend.ListSavedEvolutionPaths(r.Context(), s.Cycle); err == nil {
			markSavedEvolutionPaths(rows, saved)
		}
	}
	page, ok := serveList(w, r, evolutionPathsSchema(), rows)
	if !ok {
		return
	}
	writeJSON(w, evolutionPathsCollectionResponse{Page: page, Summary: evolutionPathsSummary(rows)})
}

func (s *Server) handleSavedEvolutionPaths(w http.ResponseWriter, r *http.Request) {
	backend, ok := s.Store.(store.SavedEvolutionPathStore)
	if !ok {
		http.Error(w, "paths salvos indisponÃ­veis neste armazenamento", http.StatusNotImplemented)
		return
	}
	saved, err := backend.ListSavedEvolutionPaths(r.Context(), s.Cycle)
	if err != nil {
		http.Error(w, "lendo paths salvos: "+err.Error(), http.StatusInternalServerError)
		return
	}
	var current map[string]EvolutionPathCandidate
	if snap, found, loadErr := s.Store.LatestSnapshot(r.Context(), s.Cycle); loadErr == nil && found {
		current = candidatesByID(buildEvolutionPlayerAnalyses(snap))
	}
	value := make([]savedEvolutionPathView, 0, len(saved))
	for _, entry := range saved {
		value = append(value, savedEvolutionPathView{Saved: entry, Status: savedPathStatus(entry, current[entry.PathID])})
	}
	writeJSON(w, savedEvolutionPathsResponse{Value: value, Count: len(value)})
}

func (s *Server) handleSavedEvolutionPathCreate(w http.ResponseWriter, r *http.Request) {
	backend, ok := s.Store.(store.SavedEvolutionPathStore)
	if !ok {
		http.Error(w, "paths salvos indisponÃ­veis neste armazenamento", http.StatusNotImplemented)
		return
	}
	var input struct {
		PathID string `json:"path_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil || strings.TrimSpace(input.PathID) == "" {
		http.Error(w, "informe o path_id que deseja salvar", http.StatusBadRequest)
		return
	}
	snap, ok := s.load(w, r)
	if !ok {
		return
	}
	for _, row := range buildEvolutionPlayerAnalyses(snap) {
		for _, candidate := range row.Paths {
			if candidate.ID != input.PathID {
				continue
			}
			saved := savedEvolutionPathFrom(row, candidate, s.Cycle)
			if err := backend.SaveEvolutionPath(r.Context(), saved); err != nil {
				http.Error(w, "gravando path salvo: "+err.Error(), http.StatusBadRequest)
				return
			}
			writeJSON(w, savedEvolutionPathView{Saved: saved, Status: savedPathStatus(saved, candidate)})
			return
		}
	}
	http.Error(w, "path nÃ£o encontrado na coleta atual", http.StatusNotFound)
}

func (s *Server) handleSavedEvolutionPathDelete(w http.ResponseWriter, r *http.Request) {
	backend, ok := s.Store.(store.SavedEvolutionPathStore)
	if !ok {
		http.Error(w, "paths salvos indisponÃ­veis neste armazenamento", http.StatusNotImplemented)
		return
	}
	if err := backend.DeleteSavedEvolutionPath(r.Context(), s.Cycle, r.PathValue("id")); err != nil {
		http.Error(w, "apagando path salvo: "+err.Error(), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func buildEvolutionPlayerAnalyses(snap store.Snapshot) []EvolutionPlayerAnalysis {
	rows := make([]EvolutionPlayerAnalysis, 0)
	for _, player := range snap.Club.Players {
		if player.Rating < evolutionAnalysisMinRating {
			continue
		}
		report, found := reportForEvolutionPlayer(snap.Cards, player)
		row := EvolutionPlayerAnalysis{Player: player, IdentityComplete: player.ClubItemID != "", Status: cards.EvolutionNotChecked, Paths: make([]EvolutionPathCandidate, 0)}
		if found {
			row.CardSlug = report.Slug
			row.Status = report.EvolutionStatus
			for _, path := range pathsForReport(report) {
				candidate := evolutionPathCandidate(snap.Cycle, report, player, path, snap.Club)
				row.Paths = append(row.Paths, candidate)
				if candidate.Potential.FinalGGRating > row.BestFinalGGRating {
					row.BestFinalGGRating = candidate.Potential.FinalGGRating
				}
				if candidate.Impact.Kind == "entra_no_xi" {
					row.EntraNoXI = true
					if candidate.Impact.Gain > row.BestXIGain {
						row.BestXIGain = candidate.Impact.Gain
					}
				}
			}
		}
		sort.SliceStable(row.Paths, func(i, j int) bool {
			if row.Paths[i].Potential.FinalGGRating != row.Paths[j].Potential.FinalGGRating {
				return row.Paths[i].Potential.FinalGGRating > row.Paths[j].Potential.FinalGGRating
			}
			return row.Paths[i].Potential.CoinsCost < row.Paths[j].Potential.CoinsCost
		})
		rows = append(rows, row)
	}
	return rows
}

func reportForEvolutionPlayer(reports []cards.CardReport, player domain.ClubPlayer) (cards.CardReport, bool) {
	if player.ClubItemID != "" {
		for _, report := range reports {
			if report.Player.ClubItemID == player.ClubItemID {
				return report, true
			}
		}
	}
	var matches []cards.CardReport
	for _, report := range reports {
		if report.Player.ID == player.ID {
			matches = append(matches, report)
		}
	}
	if len(matches) == 1 {
		return matches[0], true
	}
	return cards.CardReport{}, false
}

func pathsForReport(report cards.CardReport) []domain.EvolutionPath {
	if report.Graph != nil {
		return report.Graph.LinearPaths()
	}
	var paths []domain.EvolutionPath
	if report.Best != nil {
		paths = append(paths, report.Best.Path)
	}
	for _, alternate := range report.Alternates {
		paths = append(paths, alternate.Path)
	}
	return paths
}

func evolutionPathCandidate(cycle string, report cards.CardReport, player domain.ClubPlayer, path domain.EvolutionPath, club domain.Club) EvolutionPathCandidate {
	final := path.Final()
	gain := 0.0
	if player.GGRating > 0 && final.GGRating > 0 {
		gain = final.GGRating - player.GGRating
	}
	potential := cards.EvoPotential{Path: path, FinalOverall: final.Rating, FinalGGRating: final.GGRating, GGRatingGain: gain, GainedPlayStyles: path.GainedPlayStyles(), CoinsCost: path.CoinsCost, PointsCost: path.PointsCost, TrainingTime: path.TrainingTime}
	cardKey := evolutionCardKey(player, report.Slug)
	return EvolutionPathCandidate{ID: evolutionPathID(cycle, cardKey, path), Potential: potential, Impact: evolutionPathImpact(club, player, final), VersionHash: evolutionPathVersion(path)}
}

func evolutionPathImpact(club domain.Club, player domain.ClubPlayer, final domain.Player) EvolutionPathImpact {
	position, finalRating := final.GGRatingPos, final.GGRating
	if position == "" || finalRating <= 0 {
		return EvolutionPathImpact{Kind: "sem_comparacao"}
	}
	starters := starterRatings(club, position)
	if len(starters) == 0 {
		return EvolutionPathImpact{Kind: "sem_comparacao", Position: position, FinalGGRating: finalRating}
	}
	for _, starter := range starters {
		if samePhysicalClubCard(club, player, starter.Player) {
			if finalRating > starter.Rating {
				copy := starter.Player
				return EvolutionPathImpact{Kind: "melhora_titular", SlotIndex: starter.Index, Position: position, Starter: &copy, StarterGGRating: starter.Rating, FinalGGRating: finalRating, Gain: finalRating - starter.Rating}
			}
			return EvolutionPathImpact{Kind: "nao_supera", SlotIndex: starter.Index, Position: position, StarterGGRating: starter.Rating, FinalGGRating: finalRating}
		}
	}
	// Uma versÃ£o do mesmo atleta jÃ¡ no XI sÃ³ pode substituir aquela mesma
	// vaga; afirmar que ela ocupa outro slot criaria um XI ilegal em duplicata.
	for _, starter := range starters {
		if starter.Player.PlayerKey() == player.PlayerKey() {
			if finalRating > starter.Rating {
				copy := starter.Player
				return EvolutionPathImpact{Kind: "entra_no_xi", SlotIndex: starter.Index, Position: position, Starter: &copy, StarterGGRating: starter.Rating, FinalGGRating: finalRating, Gain: finalRating - starter.Rating}
			}
			return EvolutionPathImpact{Kind: "nao_supera", SlotIndex: starter.Index, Position: position, StarterGGRating: starter.Rating, FinalGGRating: finalRating}
		}
	}
	weakest := starters[0]
	for _, starter := range starters[1:] {
		if starter.Rating < weakest.Rating {
			weakest = starter
		}
	}
	if finalRating > weakest.Rating {
		copy := weakest.Player
		return EvolutionPathImpact{Kind: "entra_no_xi", SlotIndex: weakest.Index, Position: position, Starter: &copy, StarterGGRating: weakest.Rating, FinalGGRating: finalRating, Gain: finalRating - weakest.Rating}
	}
	return EvolutionPathImpact{Kind: "nao_supera", SlotIndex: weakest.Index, Position: position, StarterGGRating: weakest.Rating, FinalGGRating: finalRating}
}

type starterRating struct {
	Index  int
	Player domain.ClubPlayer
	Rating float64
}

func starterRatings(club domain.Club, position domain.Position) []starterRating {
	var out []starterRating
	for _, slot := range club.Squad.Starters {
		if slot.Position != position {
			continue
		}
		player, ok := clubPlayerForStarterSlot(club, slot)
		if !ok {
			continue
		}
		if rating, ok := player.GGRatingAt(position); ok {
			out = append(out, starterRating{Index: slot.Index, Player: player, Rating: rating})
		}
	}
	return out
}

func clubPlayerForStarterSlot(club domain.Club, slot domain.SquadSlot) (domain.ClubPlayer, bool) {
	var exact []domain.ClubPlayer
	var byID []domain.ClubPlayer
	for _, player := range club.Players {
		if player.ID != slot.PlayerID {
			continue
		}
		byID = append(byID, player)
		if player.InSquad && player.SquadSlot == slot.Position {
			exact = append(exact, player)
		}
	}
	if len(exact) == 1 {
		return exact[0], true
	}
	if len(byID) == 1 {
		return byID[0], true
	}
	return domain.ClubPlayer{}, false
}

func samePhysicalClubCard(club domain.Club, a, b domain.ClubPlayer) bool {
	if a.ClubItemID != "" && b.ClubItemID != "" {
		return a.ClubItemID == b.ClubItemID
	}
	if a.ID != b.ID {
		return false
	}
	count := 0
	for _, player := range club.Players {
		if player.ID == a.ID {
			count++
		}
	}
	return count == 1
}

func evolutionCardKey(player domain.ClubPlayer, slug string) string {
	if player.ClubItemID != "" {
		return "item:" + player.ClubItemID
	}
	return "slug:" + slug
}

func normalizedEvolutionChain(path domain.EvolutionPath) string {
	var parts []string
	for _, item := range path.Chain {
		for _, split := range strings.Split(item, "→") {
			if normalized := strings.Join(strings.Fields(strings.ToLower(strings.TrimSpace(split))), " "); normalized != "" {
				parts = append(parts, normalized)
			}
		}
	}
	return strings.Join(parts, "\x00")
}

func evolutionPathID(cycle, cardKey string, path domain.EvolutionPath) string {
	sum := sha256.Sum256([]byte(cycle + "\x00" + cardKey + "\x00" + normalizedEvolutionChain(path)))
	return "evo-path-" + hex.EncodeToString(sum[:10])
}

func evolutionPathVersion(path domain.EvolutionPath) string {
	type step struct {
		ID       int64           `json:"id"`
		Rating   int             `json:"rating"`
		GGRating float64         `json:"gg_rating"`
		Position domain.Position `json:"position"`
	}
	payload := struct {
		Chain    []string `json:"chain"`
		Coins    int      `json:"coins"`
		Points   int      `json:"points"`
		Expired  bool     `json:"expired"`
		Training string   `json:"training"`
		Steps    []step   `json:"steps"`
	}{Chain: path.Chain, Coins: path.CoinsCost, Points: path.PointsCost, Expired: path.IsExpired, Training: path.TrainingTime}
	for _, card := range path.Steps {
		payload.Steps = append(payload.Steps, step{ID: card.ID, Rating: card.Rating, GGRating: card.GGRating, Position: card.GGRatingPos})
	}
	encoded, _ := json.Marshal(payload)
	sum := sha256.Sum256(encoded)
	return hex.EncodeToString(sum[:])
}

func evolutionPathsSummary(rows []EvolutionPlayerAnalysis) EvolutionPathsSummary {
	summary := EvolutionPathsSummary{Players: len(rows)}
	for _, row := range rows {
		switch row.Status {
		case cards.EvolutionConfirmed:
			summary.Confirmed++
		case cards.EvolutionNoPath:
			summary.NoPath++
		case cards.EvolutionNotEligible:
			summary.NotEligible++
		case cards.EvolutionFetchError:
			summary.FetchError++
		default:
			summary.NotChecked++
		}
		if row.EntraNoXI {
			summary.EntraNoXI++
		}
		for _, path := range row.Paths {
			if path.Impact.Kind == "melhora_titular" {
				summary.MelhoraTitular++
				break
			}
		}
	}
	return summary
}

func markSavedEvolutionPaths(rows []EvolutionPlayerAnalysis, saved []domain.SavedEvolutionPath) {
	ids := map[string]bool{}
	for _, item := range saved {
		ids[item.PathID] = true
	}
	for i := range rows {
		for j := range rows[i].Paths {
			rows[i].Paths[j].Saved = ids[rows[i].Paths[j].ID]
		}
	}
}

func candidatesByID(rows []EvolutionPlayerAnalysis) map[string]EvolutionPathCandidate {
	result := map[string]EvolutionPathCandidate{}
	for _, row := range rows {
		for _, candidate := range row.Paths {
			result[candidate.ID] = candidate
		}
	}
	return result
}

func savedEvolutionPathFrom(row EvolutionPlayerAnalysis, candidate EvolutionPathCandidate, cycle string) domain.SavedEvolutionPath {
	impact := domain.SavedEvolutionImpact{Kind: candidate.Impact.Kind, SlotIndex: candidate.Impact.SlotIndex, Position: candidate.Impact.Position, StarterGGRating: candidate.Impact.StarterGGRating, FinalGGRating: candidate.Impact.FinalGGRating, Gain: candidate.Impact.Gain}
	if candidate.Impact.Starter != nil {
		impact.Starter = *candidate.Impact.Starter
	}
	return domain.SavedEvolutionPath{ID: candidate.ID, PathID: candidate.ID, Cycle: cycle, CardKey: evolutionCardKey(row.Player, row.CardSlug), CardSlug: row.CardSlug, IdentityComplete: row.IdentityComplete, Player: row.Player, Path: candidate.Potential.Path, CurrentGGRating: row.Player.GGRating, FinalOverall: candidate.Potential.FinalOverall, FinalGGRating: candidate.Potential.FinalGGRating, GGRatingGain: candidate.Potential.GGRatingGain, Impact: impact, VersionHash: candidate.VersionHash, SavedAt: time.Now()}
}

func savedPathStatus(saved domain.SavedEvolutionPath, current EvolutionPathCandidate) string {
	if current.ID == "" {
		return "indisponivel"
	}
	if current.Potential.Path.IsExpired {
		return "expirado"
	}
	if current.VersionHash != saved.VersionHash {
		return "alterado"
	}
	return "disponivel"
}
