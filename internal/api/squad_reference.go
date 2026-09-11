package api

import (
	"context"
	"fmt"

	"github.com/gscarneiro/eafc-bot/internal/analyze"
	"github.com/gscarneiro/eafc-bot/internal/chemistry"
	"github.com/gscarneiro/eafc-bot/internal/domain"
	"github.com/gscarneiro/eafc-bot/internal/store"
)

// aplicarPlanoReferencia sobrepõe somente a visão de análise da requisição.
// O snapshot gravado continua sendo a fotografia da coleta, e o plano local
// nunca toca o elenco no jogo. Um plano com pendência fica inativo para evitar
// que uma carta ausente vire um titular inferido silenciosamente.
func (s *Server) aplicarPlanoReferencia(ctx context.Context, snap *store.Snapshot) bool {
	backend, ok := s.Store.(store.SavedSquadPlanStore)
	if !ok {
		return false
	}
	plans, err := backend.ListSavedSquadPlans(ctx, squadPlanCycle(s, *snap), squadPlanClubKey(snap.Club))
	if err != nil {
		return false
	}
	for _, plan := range plans {
		if !plan.Referencia || len(pendenciasDoPlano(*snap, plan)) > 0 {
			continue
		}
		club, _, err := clubeParaPlanoSnapshot(*snap, plan)
		if err == nil {
			club = aplicarQuimicaSimulada(club, chemistry.Avaliar(s.resolveChemistryModel(), club))
			snap.Club = club
			planCopy := plan
			snap.PlanoReferencia = &planCopy
			return true
		}
		return false
	}
	return false
}

// recalcularAnalisesAtuais concentra o contrato da régua ativa. A fonte e o
// contexto escolhidos precisam governar as recomendações do mesmo jeito em
// que governam o editor; reaproveitar as listas gravadas misturaria uma
// referência nova com decisões feitas para outro XI.
func (s *Server) recalcularAnalisesAtuais(snap *store.Snapshot) {
	contexto := s.resolveEvaluationContext()
	if plan := snap.PlanoReferencia; plan != nil {
		contexto.EstiloJogo = plan.EstiloJogo
		contexto.RevisaoPlano = fmt.Sprintf("%s:%d", plan.ID, plan.Revisao)
	}
	snap.Avaliacao = contexto
	evaluator := s.resolveEvaluator()
	capital := snap.Club.Capital(s.EvolutionExtraBudget, s.MarketReserve, snap.Capital.Committed)
	snap.Capital = capital

	upgrades := analyze.DefaultUpgradeOptions(capital.Available)
	upgrades.MinGain = s.UpgradeMinGain
	if upgrades.MinGain <= 0 {
		upgrades.MinGain = analyze.DefaultUpgradeOptions(capital.Available).MinGain
	}
	upgrades.AllowOutOfPos = s.UpgradeAllowOutOfPos
	upgrades.AllowUnpriced = s.UpgradeAllowUnpriced
	upgrades.Evaluator = evaluator
	upgrades.Contexto = contexto
	var contextosPorVaga map[int]domain.ContextoAvaliacao
	if plan := snap.PlanoReferencia; plan != nil {
		contextosPorVaga = contextosDasVagas(snap.Club, *plan, contexto)
		upgrades.ContextosPorVaga = contextosPorVaga
	}
	snap.Upgrades, snap.MarketFunnel = analyze.FindUpgrades(snap.Club, snap.Market, upgrades)
	snap.EvoMatches = analyze.FindEvolutionsWithOptions(snap.Club, snap.Evolutions, analyze.EvolutionOptions{
		Budget: capital.Available, MinRating: s.EvolutionMinRating, IncludeUnaffordable: true,
		Evaluator: evaluator, Contexto: contexto, ContextosPorVaga: contextosPorVaga,
	})
	snap.SquadSwaps = analyze.FindSquadSwapsWithOptions(snap.Club, analyze.SquadSwapOptions{
		Evaluator: evaluator, Contexto: contexto, ContextosPorVaga: contextosPorVaga,
	})
	snap.SquadPlan = analyze.OptimizeSquadWithOptions(snap.Club, analyze.SquadOptions{
		Evaluator: evaluator, Contexto: contexto, ContextosPorVaga: contextosPorVaga, ChemistryModel: s.resolveChemistryModel(),
	})
	snap.Quimica = chemistry.Avaliar(s.resolveChemistryModel(), snap.Club)
	// Gauntlet não compartilha o motor de avaliação ainda. Forçar a recomposição
	// evita servir um plano cacheado para titulares que já não são a referência.
	snap.GauntletPlan = analyze.GauntletPlan{}
}

func contextosDasVagas(club domain.Club, plan domain.PlanoElencoSalvo, base domain.ContextoAvaliacao) map[int]domain.ContextoAvaliacao {
	result := make(map[int]domain.ContextoAvaliacao, len(plan.Vagas))
	for _, vaga := range plan.Vagas {
		ctx := base
		ctx.Posicao, ctx.Funcao, ctx.EstiloEntrosamento = vaga.Posicao, vaga.Funcao, vaga.EstiloEntrosamento
		if player, err := jogadorDaReferencia(club, vaga.Carta); err == nil {
			chem := player.Chemistry
			ctx.Quimica = &chem
		}
		result[vaga.Index] = ctx
	}
	return result
}

func contextosAtuaisDasVagas(snap store.Snapshot) map[int]domain.ContextoAvaliacao {
	if snap.PlanoReferencia == nil {
		return nil
	}
	return contextosDasVagas(snap.Club, *snap.PlanoReferencia, snap.Avaliacao)
}

// aplicarQuimicaSimulada liga o resultado da regra às cartas do XI antes da
// avaliação do perfil. Chemistry no retrato de origem descreve o jogo na
// coleta; o plano precisa dos pontos da escalação simulada e nunca os mistura
// com aquele valor observado.
func aplicarQuimicaSimulada(club domain.Club, result *chemistry.Resultado) domain.Club {
	if result == nil {
		return club
	}
	pointsBySlot := make(map[int]int, len(result.Jogadores))
	for _, jogador := range result.Jogadores {
		pointsBySlot[jogador.Index] = jogador.Pontos
	}
	club.Players = append([]domain.ClubPlayer(nil), club.Players...)
	for _, slot := range club.Squad.Starters {
		points, known := pointsBySlot[slot.Index]
		if !known {
			continue
		}
		for i := range club.Players {
			player := &club.Players[i]
			if (slot.ClubItemID != "" && player.ClubItemID == slot.ClubItemID) ||
				(slot.ClubItemID == "" && player.ID == slot.PlayerID) {
				player.Chemistry = points
				break
			}
		}
	}
	return club
}
