// Package store guarda o histórico entre execuções. Duas implementações:
// JSON em disco (padrão, zero dependência) e Postgres (histórico de preços
// de verdade, com consulta).
package store

import (
	"context"
	"time"

	"github.com/gscarneiro/eafc-bot/internal/analyze"
	"github.com/gscarneiro/eafc-bot/internal/cards"
	"github.com/gscarneiro/eafc-bot/internal/domain"
	"github.com/gscarneiro/eafc-bot/internal/futgg"
)

// PricePoint é uma cotação observada num instante.
type PricePoint struct {
	EAID       int64     `json:"ea_id"`
	Coins      int       `json:"coins"`
	Extinct    bool      `json:"extinct"`
	ObservedAt time.Time `json:"observed_at"`
}

// PriceTrend resume o movimento de uma carta na janela consultada.
type PriceTrend struct {
	EAID      int64   `json:"ea_id"`
	First     int     `json:"first"`
	Last      int     `json:"last"`
	Min       int     `json:"min"`
	Max       int     `json:"max"`
	ChangePct float64 `json:"change_pct"`
	Samples   int     `json:"samples"`
}

// Direction traduz a tendência para o relatório.
func (t PriceTrend) Direction() string {
	switch {
	case t.ChangePct >= 8:
		return "subindo forte"
	case t.ChangePct >= 3:
		return "subindo"
	case t.ChangePct <= -8:
		return "caindo forte"
	case t.ChangePct <= -3:
		return "caindo"
	default:
		return "estável"
	}
}

// Store é o contrato que o resto do bot enxerga. Trocar JSON por Postgres
// não muda uma linha do motor de análise nem do relatório.
type Store interface {
	// SavePrices grava as cotações observadas nesta rodada.
	SavePrices(ctx context.Context, cycle string, players []domain.Player) error

	// Trends devolve a tendência de cada carta na janela pedida.
	Trends(ctx context.Context, cycle string, eaIDs []int64, since time.Duration) (map[int64]PriceTrend, error)

	// SaveClub guarda o retrato do clube.
	SaveClub(ctx context.Context, club domain.Club) error

	// PreviousClub devolve o retrato anterior, para o diff do elenco.
	PreviousClub(ctx context.Context, gamerTag, cycle string) (domain.Club, bool, error)

	// NewPlayers devolve as cartas vistas pela primeira vez agora — a
	// resposta para "saiu carta nova hoje?".
	NewPlayers(ctx context.Context, cycle string, players []domain.Player) ([]domain.Player, error)

	// UnseenNews filtra as notícias que o bot ainda não reportou.
	UnseenNews(ctx context.Context, cycle string, news []domain.NewsItem) ([]domain.NewsItem, error)

	// SaveSnapshot grava o resultado completo da coleta+análise do dia — o
	// dado que a API e a UI leem sem recalcular. O chamador nunca grava um
	// clube vazio por cima de um snapshot bom (ver a trava em
	// cmd/eafcbot/main.go:analyzeAndBuild); esta chamada em si sempre grava
	// o que recebe.
	SaveSnapshot(ctx context.Context, snap Snapshot) error

	// LatestSnapshot devolve o snapshot mais recente do ciclo.
	LatestSnapshot(ctx context.Context, cycle string) (Snapshot, bool, error)

	// SnapshotHistory devolve um resumo por dia dos últimos `days` dias, do
	// mais antigo para o mais novo — o que alimenta o gráfico de tendência
	// do status diário sem carregar o snapshot inteiro de cada dia.
	SnapshotHistory(ctx context.Context, cycle string, days int) ([]SnapshotSummary, error)

	// PriceSeries devolve a série de preço observada de cada carta na janela
	// pedida — o mesmo histórico que Trends já lê, sem colapsar num resumo.
	PriceSeries(ctx context.Context, cycle string, eaIDs []int64, since time.Duration) (map[int64][]PricePoint, error)

	Close() error
}

// Snapshot é o resultado completo de uma coleta+análise: os dados brutos que
// futgg.Collect trouxe, mais tudo que internal/analyze decidiu em cima
// deles. É deliberadamente PLANO em vez de embutir futgg.Snapshot ou
// report.Data — este pacote não pode importar internal/report (report já
// importa store, para PriceTrend), então quem quiser a visão "pronta pro
// briefing" (nota do elenco, XI titular, tabelas cortadas) chama
// report.Build com estes mesmos campos, em vez deste tipo aprender a
// calcular isso de novo.
type Snapshot struct {
	GeneratedAt time.Time     `json:"generated_at"`
	Duration    time.Duration `json:"duration"`
	Cycle       string        `json:"cycle"`

	Club       domain.Club        `json:"club"`
	Market     []domain.Player    `json:"market"`
	Evolutions []domain.Evolution `json:"evolutions"`
	Objectives []domain.Objective `json:"objectives"`
	SBCs       []domain.SBC       `json:"sbcs"`
	News       []domain.NewsItem  `json:"news"`
	Stats      futgg.Stats        `json:"stats"`
	Errors     []string           `json:"errors"`

	// Diff, NewCards e FreshNews são derivados contra o snapshot anterior no
	// momento da gravação — não recalculados na leitura.
	Diff      ClubDiff          `json:"diff"`
	NewCards  []domain.Player   `json:"new_cards"`
	FreshNews []domain.NewsItem `json:"fresh_news"`

	Upgrades   []analyze.Upgrade    `json:"upgrades"`
	EvoMatches []analyze.EvoMatch   `json:"evo_matches"`
	SquadSwaps []analyze.SquadSwap  `json:"squad_swaps"`
	Trends     map[int64]PriceTrend `json:"trends"`
	SquadScore float64              `json:"squad_score"`

	// Cards é a análise carta-a-carta (atual x potencial) do elenco acima do
	// min_rating configurado — o trabalho caro (~1,3 MB por carta contra o
	// fut.gg) que o scheduler paga uma vez à noite em vez de sob demanda.
	Cards []cards.CardReport `json:"cards"`
}

// SnapshotSummary é o ponto leve de um dia, para o gráfico de tendência —
// sem o Market nem o Cards do dia, que são o peso do Snapshot inteiro.
type SnapshotSummary struct {
	Date       string  `json:"date"` // "2006-01-02"
	SquadScore float64 `json:"squad_score"`
	Coins      int     `json:"coins"`
}

// ClubDiff é o que mudou no elenco entre duas execuções.
type ClubDiff struct {
	Added      []domain.ClubPlayer `json:"added"`
	Removed    []domain.ClubPlayer `json:"removed"`
	CoinsDelta int                 `json:"coins_delta"`
}

// DiffClubs compara dois retratos do clube.
func DiffClubs(prev, cur domain.Club) ClubDiff {
	prevIDs := make(map[int64]domain.ClubPlayer, len(prev.Players))
	for _, p := range prev.Players {
		prevIDs[p.ID] = p
	}
	curIDs := make(map[int64]domain.ClubPlayer, len(cur.Players))
	for _, p := range cur.Players {
		curIDs[p.ID] = p
	}

	var d ClubDiff
	for id, p := range curIDs {
		if _, had := prevIDs[id]; !had {
			d.Added = append(d.Added, p)
		}
	}
	for id, p := range prevIDs {
		if _, still := curIDs[id]; !still {
			d.Removed = append(d.Removed, p)
		}
	}
	d.CoinsDelta = cur.Coins - prev.Coins
	return d
}
