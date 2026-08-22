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
	"sync"
	"time"

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

	sched := &scheduler.Scheduler{
		DailyAt:    cfg.Serve.DailyAt,
		StaleAfter: time.Duration(cfg.Serve.StaleAfterHours) * time.Hour,
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

	return serveHTTP(ctx, cfg, dist, &api.Server{
		Store: st, Cycle: cfg.FutGG.Cycle, History: cfg.Serve.RetentionDays,
		Trigger: func() { go d.run(context.Background()) },
		Status:  d.status,
	}, *open)
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

	if _, err := analyzeAndBuild(ctx, cfg, st, snap, time.Now(), false, demoCardReports(snap.Club)); err != nil {
		return err
	}

	fmt.Println("modo demo: dados fictícios, sem rede — a análise carta-a-carta de")
	fmt.Println("verdade precisa do fut.gg, então só Osimhen e Rodri têm CardReport")
	fmt.Println("simulado à mão (/api/time/osimhen-88, /api/time/rodri-89).")
	return serveHTTP(ctx, cfg, dist, &api.Server{
		Store: st, Cycle: cfg.FutGG.Cycle, History: cfg.Serve.RetentionDays,
		Trigger: func() {}, // não há job de verdade para acionar no demo
		Status:  func() api.JobStatus { return api.JobStatus{} },
	}, open)
}

func serveHTTP(ctx context.Context, cfg config.Config, dist fs.FS, apiSrv *api.Server, open bool) error {
	mux := http.NewServeMux()
	mux.Handle("/api/", apiSrv.Handler())
	mux.Handle("/", spaHandler(dist))

	// Escuta em 0.0.0.0 para o docker-compose alcançar o processo de fora
	// do container; é o compose que publica isso só em 127.0.0.1 no host
	// (ver docker-compose.yml) — sem essa distinção o serve não escuta em
	// lugar nenhum que o host acesse.
	addr := fmt.Sprintf("0.0.0.0:%d", cfg.Serve.Port)
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

// daemon guarda o estado do job diário para a API/UI consultarem. A trava
// evita que o botão "Atualizar agora" empilhe uma segunda coleta em cima de
// uma que já está rodando — a mais recente não "cancela" a que já foi.
type daemon struct {
	cfg   config.Config
	store store.Store

	mu sync.Mutex
	st api.JobStatus
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

	_, err := runJob(ctx, d.cfg, d.store, "", false)

	d.mu.Lock()
	d.st.Running = false
	if err != nil {
		d.st.LastError = err.Error()
	} else {
		success := time.Now()
		d.st.LastSuccess = &success
	}
	d.mu.Unlock()

	if err != nil {
		fmt.Fprintf(os.Stderr, "erro na coleta: %v\n", err)
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
