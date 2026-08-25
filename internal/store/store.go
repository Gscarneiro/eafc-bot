// Package store guarda o histórico entre execuções. Duas implementações:
// JSON em disco (padrão, zero dependência) e Postgres (histórico de preços
// de verdade, com consulta).
package store

import (
	"context"
	"strconv"
	"time"

	"github.com/gscarneiro/eafc-bot/internal/analyze"
	"github.com/gscarneiro/eafc-bot/internal/cards"
	"github.com/gscarneiro/eafc-bot/internal/chemistry"
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

// SBCCostPoint é o equivalente de PricePoint para o custo da solução mais
// barata de um desafio de SBC — o fut.gg já resolve o fodder mais barato
// que bate o requisito, e é esse número que sobe quando a demanda esquenta
// (ver CLAUDE.md, "SBC repetível" e "pico é no lançamento, não na
// expiração").
type SBCCostPoint struct {
	Key        string    `json:"key"` // ver SBCChallengeKey
	Coins      int       `json:"coins"`
	ObservedAt time.Time `json:"observed_at"`
}

// SBCChallengeKey identifica um desafio de SBC de forma estável entre
// coletas — não existe id próprio de challenge no fut.gg, só o do SBC pai.
// SaveSBCCost usa pra gravar; quem consulta SBCCostTrend usa a mesma
// função pra perguntar pelo challenge certo. Nome quando disponível (mais
// legível no arquivo), índice como desempate/fallback pra nome vazio.
func SBCChallengeKey(sbcID string, idx int, name string) string {
	if name != "" {
		return sbcID + "#" + name
	}
	return sbcID + "#" + strconv.Itoa(idx)
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

	// ClubHistory devolve o retrato do clube de cada snapshot guardado, do
	// mais antigo para o mais novo — o oráculo de entrosamento
	// (Squad.Chemistry + ClubPlayer.Chemistry) que a calibração de
	// internal/chemistry replaya. Devolve só o clube, não o Snapshot
	// inteiro: um snapshot passa de 30 MB e a maior parte é mercado.
	//
	// Mesmo assim é CARO (abre cada arquivo do período) — é comando de CLI
	// (`eafcbot quimica -calibrar`), nunca handler de API.
	ClubHistory(ctx context.Context, cycle string, days int) ([]domain.Club, error)

	// PriceSeries devolve a série de preço observada de cada carta na janela
	// pedida — o mesmo histórico que Trends já lê, sem colapsar num resumo.
	PriceSeries(ctx context.Context, cycle string, eaIDs []int64, since time.Duration) (map[int64][]PricePoint, error)

	// SaveSBCCost grava o custo da solução mais barata de cada desafio de
	// SBC observado nesta rodada — mesmo molde de SavePrices, mas pro
	// custo que o fut.gg já resolve por challenge em vez de preço de carta.
	SaveSBCCost(ctx context.Context, cycle string, sbcs []domain.SBC) error

	// SBCCostTrend devolve a tendência de custo de cada challenge pedido
	// (ver SBCChallengeKey) na janela — reusa PriceTrend em vez de outro
	// formato; o campo EAID do resultado não se aplica aqui e fica zerado.
	SBCCostTrend(ctx context.Context, cycle string, keys []string, since time.Duration) (map[string]PriceTrend, error)

	// SaveMomentum grava o momentum mais recente lido do fut.gg. É um
	// CACHE do último valor, não série temporal — o fut.gg já é a série
	// (domain.Player.MomentumPct já vem pronto deles); aqui só existe pra
	// internal/api conseguir montar a tela de investimentos sem nunca
	// tocar a rede (CLAUDE.md), lendo o que o ciclo de coleta rápido
	// (scheduler.FastTicker, bem mais frequente que o snapshot diário) já
	// deixou salvo.
	SaveMomentum(ctx context.Context, cycle string, momentum []domain.Player) error

	// LatestMomentum devolve o último momentum salvo — vazio (não erro)
	// se o ciclo rápido ainda não rodou nenhuma vez (ex.: `run`/`demo`
	// chamados sem `serve` no ar).
	LatestMomentum(ctx context.Context, cycle string) ([]domain.Player, error)

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
	Capital    domain.Capital     `json:"capital"`
	Market     []domain.Player    `json:"market"`
	Evolutions []domain.Evolution `json:"evolutions"`
	Objectives []domain.Objective `json:"objectives"`
	SBCs       []domain.SBC       `json:"sbcs"`
	News       []domain.NewsItem  `json:"news"`
	Stats      futgg.Stats        `json:"stats"`
	Errors     []string           `json:"errors"`
	// Capabilities é a procedência por fonte desta coleta — ver
	// futgg.Observation. Mapa vazio (nil) é o sentinela de snapshot gravado
	// antes deste campo existir; /api/saude marca esse caso como "estimado"
	// em vez de fingir que sabe a procedência de um dado antigo.
	Capabilities map[string]futgg.Observation `json:"capabilities,omitempty"`

	// Diff, NewCards e FreshNews são derivados contra o snapshot anterior no
	// momento da gravação — não recalculados na leitura.
	Diff      ClubDiff          `json:"diff"`
	NewCards  []domain.Player   `json:"new_cards"`
	FreshNews []domain.NewsItem `json:"fresh_news"`

	Upgrades []analyze.Upgrade `json:"upgrades"`
	// MarketFunnel explica uma lista de Upgrades vazia carta a carta — ver o
	// comentário de analyze.UpgradeFunnel. Zero-valor (Considered == 0) é o
	// sentinela de snapshot gravado antes deste campo existir.
	MarketFunnel analyze.UpgradeFunnel `json:"market_funnel"`
	EvoMatches   []analyze.EvoMatch    `json:"evo_matches"`
	SquadSwaps   []analyze.SquadSwap   `json:"squad_swaps"`
	SquadPlan    analyze.SquadPlan     `json:"squad_plan"`
	Trends       map[int64]PriceTrend  `json:"trends"`
	SquadScore   float64               `json:"squad_score"`

	// Cards é a análise carta-a-carta (atual x potencial) do elenco acima do
	// min_rating configurado — o trabalho caro (~1,3 MB por carta contra o
	// fut.gg) que o scheduler paga uma vez à noite em vez de sob demanda.
	Cards []cards.CardReport `json:"cards"`

	// GauntletPlan é o planejamento das quatro rodadas do modo Gauntlet
	// (ver internal/analyze/gauntlet.go). Status vazio é o sentinela de
	// snapshot gravado antes deste campo existir — internal/api recompõe o
	// plano nesse caso, direto de Club, sem tocar rede (mesmo padrão de
	// MarketFunnel.Considered==0 acima).
	GauntletPlan analyze.GauntletPlan `json:"gauntlet_plan"`

	// Quimica é o entrosamento do XI ATIVO pelo modelo configurado, já
	// confrontado com o que o próprio jogo reportou (ver
	// internal/chemistry.Avaliar). Ponteiro nil é o sentinela de snapshot
	// gravado antes deste campo existir OU de clube sem escalação
	// sincronizada — internal/api recalcula direto de Club nesse caso, sem
	// tocar rede (mesmo padrão de GauntletPlan.Status=="" acima).
	Quimica *chemistry.Resultado `json:"chemistry,omitempty"`
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

// DiffClubs compara dois retratos do clube sem transformar o elenco numa
// tabela Player.ID -> carta. Player.ID identifica a carta publicada pela EA,
// não a cópia física que o usuário possui; por isso o índice guarda uma fila
// por chave e preserva duplicatas. Quando ClubItemID existe, ele é a chave
// preferida. Quando não existe, a comparação é uma multiconjunto por EA ID e
// não promete linhagem entre duas cópias iguais.
func DiffClubs(prev, cur domain.Club) ClubDiff {
	prevByKey, prevOrder := indexClubPlayers(prev.Players)
	curByKey, curOrder := indexClubPlayers(cur.Players)

	var d ClubDiff
	for _, key := range curOrder {
		current := curByKey[key]
		previous := prevByKey[key]
		if len(current) > len(previous) {
			d.Added = append(d.Added, current[len(previous):]...)
		}
	}
	for _, key := range prevOrder {
		previous := prevByKey[key]
		current := curByKey[key]
		if len(previous) > len(current) {
			d.Removed = append(d.Removed, previous[len(current):]...)
		}
	}
	d.CoinsDelta = cur.Coins - prev.Coins
	return d
}

func indexClubPlayers(players []domain.ClubPlayer) (map[string][]domain.ClubPlayer, []string) {
	byKey := make(map[string][]domain.ClubPlayer, len(players))
	order := make([]string, 0, len(players))
	seen := make(map[string]bool, len(players))
	for _, p := range players {
		key := "card:" + strconv.FormatInt(p.ID, 10)
		if p.ClubItemID != "" {
			key = "item:" + p.ClubItemID
		}
		if !seen[key] {
			seen[key] = true
			order = append(order, key)
		}
		byKey[key] = append(byKey[key], p)
	}
	return byKey, order
}
