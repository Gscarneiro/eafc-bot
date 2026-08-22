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
	"time"

	"github.com/gscarneiro/eafc-bot/internal/analyze"
	"github.com/gscarneiro/eafc-bot/internal/cards"
	"github.com/gscarneiro/eafc-bot/internal/domain"
	"github.com/gscarneiro/eafc-bot/internal/report"
	"github.com/gscarneiro/eafc-bot/internal/store"
)

// priceSeriesWindow é até onde os gráficos de preço por carta olham para
// trás — o mesmo teto que a retenção de snapshot usa, para o front nunca
// pedir mais histórico do que existe.
const priceSeriesWindow = 30 * 24 * time.Hour

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

	Trigger func()
	Status  func() JobStatus
}

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/status", s.handleStatus)
	mux.HandleFunc("GET /api/time", s.handleTime)
	mux.HandleFunc("GET /api/time/{slug}", s.handleTimeSlug)
	mux.HandleFunc("GET /api/mercado", s.handleMercado)
	mux.HandleFunc("GET /api/evolucoes", s.handleEvolucoes)
	mux.HandleFunc("GET /api/job", s.handleJobStatus)
	mux.HandleFunc("POST /api/job", s.handleJobTrigger)
	return mux
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
	Index    int             `json:"index"`
	Position domain.Position `json:"position"`
}

// TimeResponse é o elenco: os titulares na ordem do fut.gg, e o banco.
type TimeResponse struct {
	Formation string        `json:"formation"`
	Starters  []StarterCard `json:"starters"`
	Bench     []RosterCard  `json:"bench"`
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
			RosterCard: RosterCard{Player: c.Player, CardSlug: slugByID[c.Player.ID]},
			Index:      c.Index,
			Position:   c.Position,
		}
		inSquad[c.Player.ID] = true
	}

	var bench []RosterCard
	for _, p := range snap.Club.Players {
		if !inSquad[p.ID] {
			bench = append(bench, RosterCard{Player: p, CardSlug: slugByID[p.ID]})
		}
	}

	writeJSON(w, TimeResponse{
		Formation: snap.Club.Squad.Formation,
		Starters:  starters,
		Bench:     bench,
	})
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
	Upgrades    []analyze.Upgrade            `json:"upgrades"`
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

	writeJSON(w, MercadoResponse{Upgrades: snap.Upgrades, PriceSeries: series})
}

// EvolucoesResponse são as evoluções do dia que valem a pena NO seu elenco
// — distinto da análise carta-a-carta de /api/time/{slug} (ver CLAUDE.md).
type EvolucoesResponse struct {
	Matches []analyze.EvoMatch `json:"matches"`
}

func (s *Server) handleEvolucoes(w http.ResponseWriter, r *http.Request) {
	snap, ok := s.load(w, r)
	if !ok {
		return
	}
	writeJSON(w, EvolucoesResponse{Matches: snap.EvoMatches})
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
