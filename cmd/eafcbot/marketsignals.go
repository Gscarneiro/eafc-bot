package main

import (
	"context"
	"fmt"
	"os"

	"github.com/gscarneiro/eafc-bot/internal/config"
	"github.com/gscarneiro/eafc-bot/internal/futgg"
	"github.com/gscarneiro/eafc-bot/internal/store"
)

// refreshMarketSignals é o job do ciclo de coleta RÁPIDO
// (scheduler.FastTicker) — só o que precisa ficar mais fresco que a coleta
// diária completa: momentum (o fut.gg recalcula a cada poucos minutos do
// lado deles) e o custo da solução mais barata de cada desafio de SBC.
// Nunca chama runJob: sem re-sincronizar clube, sem evoluções, sem
// cards.BuildReports — é justamente por ser barato que dá pra rodar bem
// mais vezes por dia sem incomodar o fut.gg.
//
// Uma fonte falhando não derruba a outra, mesmo princípio de
// futgg.Collect: um ciclo rápido que só grava metade do que buscou é
// melhor que um que não grava nada porque uma rota deu erro.
func refreshMarketSignals(ctx context.Context, cfg config.Config, st store.Store) {
	client := futgg.New(cfg.FutGG)

	momentum, err := client.Momentum(ctx, futgg.MomentumOptions{Hours: cfg.Serve.MomentumWindowHours})
	if err != nil {
		fmt.Fprintf(os.Stderr, "refresh de momentum: %v\n", err)
	} else if err := st.SaveMomentum(ctx, cfg.FutGG.Cycle, momentum); err != nil {
		fmt.Fprintf(os.Stderr, "gravando momentum: %v\n", err)
	}

	sbcs, err := client.SBCs(ctx)
	if err != nil {
		fmt.Fprintf(os.Stderr, "refresh de SBCs: %v\n", err)
		return
	}
	if err := st.SaveSBCCost(ctx, cfg.FutGG.Cycle, sbcs); err != nil {
		fmt.Fprintf(os.Stderr, "gravando custo de SBC: %v\n", err)
	}
}
