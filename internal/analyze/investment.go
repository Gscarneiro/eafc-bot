package analyze

import (
	"fmt"
	"sort"

	"github.com/gscarneiro/eafc-bot/internal/domain"
)

// Investment é uma carta do mercado sinalizada como oportunidade de
// compra pra revender depois — NÃO pra usar no seu time (isso é
// analyze.Upgrade/FindUpgrades). Puramente consultivo: o bot nunca compra
// nem vende sozinho, só aponta o que a pesquisa de mercado chama de sinal
// de flip.
type Investment struct {
	Candidate   domain.Player `json:"candidate"`
	MomentumPct float64       `json:"momentum_pct"` // % abaixo da própria média recente — já calculado pelo fut.gg
	// ImpliedAverage reconstrói o preço médio recente a partir do
	// desconto (currentPrice / (1 - MomentumPct/100)) — a referência de
	// "pra onde o preço tende a voltar", não uma promessa de venda.
	ImpliedAverage int `json:"implied_average"`
	// Signal distingue os dois motivos possíveis, porque são riscos
	// diferentes por trás da mesma ação (comprar agora): "desconto" é a
	// carta caindo da própria média (momentum); "out-of-packs" é o
	// jogador ganhando uma carta especial nova, cortando a oferta da
	// carta atual — sinal historicamente mais forte (+33% a +400% em ~6
	// dias segundo a pesquisa de mercado) que desconto de preço sozinho.
	Signal    string   `json:"signal"`
	Rationale []string `json:"rationale"`
}

// InvestmentOptions controla o que conta como candidato aceitável.
type InvestmentOptions struct {
	// MinMomentumPct é o desconto mínimo (% abaixo da própria média
	// recente) pra entrar na lista. 5,26% é o ponto de equilíbrio depois
	// da taxa de venda de 5% da EA (lucro = 0,95×venda − compra) — abaixo
	// disso o "desconto" nem cobre a taxa. Padrão bem acima disso: a
	// pesquisa de mercado recomenda mirar ≥15% pra taxa não comer o ganho
	// de verdade.
	MinMomentumPct float64
}

// DefaultInvestmentOptions são padrões conservadores.
func DefaultInvestmentOptions() InvestmentOptions {
	return InvestmentOptions{MinMomentumPct: 15.0}
}

// InvestmentFunnel conta, candidato a candidato, onde cada um foi
// reprovado — mesmo padrão de UpgradeFunnel: uma lista vazia sem
// explicação vira "o bot não achou nada" sem dizer se é porque nada caiu
// de preço hoje ou porque o piso configurado está alto demais.
type InvestmentFunnel struct {
	Considered int `json:"considered"` // len(candidates)
	Owned      int `json:"owned"`      // você já tem a carta
	// NotTradeable conta carta exclusiva de SBC ou extinta — não dá pra
	// comprar de jeito nenhum, mesmo com desconto real.
	NotTradeable int `json:"not_tradeable"`
	// SupersededBySibling conta carta com uma versão do MESMO jogador
	// (mesmo BasePlayerEaID) de rating maior entre os PRÓPRIOS
	// candidatos — a pesquisa de mercado é enfática que a versão
	// inferior nunca deve ser recomendada pra compra (desaba quando a
	// melhor sai ou já saiu, "consolidação de versão").
	SupersededBySibling int `json:"superseded_by_sibling"`
	BelowMinMomentum    int `json:"below_min_momentum"`
	// Suggested conta candidato que passou do corte — pode ser maior que
	// len(investments) se um teto de exibição cortar a lista depois.
	Suggested int `json:"suggested"`

	MinMomentumPct float64 `json:"min_momentum_pct"`
	// BestRejectedPct é o maior MomentumPct entre os reprovados só por
	// BelowMinMomentum — mede o quão perto o mercado chegou do piso.
	BestRejectedPct  float64 `json:"best_rejected_pct"`
	BestRejectedName string  `json:"best_rejected_name"`
	HasBestRejected  bool    `json:"has_best_rejected"`
}

// FindInvestments varre candidates (a lista de momentum, já ordenada pelo
// fut.gg por maior desconto) atrás de oportunidade de compra pra revender
// depois. newCards são as cartas vistas pela primeira vez nesta coleta
// (ver store.Store.NewPlayers) — usadas só pro sinal de out-of-packs: um
// candidato cujo jogador acabou de ganhar uma carta especial nova em
// algum lugar da coleta de hoje.
//
// Escopo do sinal de out-of-packs é deliberadamente limitado aos PRÓPRIOS
// candidates, não ao catálogo de mercado inteiro: FindInvestments não
// recebe o catálogo cheio (caro de escanear a cada ciclo rápido, e o
// objetivo do ciclo rápido — scheduler.FastTicker — é ser barato). Um
// out-of-packs cuja carta ouro não aparece entre os maiores descontos não
// é pego aqui.
func FindInvestments(club domain.Club, candidates []domain.Player, newCards []domain.Player, opt InvestmentOptions) ([]Investment, InvestmentFunnel) {
	owned := make(map[int64]bool, len(club.Players))
	for _, p := range club.Players {
		owned[p.ID] = true
	}

	freshBase := make(map[int64]bool, len(newCards))
	for _, p := range newCards {
		if p.BasePlayerEaID > 0 {
			freshBase[p.BasePlayerEaID] = true
		}
	}

	// Maior rating por BasePlayerEaID entre os PRÓPRIOS candidatos — pra
	// saber quem tem uma versão irmã melhor (nunca comprar a inferior).
	bestRatingByBase := map[int64]int{}
	for _, c := range candidates {
		if c.BasePlayerEaID > 0 && c.Rating > bestRatingByBase[c.BasePlayerEaID] {
			bestRatingByBase[c.BasePlayerEaID] = c.Rating
		}
	}

	funnel := InvestmentFunnel{Considered: len(candidates), MinMomentumPct: opt.MinMomentumPct}

	var out []Investment
	for _, cand := range candidates {
		if owned[cand.ID] {
			funnel.Owned++
			continue
		}
		if !cand.Price.Tradeable() || cand.Price.Extinct {
			funnel.NotTradeable++
			continue
		}
		if cand.BasePlayerEaID > 0 {
			if best, ok := bestRatingByBase[cand.BasePlayerEaID]; ok && cand.Rating < best {
				funnel.SupersededBySibling++
				continue
			}
		}

		if cand.MomentumPct < opt.MinMomentumPct {
			funnel.BelowMinMomentum++
			if !funnel.HasBestRejected || cand.MomentumPct > funnel.BestRejectedPct {
				funnel.HasBestRejected = true
				funnel.BestRejectedPct = cand.MomentumPct
				funnel.BestRejectedName = cand.Display()
			}
			continue
		}

		funnel.Suggested++
		signal := "desconto"
		rationale := []string{fmt.Sprintf("%.1f%% abaixo da própria média recente", cand.MomentumPct)}
		if freshBase[cand.BasePlayerEaID] {
			signal = "out-of-packs"
			rationale = append(rationale, "o jogador ganhou uma carta especial nova — esta pode sair do pool de pacotes")
		}

		implied := cand.Price.Coins
		if cand.MomentumPct < 100 {
			implied = int(float64(cand.Price.Coins) / (1 - cand.MomentumPct/100))
		}

		out = append(out, Investment{
			Candidate:      cand,
			MomentumPct:    cand.MomentumPct,
			ImpliedAverage: implied,
			Signal:         signal,
			Rationale:      rationale,
		})
	}

	// Sinal out-of-packs primeiro (mais robusto que desconto sozinho,
	// pesquisa de mercado) — desempate por maior desconto.
	sort.Slice(out, func(i, j int) bool {
		if (out[i].Signal == "out-of-packs") != (out[j].Signal == "out-of-packs") {
			return out[i].Signal == "out-of-packs"
		}
		return out[i].MomentumPct > out[j].MomentumPct
	})
	return out, funnel
}
