// Package api serve o snapshot mais recente (guardado por store.Store) como
// JSON para a UI React — uma rota por tela, envelope fino em volta de
// domain.*, analyze.*, cards.CardReport e store.*, que já são tagueados.
// Nenhuma rota aqui bate na rede: quem coleta é o job em cmd/eafcbot,
// acionado só através de Trigger.
package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/gscarneiro/eafc-bot/internal/analyze"
	"github.com/gscarneiro/eafc-bot/internal/cards"
	"github.com/gscarneiro/eafc-bot/internal/config"
	"github.com/gscarneiro/eafc-bot/internal/domain"
	"github.com/gscarneiro/eafc-bot/internal/report"
	"github.com/gscarneiro/eafc-bot/internal/store"
)

// priceSeriesWindow é até onde os gráficos de preço por carta olham para
// trás — o mesmo teto que a retenção de snapshot usa, para o front nunca
// pedir mais histórico do que existe.
const priceSeriesWindow = 30 * 24 * time.Hour

const (
	benchMinimumRating   = 88
	benchDefaultPageSize = 24
	benchMaximumPageSize = 48
)

// JobStatus é o estado do job diário de coleta+análise, para a UI mostrar
// "rodando" / "última coleta às X" / "falhou às X" e o botão de atualizar
// saber quando reconsultar.
type JobStatus struct {
	Running bool `json:"running"`
	// LastStarted/LastSuccess são ponteiro de propósito: time.Time é
	// struct, e "omitempty" não reconhece struct zero-valor como vazio —
	// só assim o JSON de fato omite em vez de mandar "0001-01-01T...".
	LastStarted *time.Time `json:"last_started,omitempty"`
	LastSuccess *time.Time `json:"last_success,omitempty"`
	LastError   string     `json:"last_error,omitempty"`
}

// Server monta as rotas. Trigger e Status são as únicas pontes para fora
// deste pacote: quem executa o job de verdade (rede, futgg.Collect) mora em
// cmd/eafcbot — aqui só se aciona o gatilho e se lê o estado.
type Server struct {
	Store   store.Store
	Cycle   string
	History int // dias de histórico para o gráfico de tendência
	// EvolutionMinRating espelha serve.cards_min_rating. Zero mantém
	// compatibilidade com servidores de teste e snapshots antigos.
	EvolutionMinRating int
	// EvolutionExtraBudget espelha market.extra_budget para o selo de
	// disponibilidade usar o mesmo orçamento do job, sem reaproveitar a
	// estimativa de analyze.EvoMatch.
	EvolutionExtraBudget int

	Trigger func()
	Status  func() JobStatus
	Config  *ConfigEditor
}

// ConfigEditor expõe somente o subconjunto de preferências que a UI pode
// alterar. Update deve persistir e trocar a configuração em execução; a API
// não conhece o caminho do arquivo nem credenciais.
type ConfigEditor struct {
	Get             func() config.UISettings
	Update          func(config.UISettings) (config.UISettings, error)
	EnvLocked       []string
	GetFavorites    func() []string
	UpdateFavorites func([]string) error
}

type ConfigResponse struct {
	Settings  config.UISettings `json:"settings"`
	EnvLocked []string          `json:"env_locked"`
}

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/status", s.handleStatus)
	mux.HandleFunc("GET /api/time", s.handleTime)
	mux.HandleFunc("GET /api/time/{slug}", s.handleTimeSlug)
	mux.HandleFunc("GET /api/mercado", s.handleMercado)
	mux.HandleFunc("GET /api/evolucoes", s.handleEvolucoes)
	mux.HandleFunc("GET /api/investimentos", s.handleInvestimentos)
	mux.HandleFunc("GET /api/job", s.handleJobStatus)
	mux.HandleFunc("POST /api/job", s.handleJobTrigger)
	if s.Config != nil {
		mux.HandleFunc("GET /api/config", s.handleConfig)
		mux.HandleFunc("PUT /api/config", s.handleConfigUpdate)
		mux.HandleFunc("GET /api/evolucoes/favoritos", s.handleFavorites)
		mux.HandleFunc("PUT /api/evolucoes/favoritos", s.handleFavoritesUpdate)
	}
	return mux
}

type FavoritesResponse struct {
	Favorites []string `json:"favorites"`
}

func (s *Server) handleFavorites(w http.ResponseWriter, r *http.Request) {
	if s.Config == nil || s.Config.GetFavorites == nil {
		http.Error(w, "favoritos indisponíveis", http.StatusNotImplemented)
		return
	}
	writeJSON(w, FavoritesResponse{Favorites: s.Config.GetFavorites()})
}

func (s *Server) handleFavoritesUpdate(w http.ResponseWriter, r *http.Request) {
	if s.Config == nil || s.Config.UpdateFavorites == nil {
		http.Error(w, "favoritos indisponíveis", http.StatusNotImplemented)
		return
	}
	var in FavoritesResponse
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&in); err != nil {
		http.Error(w, fmt.Sprintf("lendo favoritos: %v", err), http.StatusBadRequest)
		return
	}
	if len(in.Favorites) > 500 {
		http.Error(w, "lista de favoritos grande demais", http.StatusRequestEntityTooLarge)
		return
	}
	if err := s.Config.UpdateFavorites(in.Favorites); err != nil {
		http.Error(w, err.Error(), http.StatusUnprocessableEntity)
		return
	}
	writeJSON(w, FavoritesResponse{Favorites: in.Favorites})
}

func (s *Server) handleConfig(w http.ResponseWriter, r *http.Request) {
	if s.Config == nil || s.Config.Get == nil {
		http.Error(w, "configuração editável indisponível", http.StatusNotImplemented)
		return
	}
	writeJSON(w, ConfigResponse{Settings: s.Config.Get(), EnvLocked: s.Config.EnvLocked})
}

func (s *Server) handleConfigUpdate(w http.ResponseWriter, r *http.Request) {
	if s.Config == nil || s.Config.Update == nil {
		http.Error(w, "configuração editável indisponível", http.StatusNotImplemented)
		return
	}
	// A UI sem autenticação é intencionalmente local. A checagem evita que uma
	// página de outra origem faça POST silencioso contra o bot; o deploy ainda
	// deve publicar a porta somente em loopback, como no docker-compose.
	if origin := r.Header.Get("Origin"); origin != "" {
		parsed, err := url.Parse(origin)
		if err != nil || parsed.Host == "" || parsed.Host != r.Host {
			http.Error(w, "origem não autorizada", http.StatusForbidden)
			return
		}
	}
	if r.ContentLength > 1<<20 {
		http.Error(w, "configuração grande demais", http.StatusRequestEntityTooLarge)
		return
	}
	var in config.UISettings
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20))
	if err := dec.Decode(&in); err != nil {
		http.Error(w, fmt.Sprintf("lendo configuração: %v", err), http.StatusBadRequest)
		return
	}
	out, err := s.Config.Update(in)
	if err != nil {
		http.Error(w, err.Error(), http.StatusUnprocessableEntity)
		return
	}
	writeJSON(w, ConfigResponse{Settings: out, EnvLocked: s.Config.EnvLocked})
}

// load busca o snapshot mais recente, já respondendo o erro HTTP certo
// quando não há nenhum ainda — estado normal na primeira subida, antes da
// primeira coleta terminar.
func (s *Server) load(w http.ResponseWriter, r *http.Request) (store.Snapshot, bool) {
	snap, ok, err := s.Store.LatestSnapshot(r.Context(), s.Cycle)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return store.Snapshot{}, false
	}
	if !ok {
		http.Error(w, "nenhum snapshot ainda — aguarde a primeira coleta ou dispare POST /api/job",
			http.StatusServiceUnavailable)
		return store.Snapshot{}, false
	}
	return snap, true
}

// TopMove é a melhor jogada disponível hoje — um upgrade de mercado ou uma
// evolução — para o herói do status abrir com uma recomendação de verdade
// em vez de só números. Nil quando não há upgrade dentro do orçamento nem
// evolução que valha a pena (analyze.FindUpgrades/FindEvolutions vazios).
type TopMove struct {
	Kind     string          `json:"kind"` // "upgrade" | "evolution"
	Slot     domain.Position `json:"slot"`
	Headline string          `json:"headline"` // "Saliba -> Bastoni no CB", pronto pra exibir
	Gain     float64         `json:"gain"`     // escala de analyze.Score() — comparável entre os dois kinds
	NetCost  int             `json:"net_cost"`
	Link     string          `json:"link"` // "/mercado" ou "/evolucoes", pra onde levar ao clicar
}

// bestMove escolhe entre o melhor upgrade e a melhor evolução do dia pela
// mesma escala de ganho (analyze.Score()) — as duas listas já chegam
// ordenadas (FindUpgrades por eficiência, FindEvolutions por BeatsStarter e
// ganho — ver CLAUDE.md sobre as duas notas do bot), então o primeiro item
// de cada uma já é o melhor candidato daquela fonte.
func bestMove(snap store.Snapshot) *TopMove {
	var best *TopMove
	if len(snap.Upgrades) > 0 {
		u := snap.Upgrades[0]
		best = &TopMove{
			Kind:     "upgrade",
			Slot:     u.Slot,
			Headline: fmt.Sprintf("%s -> %s no %s", u.Current.Display(), u.Candidate.Display(), u.Slot),
			Gain:     u.Gain,
			NetCost:  u.NetCost,
			Link:     "/mercado",
		}
	}
	if len(snap.EvoMatches) > 0 {
		m := snap.EvoMatches[0]
		if best == nil || m.Gain > best.Gain {
			best = &TopMove{
				Kind:     "evolution",
				Slot:     m.Slot,
				Headline: fmt.Sprintf("%s em %s", m.Evolution.Name, m.Player.Display()),
				Gain:     m.Gain,
				NetCost:  m.Cost,
				Link:     "/evolucoes",
			}
		}
	}
	return best
}

// StatusResponse é o status diário: saldo, nota do elenco, elo mais fraco,
// o que mudou desde ontem, a jogada recomendada, e a tendência dos últimos
// `History` dias.
type StatusResponse struct {
	GeneratedAt time.Time `json:"generated_at"`
	Cycle       string    `json:"cycle"`

	SquadScore      float64         `json:"squad_score"`
	Coins           int             `json:"coins"`
	Raisable        int             `json:"raisable"`
	WeakestSlot     domain.Position `json:"weakest_slot"`
	WeakestName     string          `json:"weakest_name"`
	WeakestGGRating float64         `json:"weakest_gg_rating"`
	TopMove         *TopMove        `json:"top_move,omitempty"`

	Diff       store.ClubDiff     `json:"diff"`
	NewCards   []domain.Player    `json:"new_cards"`
	News       []domain.NewsItem  `json:"news"`
	SBCs       []domain.SBC       `json:"sbcs"`
	Objectives []domain.Objective `json:"objectives"`
	Errors     []string           `json:"errors"`

	History []store.SnapshotSummary `json:"history"`
}

func (s *Server) handleStatus(w http.ResponseWriter, r *http.Request) {
	snap, ok := s.load(w, r)
	if !ok {
		return
	}

	_, raisable := snap.Club.Budget()
	avg, weakSlot, weakName, weakGG := report.SquadSummary(snap.Club)
	sbcs, objs := report.RankChallenges(snap.SBCs, snap.Objectives)

	hist, err := s.Store.SnapshotHistory(r.Context(), s.Cycle, s.History)
	if err != nil {
		hist = nil // o gráfico é enfeite, não motivo para a rota inteira falhar
	}

	writeJSON(w, StatusResponse{
		GeneratedAt:     snap.GeneratedAt,
		Cycle:           snap.Cycle,
		SquadScore:      avg,
		Coins:           snap.Club.Coins,
		Raisable:        raisable,
		WeakestSlot:     weakSlot,
		WeakestName:     weakName,
		WeakestGGRating: weakGG,
		TopMove:         bestMove(snap),
		Diff:            snap.Diff,
		NewCards:        snap.NewCards,
		News:            snap.FreshNews,
		SBCs:            sbcs,
		Objectives:      objs,
		Errors:          snap.Errors,
		History:         hist,
	})
}

// RosterCard é uma carta do elenco pronta pra tela de time — com o slug da
// página de detalhe JÁ RESOLVIDO (cross-referenciado contra snap.Cards),
// porque domain.Player.slug é o slug bruto do fut.gg, não necessariamente o
// mesmo que cards.CardReport.Slug usa depois de desambiguar (ver o
// comentário de assignSlugs em internal/cards/report.go). CardSlug vem
// vazio quando a carta está abaixo do cards_min_rating configurado — não
// tem análise de evolução, então não tem página de detalhe pra linkar.
type RosterCard struct {
	Player   domain.ClubPlayer `json:"player"`
	CardSlug string            `json:"card_slug,omitempty"`
}

// StarterCard acrescenta o slot físico — ver o comentário de
// report.SquadCard sobre por que isso não é a posição natural da carta.
type StarterCard struct {
	RosterCard
	Index            int             `json:"index"`
	Position         domain.Position `json:"position"`
	PositionGGRating float64         `json:"position_gg_rating,omitempty"`
}

// TimeResponse é o elenco: os titulares na ordem do fut.gg, e o banco.
type TimeResponse struct {
	Formation     string            `json:"formation"`
	Starters      []StarterCard     `json:"starters"`
	Bench         []RosterCard      `json:"bench"`
	BenchPage     int               `json:"bench_page"`
	BenchPageSize int               `json:"bench_page_size"`
	BenchTotal    int               `json:"bench_total"`
	Optimization  SquadOptimization `json:"optimization"`
}

type SquadOptimization struct {
	Status           string                 `json:"status"`
	Reason           string                 `json:"reason,omitempty"`
	CurrentAverage   float64                `json:"current_average"`
	SuggestedAverage float64                `json:"suggested_average"`
	Gain             float64                `json:"gain"`
	Moves            []SquadMoveView        `json:"moves"`
	Alternatives     []SquadAlternativeView `json:"alternatives"`
	ChemistryWarning string                 `json:"chemistry_warning"`
}
type SquadMoveView struct {
	Index             int             `json:"index"`
	Position          domain.Position `json:"position"`
	Current           StarterCard     `json:"current"`
	Suggested         StarterCard     `json:"suggested"`
	CurrentGGRating   float64         `json:"current_gg_rating"`
	SuggestedGGRating float64         `json:"suggested_gg_rating"`
	Gain              float64         `json:"gain"`
}
type SquadAlternativeView struct {
	Index    int             `json:"index"`
	Position domain.Position `json:"position"`
	Players  []StarterCard   `json:"players"`
}

func (s *Server) handleTime(w http.ResponseWriter, r *http.Request) {
	snap, ok := s.load(w, r)
	if !ok {
		return
	}

	slugByID := make(map[int64]string, len(snap.Cards))
	for _, c := range snap.Cards {
		slugByID[c.Player.ID] = c.Slug
	}

	main := report.MainSquad(snap.Club)
	starters := make([]StarterCard, len(main))
	inSquad := make(map[int64]bool, len(main))
	for i, c := range main {
		starters[i] = StarterCard{
			RosterCard:       RosterCard{Player: c.Player, CardSlug: slugByID[c.Player.ID]},
			Index:            c.Index,
			Position:         c.Position,
			PositionGGRating: func() float64 { v, _ := c.Player.GGRatingAt(c.Position); return v }(),
		}
		inSquad[c.Player.ID] = true
	}

	benchPlayers := filteredBench(snap.Club.Players, inSquad, r)
	page, pageSize := benchPage(r)
	total := len(benchPlayers)
	from := (page - 1) * pageSize
	if from > total {
		from = total
	}
	to := from + pageSize
	if to > total {
		to = total
	}
	bench := make([]RosterCard, 0, to-from)
	for _, p := range benchPlayers[from:to] {
		bench = append(bench, RosterCard{Player: p, CardSlug: slugByID[p.ID]})
	}

	opt := SquadOptimization{Status: snap.SquadPlan.Status, Reason: snap.SquadPlan.Reason, CurrentAverage: snap.SquadPlan.CurrentAverage, SuggestedAverage: snap.SquadPlan.SuggestedAverage, Gain: snap.SquadPlan.Gain, ChemistryWarning: "A química da escalação não é simulada; valide-a antes de aplicar."}
	toCard := func(a analyze.SquadAssignment) StarterCard {
		return StarterCard{RosterCard: RosterCard{Player: a.Player, CardSlug: slugByID[a.Player.ID]}, Index: a.Index, Position: a.Position, PositionGGRating: a.Rating}
	}
	for _, m := range snap.SquadPlan.Moves {
		cur := StarterCard{RosterCard: RosterCard{Player: m.Current, CardSlug: slugByID[m.Current.ID]}, Index: m.Index, Position: m.Position, PositionGGRating: m.CurrentRating}
		opt.Moves = append(opt.Moves, SquadMoveView{m.Index, m.Position, cur, toCard(analyze.SquadAssignment{Index: m.Index, Position: m.Position, Player: m.Suggested, Rating: m.SuggestedRating}), m.CurrentRating, m.SuggestedRating, m.Gain})
	}
	for _, a := range snap.SquadPlan.Alternatives {
		av := SquadAlternativeView{Index: a.Index, Position: a.Position}
		for _, p := range a.Players {
			av.Players = append(av.Players, toCard(p))
		}
		opt.Alternatives = append(opt.Alternatives, av)
	}
	formation := snap.Club.Squad.Formation
	if formation == "" {
		formation = inferFormation(snap.Club.Squad.Starters)
	}
	writeJSON(w, TimeResponse{
		Formation:     formation,
		Starters:      starters,
		Bench:         bench,
		BenchPage:     page,
		BenchPageSize: pageSize,
		BenchTotal:    total,
		Optimization:  opt,
	})
}

func benchPage(r *http.Request) (int, int) {
	page, _ := strconv.Atoi(r.URL.Query().Get("bench_page"))
	if page < 1 {
		page = 1
	}
	size, _ := strconv.Atoi(r.URL.Query().Get("bench_size"))
	if size <= 0 {
		size = benchDefaultPageSize
	}
	if size > benchMaximumPageSize {
		size = benchMaximumPageSize
	}
	return page, size
}

func filteredBench(players []domain.ClubPlayer, starters map[int64]bool, r *http.Request) []domain.ClubPlayer {
	q := r.URL.Query()
	search := strings.ToLower(strings.TrimSpace(q.Get("bench_search")))
	position := domain.Position(strings.ToUpper(strings.TrimSpace(q.Get("bench_position"))))
	tradeable := q.Get("bench_tradeable")
	out := make([]domain.ClubPlayer, 0)
	for _, p := range players {
		if starters[p.ID] || p.Rating < benchMinimumRating {
			continue
		}
		if search != "" && !strings.Contains(strings.ToLower(p.Display()), search) {
			continue
		}
		if position != "" && !p.PlaysAt(position) {
			continue
		}
		if tradeable == "tradeable" && p.Untradeable {
			continue
		}
		if tradeable == "untradeable" && !p.Untradeable {
			continue
		}
		out = append(out, p)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].GGRating != out[j].GGRating {
			return out[i].GGRating > out[j].GGRating
		}
		if out[i].Rating != out[j].Rating {
			return out[i].Rating > out[j].Rating
		}
		return out[i].Display() < out[j].Display()
	})
	return out
}

func inferFormation(slots []domain.SquadSlot) string {
	if len(slots) != 11 {
		return ""
	}
	copySlots := append([]domain.SquadSlot(nil), slots...)
	sort.Slice(copySlots, func(i, j int) bool { return copySlots[i].Index < copySlots[j].Index })
	want := []domain.Position{domain.GK, domain.RB, domain.CB, domain.CB, domain.LB, domain.RM, domain.CM, domain.CM, domain.LM, domain.CAM, domain.ST}
	for i, pos := range want {
		if copySlots[i].Position != pos {
			return ""
		}
	}
	return "4-4-1-1"
}

// CardDetailResponse é a análise "atual x potencial" de uma carta, mais a
// série de preço dela — cards.CardReport embutido promove seus campos
// tagueados direto para o JSON de saída.
type CardDetailResponse struct {
	cards.CardReport
	PriceSeries []store.PricePoint `json:"price_series"`
}

func (s *Server) handleTimeSlug(w http.ResponseWriter, r *http.Request) {
	snap, ok := s.load(w, r)
	if !ok {
		return
	}
	slug := r.PathValue("slug")

	for _, c := range snap.Cards {
		if c.Slug != slug {
			continue
		}
		var pts []store.PricePoint
		if series, err := s.Store.PriceSeries(r.Context(), s.Cycle, []int64{c.Player.ID}, priceSeriesWindow); err == nil {
			pts = series[c.Player.ID]
		}
		writeJSON(w, CardDetailResponse{CardReport: c, PriceSeries: pts})
		return
	}
	http.NotFound(w, r)
}

// MercadoResponse são as oportunidades de mercado — os upgrades já
// ordenados por ganho por moeda gasta (ver analyze.FindUpgrades), com a
// série de preço de cada alvo.
type MercadoResponse struct {
	Upgrades []analyze.Upgrade `json:"upgrades"`
	// Funnel explica uma lista de Upgrades vazia carta a carta — ver o
	// comentário de analyze.UpgradeFunnel. Considered==0 quando o snapshot
	// foi gravado antes deste campo existir; a tela cai no texto genérico
	// nesse caso.
	Funnel      analyze.UpgradeFunnel        `json:"funnel"`
	PriceSeries map[int64][]store.PricePoint `json:"price_series"`
}

func (s *Server) handleMercado(w http.ResponseWriter, r *http.Request) {
	snap, ok := s.load(w, r)
	if !ok {
		return
	}

	ids := make([]int64, 0, len(snap.Upgrades))
	for _, u := range snap.Upgrades {
		ids = append(ids, u.Candidate.ID)
	}
	series, err := s.Store.PriceSeries(r.Context(), s.Cycle, ids, priceSeriesWindow)
	if err != nil {
		series = nil
	}

	writeJSON(w, MercadoResponse{Upgrades: snap.Upgrades, Funnel: snap.MarketFunnel, PriceSeries: series})
}

// EvolucoesResponse são as evoluções do dia que valem a pena NO seu elenco
// — distinto da análise carta-a-carta de /api/time/{slug} (ver CLAUDE.md).
type EvoMatchView struct {
	Evolution       domain.Evolution     `json:"evolution"`
	Player          domain.ClubPlayer    `json:"player"`
	Slot            domain.Position      `json:"slot"`
	Result          domain.Player        `json:"result"`
	Cost            int                  `json:"cost"`
	Affordable      bool                 `json:"affordable"`
	Acquisition     string               `json:"acquisition"`
	CardSlug        string               `json:"card_slug,omitempty"`
	BeatsStarter    bool                 `json:"beats_starter"`
	Highlights      []string             `json:"highlights"`
	Impact          float64              `json:"impact"`
	CurrentGGRating float64              `json:"current_gg_rating,omitempty"`
	FinalGGRating   float64              `json:"final_gg_rating,omitempty"`
	BestPath        *cards.EvoPotential  `json:"best_path,omitempty"`
	Alternates      []cards.EvoPotential `json:"alternates,omitempty"`
}

// evoMatchView nasce exclusivamente do relatório carta-a-carta do fut.gg.
// analyze.EvoMatch não participa: o path confirma elegibilidade, carta final
// e GG Rating; o catálogo de evoluções só completa custo, prazo e objetivos.
func evoMatchView(report cards.CardReport, evolution domain.Evolution, club domain.Club, budget int) (EvoMatchView, bool) {
	potentials := matchingEvoPotentials(report, evolution.Name)
	if len(potentials) == 0 {
		return EvoMatchView{}, false
	}
	best := potentials[0]
	final := best.Path.Final()
	slot := final.GGRatingPos
	if slot == "" {
		slot = report.Player.GGRatingPos
	}
	if slot == "" {
		slot = final.Position
	}
	if slot == "" {
		slot = report.Player.Position
	}
	view := EvoMatchView{
		Evolution:       evolution,
		Player:          report.Player,
		Slot:            slot,
		Result:          final,
		Cost:            evolution.CoinCost,
		Affordable:      evolution.CoinCost <= budget,
		Acquisition:     analyze.EvolutionAcquisition(evolution),
		CardSlug:        report.Slug,
		Impact:          best.GGRatingGain,
		CurrentGGRating: report.Player.GGRating,
		FinalGGRating:   best.FinalGGRating,
		BestPath:        &best,
		Alternates:      potentials[1:],
	}
	if starter, ok := weakestStarterByGG(club, slot); ok {
		view.BeatsStarter = starter.ID != report.Player.ID && view.FinalGGRating > starter.GGRating
	}
	return view, true
}

func confirmedEvoViews(snap store.Snapshot, minRating, extraBudget int) []EvoMatchView {
	cash, raisable := snap.Club.Budget()
	budget := cash + raisable + extraBudget
	views := make([]EvoMatchView, 0)
	for _, report := range snap.Cards {
		if (minRating > 0 && report.Player.Rating < minRating) || !report.Player.Evolvable() {
			continue
		}
		seen := map[string]bool{}
		for _, evolution := range snap.Evolutions {
			key := strings.ToLower(strings.TrimSpace(evolution.ID + "\x00" + evolution.Name))
			if seen[key] {
				continue
			}
			view, ok := evoMatchView(report, evolution, snap.Club, budget)
			if !ok {
				continue
			}
			seen[key] = true
			views = append(views, view)
		}
	}
	return views
}

func weakestStarterByGG(club domain.Club, position domain.Position) (domain.ClubPlayer, bool) {
	var weakest domain.ClubPlayer
	found := false
	for _, slot := range club.Squad.Starters {
		if slot.Position != position {
			continue
		}
		player, ok := club.PlayerByID(slot.PlayerID)
		if !ok || player.GGRating <= 0 {
			continue
		}
		if !found || player.GGRating < weakest.GGRating {
			weakest, found = player, true
		}
	}
	return weakest, found
}

func matchingEvoPotentials(report cards.CardReport, evolutionName string) []cards.EvoPotential {
	if report.Player.GGRating <= 0 {
		return nil
	}
	all := make([]cards.EvoPotential, 0, 1+len(report.Alternates))
	if report.Best != nil {
		all = append(all, *report.Best)
	}
	all = append(all, report.Alternates...)
	matched := all[:0]
	for _, potential := range all {
		if potential.GGRatingGain <= 0 || potential.FinalGGRating <= report.Player.GGRating {
			continue
		}
		for _, name := range potential.Path.Chain {
			if strings.EqualFold(strings.TrimSpace(name), strings.TrimSpace(evolutionName)) {
				matched = append(matched, potential)
				break
			}
		}
	}
	sort.SliceStable(matched, func(i, j int) bool {
		if matched[i].FinalGGRating != matched[j].FinalGGRating {
			return matched[i].FinalGGRating > matched[j].FinalGGRating
		}
		return matched[i].CoinsCost < matched[j].CoinsCost
	})
	return matched
}

type evoSortCriterion struct {
	Field string
	Desc  bool
}

func parseEvoSort(raw string) []evoSortCriterion {
	if strings.TrimSpace(raw) == "" {
		raw = "impact:desc"
	}
	criteria := make([]evoSortCriterion, 0, 4)
	seen := map[string]bool{}
	for _, item := range strings.Split(raw, ",") {
		parts := strings.SplitN(strings.ToLower(strings.TrimSpace(item)), ":", 2)
		field := parts[0]
		if field == "gain" {
			field = "impact"
		}
		defaultDesc := field == "impact" || field == "result" || field == "final"
		desc := defaultDesc
		if len(parts) == 2 {
			desc = parts[1] == "desc"
		}
		switch field {
		case "impact", "result", "cost", "final", "player", "evolution", "acquisition":
		default:
			continue
		}
		if seen[field] {
			continue
		}
		seen[field] = true
		criteria = append(criteria, evoSortCriterion{Field: field, Desc: desc})
		if len(criteria) == 4 {
			break
		}
	}
	if len(criteria) == 0 {
		return []evoSortCriterion{{Field: "impact", Desc: true}}
	}
	return criteria
}

func compareEvoViews(a, b EvoMatchView, criterion evoSortCriterion) int {
	cmp := 0
	switch criterion.Field {
	case "impact":
		cmp = compareFloat(a.Impact, b.Impact)
	case "result":
		cmp = compareBool(a.BeatsStarter, b.BeatsStarter)
	case "cost":
		cmp = compareInt(a.Cost, b.Cost)
	case "final":
		aFinal, bFinal := a.Result.Rating, b.Result.Rating
		if a.BestPath != nil && a.BestPath.FinalOverall > 0 {
			aFinal = a.BestPath.FinalOverall
		}
		if b.BestPath != nil && b.BestPath.FinalOverall > 0 {
			bFinal = b.BestPath.FinalOverall
		}
		cmp = compareInt(aFinal, bFinal)
	case "player":
		cmp = strings.Compare(strings.ToLower(a.Player.Display()), strings.ToLower(b.Player.Display()))
	case "evolution":
		cmp = strings.Compare(strings.ToLower(a.Evolution.Name), strings.ToLower(b.Evolution.Name))
	case "acquisition":
		cmp = strings.Compare(strings.ToLower(a.Acquisition), strings.ToLower(b.Acquisition))
	}
	if criterion.Desc {
		return -cmp
	}
	return cmp
}

func compareFloat(a, b float64) int {
	switch {
	case a < b:
		return -1
	case a > b:
		return 1
	default:
		return 0
	}
}

func compareInt(a, b int) int {
	switch {
	case a < b:
		return -1
	case a > b:
		return 1
	default:
		return 0
	}
}

func compareBool(a, b bool) int {
	if a == b {
		return 0
	}
	if a {
		return 1
	}
	return -1
}

type EvolucoesSummary struct {
	Matches       int            `json:"matches"`
	Players       int            `json:"players"`
	Starters      int            `json:"starters"`
	Unaffordable  int            `json:"unaffordable"`
	ExpiringSoon  int            `json:"expiring_soon"`
	ByAcquisition map[string]int `json:"by_acquisition"`
}

type EvolucoesFilters struct {
	Positions  []string `json:"positions"`
	Categories []string `json:"categories"`
}

// EvolucoesResponse são as evoluções do dia com filtros e paginação no
// servidor. O resumo usa o conjunto filtrado, não apenas a página visível.
type EvolucoesResponse struct {
	Matches  []EvoMatchView   `json:"matches"`
	Total    int              `json:"total"`
	Page     int              `json:"page"`
	PageSize int              `json:"page_size"`
	Pages    int              `json:"pages"`
	Summary  EvolucoesSummary `json:"summary"`
	Filters  EvolucoesFilters `json:"filters"`
}

func (s *Server) handleEvolucoes(w http.ResponseWriter, r *http.Request) {
	snap, ok := s.load(w, r)
	if !ok {
		return
	}
	q := r.URL.Query()
	page := parsePositive(q.Get("page"), 1)
	pageSize := parsePositive(q.Get("page_size"), 20)
	if pageSize > 100 {
		pageSize = 100
	}
	position := strings.ToUpper(strings.TrimSpace(q.Get("position")))
	impact := strings.ToLower(strings.TrimSpace(q.Get("impact")))
	category := strings.ToLower(strings.TrimSpace(q.Get("category")))
	search := strings.ToLower(strings.TrimSpace(q.Get("q")))
	status := strings.ToLower(strings.TrimSpace(q.Get("status")))
	expiring := strings.EqualFold(q.Get("expiring"), "proxima")
	sortCriteria := parseEvoSort(q.Get("sort"))

	now := time.Now()
	filterPositions := map[string]bool{}
	filterCategories := map[string]bool{}
	confirmed := confirmedEvoViews(snap, s.EvolutionMinRating, s.EvolutionExtraBudget)
	for _, view := range confirmed {
		filterPositions[string(view.Slot)] = true
		filterCategories[view.Acquisition] = true
	}

	filtered := make([]EvoMatchView, 0, len(confirmed))
	for _, view := range confirmed {
		m := view
		if position != "" && position != "TODAS" && string(m.Slot) != position {
			continue
		}
		if category != "" && category != "todas" && m.Acquisition != category {
			continue
		}
		if (status == "fora_orcamento" && m.Affordable) || (status == "disponivel" && !m.Affordable) {
			continue
		}
		if expiring && (m.Evolution.ExpiresAt.IsZero() || m.Evolution.ExpiresAt.Before(now) || m.Evolution.ExpiresAt.After(now.Add(7*24*time.Hour))) {
			continue
		}
		if search != "" {
			name := strings.ToLower(m.Evolution.Name + " " + m.Player.Name + " " + m.Player.CommonName)
			if !strings.Contains(name, search) {
				continue
			}
		}
		if (impact == "titular" && !view.BeatsStarter) || (impact == "reserva" && view.BeatsStarter) {
			continue
		}
		filtered = append(filtered, view)
	}
	sort.SliceStable(filtered, func(i, j int) bool {
		a, b := filtered[i], filtered[j]
		for _, criterion := range sortCriteria {
			if cmp := compareEvoViews(a, b, criterion); cmp != 0 {
				return cmp < 0
			}
		}
		return false
	})

	summary := EvolucoesSummary{ByAcquisition: map[string]int{}}
	players := map[int64]bool{}
	for _, m := range filtered {
		summary.Matches++
		players[m.Player.ID] = true
		if m.BeatsStarter {
			summary.Starters++
		}
		if !m.Affordable {
			summary.Unaffordable++
		}
		if !m.Evolution.ExpiresAt.IsZero() && !m.Evolution.ExpiresAt.Before(now) && !m.Evolution.ExpiresAt.After(now.Add(7*24*time.Hour)) {
			summary.ExpiringSoon++
		}
		summary.ByAcquisition[m.Acquisition]++
	}
	summary.Players = len(players)
	total := len(filtered)
	pages := 0
	if total > 0 {
		pages = (total + pageSize - 1) / pageSize
	}
	if pages > 0 && page > pages {
		page = pages
	}
	start := (page - 1) * pageSize
	if start > total {
		start = total
	}
	end := start + pageSize
	if end > total {
		end = total
	}

	views := filtered[start:end]
	positions := make([]string, 0, len(filterPositions))
	for position := range filterPositions {
		positions = append(positions, position)
	}
	categories := make([]string, 0, len(filterCategories))
	for category := range filterCategories {
		categories = append(categories, category)
	}
	sort.Strings(positions)
	sort.Strings(categories)
	writeJSON(w, EvolucoesResponse{Matches: views, Total: total, Page: page, PageSize: pageSize, Pages: pages, Summary: summary, Filters: EvolucoesFilters{Positions: positions, Categories: categories}})
}

func parsePositive(value string, fallback int) int {
	var n int
	if _, err := fmt.Sscanf(value, "%d", &n); err != nil || n < 1 {
		return fallback
	}
	return n
}

// InvestimentosResponse é o agente de trading: cartas do mercado ganhando
// valor, o que fazer com o banco de reservas, e demanda de fodder de SBC
// esquentando — ver analyze.FindInvestments/FindSellCandidates/
// FindFodderDemand. Puramente consultivo, como o resto do bot.
//
// Diferente das outras rotas, calcula os três FRESCOS a cada request em
// vez de ler um campo pronto do snapshot: Investments depende do momentum
// mais recente (Store.LatestMomentum), que o ciclo de coleta rápido
// (scheduler.FastTicker) atualiza bem mais vezes por dia que o snapshot
// completo — ler um campo congelado no snapshot diário jogaria fora
// justamente o motivo de existir um ciclo rápido. O cálculo em si é
// barato (funções puras em cima do que já está no Store), então recalcular
// por request segue o mesmo padrão que report.SquadSummary/RankChallenges
// já usam.
type InvestimentosResponse struct {
	Investments      []analyze.Investment     `json:"investments"`
	InvestmentFunnel analyze.InvestmentFunnel `json:"investment_funnel"`
	SellCandidates   []analyze.SellCandidate  `json:"sell_candidates"`
	SellFunnel       analyze.SellFunnel       `json:"sell_funnel"`
	FodderDemand     []analyze.FodderSignal   `json:"fodder_demand"`
}

func (s *Server) handleInvestimentos(w http.ResponseWriter, r *http.Request) {
	snap, ok := s.load(w, r)
	if !ok {
		return
	}
	ctx := r.Context()

	// Momentum e tendência de custo de SBC são sinais OPCIONAIS: sem eles
	// (ciclo rápido ainda não rodou) a rota ainda responde, só sem esses
	// dois sinais — mesmo princípio de tolerância a falha parcial que
	// futgg.Collect já segue.
	momentum, err := s.Store.LatestMomentum(ctx, s.Cycle)
	if err != nil {
		momentum = nil
	}

	keys := make([]string, 0, len(snap.SBCs))
	for _, sbc := range snap.SBCs {
		for idx, ch := range sbc.Challenges {
			keys = append(keys, store.SBCChallengeKey(sbc.ID, idx, ch.Name))
		}
	}
	rawTrends, err := s.Store.SBCCostTrend(ctx, s.Cycle, keys, priceSeriesWindow)
	if err != nil {
		rawTrends = nil
	}
	costTrends := make(map[string]analyze.CostTrend, len(rawTrends))
	for k, t := range rawTrends {
		costTrends[k] = analyze.CostTrend{ChangePct: t.ChangePct, Samples: t.Samples}
	}

	investments, invFunnel := analyze.FindInvestments(snap.Club, momentum, snap.NewCards, analyze.DefaultInvestmentOptions())
	sellCandidates, sellFunnel := analyze.FindSellCandidates(snap.Club, snap.Cards, snap.SquadSwaps, analyze.DefaultSellOptions())
	fodderDemand := analyze.FindFodderDemand(snap.SBCs, snap.Market, costTrends, analyze.DefaultFodderDemandOptions())

	writeJSON(w, InvestimentosResponse{
		Investments: investments, InvestmentFunnel: invFunnel,
		SellCandidates: sellCandidates, SellFunnel: sellFunnel,
		FodderDemand: fodderDemand,
	})
}

func (s *Server) handleJobStatus(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, s.Status())
}

func (s *Server) handleJobTrigger(w http.ResponseWriter, r *http.Request) {
	s.Trigger()
	w.WriteHeader(http.StatusAccepted)
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_ = json.NewEncoder(w).Encode(v)
}
