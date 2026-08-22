package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/gscarneiro/eafc-bot/internal/config"
	"github.com/gscarneiro/eafc-bot/internal/discover"
	"github.com/gscarneiro/eafc-bot/internal/futgg"
)

// cmdAutoconfig descobre sozinho como o fut.gg serve os dados e escreve o
// config. Substitui o trabalho manual de abrir o DevTools e copiar rotas.
func cmdAutoconfig(ctx context.Context, args []string) error {
	fs := flag.NewFlagSet("autoconfig", flag.ExitOnError)
	cfgPath := fs.String("config", config.DefaultPath(), "arquivo de configuração")
	dryRun := fs.Bool("dry-run", false, "só mostrar o que foi descoberto, sem gravar")
	quiet := fs.Bool("quiet", false, "esconder o progresso da varredura")
	dumpPath := fs.String("dump", "", "gravar o resultado bruto da descoberta em JSON")
	maxScripts := fs.Int("max-scripts", 60, "quantos bundles JS baixar")
	maxProbes := fs.Int("max-probes", 200, "quantas rotas candidatas sondar")
	ignoreRobots := fs.Bool("ignore-robots", false,
		"sondar também as rotas que o robots.txt do site pede para não bater")
	if err := fs.Parse(args); err != nil {
		return err
	}

	cfg, err := config.Load(*cfgPath)
	if err != nil {
		return err
	}

	// A descoberta precisa ver o estado atual do site, não o cache; e pode
	// pedir bastante coisa de uma vez, então afrouxamos a concorrência mas
	// mantemos o rate limit para não incomodar o fut.gg.
	fcfg := cfg.FutGG
	fcfg.CacheTTL = 0
	fcfg.Concurrency = 8
	client := futgg.New(fcfg)

	opt := discover.DefaultOptions()
	opt.ArgValues = map[string]string{"gamertag": cfg.GamerTag}
	opt.MaxProbes = *maxProbes
	opt.Mine.MaxScripts = *maxScripts
	opt.RespectRobots = !*ignoreRobots
	if *ignoreRobots {
		fmt.Println("aviso: -ignore-robots ligado. O fut.gg pede, no robots.txt deles,")
		fmt.Println("para não bater em /api/* nem em /gg-club/ — e é exatamente onde")
		fmt.Println("ficam os dados. A escolha de ler assim mesmo é sua; o bot só não")
		fmt.Println("finge que a regra não existe. Ele continua identificando-se pelo")
		fmt.Println("User-Agent e respeitando o limite de requisições.")
		fmt.Println()
	}
	if !*quiet {
		opt.Verbose = func(format string, a ...any) {
			fmt.Printf(format+"\n", a...)
		}
	}

	fmt.Printf("descobrindo a API de %s\n\n", cfg.FutGG.BaseURL)
	res, err := discover.Run(ctx, client, cfg.FutGG.BaseURL, opt)
	if err != nil {
		return err
	}

	if *dumpPath != "" {
		b, _ := json.MarshalIndent(res, "", "  ")
		if err := os.WriteFile(*dumpPath, b, 0o644); err != nil {
			return err
		}
		fmt.Printf("\nresultado bruto: %s\n", *dumpPath)
	}

	printReport(res)

	if len(res.Best) == 0 {
		// O caso mais comum não é o site recusar: é o robots.txt ter tirado
		// as rotas de dados da fila antes da primeira requisição. Dizer
		// "o site não quer ser lido" quando a API responde 200 manda o
		// usuário para o caminho errado.
		if res.BlockedCount > 0 && !*ignoreRobots {
			return fmt.Errorf(
				"nada foi identificado, e %d rotas nem chegaram a ser sondadas "+
					"porque o robots.txt do site pede para não bater nelas — é lá "+
					"que ficam os dados. Se você aceita fazer essa leitura mesmo "+
					"assim, rode: eafcbot autoconfig -ignore-robots",
				res.BlockedCount)
		}
		return fmt.Errorf("nada foi identificado — veja o diagnóstico acima. " +
			"Se a maioria caiu em 403 ou 404, as rotas mudaram de lugar; se caiu " +
			"em HTML, o site serve os dados dentro da página e o caminho é o " +
			"`eafcbot pages`")
	}

	if *dryRun {
		fmt.Println("\n-dry-run: nada foi gravado.")
		return nil
	}

	applied := apply(&cfg, res)
	if err := cfg.Save(*cfgPath); err != nil {
		return err
	}

	fmt.Printf("\n%s atualizado: %d endpoints, %d mapas de campo.\n",
		*cfgPath, applied.endpoints, applied.fieldMaps)
	if missing := missingKinds(cfg, res); len(missing) > 0 {
		fmt.Printf("ainda faltando: %s — o relatório sai sem essas seções.\n",
			strings.Join(missing, ", "))
	}
	fmt.Println("\npróximo passo: eafcbot run")
	return nil
}

type applyStats struct{ endpoints, fieldMaps int }

// apply grava as descobertas no config. Só sobrescreve o que foi
// identificado: um endpoint que você ajustou à mão e a descoberta não
// achou continua onde está.
func apply(cfg *config.Config, res *discover.Result) applyStats {
	if cfg.FutGG.Endpoints == nil {
		cfg.FutGG.Endpoints = map[string]string{}
	}
	if cfg.FutGG.FieldMaps == nil {
		cfg.FutGG.FieldMaps = map[string]map[string]string{}
	}
	if cfg.FutGG.Wrappers == nil {
		cfg.FutGG.Wrappers = map[string]string{}
	}

	var st applyStats
	for kind, f := range res.Best {
		// Payload embutido na página não é endpoint de API: guardamos o
		// caminho mesmo assim, mas sinalizamos com o wrapper especial.
		cfg.FutGG.Endpoints[kind] = f.Path
		st.endpoints++

		if len(f.Fields) > 0 {
			cfg.FutGG.FieldMaps[kind] = f.Fields
			st.fieldMaps++
		}
		if f.Wrapper != "" {
			cfg.FutGG.Wrappers[kind] = f.Wrapper
		} else {
			delete(cfg.FutGG.Wrappers, kind)
		}
	}
	return st
}

var wantedKinds = []string{"players", "evolutions", "sbcs", "objectives", "news", "club"}

// missingKinds lista o que o relatório vai mesmo sair sem.
//
// "Não descoberto" não é o mesmo que "faltando": o clube vem de uma rota
// pública por gamertag que nenhuma assinatura reconhece sozinha, e você a
// configurou à mão. Anunciá-la como faltando toda vez que o autoconfig roda
// faria você reconfigurar uma coisa que já está funcionando.
func missingKinds(cfg config.Config, res *discover.Result) []string {
	var out []string
	for _, k := range wantedKinds {
		if _, ok := res.Best[k]; ok {
			continue
		}
		if p := cfg.FutGG.Endpoints[k]; p != "" {
			continue
		}
		out = append(out, k)
	}
	return out
}

func printReport(res *discover.Result) {
	fmt.Printf("\n%s\n", strings.Repeat("─", 64))
	fmt.Printf("%d rotas mineradas · %d sondadas · %d identificadas\n",
		res.Mined, res.Probed, len(res.Best))
	if res.Registry > 0 {
		fmt.Printf("%d delas vieram do registro de rotas do próprio site\n", res.Registry)
	}
	fmt.Printf("%s\n\n", strings.Repeat("─", 64))

	for _, kind := range wantedKinds {
		f, ok := res.Best[kind]
		if !ok {
			fmt.Printf("%-11s  não encontrado\n\n", kind)
			continue
		}
		fmt.Printf("%-11s  %s\n", kind, f.Path)
		fmt.Printf("%-11s  confiança %.0f%% · %d amostras · via %s",
			"", f.Score*100, f.Samples, f.Source)
		if f.Wrapper != "" {
			fmt.Printf(" · lista em \"%s\"", f.Wrapper)
		}
		fmt.Println()

		if len(f.Fields) > 0 {
			// Mostra só os campos cuja chave real difere do nome lógico —
			// são esses que o parser não acertaria sozinho.
			var surprises []string
			for _, logical := range sortedKeys(f.Fields) {
				if real := f.Fields[logical]; !strings.EqualFold(real, logical) {
					surprises = append(surprises, logical+" → "+real)
				}
			}
			if len(surprises) > 0 {
				fmt.Printf("%-11s  aprendeu: %s\n", "", strings.Join(surprises, ", "))
			}
		}
		// Campo que a descoberta não conseguiu cravar não vira chute: cai
		// nos aliases do código. Vale avisar, porque é aí que mora o risco
		// de o parser ler o campo errado sem reclamar.
		if amb := ambiguous(f); len(amb) > 0 {
			fmt.Printf("%-11s  ambíguos (usando os aliases do código): %s\n",
				"", strings.Join(amb, ", "))
		}
		fmt.Println()
	}

	// "Nada respondeu" sem explicação é inútil. Aqui sai o porquê.
	if len(res.Outcomes) > 0 {
		fmt.Println("o que as sondagens devolveram:")
		for _, k := range sortedCounts(res.Outcomes) {
			fmt.Printf("  %-32s %d\n", k, res.Outcomes[k])
		}
		fmt.Println()
	}

	// Um 403 por falta de login tem conserto e merece ser dito, senão o
	// clube some do relatório como se a rota não existisse.
	if res.Outcomes["403 exige sessão (preencha session_cookie)"] > 0 {
		fmt.Println("algumas rotas responderam \"precisa estar logado\" — são as do GG")
		fmt.Println("Club com sessão. Você NÃO precisa delas: o mesmo elenco sai pela")
		fmt.Println("rota pública do seu perfil, sem cookie e sem token para renovar.")
		fmt.Println("No config, em \"endpoints\", deixe:")
		fmt.Println()
		fmt.Println("  \"club\": \"/api/gg-club/{gamertag}/players/\"")
		fmt.Println()
		fmt.Println("O {gamertag} é preenchido com o \"gamer_tag\" do config, e o seu")
		fmt.Println("perfil precisa estar público em fut.gg/gg-club/.")
		fmt.Println()
	}

	if res.BlockedCount > 0 {
		fmt.Printf("%d rotas NÃO foram sondadas porque o robots.txt do site pede\n"+
			"para não bater nelas. Isso é respeitado de propósito:\n", res.BlockedCount)
		for i, u := range res.Blocked {
			if i >= 5 {
				fmt.Printf("  ... e mais %d\n", res.BlockedCount-5)
				break
			}
			fmt.Printf("  %s\n", u)
		}
		fmt.Println()
	}

	if len(res.Sitemaps) > 0 {
		fmt.Printf("sitemaps anunciados pelo site (%d) — é por aqui que eles\n"+
			"abrem a enumeração de conteúdo:\n", len(res.Sitemaps))
		for i, u := range res.Sitemaps {
			if i >= 3 {
				fmt.Printf("  ... e mais %d\n", len(res.Sitemaps)-3)
				break
			}
			fmt.Printf("  %s\n", u)
		}
		fmt.Println()
	}

	if len(res.Unmatched) > 0 {
		fmt.Printf("responderam JSON mas não bateram com nenhuma assinatura (%d):\n", len(res.Unmatched))
		for i, u := range res.Unmatched {
			if i >= 8 {
				fmt.Printf("  ... e mais %d\n", len(res.Unmatched)-8)
				break
			}
			fmt.Printf("  %s\n", u)
		}
	}
}

// expectedFields são os campos que cada tipo idealmente resolve. O que
// sobrar aqui é o que a descoberta não conseguiu distinguir.
var expectedFields = map[string][]string{
	"players": {"id", "rating", "position", "name", "price",
		"pace", "shooting", "passing", "dribbling", "defending", "physical"},
	"evolutions": {"name", "levels", "requirements", "coin_cost"},
	"sbcs":       {"name", "challenges", "solution_cost"},
	"objectives": {"name", "tasks", "rewards"},
	"news":       {"title", "published_at", "slug"},
	"club":       {"players", "coins", "squad"},
}

func ambiguous(f discover.Finding) []string {
	var out []string
	for _, want := range expectedFields[f.Kind] {
		if _, ok := f.Fields[want]; !ok {
			out = append(out, want)
		}
	}
	return out
}

// sortedCounts ordena por contagem decrescente: o motivo mais comum
// primeiro, que é o que explica a falha.
func sortedCounts(m map[string]int) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Slice(out, func(i, j int) bool {
		if m[out[i]] != m[out[j]] {
			return m[out[i]] > m[out[j]]
		}
		return out[i] < out[j]
	})
	return out
}

func sortedKeys(m map[string]string) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
