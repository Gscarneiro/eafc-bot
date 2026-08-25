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
	"github.com/gscarneiro/eafc-bot/internal/chemistry"
	"github.com/gscarneiro/eafc-bot/internal/config"
	"github.com/gscarneiro/eafc-bot/internal/domain"
	"github.com/gscarneiro/eafc-bot/internal/futgg"
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
	// MarketReserve espelha market.reserve — moedas que nunca entram no
	// orçamento de compra recalculado por request (evoluções confirmadas,
	// status). O mesmo valor que cmd/eafcbot usa para o snapshot gravado.
	MarketReserve int
	// ChemistryModel espelha chemistry.model. Zero-valor (Nome=="") cai no
	// modelo padrão via resolveChemistryModel — mantém compatibilidade com
	// servidores de teste que não configuram isto.
	ChemistryModel chemistry.Modelo
	// CacheTTL evita reler snapshots grandes a cada chamada da UI. Zero mantém
	// o comportamento sem cache e é útil para testes que trocam o store entre
	// requisições.
	CacheTTL time.Duration
	cache    snapshotCache

	Trigger func()
	Status  func() JobStatus
	Config  *ConfigEditor
}

// resolveChemistryModel cai no modelo padrão quando o Server não configurou
// um (Nome vazio é o zero-valor de chemistry.Modelo, que não tem limiar
// nenhum preenchido — usar ele cru daria química sempre zero, não "padrão").
func (s *Server) resolveChemistryModel() chemistry.Modelo {
	if s.ChemistryModel.Nome == "" {
		return chemistry.ModeloPadrao()
	}
	return s.ChemistryModel
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
	mux.HandleFunc("GET /api/saude", s.handleSaude)
	mux.HandleFunc("GET /api/time", s.handleTime)
	mux.HandleFunc("GET /api/time/{slug}", s.handleTimeSlug)
	mux.HandleFunc("GET /api/gauntlet", s.handleGauntlet)
	mux.HandleFunc("GET /api/mercado", s.handleMercado)
	mux.HandleFunc("GET /api/evolucoes", s.handleEvolucoes)
	mux.HandleFunc("GET /api/investimentos", s.handleInvestimentos)
	mux.HandleFunc("GET /api/elenco/titulares", s.handleTitulares)
	mux.HandleFunc("GET /api/elenco/reservas", s.handleReservas)
	mux.HandleFunc("GET /api/capital/investimentos", s.handleCapitalInvestimentos)
	mux.HandleFunc("GET /api/capital/vendas", s.handleCapitalVendas)
	mux.HandleFunc("GET /api/capital/sbcs", s.handleCapitalSBCs)
	mux.HandleFunc("GET /api/hoje/novidades", s.handleHojeNovidades)
	mux.HandleFunc("GET /api/hoje/noticias", s.handleHojeNoticias)
	mux.HandleFunc("GET /api/hoje/sbcs", s.handleHojeSBCs)
	mux.HandleFunc("GET /api/hoje/objetivos", s.handleHojeObjetivos)
	mux.HandleFunc("GET /api/hoje/movimentacao", s.handleHojeMovimentacao)
	mux.HandleFunc("GET /api/historico", s.handleHistorico)
	mux.HandleFunc("GET /api/job", s.handleJobStatus)
	mux.HandleFunc("POST /api/job", s.guardLocalWrite(s.handleJobTrigger))
	mux.HandleFunc("POST /api/planos/elenco", s.guardLocalWrite(s.handleSquadPlan))
	if s.Config != nil {
		mux.HandleFunc("GET /api/config", s.handleConfig)
		mux.HandleFunc("PUT /api/config", s.guardLocalWrite(s.handleConfigUpdate))
		mux.HandleFunc("GET /api/evolucoes/favoritos", s.handleFavorites)
		mux.HandleFunc("PUT /api/evolucoes/favoritos", s.guardLocalWrite(s.handleFavoritesUpdate))
	}
	return mux
}

// maxLocalWriteBody é o teto de corpo para toda escrita local — folgado o
// bastante para a lista de favoritos ou o bloco de configuração, pequeno o
// bastante para não deixar um corpo absurdo consumir memória à toa.
const maxLocalWriteBody = 1 << 20

// guardLocalWrite centraliza a defesa das rotas de escrita (POST/PUT) desta
// API local: checagem de Origin (a UI sem autenticação é intencionalmente
// local — isto evita que uma página de outra origem dispare uma escrita
// silenciosa; o deploy ainda deve publicar a porta só em loopback, como no
// docker-compose) e o teto de corpo, num lugar só — para uma rota de escrita
// nova não esquecer de aplicar os dois, o que já aconteceu aqui (POST
// /api/job e PUT /api/evolucoes/favoritos tinham cada um sua própria
// cobertura parcial antes desta função existir).
func (s *Server) guardLocalWrite(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if origin := r.Header.Get("Origin"); origin != "" {
			parsed, err := url.Parse(origin)
			if err != nil || parsed.Host == "" || parsed.Host != r.Host {
				http.Error(w, "origem não autorizada", http.StatusForbidden)
				return
			}
		}
		r.Body = http.MaxBytesReader(w, r.Body, maxLocalWriteBody)
		next(w, r)
	}
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
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
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
	var in config.UISettings
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
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
	snap, ok, err := s.loadSnapshot(r.Context())
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
	Capital         domain.Capital  `json:"capital"`
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

	capital := snap.Capital
	if capital == (domain.Capital{}) {
		// Snapshots anteriores ao contrato de capital continuam legíveis.
		capital = snap.Club.Capital(s.EvolutionExtraBudget, s.MarketReserve, 0)
	}
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
		Raisable:        capital.NetRaisable,
		Capital:         capital,
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

// SaudeResponse expõe a procedência de cada capability desta coleta — fonte,
// horário, cobertura, avisos, erro e estado (ver futgg.Observation). É o que
// fecha o gate da fase 01: erro e incompletude não podem ficar escondidos
// dentro de um snapshot que "parece bom" só porque as outras telas leem os
// campos que já tinham dado certo.
type SaudeResponse struct {
	GeneratedAt  time.Time                    `json:"generated_at"`
	Cycle        string                       `json:"cycle"`
	Capabilities map[string]futgg.Observation `json:"capabilities"`
	// LegacySnapshot é true quando o snapshot foi gravado antes deste
	// contrato existir — Capabilities vem vazio nesse caso porque a coleta
	// não gravou o metadado, não porque ela foi perfeita.
	LegacySnapshot bool     `json:"legacy_snapshot"`
	Errors         []string `json:"errors"`
	// Healthy resume num booleano se dá para confiar no snapshot sem olhar
	// capability por capability: falso havendo erro de coleta, qualquer
	// capability fora de StatusConfirmado, ou snapshot anterior a este
	// contrato (procedência desconhecida não é o mesmo que procedência boa).
	Healthy bool `json:"healthy"`
}

func (s *Server) handleSaude(w http.ResponseWriter, r *http.Request) {
	snap, ok := s.load(w, r)
	if !ok {
		return
	}
	legacy := snap.Capabilities == nil
	healthy := !legacy && len(snap.Errors) == 0
	for _, obs := range snap.Capabilities {
		if obs.Status != futgg.StatusConfirmado {
			healthy = false
		}
	}
	writeJSON(w, SaudeResponse{
		GeneratedAt:    snap.GeneratedAt,
		Cycle:          snap.Cycle,
		Capabilities:   snap.Capabilities,
		LegacySnapshot: legacy,
		Errors:         snap.Errors,
		Healthy:        healthy,
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
	// Quimica é o entrosamento efetivo desta carta no XI ativo — nil quando
	// não há entrosamento calculado para o snapshot (ver
	// store.Snapshot.Quimica).
	Quimica *chemistry.Jogador `json:"chemistry,omitempty"`
}

// TimeResponse é o elenco: os titulares na ordem do fut.gg, e o banco.
type TimeResponse struct {
	Formation     string               `json:"formation"`
	Starters      []StarterCard        `json:"starters"`
	Bench         []RosterCard         `json:"bench"`
	BenchPage     int                  `json:"bench_page"`
	BenchPageSize int                  `json:"bench_page_size"`
	BenchTotal    int                  `json:"bench_total"`
	Optimization  SquadOptimization    `json:"optimization"`
	Quimica       *chemistry.Resultado `json:"chemistry,omitempty"`
}

type SquadOptimization struct {
	Status           string                 `json:"status"`
	Reason           string                 `json:"reason,omitempty"`
	CurrentAverage   float64                `json:"current_average"`
	SuggestedAverage float64                `json:"suggested_average"`
	Gain             float64                `json:"gain"`
	Moves            []SquadMoveView        `json:"moves"`
	Alternatives     []SquadAlternativeView `json:"alternatives"`
	// ChemistryNote explica o entrosamento da sugestão em texto pronto pra
	// tela — ver chemistryNote. Substitui o antigo aviso fixo "a química não
	// é simulada": agora ela É calculada, e o texto muda com o resultado.
	ChemistryNote string               `json:"chemistry_note"`
	Quimica       *chemistry.Resultado `json:"chemistry,omitempty"`
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

// currentChemistry devolve o entrosamento do XI ativo do snapshot. Usa o
// valor persistido quando existe (o normal); recalcula só para snapshot
// gravado antes deste campo existir (ponteiro nil) — mesmo padrão de
// snap.GauntletPlan.Status=="".
func (s *Server) currentChemistry(snap store.Snapshot) *chemistry.Resultado {
	if snap.Quimica != nil {
		return snap.Quimica
	}
	return chemistry.Avaliar(s.resolveChemistryModel(), snap.Club)
}

// chemistryByPlayer indexa um Resultado por carta, para popular
// StarterCard.Quimica sem busca linear por titular.
func chemistryByPlayer(res *chemistry.Resultado) map[int64]chemistry.Jogador {
	if res == nil {
		return nil
	}
	m := make(map[int64]chemistry.Jogador, len(res.Jogadores))
	for _, j := range res.Jogadores {
		m[j.PlayerID] = j
	}
	return m
}

// chemistryNote explica em texto pronto pra tela como a química da sugestão
// se compara com a do XI atual — três frases possíveis, na ordem em que
// fazem sentido pro usuário decidir se aplica a sugestão.
func chemistryNote(current, suggested *chemistry.Resultado) string {
	if current != nil && !current.Verificacao.Confiavel() {
		if current.Verificacao.Status == chemistry.StatusSemOraculo {
			return "O jogo ainda não confirmou o entrosamento desta coleta — mostrando só o calculado."
		}
		return fmt.Sprintf(
			"O modelo de química não confere com o jogo (calculado %d, o jogo reporta %d) — a sugestão foi feita só por GG Rating. Rode `eafcbot quimica -calibrar`.",
			current.Verificacao.Calculado, current.Verificacao.Observado)
	}
	if current == nil || suggested == nil {
		return ""
	}
	if suggested.Total == current.Total {
		return fmt.Sprintf("Química da sugestão: %d/%d, igual à atual.", suggested.Total, suggested.Maximo)
	}
	return fmt.Sprintf("Química: %d → %d (%s)", current.Total, suggested.Total, formatDeltaInt(suggested.Total-current.Total))
}

func formatDeltaInt(v int) string {
	if v >= 0 {
		return fmt.Sprintf("+%d", v)
	}
	return fmt.Sprintf("%d", v)
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

	quimicaAtual := s.currentChemistry(snap)
	quimicaPorCarta := chemistryByPlayer(quimicaAtual)

	main := report.MainSquad(snap.Club)
	starters := make([]StarterCard, len(main))
	inSquad := make(map[int64]bool, len(main))
	for i, c := range main {
		j, temQuimica := quimicaPorCarta[c.Player.ID]
		starters[i] = StarterCard{
			RosterCard:       RosterCard{Player: c.Player, CardSlug: slugByID[c.Player.ID]},
			Index:            c.Index,
			Position:         c.Position,
			PositionGGRating: func() float64 { v, _ := c.Player.GGRatingAt(c.Position); return v }(),
		}
		if temQuimica {
			starters[i].Quimica = &j
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

	// snap.SquadPlan.Quimica nil é o sentinela de snapshot gravado antes
	// deste campo existir (mesmo padrão de GauntletPlan.Status=="") —
	// recompõe o plano inteiro, sem tocar rede: OptimizeSquad é puro.
	squadPlan := snap.SquadPlan
	if squadPlan.Quimica == nil {
		squadPlan = analyze.OptimizeSquadWithOptions(snap.Club, analyze.SquadOptions{ChemistryModel: s.resolveChemistryModel()})
	}

	opt := SquadOptimization{
		Status: squadPlan.Status, Reason: squadPlan.Reason,
		CurrentAverage: squadPlan.CurrentAverage, SuggestedAverage: squadPlan.SuggestedAverage, Gain: squadPlan.Gain,
		ChemistryNote: chemistryNote(squadPlan.CurrentQuimica, squadPlan.Quimica),
		Quimica:       squadPlan.Quimica,
	}
	toCard := func(a analyze.SquadAssignment) StarterCard {
		return StarterCard{RosterCard: RosterCard{Player: a.Player, CardSlug: slugByID[a.Player.ID]}, Index: a.Index, Position: a.Position, PositionGGRating: a.Rating}
	}
	for _, m := range squadPlan.Moves {
		cur := StarterCard{RosterCard: RosterCard{Player: m.Current, CardSlug: slugByID[m.Current.ID]}, Index: m.Index, Position: m.Position, PositionGGRating: m.CurrentRating}
		opt.Moves = append(opt.Moves, SquadMoveView{m.Index, m.Position, cur, toCard(analyze.SquadAssignment{Index: m.Index, Position: m.Position, Player: m.Suggested, Rating: m.SuggestedRating}), m.CurrentRating, m.SuggestedRating, m.Gain})
	}
	for _, a := range squadPlan.Alternatives {
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
		Quimica:       quimicaAtual,
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

// gauntletRulesText explica a regra que motiva a tela: elenco inteiramente
// diferente, banco incluso, a cada rodada — para o formato padrão de 4 isso
// é a regra oficial documentada (ver EA FC 26 FUT Deep Dive, pitch notes);
// para 3 ou 5 (analyze.GauntletRules.Rodadas, pedido via query string) o
// texto se ajusta ao formato pedido, sem fingir que a EA documentou esse
// número específico. Não é dado do fut.gg — por isso mora aqui, no envelope
// da API, e não em internal/analyze junto do plano calculado.
func gauntletRulesText(rules analyze.GauntletRules) string {
	total := rules.Titulares + rules.Reservas
	msg := fmt.Sprintf(
		"O Gauntlet exige um elenco de %d jogadores (%d titulares + %d reservas) "+
			"inteiramente diferente a cada uma das %d partidas — nenhuma carta pode repetir, nem no banco.",
		total, rules.Titulares, rules.Reservas, rules.Rodadas)
	if rules.Rodadas == GauntletRoundsPadrao {
		msg += " Ver EA FC 26 FUT Deep Dive: https://www.ea.com/games/ea-sports-fc/fc-26/news/pitch-notes-fc26-fut-deep-dive"
	}
	return msg
}

// GauntletRoundsPadrao é o formato oficial documentado (EA FC 26 FUT Deep
// Dive) — só ele carrega a referência à fonte no texto de regras; 3 ou 5
// rodadas são formatos ALTERNATIVOS que este bot aceita calcular, não algo
// que a EA publicou.
const GauntletRoundsPadrao = analyze.GauntletRounds

// GauntletStarterView é um titular do Gauntlet pronto pra tela: a carta, o
// slot físico que ocupa naquela rodada, e os potenciais de evolução
// confirmados pelo fut.gg PARA AQUELA POSIÇÃO (ver gauntletPotentials) —
// omitido quando não há caminho confirmado, mesma convenção de
// CardReport.Best/CardDetailResponse.
type GauntletStarterView struct {
	Index      int                  `json:"index"`
	Position   domain.Position      `json:"position"`
	Player     domain.ClubPlayer    `json:"player"`
	Rating     float64              `json:"rating"`
	CardSlug   string               `json:"card_slug,omitempty"`
	Potentials []cards.EvoPotential `json:"potentials,omitempty"`
}

// GauntletRoundView é uma das 4 rodadas do Gauntlet, pronta pra tela.
type GauntletRoundView struct {
	Round         int                   `json:"round"`
	Starters      []GauntletStarterView `json:"starters"`
	Bench         []RosterCard          `json:"bench"`
	TotalRating   float64               `json:"total_rating"`
	AverageRating float64               `json:"average_rating"`
	Quimica       *chemistry.Resultado  `json:"chemistry,omitempty"`
}

// GauntletResponse é a tela inteira do Gauntlet: as 4 rodadas, mais o que
// motivou o plano (regras, avisos) e os objetivos ativos relacionados.
type GauntletResponse struct {
	GeneratedAt time.Time           `json:"generated_at"`
	Formation   string              `json:"formation"`
	Status      string              `json:"status"`
	Reason      string              `json:"reason,omitempty"`
	Rules       string              `json:"rules"`
	Strategy    string              `json:"strategy,omitempty"`
	Warnings    []string            `json:"warnings,omitempty"`
	Objectives  []domain.Objective  `json:"objectives"`
	Rounds      []GauntletRoundView `json:"rounds"`
}

// hasGauntletPlanQuery diz se a requisição pede um plano DIFERENTE do
// persistido no snapshot — estratégia, número de rodadas ou peso de
// química. Sem nenhum desses, a rota devolve o plano já calculado na
// coleta, sem tocar rede nem reprocessar o matching à toa.
func hasGauntletPlanQuery(r *http.Request) bool {
	q := r.URL.Query()
	for _, key := range []string{"strategy", "rodadas", "chemistry_weight"} {
		if q.Get(key) != "" {
			return true
		}
	}
	return false
}

// applyGauntletQuery sobrepõe req com os parâmetros pedidos — só um erro de
// PARSING (número inválido) vira 400; uma estratégia desconhecida ou um
// número de rodadas fora da faixa é decisão de domínio, e cai em
// plan.Status/Reason como qualquer outra inviabilidade (mesmo padrão do
// resto da API: erro de sintaxe é HTTP, erro de negócio é corpo de 200).
func applyGauntletQuery(req *analyze.GauntletRequest, r *http.Request) error {
	q := r.URL.Query()
	if v := q.Get("strategy"); v != "" {
		req.Strategy = analyze.GauntletStrategy(v)
	}
	if v := q.Get("rodadas"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil {
			return fmt.Errorf("rodadas inválido: %v", err)
		}
		req.Rules.Rodadas = n
	}
	if v := q.Get("chemistry_weight"); v != "" {
		peso, err := strconv.ParseFloat(v, 64)
		if err != nil {
			return fmt.Errorf("chemistry_weight inválido: %v", err)
		}
		req.ChemistryWeight = peso
	}
	return nil
}

func (s *Server) handleGauntlet(w http.ResponseWriter, r *http.Request) {
	snap, ok := s.load(w, r)
	if !ok {
		return
	}

	// Snapshot antigo gravado antes deste campo existir: Status vazio é o
	// sentinela (ver o comentário de store.Snapshot.GauntletPlan) — recompõe
	// direto do clube já carregado, sem tocar rede. O mesmo recompute vale
	// quando a requisição pede estratégia/rodadas/peso de química diferentes
	// do que a coleta calculou (ver hasGauntletPlanQuery) — /api/gauntlet
	// continua sendo a única rota, sem esperar POST /api/planos/elenco (fase
	// 02 do plano do copiloto, ainda não construída).
	plan := snap.GauntletPlan
	rules := analyze.DefaultGauntletRules()
	if plan.Status == "" || hasGauntletPlanQuery(r) {
		req := analyze.DefaultGauntletRequest()
		req.ChemistryModel = s.resolveChemistryModel()
		if err := applyGauntletQuery(&req, r); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		rules = req.Rules
		plan = analyze.BuildGauntletPlanFromRequest(snap.Club, req)
	}

	slugByID := make(map[int64]string, len(snap.Cards))
	cardByID := make(map[int64]cards.CardReport, len(snap.Cards))
	for _, c := range snap.Cards {
		slugByID[c.Player.ID] = c.Slug
		cardByID[c.Player.ID] = c
	}

	resp := GauntletResponse{
		GeneratedAt: snap.GeneratedAt,
		Formation:   plan.Formation,
		Status:      plan.Status,
		Reason:      plan.Reason,
		Rules:       gauntletRulesText(rules),
		Strategy:    plan.Strategy,
		Warnings:    plan.Warnings,
		Objectives:  gauntletObjectives(snap.Objectives),
		Rounds:      make([]GauntletRoundView, 0, len(plan.Rounds)),
	}
	for _, round := range plan.Rounds {
		rv := GauntletRoundView{
			Round:       round.Round,
			TotalRating: round.TotalRating, AverageRating: round.AverageRating,
			Quimica:  round.Quimica,
			Starters: make([]GauntletStarterView, 0, len(round.Starters)),
			Bench:    make([]RosterCard, 0, len(round.Bench)),
		}
		for _, a := range round.Starters {
			sv := GauntletStarterView{
				Index: a.Index, Position: a.Position, Player: a.Player, Rating: a.Rating,
				CardSlug: slugByID[a.Player.ID],
			}
			if report, ok := cardByID[a.Player.ID]; ok {
				sv.Potentials = gauntletPotentials(report, a.Position)
			}
			rv.Starters = append(rv.Starters, sv)
		}
		for _, b := range round.Bench {
			rv.Bench = append(rv.Bench, RosterCard{Player: b, CardSlug: slugByID[b.ID]})
		}
		resp.Rounds = append(resp.Rounds, rv)
	}
	writeJSON(w, resp)
}

// gauntletObjectives filtra os objetivos ativos cujo grupo, nome ou alguma
// tarefa mencione "Gauntlet" — pra tela abrir já sabendo se há objetivo
// ligado ao modo, sem o usuário caçar na lista geral de objetivos.
func gauntletObjectives(objs []domain.Objective) []domain.Objective {
	out := make([]domain.Objective, 0)
	for _, o := range objs {
		if containsFold(o.Group, "gauntlet") || containsFold(o.Name, "gauntlet") {
			out = append(out, o)
			continue
		}
		for _, task := range o.Tasks {
			if containsFold(task, "gauntlet") {
				out = append(out, o)
				break
			}
		}
	}
	return out
}

func containsFold(s, substr string) bool {
	return strings.Contains(strings.ToLower(s), strings.ToLower(substr))
}

// gauntletPotentials filtra os EvoPotential de uma carta aos que preservam a
// posição do SLOT que ela ocupa no Gauntlet (não a posição natural da carta
// — ver CLAUDE.md sobre SquadSlot) e reduz a lista a um representante por
// combinação de PlayStyle ganho, mantendo sempre a melhor nota final
// confirmada — evita mostrar vários caminhos quase idênticos que só mudam
// o PlayStyle "de brinde" por um GG Rating final um pouco maior ou menor.
func gauntletPotentials(report cards.CardReport, slotPosition domain.Position) []cards.EvoPotential {
	all := make([]cards.EvoPotential, 0, 1+len(report.Alternates))
	if report.Best != nil {
		all = append(all, *report.Best)
	}
	all = append(all, report.Alternates...)

	bestByStyles := map[string]cards.EvoPotential{}
	var order []string
	for _, potential := range all {
		if potential.Path.Final().GGRatingPos != slotPosition {
			continue
		}
		key := gainedPlayStyleKey(potential.GainedPlayStyles)
		current, seen := bestByStyles[key]
		if !seen {
			order = append(order, key)
			bestByStyles[key] = potential
			continue
		}
		if potential.FinalGGRating > current.FinalGGRating ||
			(potential.FinalGGRating == current.FinalGGRating && potential.CoinsCost < current.CoinsCost) {
			bestByStyles[key] = potential
		}
	}

	out := make([]cards.EvoPotential, 0, len(order))
	for _, key := range order {
		out = append(out, bestByStyles[key])
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].FinalGGRating > out[j].FinalGGRating })
	return out
}

func gainedPlayStyleKey(styles []domain.PlayStyle) string {
	names := make([]string, len(styles))
	for i, s := range styles {
		names[i] = s.String()
	}
	sort.Strings(names)
	return strings.Join(names, ",")
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

func (s *Server) handleMercadoLegacy(w http.ResponseWriter, r *http.Request) {
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

func confirmedEvoViews(snap store.Snapshot, minRating, extraBudget, reserve int) []EvoMatchView {
	budget := snap.Club.Capital(extraBudget, reserve, 0).Available
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

func (s *Server) handleEvolucoesLegacy(w http.ResponseWriter, r *http.Request) {
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
	confirmed := confirmedEvoViews(snap, s.EvolutionMinRating, s.EvolutionExtraBudget, s.MarketReserve)
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
