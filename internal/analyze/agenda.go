package analyze

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/gscarneiro/eafc-bot/internal/domain"
)

type FaixaAgenda string

const (
	AgendaAgora      FaixaAgenda = "agora"
	AgendaSemana     FaixaAgenda = "esta_semana"
	AgendaObservando FaixaAgenda = "observando"
)

// AcaoAgenda e a unidade de leitura da semana: possui identificador estavel
// para feedback futuro e aponta de onde veio, sem executar ou recalcular a
// recomendacao de origem.
type AcaoAgenda struct {
	ID           string      `json:"id"`
	Faixa        FaixaAgenda `json:"faixa"`
	Tipo         string      `json:"tipo"`
	Alvo         string      `json:"alvo"`
	Impacto      string      `json:"impacto"`
	Moedas       int         `json:"moedas,omitempty"`
	Prazo        *time.Time  `json:"prazo,omitempty"`
	Confianca    string      `json:"confianca"`
	Proveniencia string      `json:"proveniencia"`
	Conflitos    []string    `json:"conflitos,omitempty"`
	Link         string      `json:"link"`
}

type AgendaInput struct {
	Mercado   MarketPlan
	Evolucoes []EvoMatch
	SBCs      []domain.SBC
	Watchlist []domain.WatchlistEntry
	Agora     time.Time
}
type Agenda struct {
	Agora      []AcaoAgenda `json:"agora"`
	EstaSemana []AcaoAgenda `json:"esta_semana"`
	Observando []AcaoAgenda `json:"observando"`
}

// MontarAgenda somente organiza saidas prontas dos planejadores. Ela nao
// chama Score, nao estima preco e nao altera nenhuma decisao de origem.
func MontarAgenda(in AgendaInput) Agenda {
	if in.Agora.IsZero() {
		in.Agora = time.Now()
	}
	all := make([]AcaoAgenda, 0)
	for _, a := range in.Mercado.Actions {
		all = append(all, acaoAgendaMercado(a, in.Agora))
	}
	for _, e := range in.Evolucoes {
		if !e.Evolution.ExpiresAt.IsZero() {
			all = append(all, AcaoAgenda{ID: fmt.Sprintf("evo:%d:%s", e.Player.ID, e.Evolution.ID), Tipo: "evolucao", Alvo: e.Evolution.Name, Impacto: fmt.Sprintf("%+.1f BotScore", e.Gain), Moedas: e.Cost, Prazo: prazoAgenda(e.Evolution.ExpiresAt), Confianca: "alta", Proveniencia: "plano_evolucao", Link: "/evolucoes"})
		}
	}
	for _, s := range in.SBCs {
		if !s.ExpiresAt.IsZero() {
			all = append(all, AcaoAgenda{ID: "sbc:" + s.ID, Tipo: "sbc", Alvo: s.Name, Impacto: "SBC expira", Moedas: s.SolutionCost, Prazo: prazoAgenda(s.ExpiresAt), Confianca: "media", Proveniencia: "sbc_ativo", Link: "/hoje"})
		}
	}
	for _, w := range in.Watchlist {
		all = append(all, AcaoAgenda{ID: "watch:" + w.ID, Tipo: "watchlist", Alvo: w.Name, Impacto: "acompanhar preco alvo", Moedas: w.TargetCoins, Confianca: "media", Proveniencia: "watchlist", Link: "/mercado/plano"})
	}
	sort.SliceStable(all, func(i, j int) bool { return all[i].ID < all[j].ID })
	out := Agenda{}
	for _, a := range all {
		a.Faixa = faixaAgenda(a, in.Agora)
		switch a.Faixa {
		case AgendaAgora:
			out.Agora = append(out.Agora, a)
		case AgendaSemana:
			out.EstaSemana = append(out.EstaSemana, a)
		default:
			out.Observando = append(out.Observando, a)
		}
	}
	return out
}
func acaoAgendaMercado(a MarketAction, now time.Time) AcaoAgenda {
	faixa := AgendaObservando
	if a.Kind == MarketBuy || a.Kind == MarketSell {
		faixa = AgendaAgora
	}
	if a.Kind == MarketWait || a.Kind == MarketObserve {
		faixa = AgendaObservando
	}
	return AcaoAgenda{ID: fmt.Sprintf("mercado:%s:%d:%s", a.Kind, a.EAID, strings.ToLower(a.Name)), Faixa: faixa, Tipo: string(a.Kind), Alvo: a.Name, Impacto: strings.Join(a.Rationale, "; "), Moedas: a.NetCost, Confianca: a.Confidence, Proveniencia: "plano_mercado", Conflitos: a.Conflicts, Link: "/mercado/plano"}
}
func faixaAgenda(a AcaoAgenda, now time.Time) FaixaAgenda {
	if a.Prazo != nil {
		if !a.Prazo.After(now.Add(72 * time.Hour)) {
			return AgendaAgora
		}
		if !a.Prazo.After(now.Add(7 * 24 * time.Hour)) {
			return AgendaSemana
		}
	}
	if a.Faixa == AgendaAgora {
		return AgendaAgora
	}
	if a.Confianca == "baixa" || a.Confianca == "incompleta" || a.Tipo == "watchlist" {
		return AgendaObservando
	}
	return AgendaSemana
}

func prazoAgenda(v time.Time) *time.Time { return &v }
