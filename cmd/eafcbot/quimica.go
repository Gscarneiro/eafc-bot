package main

import (
	"context"
	"flag"
	"fmt"

	"github.com/gscarneiro/eafc-bot/internal/chemistry"
	"github.com/gscarneiro/eafc-bot/internal/config"
	"github.com/gscarneiro/eafc-bot/internal/domain"
)

// cmdQuimica mostra o entrosamento do XI de hoje com a conta ABERTA, e
// confronta o modelo com o número que o próprio jogo reportou. Lê só o
// snapshot guardado — não toca a rede.
//
// `-calibrar` replaya o modelo contra todos os retratos de clube guardados.
// Isso é comando de CLI e não rota de API de propósito: cada snapshot passa
// de 30 MB, e um replay de 30 dias lê quase um giga.
func cmdQuimica(ctx context.Context, args []string) error {
	fs := flag.NewFlagSet("quimica", flag.ExitOnError)
	cfgPath := fs.String("config", config.DefaultPath(), "arquivo de configuração")
	modelo := fs.String("modelo", "", "modelo de química (padrão: chemistry.model do config)")
	calibrar := fs.Bool("calibrar", false, "replayar o modelo contra todos os snapshots guardados")
	dias := fs.Int("dias", 0, "quantos dias replayar (padrão: a retenção configurada)")
	if err := fs.Parse(args); err != nil {
		return err
	}

	cfg, err := config.Load(*cfgPath)
	if err != nil {
		return err
	}
	m := cfg.ChemistryModel()
	if *modelo != "" {
		if m, err = chemistry.Escolher(*modelo); err != nil {
			return err
		}
	}

	st, err := openStore(ctx, cfg)
	if err != nil {
		return err
	}
	defer st.Close()

	if *calibrar {
		janela := *dias
		if janela <= 0 {
			janela = cfg.Serve.RetentionDays
		}
		historico, err := st.ClubHistory(ctx, cfg.FutGG.Cycle, janela)
		if err != nil {
			return err
		}
		if len(historico) == 0 {
			return fmt.Errorf("nenhum snapshot guardado ainda — rode `eafcbot run` pelo menos uma vez")
		}
		printCalibracao(chemistry.Calibrar(m, historico))
		return nil
	}

	snap, ok, err := st.LatestSnapshot(ctx, cfg.FutGG.Cycle)
	if err != nil {
		return err
	}
	if !ok {
		return fmt.Errorf("nenhum snapshot ainda — rode `eafcbot run` primeiro")
	}
	printQuimica(m, snap.Club)
	return nil
}

func printQuimica(m chemistry.Modelo, club domain.Club) {
	xi, ok := chemistry.DoClube(club)
	if !ok {
		fmt.Println("escalação titular não sincronizada — confira em fut.gg/gg-club")
		return
	}
	res := chemistry.Calcular(m, xi)
	v := chemistry.Verificar(m, club)

	fmt.Printf("XI de %s · modelo %s\n\n", club.Squad.Formation, m.Nome)
	for _, j := range res.Jogadores {
		p, _ := club.PlayerByID(j.PlayerID)
		slot := ""
		for _, s := range club.Squad.Starters {
			if s.PlayerID == j.PlayerID {
				slot = string(s.Position)
				break
			}
		}
		marca := ""
		if j.ForaDePosicao {
			marca = "  FORA DE POSIÇÃO"
		}
		if j.Curinga != "" {
			marca += "  " + j.Curinga
		}
		fmt.Printf("  %-4s %-22s %d  (base %d · clube %d · liga %d · nação %d)  jogo: %d%s\n",
			slot, truncar(p.Display(), 22), j.Pontos,
			j.Base, j.Clube, j.Liga, j.Nacao, p.Chemistry, marca)
	}

	fmt.Printf("\n  total: %d/%d", res.Total, res.Maximo)
	switch v.Status {
	case chemistry.StatusConfere:
		fmt.Printf("   jogo: %d   confere (%d/%d jogadores)\n", v.Observado, v.Conferem, v.Total)
	case chemistry.StatusSemOraculo:
		fmt.Printf("   (sem oráculo: %s)\n", v.Detalhe)
	default:
		fmt.Printf("   jogo: %d\n\n  DIVERGE: %s\n", v.Observado, v.Detalhe)
	}

	if len(res.NaoModelado) > 0 {
		fmt.Println("\n  não modelado:")
		for _, s := range res.NaoModelado {
			fmt.Printf("    · %s\n", s)
		}
	}
}

func printCalibracao(r chemistry.Relatorio) {
	fmt.Printf("modelo %s · %d dias replayados\n", r.Modelo, r.Dias)
	fmt.Printf("  conferem: %d · divergem: %d · sem oráculo: %d\n", r.Conferem, r.Divergem, r.SemOraculo)

	for _, d := range r.Piores {
		fmt.Printf("    diverge: calculado %d, jogo %d (%d/%d jogadores conferem)\n",
			d.Calculado, d.Observado, d.Conferem, d.Total)
	}

	fmt.Println()
	switch {
	case r.Divergem > 0:
		fmt.Printf("  DIVERGE em %d dos %d dias com oráculo.\n", r.Divergem, r.Conferem+r.Divergem)
		fmt.Println("  Próximo passo: se a regra do jogo mudou, ajuste o Modelo em")
		fmt.Println("  internal/chemistry/modelos.go e rode este comando de novo.")
	case r.Confirma():
		fmt.Printf("  CONFIRMADO: o modelo reproduz o jogo em %d dias, com %d valores distintos de química.\n",
			r.Conferem, r.ValoresDistintos)
	case r.Conferem == 0:
		fmt.Println("  SEM ORÁCULO: nenhum dia guardado trouxe química do jogo para comparar.")
		fmt.Println("  Próximo passo: rode `eafcbot run` com o clube sincronizado em fut.gg/gg-club.")
	default:
		// O caso que impede o relatório de virar autoengano: sem variação na
		// amostra, um modelo que devolvesse um número fixo passaria igual.
		fmt.Printf("  NÃO CONTRARIADO (não é o mesmo que confirmado): %d dias conferem, mas o jogo\n", r.Conferem)
		fmt.Printf("  reportou só %d valor distinto de química em toda a amostra — um modelo que\n", r.ValoresDistintos)
		fmt.Println("  devolvesse esse número fixo passaria neste mesmo teste.")
		fmt.Println("  Próximo passo: mude a escalação para um XI de composição bem diferente,")
		fmt.Println("  rode `eafcbot run`, e depois este comando de novo.")
	}
}

func truncar(s string, n int) string {
	if len([]rune(s)) <= n {
		return s
	}
	return string([]rune(s)[:n-1]) + "…"
}
