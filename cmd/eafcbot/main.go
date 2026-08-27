// Comando eafcbot: coleta os dados do fut.gg, compara com o seu elenco e
// gera o briefing diário.
//
//	eafcbot autoconfig           descobre as rotas do fut.gg e grava o config
//	eafcbot pages                coleta cartas pelas páginas, via sitemap
//	eafcbot init                 grava a configuração padrão
//	eafcbot discover <endpoint>  mostra a resposta crua de um endpoint
//	eafcbot run                  roda o job uma vez: coleta, analisa e grava
//	eafcbot serve                sobe a API + a UI, com scheduler diário embutido
//	eafcbot demo                 gera um relatório de exemplo, sem rede
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/gscarneiro/eafc-bot/internal/analyze"
	"github.com/gscarneiro/eafc-bot/internal/cards"
	"github.com/gscarneiro/eafc-bot/internal/chemistry"
	"github.com/gscarneiro/eafc-bot/internal/config"
	"github.com/gscarneiro/eafc-bot/internal/domain"
	"github.com/gscarneiro/eafc-bot/internal/futgg"
	"github.com/gscarneiro/eafc-bot/internal/report"
	"github.com/gscarneiro/eafc-bot/internal/store"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "erro: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	if len(os.Args) < 2 {
		usage()
		return fmt.Errorf("informe um comando")
	}

	// Ctrl-C cancela a coleta em andamento em vez de matar o processo no meio
	// de uma escrita no banco.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	cmd, args := os.Args[1], os.Args[2:]
	switch cmd {
	case "init":
		return cmdInit(args)
	case "discover":
		return cmdDiscover(ctx, args)
	case "autoconfig":
		return cmdAutoconfig(ctx, args)
	case "pages":
		return cmdPages(ctx, args)
	case "run":
		return cmdRun(ctx, args)
	case "serve":
		return cmdServe(ctx, args)
	case "demo":
		return cmdDemo(args)
	case "quimica":
		return cmdQuimica(ctx, args)
	case "help", "-h", "--help":
		usage()
		return nil
	default:
		usage()
		return fmt.Errorf("comando desconhecido: %q", cmd)
	}
}

func usage() {
	fmt.Fprint(os.Stderr, `eafcbot — briefing diário do seu Ultimate Team

  init                   grava a configuração padrão em .eafc-bot/config.json
  autoconfig             descobre sozinho as rotas do fut.gg e grava o config
                         (-dry-run só mostra, -dump grava o resultado bruto)
  pages                  coleta cartas pelas páginas de detalhe listadas nos
                         sitemaps do site (sem tocar em rota de API)
  discover <endpoint>    busca um endpoint e mostra a resposta crua
                         (players, evolutions, objectives, sbcs, news, club)
  run                    roda o job uma vez: coleta, analisa carta a carta,
                         grava o snapshot e o relatório HTML (-dry-run não
                         grava nada; -out escolhe o caminho do HTML)
  serve                  sobe a API + a UI React numa porta só, com um
                         scheduler embutido que roda o mesmo job de "run"
                         uma vez por dia sozinho — Ctrl+C encerra
                         (-port; -demo sobe as 5 telas com dado fictício,
                         sem rede nem scheduler; -open=false não abre o
                         navegador sozinho)
  demo                   gera um relatório de exemplo com dados fictícios
  quimica                mostra o entrosamento do XI de hoje, carta a carta
                         (-modelo escolhe outro modelo; -calibrar reproduz o
                         modelo contra os snapshots guardados, -dias limita
                         quantos)

Variáveis de ambiente:
  EAFC_GAMERTAG      o nome do seu perfil no fut.gg (não o EA ID)
  EAFC_DSN           DSN do Postgres (liga o histórico de preços)
  EAFC_FUTGG_COOKIE  cookie de sessão do fut.gg, se o clube for privado
  EAFC_CONFIG        caminho alternativo do arquivo de configuração
  EAFC_BUDGET        moedas extras a considerar no orçamento
`)
}

func cmdInit(args []string) error {
	fs := flag.NewFlagSet("init", flag.ExitOnError)
	path := fs.String("config", config.DefaultPath(), "onde gravar")
	force := fs.Bool("force", false, "sobrescrever se já existir")
	if err := fs.Parse(args); err != nil {
		return err
	}

	if _, err := os.Stat(*path); err == nil && !*force {
		return fmt.Errorf("%s já existe (use -force para sobrescrever)", *path)
	}
	cfg := config.Default()
	if err := cfg.Save(*path); err != nil {
		return err
	}
	fmt.Printf("configuração gravada em %s\n", *path)
	fmt.Println("preencha \"gamer_tag\" com o nome do SEU PERFIL no fut.gg —")
	fmt.Println("não é a sua gamertag da EA. Abra https://www.fut.gg/gg-club/")
	fmt.Println("logado e copie o nome que aparece na URL (pode colar a URL")
	fmt.Println("inteira, ela é normalizada sozinha).")
	return nil
}

// cmdDiscover é a ferramenta de calibração: o fut.gg não publica contrato,
// então este comando mostra o que o endpoint devolve de verdade para os
// nomes de campo em internal/futgg/map.go serem ajustados.
func cmdDiscover(ctx context.Context, args []string) error {
	fs := flag.NewFlagSet("discover", flag.ExitOnError)
	cfgPath := fs.String("config", config.DefaultPath(), "arquivo de configuração")
	rawURL := fs.String("url", "", "buscar uma URL literal em vez de um endpoint nomeado")
	out := fs.String("out", "", "gravar a resposta neste arquivo")
	limit := fs.Int("limit", 3000, "quantos bytes mostrar no terminal")
	if err := fs.Parse(args); err != nil {
		return err
	}

	cfg, err := config.Load(*cfgPath)
	if err != nil {
		return err
	}
	// Discover nunca usa cache: o objetivo é ver o estado atual do site.
	cfg.FutGG.CacheTTL = 0
	client := futgg.New(cfg.FutGG)

	target := *rawURL
	if target == "" {
		if fs.NArg() < 1 {
			return fmt.Errorf("informe um endpoint (players, evolutions, objectives, sbcs, news, club) ou -url")
		}
		name := fs.Arg(0)
		vars := map[string]string{"gamertag": cfg.GamerTag}
		if fs.NArg() > 1 {
			vars["id"], vars["slug"] = fs.Arg(1), fs.Arg(1)
		}
		if target, err = client.URL(name, vars); err != nil {
			return err
		}
	}

	fmt.Printf("GET %s\n\n", target)
	body, err := client.GetRaw(ctx, target)
	if err != nil {
		return err
	}

	if *out != "" {
		if err := os.WriteFile(*out, body, 0o644); err != nil {
			return err
		}
		fmt.Printf("resposta gravada em %s (%d bytes)\n\n", *out, len(body))
	}

	// Se for JSON, mostra as chaves de topo: é o que interessa para o mapeamento.
	var probe any
	if json.Unmarshal(body, &probe) == nil {
		if keys := topKeys(probe); len(keys) > 0 {
			fmt.Printf("chaves encontradas: %v\n\n", keys)
		}
	}

	preview := body
	if len(preview) > *limit {
		preview = preview[:*limit]
	}
	fmt.Println(string(preview))
	if len(body) > *limit {
		fmt.Printf("\n... (%d bytes no total)\n", len(body))
	}
	return nil
}

// topKeys lista as chaves do objeto, entrando um nível no primeiro item
// quando a resposta é uma lista.
func topKeys(v any) []string {
	switch t := v.(type) {
	case map[string]any:
		keys := make([]string, 0, len(t))
		for k := range t {
			keys = append(keys, k)
		}
		return keys
	case []any:
		if len(t) > 0 {
			return topKeys(t[0])
		}
	}
	return nil
}

func cmdRun(ctx context.Context, args []string) error {
	fs := flag.NewFlagSet("run", flag.ExitOnError)
	cfgPath := fs.String("config", config.DefaultPath(), "arquivo de configuração")
	outPath := fs.String("out", "", "caminho do relatório (padrão: .eafc-bot/reports/briefing-AAAA-MM-DD.html)")
	dryRun := fs.Bool("dry-run", false, "não gravar nada no histórico")
	if err := fs.Parse(args); err != nil {
		return err
	}

	cfg, err := config.Load(*cfgPath)
	if err != nil {
		return err
	}
	if cfg.GamerTag == "" {
		return fmt.Errorf("gamer_tag não configurado — rode `eafcbot init` e preencha, ou defina EAFC_GAMERTAG")
	}

	st, err := openStore(ctx, cfg)
	if err != nil {
		return err
	}
	defer st.Close()

	_, err = runJob(ctx, cfg, st, *outPath, *dryRun)
	return err
}

// runJob é o trabalho diário inteiro: coleta do fut.gg, análise carta a
// carta do elenco (o custo caro que `cards` pagava sob demanda — ver
// internal/cards), análise de mercado/evolução, e a gravação do snapshot e
// do relatório HTML. `run` e o scheduler de `serve` chamam exatamente esta
// função; a única diferença entre os dois é QUEM decide a hora de rodar.
func runJob(ctx context.Context, cfg config.Config, st store.Store, outPath string, dryRun bool) (report.Data, error) {
	started := time.Now()
	client := futgg.New(cfg.FutGG)
	filter := futgg.PlayerFilter{
		MinRating: cfg.Market.MinRating,
		MaxRating: cfg.Market.MaxRating,
		MaxPrice:  cfg.Market.MaxPrice,
		Pages:     cfg.Market.Pages,
		PerPage:   cfg.Market.PerPage,
	}

	fmt.Println("coletando do fut.gg...")
	snap, err := client.Collect(ctx, cfg.GamerTag, filter)
	if err != nil {
		return report.Data{}, err
	}
	snap.PlayStyleCatalog = client.PlayStyleCatalog(ctx)
	snap.RoleCatalog = client.Roles(ctx)
	fmt.Printf("  %d cartas · %d evoluções · %d SBCs · %d notícias\n",
		len(snap.Market), len(snap.Evolutions), len(snap.SBCs), len(snap.News))
	if snap.Stats.RobotsBypassed > 0 {
		fmt.Printf("  aviso: %d rotas configuradas lidas apesar do robots.txt do site\n",
			snap.Stats.RobotsBypassed)
	}
	if snap.Stats.MarketPriceSkipped > 0 {
		fmt.Printf("  %d cartas do mercado descartadas por passar de market.max_price\n",
			snap.Stats.MarketPriceSkipped)
	}
	if len(snap.Club.Players) == 0 {
		// O clube é o dado central do bot — quando ele falha, o resto do
		// briefing perde o sentido. "N fontes falharam" deixava esse erro
		// diluído junto com o de notícia; o texto completo vai direto ao
		// console, porque é ele que ensina o que fazer (ver clubNotFoundError).
		if msg := clubErrorMessage(snap.Errors); msg != "" {
			fmt.Printf("  aviso: %s\n", msg)
		} else {
			fmt.Println("  aviso: clube vazio — confira a sincronização em fut.gg/gg-club")
		}
	}

	// Calculado logo após a coleta do clube, antes da análise carta-a-carta:
	// os titulares que ele escolhe entram em requiredIDs abaixo, para
	// forçar CardReport deles mesmo abaixo do corte normal de rating.
	chemModel := cfg.ChemistryModel()
	gauntletPlan := analyze.BuildGauntletPlanWithOptions(snap.Club, analyze.GauntletOptions{ChemistryModel: chemModel})

	var cardReports []cards.CardReport
	if !dryRun && len(snap.Club.Players) > 0 {
		fmt.Printf("analisando cartas a partir de %d de overall (atual x potencial)...\n", cfg.Serve.CardsMinRating)
		if cardReports, err = cards.BuildReports(ctx, client, snap.Club, cfg.Serve.CardsMinRating, gauntletPlan.StarterIDs()); err != nil {
			snap.Errors = append(snap.Errors, "análise carta-a-carta: "+err.Error())
		}
	}

	data, err := analyzeAndBuild(ctx, cfg, st, snap, started, dryRun, cardReports, gauntletPlan)
	if err != nil {
		return report.Data{}, err
	}

	path := outPath
	if path == "" {
		path = filepath.Join(cfg.Report.OutputDir,
			fmt.Sprintf("briefing-%s.html", time.Now().Format("2006-01-02")))
	}
	if err := writeReport(path, data); err != nil {
		return data, err
	}

	fmt.Printf("\nrelatório: %s\n", path)
	fmt.Printf("  %d upgrades · %d evoluções · %d SBCs · %d cartas novas · %d cartas analisadas\n",
		len(data.Upgrades), len(data.Evolutions), len(data.SBCs), len(data.NewCards), len(cardReports))
	if len(snap.Errors) > 0 {
		fmt.Printf("  %d fontes falharam (veja o topo do relatório)\n", len(snap.Errors))
	}
	return data, nil
}

// analyzeAndBuild é o miolo compartilhado entre runJob e `demo`.
func analyzeAndBuild(ctx context.Context, cfg config.Config, st store.Store,
	snap *futgg.Snapshot, started time.Time, dryRun bool, cardReports []cards.CardReport,
	gauntletPlan analyze.GauntletPlan) (report.Data, error) {

	// Uma capital só, usada tanto para exibição quanto para decisão: Budget
	// era cash+raisable ad-hoc, sem reserva nenhuma — orçamento de compra e
	// capital exibido podiam divergir sobre quanto dava para gastar. Available
	// já desconta reserva e comprometido (ver domain.Capital).
	capital := snap.Club.Capital(cfg.Market.ExtraBudget, cfg.Market.Reserve, 0)
	budget := capital.Available

	upOpt := analyze.DefaultUpgradeOptions(budget)
	upOpt.MinGain = cfg.Report.MinGain
	upOpt.AllowOutOfPos = cfg.Report.AllowOutOfPos
	upOpt.AllowUnpriced = cfg.Report.AllowUnpriced

	upgrades, upFunnel := analyze.FindUpgrades(snap.Club, snap.Market, upOpt)
	evos := analyze.FindEvolutionsWithOptions(snap.Club, snap.Evolutions, analyze.EvolutionOptions{
		Budget:              budget,
		MinRating:           cfg.Serve.CardsMinRating,
		IncludeUnaffordable: true,
	})
	if len(cardReports) > 0 {
		slugByID := make(map[int64]string, len(cardReports))
		for _, card := range cardReports {
			slugByID[card.Player.ID] = card.Slug
		}
		for i := range evos {
			evos[i].CardSlug = slugByID[evos[i].Player.ID]
		}
	}

	var newCards []domain.Player
	var freshNews []domain.NewsItem
	trends := map[int64]store.PriceTrend{}
	var momentum []domain.Player
	sbcCostTrends := map[string]store.PriceTrend{}
	window := time.Duration(cfg.Report.TrendWindowHrs) * time.Hour

	if !dryRun && st != nil {
		var err error
		if newCards, err = st.NewPlayers(ctx, cfg.FutGG.Cycle, snap.Market); err != nil {
			snap.Errors = append(snap.Errors, "detectando cartas novas: "+err.Error())
		}
		if freshNews, err = st.UnseenNews(ctx, cfg.FutGG.Cycle, snap.News); err != nil {
			snap.Errors = append(snap.Errors, "filtrando notícias: "+err.Error())
		}

		// Tendências vêm ANTES de gravar os preços de hoje, para a variação
		// comparar contra o histórico e não contra o ponto recém-inserido.
		ids := interestingIDs(snap.Club, upgrades)
		if trends, err = st.Trends(ctx, cfg.FutGG.Cycle, ids, window); err != nil {
			snap.Errors = append(snap.Errors, "consultando tendências: "+err.Error())
		}

		// Momentum é o último valor lido pelo ciclo de coleta rápido
		// (scheduler.FastTicker) — pode ser mais fresco que este snapshot
		// diário; vazio (não erro) enquanto o ciclo rápido não rodou.
		if momentum, err = st.LatestMomentum(ctx, cfg.FutGG.Cycle); err != nil {
			snap.Errors = append(snap.Errors, "lendo momentum: "+err.Error())
		}
		if sbcCostTrends, err = st.SBCCostTrend(ctx, cfg.FutGG.Cycle, sbcChallengeKeys(snap.SBCs), window); err != nil {
			snap.Errors = append(snap.Errors, "consultando tendência de custo de SBC: "+err.Error())
		}

		if err := st.SavePrices(ctx, cfg.FutGG.Cycle, snap.Market); err != nil {
			snap.Errors = append(snap.Errors, "gravando preços: "+err.Error())
		}
		if err := st.SaveClub(ctx, snap.Club); err != nil {
			snap.Errors = append(snap.Errors, "gravando clube: "+err.Error())
		}
	} else {
		freshNews = snap.News
	}
	// Trends consulta o histórico antes de gravar a rodada atual para não
	// comparar o ponto recém-observado consigo mesmo. O relatório, porém,
	// precisa terminar na cotação que o usuário vê agora; anexá-la aqui
	// preserva a primeira amostra histórica e evita uma tabela um dia atrás.
	mergeCurrentPriceTrends(trends, snap.Club, snap.Market)

	chemModel := cfg.ChemistryModel()
	data := report.Build(report.Input{
		Snapshot:       snap,
		NewCards:       newCards,
		FreshNews:      freshNews,
		Upgrades:       upgrades,
		Evolutions:     evos,
		Funnel:         upFunnel,
		Trends:         trends,
		TrendWindow:    window,
		Started:        started,
		MaxRows:        cfg.Report.MaxRows,
		CardReports:    cardReports,
		Momentum:       momentum,
		SBCCostTrends:  sbcCostTrends,
		ChemistryModel: chemModel,
	})

	if !dryRun && st != nil {
		if len(snap.Club.Players) == 0 {
			// Nunca sobrescreve um dia bom com um clube vazio: o snapshot
			// gravado ontem continua sendo o que a API e a UI leem hoje.
			data.Errors = append(data.Errors,
				"snapshot não gravado: clube veio vazio, mantendo o último snapshot bom")
		} else {
			diff := store.ClubDiff{}
			if prev, ok, err := st.PreviousClub(ctx, snap.Club.GamerTag, cfg.FutGG.Cycle); err == nil && ok {
				diff = store.DiffClubs(prev, snap.Club)
			}
			err := st.SaveSnapshot(ctx, store.Snapshot{
				GeneratedAt:      data.GeneratedAt,
				Duration:         time.Since(started),
				Cycle:            snap.Club.Cycle,
				Club:             snap.Club,
				Capital:          capital,
				Market:           snap.Market,
				Evolutions:       snap.Evolutions,
				Objectives:       snap.Objectives,
				SBCs:             snap.SBCs,
				News:             snap.News,
				Stats:            snap.Stats,
				Errors:           snap.Errors,
				Capabilities:     snap.Capabilities,
				PlayStyleCatalog: snap.PlayStyleCatalog,
				RoleCatalog:      snap.RoleCatalog,
				Diff:             diff,
				NewCards:         newCards,
				FreshNews:        freshNews,
				Upgrades:         upgrades,
				MarketFunnel:     upFunnel,
				EvoMatches:       evos,
				SquadSwaps:       data.SquadSwaps,
				SquadPlan:        data.SquadPlan,
				Trends:           trends,
				SquadScore:       data.SquadScore,
				Cards:            cardReports,
				GauntletPlan:     gauntletPlan,
				Quimica:          chemistry.Avaliar(chemModel, snap.Club),
			})
			if err != nil {
				data.Errors = append(data.Errors, "gravando snapshot: "+err.Error())
			}
		}
	}

	return data, nil
}

func mergeCurrentPriceTrends(trends map[int64]store.PriceTrend, club domain.Club, market []domain.Player) {
	if trends == nil {
		return
	}
	seen := make(map[int64]bool, len(club.Players)+len(market))
	add := func(p domain.Player) {
		if p.ID == 0 || seen[p.ID] || p.Price.Coins <= 0 {
			return // preço desconhecido não pode ser inventado como zero
		}
		seen[p.ID] = true
		t := trends[p.ID]
		if t.EAID == 0 {
			t.EAID = p.ID
		}
		if t.Samples == 0 || t.First <= 0 {
			t.First = p.Price.Coins
			t.Min = p.Price.Coins
			t.Max = p.Price.Coins
			t.Samples = 0
		}
		t.Last = p.Price.Coins
		if t.Min == 0 || p.Price.Coins < t.Min {
			t.Min = p.Price.Coins
		}
		if p.Price.Coins > t.Max {
			t.Max = p.Price.Coins
		}
		t.Samples++
		t.ChangePct = 0
		if t.First > 0 {
			t.ChangePct = float64(t.Last-t.First) / float64(t.First) * 100
		}
		trends[p.ID] = t
	}
	for _, p := range club.Players {
		add(p.Player)
	}
	for _, p := range market {
		add(p)
	}
}

// interestingIDs limita a consulta de tendências ao que você tem e ao que
// o bot está mirando — não faz sentido puxar histórico do catálogo inteiro.
func interestingIDs(club domain.Club, ups []analyze.Upgrade) []int64 {
	seen := map[int64]bool{}
	var ids []int64
	add := func(id int64) {
		if id != 0 && !seen[id] {
			seen[id] = true
			ids = append(ids, id)
		}
	}
	for _, p := range club.Players {
		add(p.ID)
	}
	for _, u := range ups {
		add(u.Candidate.ID)
	}
	return ids
}

// sbcChallengeKeys lista a chave (store.SBCChallengeKey) de todo desafio
// de SBC ativo hoje — o que analyze.FindFodderDemand precisa pra saber a
// tendência de custo de cada um.
func sbcChallengeKeys(sbcs []domain.SBC) []string {
	var keys []string
	for _, sbc := range sbcs {
		for idx, ch := range sbc.Challenges {
			keys = append(keys, store.SBCChallengeKey(sbc.ID, idx, ch.Name))
		}
	}
	return keys
}

func writeReport(path string, data report.Data) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	// Um diretório com o nome do arquivo faz os.Create falhar com uma
	// mensagem que não ajuda ninguém ("Access is denied" no Windows).
	if fi, err := os.Stat(path); err == nil && fi.IsDir() {
		return fmt.Errorf("%s é um diretório, não dá para gravar o relatório ali "+
			"— apague a pasta ou escolha outro -out", path)
	}
	f, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("criando %s: %w", path, err)
	}
	defer f.Close()
	return report.Render(f, data)
}

func openStore(ctx context.Context, cfg config.Config) (store.Store, error) {
	if cfg.Postgres.Enabled {
		st, err := store.OpenPostgresWithRetention(ctx, cfg.Postgres.Driver, cfg.Postgres.DSN, cfg.Serve.RetentionDays)
		if err != nil {
			// %s com a mensagem já redigida, não %w: alguns erros de conexão do
			// driver ecoam a DSN inteira de volta (ver config.Config.RedactSecrets),
			// e esse texto vai direto pro stderr do processo.
			return nil, fmt.Errorf("%s\n(dica: compile com `go build -tags postgres ./cmd/eafcbot` para incluir o driver)", cfg.RedactSecrets(err.Error()))
		}
		fmt.Println("histórico: Postgres")
		return st, nil
	}
	st, err := store.NewJSONWithRetention(cfg.DataDir, cfg.Serve.RetentionDays)
	if err != nil {
		return nil, err
	}
	fmt.Printf("histórico: JSON em %s\n", cfg.DataDir)
	return st, nil
}

// clubErrorMessage acha, entre as fontes que falharam, a mensagem da fonte
// "clube" — que já vem pronta para o usuário agir (ver
// futgg.clubNotFoundError). O prefixo "clube: " que Collect() acrescenta
// (fail("clube", err) em collect.go) é cortado para não duplicar a palavra.
func clubErrorMessage(errs []string) string {
	for _, e := range errs {
		if msg, ok := strings.CutPrefix(e, "clube: "); ok {
			return msg
		}
	}
	return ""
}
