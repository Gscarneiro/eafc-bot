package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"sort"

	"github.com/gscarneiro/eafc-bot/internal/config"
	"github.com/gscarneiro/eafc-bot/internal/futgg"
)

// cmdPages coleta cartas pelas páginas de detalhe enumeradas nos sitemaps
// que o próprio site anuncia no robots.txt — sem tocar em rota de API.
// É mais caro (uma requisição por carta) e não traz preço, mas é o caminho
// que o fut.gg deixou aberto de propósito, e não quebra quando eles mexem
// no front.
func cmdPages(ctx context.Context, args []string) error {
	fs := flag.NewFlagSet("pages", flag.ExitOnError)
	cfgPath := fs.String("config", config.DefaultPath(), "arquivo de configuração")
	max := fs.Int("max", 200, "quantas cartas buscar")
	sitemaps := fs.Int("sitemaps", 2, "quantos sub-sitemaps percorrer")
	out := fs.String("out", "", "gravar as cartas em JSON neste arquivo")
	if err := fs.Parse(args); err != nil {
		return err
	}

	cfg, err := config.Load(*cfgPath)
	if err != nil {
		return err
	}
	client := futgg.New(cfg.FutGG)

	fmt.Printf("coletando pelas páginas de %s\n", cfg.FutGG.BaseURL)
	players, err := client.PlayersFromPages(ctx, futgg.PageCollectOptions{
		MaxPlayers:     *max,
		SitemapsToRead: *sitemaps,
		Newest:         true,
		Verbose:        func(f string, a ...any) { fmt.Printf(f+"\n", a...) },
	})
	if err != nil {
		return err
	}

	sort.Slice(players, func(i, j int) bool { return players[i].Rating > players[j].Rating })

	fmt.Printf("\n%d cartas lidas. As 10 melhores:\n\n", len(players))
	for i, p := range players {
		if i >= 10 {
			break
		}
		a := p.Attributes
		fmt.Printf("  %-24s %3d %-4s  RIT %2d FIN %2d PAS %2d DRI %2d DEF %2d FIS %2d",
			p.Display(), p.Rating, p.Position,
			a.Pace, a.Shooting, a.Passing, a.Dribbling, a.Defending, a.Physical)
		if len(p.PlayStyles) > 0 {
			fmt.Printf("  [%d PS]", len(p.PlayStyles))
		}
		fmt.Println()
	}

	// O preço não vem renderizado no servidor: ele carrega por JS, ou seja,
	// pela rota de API que o robots.txt bloqueia. Melhor dizer isso alto do
	// que deixar o relatório sugerir troca com preço zerado.
	comPreco := 0
	for _, p := range players {
		if p.Price.Coins > 0 {
			comPreco++
		}
	}
	fmt.Printf("\ncom preço: %d de %d\n", comPreco, len(players))
	if comPreco == 0 {
		fmt.Println("nenhum preço veio na página — o ranking por moeda fica indisponível")
		fmt.Println("e as sugestões saem ordenadas só por ganho na função.")
	}

	if *out != "" {
		b, err := json.MarshalIndent(players, "", "  ")
		if err != nil {
			return err
		}
		if err := os.WriteFile(*out, b, 0o644); err != nil {
			return err
		}
		fmt.Printf("\ncartas gravadas em %s\n", *out)
	}
	return nil
}
