package api

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/gscarneiro/eafc-bot/internal/advisor"
	"github.com/gscarneiro/eafc-bot/internal/analyze"
	"github.com/gscarneiro/eafc-bot/internal/cards"
	"github.com/gscarneiro/eafc-bot/internal/domain"
	"github.com/gscarneiro/eafc-bot/internal/query"
	"github.com/gscarneiro/eafc-bot/internal/store"
)

const evolutionCatalogSource = "https://www.fut.gg/evolutions/"

// EvolutionCost separa as moedas dos pontos/tokens; custo não participa da
// categoria. Free é apenas um atalho de apresentação quando os três valores
// são zero.
type EvolutionCost struct {
	Coins  int  `json:"coins"`
	Points int  `json:"points"`
	Tokens int  `json:"tokens"`
	Free   bool `json:"free"`
}

type EvolutionCatalogItem struct {
	Evolution      domain.Evolution        `json:"evolution"`
	Category       string                  `json:"category"`
	CategoryLabel  string                  `json:"category_label"`
	CategorySource string                  `json:"category_source"`
	Origin         string                  `json:"origin"`
	OriginLabel    string                  `json:"origin_label"`
	Cost           EvolutionCost           `json:"cost"`
	EligibleCount  int                     `json:"eligible_count"`
	Eligible       bool                    `json:"eligible"`
	Expired        bool                    `json:"expired"`
	Repeatable     bool                    `json:"repeatable"`
	LabSubtype     string                  `json:"lab_subtype,omitempty"`
	Sources        []domain.AnalysisSource `json:"sources"`
	Warnings       []string                `json:"warnings,omitempty"`
}

type EvolutionCategoryCount struct {
	Key   string `json:"key"`
	Label string `json:"label"`
	Count int    `json:"count"`
}

type EvolutionCatalogSummary struct {
	Total      int                      `json:"total"`
	Eligible   int                      `json:"eligible"`
	Expired    int                      `json:"expired"`
	Categories []EvolutionCategoryCount `json:"categories"`
	Origins    map[string]int           `json:"origins"`
}

type evolutionCatalogResponse struct {
	query.Page[EvolutionCatalogItem]
	Summary EvolutionCatalogSummary `json:"@eafc.summary"`
	Source  string                  `json:"@eafc.source"`
}

type EvolutionEligiblePlayer struct {
	Key              string            `json:"key"`
	IdentityComplete bool              `json:"identity_complete"`
	Player           domain.ClubPlayer `json:"player"`
	CardSlug         string            `json:"card_slug,omitempty"`
	Eligible         bool              `json:"eligible"`
	Reasons          []string          `json:"reasons,omitempty"`
}

type EvolutionNumberChange struct {
	Key       string `json:"key"`
	Label     string `json:"label"`
	Group     string `json:"group"`
	Before    int    `json:"before"`
	After     int    `json:"after"`
	Delta     int    `json:"delta"`
	Available bool   `json:"available"`
	Capped    bool   `json:"capped,omitempty"`
}

type EvolutionPlayStyleChange struct {
	Name     string `json:"name"`
	Plus     bool   `json:"plus"`
	Status   string `json:"status"` // adicionado, elevado, mantido
	Existing bool   `json:"existing"`
}

type EvolutionProjection struct {
	Before          domain.Player              `json:"before"`
	After           domain.Player              `json:"after"`
	MainAttributes  []EvolutionNumberChange    `json:"main_attributes"`
	DetailedChanges []EvolutionNumberChange    `json:"detailed_attributes"`
	PlayStyles      []EvolutionPlayStyleChange `json:"playstyles"`
	PositionsAdded  []domain.Position          `json:"positions_added"`
	OverallDelta    int                        `json:"overall_delta"`
	Warnings        []string                   `json:"warnings,omitempty"`
}

type EvolutionPathEvidence struct {
	CardSlug      string                  `json:"card_slug,omitempty"`
	Confirmed     bool                    `json:"confirmed"`
	FinalOverall  int                     `json:"final_overall"`
	FinalGGRating float64                 `json:"final_gg_rating"`
	GGRatingGain  float64                 `json:"gg_rating_gain"`
	Path          domain.EvolutionPath    `json:"path"`
	SourceURLs    []domain.AnalysisSource `json:"source_urls"`
}

type EvolutionCatalogDetailResponse struct {
	Item              EvolutionCatalogItem       `json:"item"`
	Players           []EvolutionEligiblePlayer  `json:"players"`
	SelectedPlayerKey string                     `json:"selected_player_key,omitempty"`
	Projection        *EvolutionProjection       `json:"projection,omitempty"`
	Paths             []EvolutionPathEvidence    `json:"paths,omitempty"`
	Warnings          []string                   `json:"warnings,omitempty"`
	Sources           []domain.AnalysisSource    `json:"sources"`
	AgentEnabled      bool                       `json:"agent_enabled"`
	Analyses          []domain.EvolutionAnalysis `json:"analyses,omitempty"`
}

type EvolutionAnalysisRequest struct {
	PlayerKey string `json:"player_key"`
	Force     bool   `json:"force,omitempty"`
}

type EvolutionAnalysisResponse struct {
	Analysis domain.EvolutionAnalysis `json:"analysis"`
	Reused   bool                     `json:"reused,omitempty"`
}

type evolutionAgentPayload struct {
	ContractVersion string                  `json:"contract_version"`
	Evolution       domain.Evolution        `json:"evolution"`
	Player          domain.ClubPlayer       `json:"player"`
	Projection      EvolutionProjection     `json:"projection"`
	Sources         []domain.AnalysisSource `json:"sources"`
}

func (s *Server) handleEvolutionSubpath(w http.ResponseWriter, r *http.Request) {
	relative := strings.Trim(strings.TrimPrefix(r.URL.Path, "/api/evolucoes/"), "/")
	parts := strings.Split(relative, "/")
	if len(parts) == 2 && parts[1] == "plano" {
		r.SetPathValue("slug", parts[0])
		s.handleEvolucoesPlano(w, r)
		return
	}
	http.NotFound(w, r)
}

func evolutionCatalogSchema() query.Schema[EvolutionCatalogItem] {
	return query.NewSchema("evolucoes/catalogo", "expired asc,evolution/expires_at asc,evolution/name asc", 100,
		text("category", func(v EvolutionCatalogItem) string { return v.Category }, true, false),
		text("category_label", func(v EvolutionCatalogItem) string { return v.CategoryLabel }, true, false),
		text("origin", func(v EvolutionCatalogItem) string { return v.Origin }, true, false),
		text("origin_label", func(v EvolutionCatalogItem) string { return v.OriginLabel }, true, false),
		text("evolution/name", func(v EvolutionCatalogItem) string { return v.Evolution.Name }, false, true),
		text("evolution/slug", func(v EvolutionCatalogItem) string { return v.Evolution.Slug }, false, false),
		integer("evolution/coin_cost", func(v EvolutionCatalogItem) int { return v.Evolution.CoinCost }),
		integer("evolution/point_cost", func(v EvolutionCatalogItem) int { return v.Evolution.PointCost }),
		integer("evolution/token_cost", func(v EvolutionCatalogItem) int { return v.Evolution.TokenCost }),
		integer("eligible_count", func(v EvolutionCatalogItem) int { return v.EligibleCount }),
		boolean("eligible", func(v EvolutionCatalogItem) bool { return v.Eligible }, true),
		boolean("expired", func(v EvolutionCatalogItem) bool { return v.Expired }, true),
		boolean("repeatable", func(v EvolutionCatalogItem) bool { return v.Repeatable }, true),
		timeField("evolution/expires_at", func(v EvolutionCatalogItem) time.Time { return v.Evolution.ExpiresAt }),
	)
}

func (s *Server) handleEvolutionCatalog(w http.ResponseWriter, r *http.Request) {
	snap, ok := s.load(w, r)
	if !ok {
		return
	}
	items := buildEvolutionCatalog(snap)
	page, ok := serveList(w, r, evolutionCatalogSchema(), items)
	if !ok {
		return
	}
	writeJSON(w, evolutionCatalogResponse{Page: page, Summary: summarizeEvolutionCatalog(items), Source: evolutionCatalogSource})
}

func buildEvolutionCatalog(snap store.Snapshot) []EvolutionCatalogItem {
	items := make([]EvolutionCatalogItem, 0, len(snap.Evolutions))
	now := time.Now()
	for _, raw := range snap.Evolutions {
		evo := raw.ClassifyEvolution()
		classification := evo.Classification
		eligibleCount := 0
		for _, player := range snap.Club.Players {
			if player.Evolvable() && analyze.Eligible(player.Player, evo) {
				eligibleCount++
			}
		}
		expires := evo.ExpiresAt
		if expires.IsZero() {
			expires = evo.EndSubmissionAt
		}
		expired := !expires.IsZero() && expires.Before(now)
		cost := EvolutionCost{Coins: evo.CoinCost, Points: evo.PointCost, Tokens: evo.TokenCost}
		cost.Free = cost.Coins == 0 && cost.Points == 0 && cost.Tokens == 0
		sources := evolutionSources(evo)
		item := EvolutionCatalogItem{
			Evolution: evo, Category: classification.Category,
			CategoryLabel: classification.CategoryLabel, CategorySource: classification.CategorySource,
			Origin: classification.Origin, OriginLabel: classification.OriginLabel,
			Cost: cost, EligibleCount: eligibleCount, Eligible: eligibleCount > 0,
			Expired: expired, Repeatable: evo.Repeatable || evo.RepeatabilityCount > 1,
			Sources: sources, Warnings: append([]string(nil), classification.Warnings...),
		}
		if classification.Category == domain.EvolutionCategoryPlayStyles || classification.Category == domain.EvolutionCategoryPlayStylesPlus {
			item.LabSubtype = classification.Category
		}
		items = append(items, item)
	}
	sort.SliceStable(items, func(i, j int) bool {
		if items[i].Expired != items[j].Expired {
			return !items[i].Expired
		}
		return strings.ToLower(items[i].Evolution.Name) < strings.ToLower(items[j].Evolution.Name)
	})
	return items
}

func summarizeEvolutionCatalog(items []EvolutionCatalogItem) EvolutionCatalogSummary {
	summary := EvolutionCatalogSummary{Total: len(items), Origins: map[string]int{}}
	categoryMap := map[string]EvolutionCategoryCount{}
	for _, item := range items {
		if item.Eligible {
			summary.Eligible++
		}
		if item.Expired {
			summary.Expired++
		}
		key := item.Category
		entry := categoryMap[key]
		entry.Key, entry.Label, entry.Count = key, item.CategoryLabel, entry.Count+1
		categoryMap[key] = entry
		if item.Origin != "" {
			summary.Origins[item.Origin]++
		}
	}
	for _, entry := range categoryMap {
		summary.Categories = append(summary.Categories, entry)
	}
	sort.Slice(summary.Categories, func(i, j int) bool {
		if summary.Categories[i].Count != summary.Categories[j].Count {
			return summary.Categories[i].Count > summary.Categories[j].Count
		}
		return summary.Categories[i].Label < summary.Categories[j].Label
	})
	return summary
}

func evolutionSources(evo domain.Evolution) []domain.AnalysisSource {
	url := strings.TrimSpace(evo.URL)
	if url == "" {
		url = evolutionCatalogSource
	}
	return []domain.AnalysisSource{{Title: "Evolução no fut.gg", URL: url}}
}

func (s *Server) handleEvolutionCatalogDetail(w http.ResponseWriter, r *http.Request) {
	snap, ok := s.load(w, r)
	if !ok {
		return
	}
	evo, found := findEvolution(snap.Evolutions, r.PathValue("slug"))
	if !found {
		http.Error(w, "evolução não encontrada no snapshot atual", http.StatusNotFound)
		return
	}
	item := buildEvolutionCatalog(store.Snapshot{Evolutions: []domain.Evolution{evo}, Club: snap.Club})[0]
	players := evolutionPlayers(snap, evo)
	selectedKey := strings.TrimSpace(r.URL.Query().Get("player_key"))
	if selectedKey == "" {
		for _, player := range players {
			if player.Eligible {
				selectedKey = player.Key
				break
			}
		}
	}
	resp := EvolutionCatalogDetailResponse{
		Item: item, Players: players, SelectedPlayerKey: selectedKey,
		Sources: evolutionSources(evo), AgentEnabled: s.EvolutionAdvisor != nil,
	}
	if selected, ok := findEvolutionPlayer(players, selectedKey); ok && selected.Eligible {
		projection := projectEvolution(selected.Player.Player, evo)
		resp.Projection = &projection
		resp.Paths = evolutionPathEvidence(snap, selected, evo)
		if !selected.IdentityComplete {
			resp.Warnings = append(resp.Warnings, "A fonte não forneceu ClubItemID; a identidade desta cópia é aproximada pelo índice do snapshot.")
		}
		if analyses, err := s.listAnalyses(r.Context(), analysisHashFor(evo, selected.Player, projection)); err == nil {
			resp.Analyses = analyses
		}
	} else if selectedKey != "" {
		resp.Warnings = append(resp.Warnings, "Jogador selecionado não atende todos os requisitos da evolução.")
	}
	writeJSON(w, resp)
}

func findEvolution(evos []domain.Evolution, slug string) (domain.Evolution, bool) {
	want := strings.Trim(strings.ToLower(slug), "/")
	for _, raw := range evos {
		evo := raw.ClassifyEvolution()
		for _, candidate := range []string{evo.Slug, evo.ID, evo.Name} {
			if strings.Trim(strings.ToLower(candidate), "/") == want {
				return evo, true
			}
		}
	}
	return domain.Evolution{}, false
}

func evolutionPlayers(snap store.Snapshot, evo domain.Evolution) []EvolutionEligiblePlayer {
	players := make([]EvolutionEligiblePlayer, 0, len(snap.Club.Players))
	for index, player := range snap.Club.Players {
		key, complete := evolutionPlayerKey(player, index)
		row := EvolutionEligiblePlayer{Key: key, IdentityComplete: complete, Player: player, Eligible: player.Evolvable() && analyze.Eligible(player.Player, evo)}
		if slug := cardSlugFor(snap.Cards, player); slug != "" {
			row.CardSlug = slug
		}
		if player.EvoExhausted {
			row.Reasons = append(row.Reasons, "carta já tem uma evolução aplicada")
		}
		if !analyze.Eligible(player.Player, evo) {
			row.Reasons = append(row.Reasons, "não atende aos requisitos publicados")
		}
		if len(row.Reasons) == 0 && !row.Eligible {
			row.Reasons = append(row.Reasons, "carta não pode receber outra evolução")
		}
		players = append(players, row)
	}
	sort.SliceStable(players, func(i, j int) bool {
		if players[i].Eligible != players[j].Eligible {
			return players[i].Eligible
		}
		return strings.ToLower(players[i].Player.Display()) < strings.ToLower(players[j].Player.Display())
	})
	return players
}

func evolutionPlayerKey(player domain.ClubPlayer, index int) (string, bool) {
	if strings.TrimSpace(player.ClubItemID) != "" {
		return "item:" + player.ClubItemID, true
	}
	return "card:" + strconv.FormatInt(player.ID, 10) + ":" + strconv.Itoa(index), false
}

func findEvolutionPlayer(players []EvolutionEligiblePlayer, key string) (EvolutionEligiblePlayer, bool) {
	for _, player := range players {
		if player.Key == key {
			return player, true
		}
	}
	return EvolutionEligiblePlayer{}, false
}

func cardSlugFor(reports []cards.CardReport, player domain.ClubPlayer) string {
	for _, report := range reports {
		if report.Player.ClubItemID != "" && player.ClubItemID != "" && report.Player.ClubItemID == player.ClubItemID {
			return report.Slug
		}
	}
	for _, report := range reports {
		if report.Player.ID == player.ID {
			return report.Slug
		}
	}
	return ""
}

type detailedField struct {
	Key   string
	Label string
	Group string
	Get   func(*domain.DetailedAttributes) *int
}

func detailedFields() []detailedField {
	return []detailedField{
		{"acceleration", "Aceleração", "Ritmo", func(d *domain.DetailedAttributes) *int { return d.Acceleration }},
		{"sprint_speed", "Velocidade", "Ritmo", func(d *domain.DetailedAttributes) *int { return d.SprintSpeed }},
		{"agility", "Agilidade", "Físico", func(d *domain.DetailedAttributes) *int { return d.Agility }},
		{"balance", "Equilíbrio", "Físico", func(d *domain.DetailedAttributes) *int { return d.Balance }},
		{"jumping", "Impulsão", "Físico", func(d *domain.DetailedAttributes) *int { return d.Jumping }},
		{"stamina", "Fôlego", "Físico", func(d *domain.DetailedAttributes) *int { return d.Stamina }},
		{"strength", "Força", "Físico", func(d *domain.DetailedAttributes) *int { return d.Strength }},
		{"reactions", "Reações", "Físico", func(d *domain.DetailedAttributes) *int { return d.Reactions }},
		{"aggression", "Agressividade", "Defesa", func(d *domain.DetailedAttributes) *int { return d.Aggression }},
		{"composure", "Compostura", "Mental", func(d *domain.DetailedAttributes) *int { return d.Composure }},
		{"interceptions", "Interceptações", "Defesa", func(d *domain.DetailedAttributes) *int { return d.Interceptions }},
		{"positioning", "Posicionamento", "Mental", func(d *domain.DetailedAttributes) *int { return d.Positioning }},
		{"vision", "Visão", "Passe", func(d *domain.DetailedAttributes) *int { return d.Vision }},
		{"ball_control", "Controle de bola", "Drible", func(d *domain.DetailedAttributes) *int { return d.BallControl }},
		{"crossing", "Cruzamento", "Passe", func(d *domain.DetailedAttributes) *int { return d.Crossing }},
		{"dribbling", "Drible", "Drible", func(d *domain.DetailedAttributes) *int { return d.Dribbling }},
		{"finishing", "Finalização", "Chute", func(d *domain.DetailedAttributes) *int { return d.Finishing }},
		{"fk_accuracy", "Precisão de falta", "Chute", func(d *domain.DetailedAttributes) *int { return d.FKAccuracy }},
		{"heading_accuracy", "Precisão de cabeceio", "Chute", func(d *domain.DetailedAttributes) *int { return d.HeadingAccuracy }},
		{"long_passing", "Passe longo", "Passe", func(d *domain.DetailedAttributes) *int { return d.LongPassing }},
		{"short_passing", "Passe curto", "Passe", func(d *domain.DetailedAttributes) *int { return d.ShortPassing }},
		{"defensive_awareness", "Consciência defensiva", "Defesa", func(d *domain.DetailedAttributes) *int { return d.DefensiveAwareness }},
		{"shot_power", "Força do chute", "Chute", func(d *domain.DetailedAttributes) *int { return d.ShotPower }},
		{"long_shots", "Chutes de longe", "Chute", func(d *domain.DetailedAttributes) *int { return d.LongShots }},
		{"standing_tackle", "Desarme em pé", "Defesa", func(d *domain.DetailedAttributes) *int { return d.StandingTackle }},
		{"sliding_tackle", "Carrinho", "Defesa", func(d *domain.DetailedAttributes) *int { return d.SlidingTackle }},
		{"volleys", "Voleios", "Chute", func(d *domain.DetailedAttributes) *int { return d.Volleys }},
		{"curve", "Curva", "Passe", func(d *domain.DetailedAttributes) *int { return d.Curve }},
		{"penalties", "Pênaltis", "Chute", func(d *domain.DetailedAttributes) *int { return d.Penalties }},
		{"gk_diving", "Mergulho", "Goleiro", func(d *domain.DetailedAttributes) *int { return d.GKDiving }},
		{"gk_handling", "Defesa com as mãos", "Goleiro", func(d *domain.DetailedAttributes) *int { return d.GKHandling }},
		{"gk_kicking", "Reposição", "Goleiro", func(d *domain.DetailedAttributes) *int { return d.GKKicking }},
		{"gk_reflexes", "Reflexos", "Goleiro", func(d *domain.DetailedAttributes) *int { return d.GKReflexes }},
		{"gk_speed", "Velocidade GK", "Goleiro", func(d *domain.DetailedAttributes) *int { return d.GKSpeed }},
		{"gk_positioning", "Posicionamento GK", "Goleiro", func(d *domain.DetailedAttributes) *int { return d.GKPositioning }},
	}
}

func projectEvolution(before domain.Player, evo domain.Evolution) EvolutionProjection {
	after := evo.Apply(before)
	projection := EvolutionProjection{Before: before, After: after, OverallDelta: after.Rating - before.Rating}
	main := []struct {
		key, label    string
		before, after int
	}{
		{"pace", "Ritmo", before.Attributes.Pace, after.Attributes.Pace},
		{"shooting", "Chute", before.Attributes.Shooting, after.Attributes.Shooting},
		{"passing", "Passe", before.Attributes.Passing, after.Attributes.Passing},
		{"dribbling", "Drible", before.Attributes.Dribbling, after.Attributes.Dribbling},
		{"defending", "Defesa", before.Attributes.Defending, after.Attributes.Defending},
		{"physical", "Físico", before.Attributes.Physical, after.Attributes.Physical},
	}
	for _, item := range main {
		projection.MainAttributes = append(projection.MainAttributes, EvolutionNumberChange{
			Key: item.key, Label: item.label, Group: "face", Before: item.before, After: item.after,
			Delta: item.after - item.before, Available: item.before > 0 || item.after > 0,
		})
	}
	for _, field := range detailedFields() {
		var beforeValue, afterValue int
		beforeOK, afterOK := false, false
		if before.DetailedAttributes != nil {
			if value := field.Get(before.DetailedAttributes); value != nil {
				beforeValue, beforeOK = *value, true
			}
		}
		if after.DetailedAttributes != nil {
			if value := field.Get(after.DetailedAttributes); value != nil {
				afterValue, afterOK = *value, true
			}
		}
		projection.DetailedChanges = append(projection.DetailedChanges, EvolutionNumberChange{
			Key: field.Key, Label: field.Label, Group: field.Group,
			Before: beforeValue, After: afterValue, Delta: afterValue - beforeValue,
			Available: beforeOK && afterOK,
		})
	}
	missingDetailed := false
	for _, change := range projection.DetailedChanges {
		if !change.Available {
			missingDetailed = true
			break
		}
	}
	if missingDetailed && before.DetailedAttributes != nil {
		projection.Warnings = append(projection.Warnings, "Alguns subatributos não foram publicados para esta carta; as linhas indisponíveis ficam sem valor projetado.")
	}
	targetStyles := map[string]bool{}
	for _, up := range evo.TotalUpgrades() {
		if up.Kind == "playstyle" && strings.TrimSpace(up.PlayStyle.Name) != "" {
			targetStyles[strings.ToLower(strings.TrimSpace(up.PlayStyle.Name))] = true
		}
	}
	seenStyles := map[string]domain.PlayStyle{}
	for _, style := range before.PlayStyles {
		seenStyles[strings.ToLower(strings.TrimSpace(style.Name))] = style
	}
	for _, style := range after.PlayStyles {
		key := strings.ToLower(strings.TrimSpace(style.Name))
		if !targetStyles[key] {
			continue
		}
		old, exists := seenStyles[key]
		change := EvolutionPlayStyleChange{Name: style.Name, Plus: style.Plus, Existing: exists, Status: "adicionado"}
		if exists {
			change.Status = "mantido"
			if style.Plus && !old.Plus {
				change.Status = "elevado"
			}
			switch {
			case style.Plus && old.Plus:
				projection.Warnings = append(projection.Warnings, "A carta já possui "+style.Name+"+; o PlayStyle será mantido sem duplicata.")
			case style.Plus:
				projection.Warnings = append(projection.Warnings, "A carta já possui "+style.Name+"; a evolução eleva o PlayStyle para +.")
			default:
				projection.Warnings = append(projection.Warnings, "A carta já possui "+style.Name+"; a evolução não cria uma duplicata.")
			}
		}
		projection.PlayStyles = append(projection.PlayStyles, change)
	}
	beforePositions := map[domain.Position]bool{before.Position: true}
	for _, pos := range before.AltPositions {
		beforePositions[pos] = true
	}
	for _, pos := range after.AltPositions {
		if !beforePositions[pos] {
			projection.PositionsAdded = append(projection.PositionsAdded, pos)
		}
	}
	for _, up := range evo.TotalUpgrades() {
		if up.Kind == "unknown" {
			projection.Warnings = append(projection.Warnings, "Um upgrade da fonte não foi interpretado e não entrou na projeção: "+up.Raw)
		}
		if (up.Kind == "sub_attribute" || up.Kind == "ignored") && before.DetailedAttributes == nil {
			projection.Warnings = append(projection.Warnings, "A fonte não publicou os subatributos desta carta; o gráfico detalhado não inventa valores.")
			break
		}
	}
	return projection
}

func evolutionPathEvidence(snap store.Snapshot, player EvolutionEligiblePlayer, evo domain.Evolution) []EvolutionPathEvidence {
	var out []EvolutionPathEvidence
	for _, report := range snap.Cards {
		if !sameClubCard(report.Player, player.Player) {
			continue
		}
		potentials := make([]cards.EvoPotential, 0, 1+len(report.Alternates))
		if report.Best != nil {
			potentials = append(potentials, *report.Best)
		}
		potentials = append(potentials, report.Alternates...)
		for _, potential := range potentials {
			matched := false
			for _, name := range potential.Path.Chain {
				if strings.EqualFold(strings.TrimSpace(name), strings.TrimSpace(evo.Name)) || strings.EqualFold(strings.TrimSpace(name), strings.TrimSpace(evo.Slug)) {
					matched = true
					break
				}
			}
			if !matched {
				continue
			}
			out = append(out, EvolutionPathEvidence{
				CardSlug: report.Slug, Confirmed: report.EvolutionStatus == cards.EvolutionConfirmed,
				FinalOverall: potential.FinalOverall, FinalGGRating: potential.FinalGGRating,
				GGRatingGain: potential.GGRatingGain, Path: potential.Path,
				SourceURLs: evolutionSources(evo),
			})
		}
	}
	return out
}

func sameClubCard(report, selected domain.ClubPlayer) bool {
	if report.ClubItemID != "" && selected.ClubItemID != "" {
		return report.ClubItemID == selected.ClubItemID
	}
	return report.ID != 0 && report.ID == selected.ID
}

func analysisHashFor(evo domain.Evolution, player domain.ClubPlayer, projection EvolutionProjection) string {
	payload := evolutionAgentPayload{ContractVersion: advisor.ContractVersion, Evolution: evo, Player: player, Projection: projection, Sources: evolutionSources(evo)}
	b, _ := json.Marshal(payload)
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

func (s *Server) listAnalyses(ctx context.Context, inputHash string) ([]domain.EvolutionAnalysis, error) {
	var out []domain.EvolutionAnalysis
	s.analysisMu.Lock()
	for _, entry := range s.analysisJobs {
		if inputHash == "" || entry.InputHash == inputHash {
			out = append(out, *entry)
		}
	}
	s.analysisMu.Unlock()
	if backend, ok := s.Store.(store.EvolutionAnalysisStore); ok {
		persisted, err := backend.ListEvolutionAnalyses(ctx, s.Cycle, inputHash)
		if err != nil {
			return out, err
		}
		out = append(out, persisted...)
	}
	// IDs são únicos, mas a mesma entrada pode estar em memória e no disco.
	seen := map[string]bool{}
	unique := out[:0]
	for _, entry := range out {
		if seen[entry.ID] {
			continue
		}
		seen[entry.ID] = true
		unique = append(unique, entry)
	}
	sort.SliceStable(unique, func(i, j int) bool { return unique[i].UpdatedAt.After(unique[j].UpdatedAt) })
	return unique, nil
}

func (s *Server) handleEvolutionAnalysisCreate(w http.ResponseWriter, r *http.Request) {
	if s.EvolutionAdvisor == nil {
		http.Error(w, "agente indisponível — defina EAFC_EVO_AGENT_URL para habilitar o webhook", http.StatusServiceUnavailable)
		return
	}
	snap, ok := s.load(w, r)
	if !ok {
		return
	}
	var request EvolutionAnalysisRequest
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		http.Error(w, fmt.Sprintf("lendo pedido de análise: %v", err), http.StatusBadRequest)
		return
	}
	evo, found := findEvolution(snap.Evolutions, r.PathValue("slug"))
	if !found {
		http.Error(w, "evolução não encontrada no snapshot atual", http.StatusNotFound)
		return
	}
	players := evolutionPlayers(snap, evo)
	selected, found := findEvolutionPlayer(players, strings.TrimSpace(request.PlayerKey))
	if !found {
		http.Error(w, "player_key não pertence ao elenco deste snapshot", http.StatusUnprocessableEntity)
		return
	}
	if !selected.Eligible {
		http.Error(w, "o jogador escolhido não atende aos requisitos da evolução", http.StatusUnprocessableEntity)
		return
	}
	projection := projectEvolution(selected.Player.Player, evo)
	payload := evolutionAgentPayload{ContractVersion: advisor.ContractVersion, Evolution: evo, Player: selected.Player, Projection: projection, Sources: evolutionSources(evo)}
	body, err := json.Marshal(payload)
	if err != nil {
		http.Error(w, "não foi possível preparar o pedido do agente", http.StatusInternalServerError)
		return
	}
	sum := sha256.Sum256(body)
	inputHash := hex.EncodeToString(sum[:])
	if !request.Force {
		if entries, listErr := s.listAnalyses(r.Context(), inputHash); listErr == nil {
			for _, entry := range entries {
				if entry.Status == "completed" {
					writeJSON(w, EvolutionAnalysisResponse{Analysis: entry, Reused: true})
					return
				}
			}
		}
	}
	now := time.Now()
	entry := domain.EvolutionAnalysis{
		ID: newAnalysisID(inputHash), Cycle: s.Cycle, EvolutionID: evo.ID, EvolutionSlug: evo.Slug,
		PlayerKey: selected.Key, InputHash: inputHash, ContractVersion: advisor.ContractVersion,
		Status: "queued", CreatedAt: now, UpdatedAt: now,
	}
	s.putAnalysis(entry)
	if backend, ok := s.Store.(store.EvolutionAnalysisStore); ok {
		if err := backend.SaveEvolutionAnalysis(r.Context(), entry); err != nil {
			http.Error(w, "não foi possível guardar a fila de análise", http.StatusInternalServerError)
			return
		}
	}
	go s.runEvolutionAnalysis(entry, body)
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusAccepted)
	_ = json.NewEncoder(w).Encode(EvolutionAnalysisResponse{Analysis: entry})
}

func newAnalysisID(inputHash string) string {
	stamp := strconv.FormatInt(time.Now().UnixNano(), 10)
	if len(inputHash) > 12 {
		inputHash = inputHash[:12]
	}
	return "evo-analysis-" + stamp + "-" + inputHash
}

func (s *Server) putAnalysis(entry domain.EvolutionAnalysis) {
	s.analysisMu.Lock()
	if s.analysisJobs == nil {
		s.analysisJobs = map[string]*domain.EvolutionAnalysis{}
	}
	copy := entry
	s.analysisJobs[entry.ID] = &copy
	s.analysisMu.Unlock()
}

func (s *Server) runEvolutionAnalysis(entry domain.EvolutionAnalysis, payload []byte) {
	entry.Status = "running"
	entry.UpdatedAt = time.Now()
	s.putAnalysis(entry)
	s.persistAnalysis(entry)
	ctx, cancel := context.WithTimeout(context.Background(), 95*time.Second)
	defer cancel()
	result, err := s.EvolutionAdvisor.Analyze(ctx, payload)
	if err != nil {
		entry.Status, entry.Error, entry.UpdatedAt = "failed", err.Error(), time.Now()
		s.putAnalysis(entry)
		s.persistAnalysis(entry)
		return
	}
	if err := advisor.ValidateResult(result); err != nil {
		entry.Status, entry.Error, entry.UpdatedAt = "failed", err.Error(), time.Now()
		s.putAnalysis(entry)
		s.persistAnalysis(entry)
		return
	}
	entry.Status, entry.UpdatedAt = "completed", time.Now()
	entry.Verdict, entry.Summary, entry.Strengths, entry.Risks, entry.BestPositions = result.Verdict, result.Summary, append([]string(nil), result.Strengths...), append([]string(nil), result.Risks...), append([]string(nil), result.BestPositions...)
	entry.Sources = make([]domain.AnalysisSource, 0, len(result.Sources))
	for _, source := range result.Sources {
		entry.Sources = append(entry.Sources, domain.AnalysisSource{Title: source.Title, URL: source.URL})
	}
	s.putAnalysis(entry)
	s.persistAnalysis(entry)
}

func (s *Server) persistAnalysis(entry domain.EvolutionAnalysis) {
	if backend, ok := s.Store.(store.EvolutionAnalysisStore); ok {
		_ = backend.SaveEvolutionAnalysis(context.Background(), entry)
	}
}

func (s *Server) handleEvolutionAnalysis(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimSpace(r.PathValue("id"))
	s.analysisMu.Lock()
	var entry *domain.EvolutionAnalysis
	if s.analysisJobs != nil {
		if got := s.analysisJobs[id]; got != nil {
			copy := *got
			entry = &copy
		}
	}
	s.analysisMu.Unlock()
	if entry == nil {
		if backend, ok := s.Store.(store.EvolutionAnalysisStore); ok {
			entries, err := backend.ListEvolutionAnalyses(r.Context(), s.Cycle, "")
			if err == nil {
				for _, candidate := range entries {
					if candidate.ID == id {
						entry = &candidate
						break
					}
				}
			}
		}
	}
	if entry == nil {
		http.Error(w, "análise não encontrada", http.StatusNotFound)
		return
	}
	writeJSON(w, EvolutionAnalysisResponse{Analysis: *entry})
}
