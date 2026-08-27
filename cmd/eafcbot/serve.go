package main

import (
	"context"
	"flag"
	"fmt"
	"io/fs"
	"math/rand"
	"net/http"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/gscarneiro/eafc-bot/internal/analyze"
	"github.com/gscarneiro/eafc-bot/internal/api"
	"github.com/gscarneiro/eafc-bot/internal/config"
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
		ChemistryModel: cfg.ChemistryModel(),
		CacheTTL:       10 * time.Second,
		Trigger:        func() { go d.run(context.Background()) },
		Status:         d.status,
	}
	apiSrv.Config = &api.ConfigEditor{
		Get:          func() config.UISettings { return d.config().Editable() },
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

	gauntletPlan := analyze.BuildGauntletPlanWithOptions(snap.Club, analyze.GauntletOptions{ChemistryModel: cfg.ChemistryModel()})
	if _, err := analyzeAndBuild(ctx, cfg, st, snap, time.Now(), false, demoCardReports(snap.Club), gauntletPlan); err != nil {
		return err
	}

	fmt.Println("modo demo: dados fictícios, sem rede — a análise carta-a-carta de")
	fmt.Println("verdade precisa do fut.gg, então só Osimhen e Rodri têm CardReport")
	fmt.Println("simulado à mão (/api/time/osimhen-88, /api/time/rodri-89).")
	demoCfg := cfg
	apiSrv := &api.Server{
		Store: st, Cycle: cfg.FutGG.Cycle, History: cfg.Serve.RetentionDays,
		EvolutionMinRating: cfg.Serve.CardsMinRating, EvolutionExtraBudget: cfg.Market.ExtraBudget,
		MarketReserve:  cfg.Market.Reserve,
		ChemistryModel: cfg.ChemistryModel(),
		CacheTTL:       10 * time.Second,
		Trigger:        func() {}, // não há job de verdade para acionar no demo
		Status:         func() api.JobStatus { return api.JobStatus{} },
	}
	apiSrv.Config = &api.ConfigEditor{
		Get:          func() config.UISettings { return demoCfg.Editable() },
		GetFavorites: func() []string { return splitFavorites(demoCfg.Serve.EvolutionFavorites) },
		UpdateFavorites: func(favorites []string) error {
			demoCfg.Serve.EvolutionFavorites = strings.Join(favorites, ",")
			return nil
		},
		GetProgress: func(slug string) []string { return demoCfg.Serve.EvolutionProgress[slug] },
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
	return serveHTTP(ctx, demoCfg, dist, apiSrv, open)
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
	defer d.mu.Unlock()
	return d.st
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
