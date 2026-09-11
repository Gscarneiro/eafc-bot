package main

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io/fs"
	"math/rand"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/gscarneiro/eafc-bot/internal/advisor"
	"github.com/gscarneiro/eafc-bot/internal/analyze"
	"github.com/gscarneiro/eafc-bot/internal/api"
	"github.com/gscarneiro/eafc-bot/internal/config"
	"github.com/gscarneiro/eafc-bot/internal/domain"
	"github.com/gscarneiro/eafc-bot/internal/scheduler"
	"github.com/gscarneiro/eafc-bot/internal/store"
	"github.com/gscarneiro/eafc-bot/internal/webui"
)

// cmdServe sobe a API + o app React embutido numa porta só, com um
// scheduler interno (internal/scheduler) que roda runJob sozinho uma vez
// por dia — o "esquece e funciona" que tira o botão "rodar na mão" do
// caminho normal de uso. `run` continua existindo para disparar o mesmo
// trabalho manualmente (depurar sem esperar o horário configurado).
func cmdServe(ctx context.Context, args []string) error {
	fs := flag.NewFlagSet("serve", flag.ExitOnError)
	cfgPath := fs.String("config", config.DefaultPath(), "arquivo de configuração")
	demo := fs.Bool("demo", false, "sobe as telas com dado fictício, sem rede nem scheduler")
	port := fs.Int("port", 0, "porta do servidor (padrão: serve.port do config)")
	open := fs.Bool("open", true, "abrir o navegador sozinho")
	if err := fs.Parse(args); err != nil {
		return err
	}

	cfg, err := config.Load(*cfgPath)
	if err != nil {
		return err
	}
	if *port > 0 {
		cfg.Serve.Port = *port
	}

	dist, err := webui.DistFS()
	if err != nil {
		return fmt.Errorf("abrindo o app embutido: %w (rodou `npm run build` em web/ antes de compilar?)", err)
	}

	if *demo {
		return serveDemo(ctx, cfg, dist, *open)
	}

	if cfg.GamerTag == "" {
		return fmt.Errorf("gamer_tag não configurado — rode `eafcbot init` e preencha, ou defina EAFC_GAMERTAG")
	}

	st, err := openStore(ctx, cfg)
	if err != nil {
		return err
	}
	defer st.Close()
	evolutionAdvisor, err := advisor.NewFromEnv()
	if err != nil {
		return err
	}

	d := &daemon{cfg: cfg, store: st}
	// O status vive na memória do daemon, mas a coleta continua sendo um
	// processo diário. Restaurar o horário do último snapshot evita que a UI
	// diga "nunca executou" depois de um restart limpo.
	d.st.LastSuccess = restoreLastSuccess(ctx, st, cfg.FutGG.Cycle)

	sched := &scheduler.Scheduler{
		DailyAt:    cfg.Serve.DailyAt,
		StaleAfter: time.Duration(cfg.Serve.StaleAfterHours) * time.Hour,
		Settings: func() (string, time.Duration) {
			current := d.config()
			return current.Serve.DailyAt, time.Duration(current.Serve.StaleAfterHours) * time.Hour
		},
		LastGood: func(ctx context.Context) time.Time {
			snap, ok, err := st.LatestSnapshot(ctx, cfg.FutGG.Cycle)
			if err != nil || !ok {
				return time.Time{}
			}
			return snap.GeneratedAt
		},
		Job: d.run,
		Log: func(line string) { fmt.Println(line) },
	}
	go sched.Run(ctx)

	// Ciclo leve à parte da coleta diária — momentum e custo de SBC ficam
	// velhos rápido demais pra esperar o próximo runJob (ver
	// refreshMarketSignals). FastRefreshMinutes<=0 desliga.
	go scheduler.FastTickerDynamic(ctx, func() time.Duration {
		return time.Duration(d.config().Serve.FastRefreshMinutes) * time.Minute
	}, func(ctx context.Context) {
		refreshMarketSignals(ctx, d.config(), st)
	})

	apiSrv := &api.Server{
		Store: st, Cycle: cfg.FutGG.Cycle, History: cfg.Serve.RetentionDays,
		EvolutionMinRating: cfg.Serve.CardsMinRating, EvolutionExtraBudget: cfg.Market.ExtraBudget,
		MarketReserve:  cfg.Market.Reserve,
		UpgradeMinGain: cfg.Report.MinGain, UpgradeAllowOutOfPos: cfg.Report.AllowOutOfPos,
		UpgradeAllowUnpriced: cfg.Report.AllowUnpriced,
		ChemistryModel:       cfg.ChemistryModel(),
		EvaluationContext: func() domain.ContextoAvaliacao {
			current := d.config()
			fonte := domain.FonteAvaliacao(current.Evaluation.ExternalSource)
			if current.Evaluation.UseBot {
				fonte = domain.FonteBot
			}
			return domain.ContextoAvaliacao{Fonte: fonte, Perfil: current.Evaluation.Profile, Ciclo: current.FutGG.Cycle, Patch: current.Evaluation.Patch, Plataforma: current.Platform, EstiloJogo: current.Evaluation.PlayStyle}
		},
		CacheTTL:         10 * time.Second,
		Trigger:          func() { go d.run(context.Background()) },
		Status:           d.status,
		EvolutionAdvisor: evolutionAdvisor,
	}
	apiSrv.Config = &api.ConfigEditor{
		Get:      func() config.UISettings { return d.config().Editable() },
		GetCoins: func() *int { return d.config().Market.ManualCoins },
		UpdateCoins: func(coins int) (int, error) {
			current := d.config()
			current.Market.ManualCoins = &coins
			if err := current.Validate(); err != nil {
				return 0, err
			}
			if err := current.SaveManualCoins(*cfgPath, coins); err != nil {
				return 0, fmt.Errorf("gravando saldo: %w", err)
			}
			d.setConfig(current)
			return coins, nil
		},
		GetFavorites: func() []string { return splitFavorites(d.config().Serve.EvolutionFavorites) },
		UpdateFavorites: func(favorites []string) error {
			current := d.config()
			current.Serve.EvolutionFavorites = strings.Join(favorites, ",")
			if err := current.SaveEditable(*cfgPath, current.Editable()); err != nil {
				return fmt.Errorf("gravando favoritos: %w", err)
			}
			d.setConfig(current)
			return nil
		},
		GetProgress: func(slug string) []string { return d.config().Serve.EvolutionProgress[slug] },
		AllProgress: func() map[string][]string { return d.config().Serve.EvolutionProgress },
		UpdateProgress: func(slug string, completed []string) error {
			current := d.config()
			if err := current.SaveEvolutionProgress(*cfgPath, slug, completed); err != nil {
				return fmt.Errorf("gravando progresso: %w", err)
			}
			if current.Serve.EvolutionProgress == nil {
				current.Serve.EvolutionProgress = map[string][]string{}
			}
			if len(completed) == 0 {
				delete(current.Serve.EvolutionProgress, slug)
			} else {
				current.Serve.EvolutionProgress[slug] = completed
			}
			d.setConfig(current)
			return nil
		},
		Update: func(v config.UISettings) (config.UISettings, error) {
			current := d.config()
			if err := rejectEnvEdits(current.Editable(), v); err != nil {
				return config.UISettings{}, err
			}
			if err := current.ApplyEditable(v); err != nil {
				return config.UISettings{}, err
			}
			if err := current.SaveEditable(*cfgPath, current.Editable()); err != nil {
				return config.UISettings{}, fmt.Errorf("gravando configuração: %w", err)
			}
			d.setConfig(current)
			apiSrv.History = current.Serve.RetentionDays
			apiSrv.EvolutionMinRating = current.Serve.CardsMinRating
			apiSrv.EvolutionExtraBudget = current.Market.ExtraBudget
			apiSrv.MarketReserve = current.Market.Reserve
			return current.Editable(), nil
		},
		EnvLocked: envLockedSettings(),
	}
	if host := listenHost(); host != "127.0.0.1" && host != "localhost" && host != "::1" {
		apiSrv.PairingToken = strings.TrimSpace(os.Getenv("EAFC_LAN_PAIRING_TOKEN"))
		if apiSrv.PairingToken == "" {
			return fmt.Errorf("EAFC_LISTEN_HOST fora de loopback exige EAFC_LAN_PAIRING_TOKEN; mantenha 127.0.0.1 para uso local")
		}
	}
	return serveHTTP(ctx, cfg, dist, apiSrv, *open)
}

func restoreLastSuccess(ctx context.Context, st store.Store, cycle string) *time.Time {
	snap, ok, err := st.LatestSnapshot(ctx, cycle)
	if err != nil || !ok || snap.GeneratedAt.IsZero() {
		return nil
	}
	last := snap.GeneratedAt
	return &last
}

func envLockedSettings() []string {
	locked := []string{}
	if os.Getenv("EAFC_BUDGET") != "" {
		locked = append(locked, "market.extra_budget")
	}
	return locked
}

func splitFavorites(value string) []string {
	var out []string
	for _, item := range strings.Split(value, ",") {
		if item = strings.TrimSpace(item); item != "" {
			out = append(out, item)
		}
	}
	return out
}

func rejectEnvEdits(current, next config.UISettings) error {
	if os.Getenv("EAFC_BUDGET") != "" && current.Market.ExtraBudget != next.Market.ExtraBudget {
		return fmt.Errorf("market.extra_budget é controlado por EAFC_BUDGET; altere a variável de ambiente")
	}
	return nil
}

// serveDemo grava o mesmo snapshot fictício do `demo` num store JSON
// temporário (nunca no .eafc-bot/ de verdade) e serve as telas em cima
// dele — mesma API do modo real, sem rede.
func serveDemo(ctx context.Context, cfg config.Config, dist fs.FS, open bool) error {
	dir, err := os.MkdirTemp("", "eafcbot-demo-*")
	if err != nil {
		return err
	}
	st, err := store.NewJSON(dir)
	if err != nil {
		return err
	}
	defer st.Close()

	snap := demoSnapshot(rand.New(rand.NewSource(26)))
	cfg.GamerTag = snap.Club.GamerTag

	// Semeia o que o ciclo de coleta rápido (scheduler.FastTicker) deixaria
	// salvo no Store de verdade — analyzeAndBuild lê os dois de lá, não do
	// snapshot. SaveSBCCost só entra com 1 amostra aqui (sem dedupe pra
	// burlar), então a fase de cada sinal de fodder sai "recente" neste
	// modo — a variedade de fase (pico/esfriando) já é demonstrada pelo
	// `eafcbot demo` (HTML), que monta a tendência na mão.
	if err := st.SaveMomentum(ctx, cfg.FutGG.Cycle, demoMomentum()); err != nil {
		return err
	}
	if err := st.SaveSBCCost(ctx, cfg.FutGG.Cycle, snap.SBCs); err != nil {
		return err
	}

	agora := time.Now()

	// Watchlist, ledger e 29 dias de preço ANTES de analyzeAndBuild — nessa
	// ordem de propósito, por dois motivos:
	//  1. analyzeAndBuild lê o ledger (Committed, pro orçamento disponível)
	//     e o histórico de preço (Trends) pra calcular Upgrades/Evolutions;
	//     semear depois faria a primeira análise ignorar os dois.
	//  2. demoSeedPriceHistory precisa rodar ANTES do SavePrices que
	//     analyzeAndBuild dispara sozinho pro mercado — ver o comentário da
	//     função pra por quê (a mesma trava de 1h de SavePricesAt morde dos
	//     dois lados, dependendo da ordem).
	for _, entry := range demoWatchlist() {
		if err := st.UpsertWatchlist(ctx, cfg.FutGG.Cycle, entry); err != nil {
			return fmt.Errorf("semeando watchlist de demo: %w", err)
		}
	}
	for _, entry := range demoLedger(agora) {
		if err := st.AppendLedger(ctx, cfg.FutGG.Cycle, entry); err != nil {
			return fmt.Errorf("semeando ledger de demo: %w", err)
		}
	}
	if err := demoSeedPriceHistory(ctx, st, cfg.FutGG.Cycle, agora); err != nil {
		return err
	}

	gauntletPlan := analyze.BuildGauntletPlanWithOptions(snap.Club, analyze.GauntletOptions{ChemistryModel: cfg.ChemistryModel()})
	data, err := analyzeAndBuild(ctx, cfg, st, snap, agora, false, demoCardReports(snap.Club), gauntletPlan)
	if err != nil {
		return err
	}

	// Snapshots dos 29 dias ANTERIORES, com Nota/Saldo derivando até o valor
	// de hoje — precisa de data.SquadScore, por isso só depois daqui.
	if err := demoHistoricalSnapshots(ctx, st, cfg.FutGG.Cycle, agora, data.SquadScore, snap.Club.Coins); err != nil {
		return err
	}

	fmt.Println("modo demo: dados fictícios, sem rede — a análise mostra paths")
	fmt.Println("confirmados, alternativos, sem path, falha de coleta e um grafo com")
	fmt.Println("ramificação e reencontro; 30 dias de preço/nota, watchlist, extrato,")
	fmt.Println("tarefas concluídas e paths salvos já vêm semeados.")
	demoCfg := cfg
	// Progresso de evolução (PUT /api/evolucoes/{slug}/progresso) é uma
	// anotação local por slug+nome — os nomes batem com os Chain de
	// demoCardReports/demoBranchingCardReport de propósito, senão o
	// checklist do Workbench nunca bateria com nada.
	demoCfg.Serve.EvolutionProgress = map[string][]string{
		"osimhen-88": {"Ponta Explosiva"},
		"j-david-88": {"Caçador de Área", "Artilheiro Implacável", "Matador de Área"},
		"yildiz-79":  {"Ala Veloz"},
	}
	apiSrv := &api.Server{
		Store: st, Cycle: cfg.FutGG.Cycle, History: cfg.Serve.RetentionDays,
		EvolutionMinRating: cfg.Serve.CardsMinRating, EvolutionExtraBudget: cfg.Market.ExtraBudget,
		MarketReserve:  cfg.Market.Reserve,
		UpgradeMinGain: demoCfg.Report.MinGain, UpgradeAllowOutOfPos: demoCfg.Report.AllowOutOfPos,
		UpgradeAllowUnpriced: demoCfg.Report.AllowUnpriced,
		ChemistryModel:       cfg.ChemistryModel(),
		EvaluationContext: func() domain.ContextoAvaliacao {
			fonte := domain.FonteAvaliacao(demoCfg.Evaluation.ExternalSource)
			if demoCfg.Evaluation.UseBot {
				fonte = domain.FonteBot
			}
			return domain.ContextoAvaliacao{Fonte: fonte, Perfil: demoCfg.Evaluation.Profile, Ciclo: demoCfg.FutGG.Cycle, Patch: demoCfg.Evaluation.Patch, Plataforma: demoCfg.Platform, EstiloJogo: demoCfg.Evaluation.PlayStyle}
		},
		CacheTTL: 10 * time.Second,
		Trigger:  func() {}, // não há job de verdade para acionar no demo
		Status: func() api.JobStatus {
			st := api.JobStatus{DailyAt: demoCfg.Serve.DailyAt}
			if next, err := scheduler.Next(time.Now(), demoCfg.Serve.DailyAt); err == nil {
				st.NextRun = &next
			}
			return st
		},
		EvolutionAdvisor: advisor.Func(func(_ context.Context, _ []byte) (advisor.AnalysisResult, error) {
			return advisor.AnalysisResult{
				Verdict:       "situacional",
				Summary:       "No demo, a evolução parece interessante quando o jogador cobre uma lacuna do XI.",
				Strengths:     []string{"ganho de atributos confirmado no catálogo"},
				Risks:         []string{"confirme os objetivos antes de gastar recursos"},
				BestPositions: []string{"ST", "CAM"},
				Sources:       []advisor.Source{{Title: "Evoluções no fut.gg", URL: "https://www.fut.gg/evolutions/"}},
			}, nil
		}),
	}
	apiSrv.Config = &api.ConfigEditor{
		Get:      func() config.UISettings { return demoCfg.Editable() },
		GetCoins: func() *int { return demoCfg.Market.ManualCoins },
		UpdateCoins: func(coins int) (int, error) {
			demoCfg.Market.ManualCoins = &coins
			if err := demoCfg.Validate(); err != nil {
				return 0, err
			}
			return coins, nil
		},
		GetFavorites: func() []string { return splitFavorites(demoCfg.Serve.EvolutionFavorites) },
		UpdateFavorites: func(favorites []string) error {
			demoCfg.Serve.EvolutionFavorites = strings.Join(favorites, ",")
			return nil
		},
		GetProgress: func(slug string) []string { return demoCfg.Serve.EvolutionProgress[slug] },
		AllProgress: func() map[string][]string { return demoCfg.Serve.EvolutionProgress },
		UpdateProgress: func(slug string, completed []string) error {
			if demoCfg.Serve.EvolutionProgress == nil {
				demoCfg.Serve.EvolutionProgress = map[string][]string{}
			}
			if len(completed) == 0 {
				delete(demoCfg.Serve.EvolutionProgress, slug)
			} else {
				demoCfg.Serve.EvolutionProgress[slug] = completed
			}
			return nil
		},
		Update: func(v config.UISettings) (config.UISettings, error) {
			if err := demoCfg.ApplyEditable(v); err != nil {
				return config.UISettings{}, err
			}
			apiSrv.EvolutionMinRating = demoCfg.Serve.CardsMinRating
			apiSrv.EvolutionExtraBudget = demoCfg.Market.ExtraBudget
			apiSrv.MarketReserve = demoCfg.Market.Reserve
			return demoCfg.Editable(), nil
		},
	}

	// As duas semeaduras abaixo passam pelo handler HTTP de verdade em vez
	// de reimplementar a montagem de agenda/id de path — os dois cálculos
	// (analyze.AcaoID por trás de /api/agenda, evolutionPathID por trás de
	// /api/evolucoes/caminhos) são privados a internal/api de propósito, e
	// reproduzi-los aqui arriscaria os dois divergirem em silêncio. Rodar
	// contra o próprio servidor garante que o dado semeado é exatamente o
	// que a tela veria batendo nessas rotas.
	if err := demoSeedAgendaFeedback(ctx, apiSrv, st, cfg.FutGG.Cycle, 2); err != nil {
		return fmt.Errorf("semeando feedback de demo: %w", err)
	}
	if err := demoSeedSavedPaths(apiSrv, "osimhen-88", "j-david-88"); err != nil {
		return fmt.Errorf("semeando paths salvos de demo: %w", err)
	}

	return serveHTTP(ctx, demoCfg, dist, apiSrv, open)
}

// demoSeedAgendaFeedback marca até `n` ações de "agora" como já aceitas,
// usando o id que o próprio /api/agenda calcula — um id digitado à mão
// tornaria "concluídas" coincidência, não prova de join real com o
// feedback (ver AgendaResponse.Concluidas em internal/api/agenda.go).
func demoSeedAgendaFeedback(ctx context.Context, apiSrv *api.Server, st store.Store, cycle string, n int) error {
	req := httptest.NewRequest(http.MethodGet, "/api/agenda", nil)
	w := httptest.NewRecorder()
	apiSrv.Handler().ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		return fmt.Errorf("consultando agenda: status %d: %s", w.Code, w.Body.String())
	}
	var resp struct {
		Agenda struct {
			Agora []struct {
				ID string `json:"id"`
			} `json:"agora"`
		} `json:"agenda"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		return fmt.Errorf("decodificando agenda: %w", err)
	}
	for i, item := range resp.Agenda.Agora {
		if i >= n {
			break
		}
		entry := domain.DecisionFeedback{
			ID: fmt.Sprintf("demo-feedback-%d", i+1), ActionID: item.ID, Cycle: cycle,
			Status: domain.FeedbackAceita, Reason: "demo: já resolvido", RecordedAt: time.Now(),
		}
		if err := st.AppendFeedback(ctx, cycle, entry); err != nil {
			return err
		}
	}
	return nil
}

// demoSeedSavedPaths salva, para cada slug pedido, o candidato de caminho
// com cadeia de 3 evoluções (ver demoCardReports: só Osimhen tem um
// alternate assim, e David só tem isso como Best) — é o que dá pra
// /api/evolucoes/progresso mostrar "1 de 3" ao lado de "3 de 3" em vez de
// dois caminhos de um passo só. O id vem do próprio /api/evolucoes/caminhos,
// nunca calculado aqui (ver o comentário em serveDemo).
func demoSeedSavedPaths(apiSrv *api.Server, slugs ...string) error {
	req := httptest.NewRequest(http.MethodGet, "/api/evolucoes/caminhos", nil)
	w := httptest.NewRecorder()
	apiSrv.Handler().ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		return fmt.Errorf("consultando caminhos: status %d: %s", w.Code, w.Body.String())
	}
	var resp struct {
		Value []struct {
			CardSlug string `json:"card_slug"`
			Paths    []struct {
				ID        string `json:"id"`
				Potential struct {
					Path struct {
						Chain []string `json:"chain"`
					} `json:"path"`
				} `json:"potential"`
			} `json:"paths"`
		} `json:"value"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		return fmt.Errorf("decodificando caminhos: %w", err)
	}
	want := make(map[string]bool, len(slugs))
	for _, slug := range slugs {
		want[slug] = true
	}
	for _, row := range resp.Value {
		if !want[row.CardSlug] {
			continue
		}
		for _, path := range row.Paths {
			if len(path.Potential.Path.Chain) != 3 {
				continue
			}
			body, _ := json.Marshal(map[string]string{"path_id": path.ID})
			saveReq := httptest.NewRequest(http.MethodPost, "/api/evolucoes/caminhos/salvos", bytes.NewReader(body))
			saveW := httptest.NewRecorder()
			apiSrv.Handler().ServeHTTP(saveW, saveReq)
			if saveW.Code != http.StatusOK {
				return fmt.Errorf("salvando path de %s: status %d: %s", row.CardSlug, saveW.Code, saveW.Body.String())
			}
			break
		}
	}
	return nil
}

func serveHTTP(ctx context.Context, cfg config.Config, dist fs.FS, apiSrv *api.Server, open bool) error {
	mux := http.NewServeMux()
	mux.Handle("/api/", apiSrv.Handler())
	mux.Handle("/", spaHandler(dist))

	// O binário nativo fica em loopback por padrão: sem autenticação, uma API
	// que escuta na LAN é uma superfície desnecessária. O compose opta
	// explicitamente por 0.0.0.0 dentro do container via EAFC_LISTEN_HOST e
	// ainda publica a porta somente em 127.0.0.1 no host.
	addr := fmt.Sprintf("%s:%d", listenHost(), cfg.Serve.Port)
	srv := &http.Server{Addr: addr, Handler: mux}

	errCh := make(chan error, 1)
	go func() { errCh <- srv.ListenAndServe() }()

	// Uma falha de bind (porta ocupada, por exemplo) chega quase na hora —
	// vale a pena esperar um instante pra devolver um erro claro em vez de
	// deixar o navegador abrir sozinho contra um servidor que já morreu.
	select {
	case err := <-errCh:
		return fmt.Errorf("subindo o servidor em %s: %w", addr, err)
	case <-time.After(200 * time.Millisecond):
	}

	url := fmt.Sprintf("http://127.0.0.1:%d", cfg.Serve.Port)
	fmt.Printf("\nservindo em %s — Ctrl+C para parar\n", url)
	if open {
		if err := openBrowser(url); err != nil {
			fmt.Printf("  (não consegui abrir o navegador sozinho: %v — acesse %s)\n", err, url)
		}
	}

	select {
	case <-ctx.Done():
	case err := <-errCh:
		return fmt.Errorf("servidor caiu: %w", err)
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	fmt.Println("\nparando o servidor...")
	return srv.Shutdown(shutdownCtx)
}

func listenHost() string {
	if host := strings.TrimSpace(os.Getenv("EAFC_LISTEN_HOST")); host != "" {
		return host
	}
	return "127.0.0.1"
}

// daemon guarda o estado do job diário para a API/UI consultarem. A trava
// evita que o botão "Atualizar agora" empilhe uma segunda coleta em cima de
// uma que já está rodando — a mais recente não "cancela" a que já foi.
type daemon struct {
	cfg   config.Config
	store store.Store

	mu    sync.Mutex
	cfgMu sync.RWMutex
	st    api.JobStatus
}

func (d *daemon) config() config.Config {
	d.cfgMu.RLock()
	defer d.cfgMu.RUnlock()
	return d.cfg
}

func (d *daemon) setConfig(cfg config.Config) {
	d.cfgMu.Lock()
	d.cfg = cfg
	d.cfgMu.Unlock()
}

func (d *daemon) status() api.JobStatus {
	d.mu.Lock()
	st := d.st
	d.mu.Unlock()
	// NextRun mora aqui, não em internal/api, porque só cmd conhece
	// internal/scheduler — api.JobStatus é só o envelope.
	dailyAt := d.config().Serve.DailyAt
	st.DailyAt = dailyAt
	if next, err := scheduler.Next(time.Now(), dailyAt); err == nil {
		st.NextRun = &next
	}
	return st
}

func (d *daemon) run(ctx context.Context) {
	d.mu.Lock()
	if d.st.Running {
		d.mu.Unlock()
		return
	}
	d.st.Running = true
	d.st.LastError = ""
	started := time.Now()
	d.st.LastStarted = &started
	d.mu.Unlock()

	cfg := d.config()
	_, err := runJob(ctx, cfg, d.store, "", false)

	d.mu.Lock()
	d.st.Running = false
	if err != nil {
		// LastError vai para /api/job, visível na UI — a mensagem não pode
		// ecoar DSN nem cookie de sessão (ver config.Config.RedactSecrets).
		d.st.LastError = cfg.RedactSecrets(err.Error())
	} else {
		success := time.Now()
		d.st.LastSuccess = &success
	}
	d.mu.Unlock()

	if err != nil {
		fmt.Fprintf(os.Stderr, "erro na coleta: %s\n", cfg.RedactSecrets(err.Error()))
	}
}

// spaHandler serve o arquivo pedido quando ele existe no build (JS, CSS,
// imagens); senão devolve index.html. É o que faz uma rota do React Router
// como /time/26-84161408 funcionar num F5 direto — sem isto o servidor
// devolveria 404 pra qualquer caminho que não seja literalmente um arquivo
// do build, porque quem resolve essa rota é o JavaScript no navegador, não
// o servidor.
func spaHandler(dist fs.FS) http.Handler {
	fileServer := http.FileServer(http.FS(dist))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		name := r.URL.Path
		if len(name) > 0 && name[0] == '/' {
			name = name[1:]
		}
		if name == "" {
			name = "index.html"
		}
		if _, err := fs.Stat(dist, name); err == nil {
			fileServer.ServeHTTP(w, r)
			return
		}
		// Rota do React Router (ex.: /time/...): não existe como arquivo no
		// build. NÃO delega pro http.FileServer pra servir o index.html —
		// testado ao vivo, ele devolve 301 com "Location: ./" pra qualquer
		// pedido cujo caminho resolvido seja literalmente "index.html" (é o
		// próprio stdlib evitando expor esse nome na URL; ver
		// net/http.serveFile). Aqui é exatamente o oposto do que se quer: a
		// URL TEM que continuar /time/... pro React Router, já carregado,
		// saber qual página mostrar. Serve o arquivo cru em vez disso.
		data, err := fs.ReadFile(dist, "index.html")
		if err != nil {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Write(data)
	})
}

// openBrowser abre a URL no navegador padrão, sem depender de nada além da
// biblioteca padrão. exec.Command não passa por shell nenhum — a URL vai
// como argumento único do processo, não interpolada numa linha de comando.
func openBrowser(url string) error {
	switch runtime.GOOS {
	case "windows":
		return exec.Command("rundll32", "url.dll,FileProtocolHandler", url).Start()
	case "darwin":
		return exec.Command("open", url).Start()
	default:
		return exec.Command("xdg-open", url).Start()
	}
}
