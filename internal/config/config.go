// Package config carrega as preferências do bot de um arquivo JSON, com
// variáveis de ambiente por cima — o padrão que evita segredo no repositório.
package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/gscarneiro/eafc-bot/internal/futgg"
)

// Config é o arquivo eafc-bot.json.
type Config struct {
	// GamerTag é o nome do SEU PERFIL NO FUT.GG — não a sua gamertag da EA.
	// São identidades diferentes: o fut.gg deixa você escolher um nome de
	// perfil ao sincronizar o GG Club, e é esse nome que aparece na URL
	// (fut.gg/gg-club/<nome>/). Pode colar a URL inteira aqui; ela é
	// normalizada em Load().
	GamerTag string       `json:"gamer_tag"`
	Platform string       `json:"platform"`
	DataDir  string       `json:"data_dir"`
	Postgres PostgresConf `json:"postgres"`
	FutGG    futgg.Config `json:"futgg"`
	Market   MarketConf   `json:"market"`
	Report   ReportConf   `json:"report"`
	Serve    ServeConf    `json:"serve"`
}

// ServeConf governa o `eafcbot serve`: a porta da API/UI e o scheduler que
// roda a coleta sozinho, sem precisar de cron externo nem de rodar comando
// na mão todo dia.
type ServeConf struct {
	Port int `json:"port"`
	// DailyAt é o horário da coleta diária, "HH:MM" em 24h, no fuso do
	// processo — ver TZ no docker-compose.yml para o container.
	DailyAt string `json:"daily_at"`
	// StaleAfterHours: se o último snapshot bom já passou disso na subida,
	// o scheduler coleta na hora em vez de esperar o próximo DailyAt.
	StaleAfterHours int `json:"stale_after_hours"`
	// RetentionDays é quantos dias de snapshot ficam guardados — ver
	// store.SnapshotHistory.
	RetentionDays int `json:"retention_days"`
	// CardsMinRating é o piso de overall para a análise carta-a-carta
	// (evolução atual x potencial) que o scheduler paga à noite, uma vez
	// por dia, em vez de sob demanda a cada clique em /time/:slug.
	CardsMinRating int `json:"cards_min_rating"`
}

type PostgresConf struct {
	Enabled bool   `json:"enabled"`
	Driver  string `json:"driver"` // "pgx" ou "postgres"
	DSN     string `json:"dsn"`    // vazio: usa EAFC_DSN
}

// MarketConf limita quais cartas o bot considera como possível reforço.
type MarketConf struct {
	MinRating   int `json:"min_rating"`
	MaxRating   int `json:"max_rating"`
	MaxPrice    int `json:"max_price"`
	Pages       int `json:"pages"`
	PerPage     int `json:"per_page"`
	ExtraBudget int `json:"extra_budget"` // moedas além do saldo, se você pretende vender coisas
}

type ReportConf struct {
	OutputDir      string  `json:"output_dir"`
	MaxRows        int     `json:"max_rows"`
	MinGain        float64 `json:"min_gain"`
	TrendWindowHrs int     `json:"trend_window_hours"`
	AllowOutOfPos  bool    `json:"allow_out_of_position"`
	// AllowUnpriced deixa o relatório sugerir cartas sem cotação, marcadas
	// como "?" e ranqueadas por ganho. Necessário quando os dados vêm das
	// páginas (o preço do fut.gg não é renderizado no servidor).
	AllowUnpriced bool `json:"allow_unpriced"`
}

// baseDir é onde tudo que o bot grava mora por padrão: config, cache,
// histórico e relatórios, todos debaixo de ".eafc-bot" NO DIRETÓRIO ATUAL —
// não em $HOME nem em $TEMP. O bot só é chamado de dentro do repo (é o que
// o README instrui), então "diretório atual" na prática É o repo, e um
// `git clone` novo já nasce com tudo no lugar certo, sem espalhar estado
// pela máquina. ".eafc-bot/" já está no .gitignore.
const baseDir = ".eafc-bot"

// Default é a configuração inicial, já utilizável.
func Default() Config {
	fcfg := futgg.DefaultConfig()
	// futgg.DefaultConfig sozinho não sabe do repo — é um pacote genérico —
	// e por padrão manda o cache para $TEMP. Aqui, que já conhece o baseDir,
	// trazemos o cache para debaixo dele também.
	fcfg.CacheDir = filepath.Join(baseDir, "cache")

	return Config{
		Platform: "ps5",
		DataDir:  baseDir,
		Postgres: PostgresConf{Enabled: false, Driver: "pgx"},
		FutGG:    fcfg,
		Market: MarketConf{
			MinRating: 80,
			MaxRating: 99,
			MaxPrice:  0,
			Pages:     8,
			PerPage:   50,
		},
		Report: ReportConf{
			OutputDir:      filepath.Join(baseDir, "reports"),
			MaxRows:        12,
			MinGain:        2.0,
			TrendWindowHrs: 72,
		},
		Serve: ServeConf{
			Port:            4173,
			DailyAt:         "05:00",
			StaleAfterHours: 20,
			RetentionDays:   30,
			CardsMinRating:  88,
		},
	}
}

// Load lê o arquivo e aplica os overrides de ambiente.
// Um arquivo ausente não é erro: o bot roda no padrão.
func Load(path string) (Config, error) {
	cfg := Default()

	b, err := os.ReadFile(path)
	if err == nil {
		if err := json.Unmarshal(b, &cfg); err != nil {
			return cfg, fmt.Errorf("lendo %s: %w", path, err)
		}
	} else if !os.IsNotExist(err) {
		return cfg, fmt.Errorf("abrindo %s: %w", path, err)
	}

	applyEnv(&cfg)
	cfg.GamerTag = normalizeProfile(cfg.GamerTag)
	return cfg, cfg.Validate()
}

// normalizeProfile aceita o que uma pessoa naturalmente cola: o link inteiro
// da página do clube, o link sem esquema, ou só o nome. As três formas
// produzem o mesmo slug — que é o único que o fut.gg entende:
//
//	https://www.fut.gg/gg-club/BilingualBee/  -> BilingualBee
//	fut.gg/gg-club/BilingualBee               -> BilingualBee
//	BilingualBee                              -> BilingualBee
//
// Sem isto, colar o link inteiro produz um "gamer_tag" que não bate com
// nenhum perfil e o clube volta vazio sem pista do motivo.
func normalizeProfile(v string) string {
	v = strings.TrimSpace(v)
	if v == "" || !strings.Contains(v, "/") {
		return v
	}
	v = strings.TrimPrefix(v, "https://")
	v = strings.TrimPrefix(v, "http://")
	const marker = "gg-club/"
	if i := strings.Index(v, marker); i >= 0 {
		v = v[i+len(marker):]
	}
	if i := strings.IndexAny(v, "/?#"); i >= 0 {
		v = v[:i]
	}
	return strings.TrimSpace(v)
}

// applyEnv deixa segredo e ajuste rápido fora do arquivo — útil para rodar
// em cron ou container sem editar JSON.
func applyEnv(cfg *Config) {
	if v := os.Getenv("EAFC_GAMERTAG"); v != "" {
		cfg.GamerTag = v
	}
	if v := os.Getenv("EAFC_DSN"); v != "" {
		cfg.Postgres.DSN = v
		cfg.Postgres.Enabled = true
	}
	if v := os.Getenv("EAFC_DATA_DIR"); v != "" {
		cfg.DataDir = v
	}
	if v := os.Getenv("EAFC_FUTGG_COOKIE"); v != "" {
		cfg.FutGG.SessionCookie = v
	}
	if v := os.Getenv("EAFC_CYCLE"); v != "" {
		cfg.FutGG.Cycle = v
	}
	if v := os.Getenv("EAFC_BUDGET"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			cfg.Market.ExtraBudget = n
		}
	}
}

func (c Config) Validate() error {
	if c.FutGG.BaseURL == "" {
		return fmt.Errorf("futgg.base_url não pode ser vazio")
	}
	if c.FutGG.Cycle == "" {
		return fmt.Errorf("futgg.cycle não pode ser vazio (use \"26\" ou \"27\")")
	}
	if c.Postgres.Enabled && c.Postgres.DSN == "" {
		return fmt.Errorf("postgres habilitado mas sem DSN (defina postgres.dsn ou a variável EAFC_DSN)")
	}
	if _, err := time.Parse("15:04", c.Serve.DailyAt); err != nil {
		return fmt.Errorf("serve.daily_at %q inválido — use o formato \"05:00\" (HH:MM, 24h)", c.Serve.DailyAt)
	}
	return nil
}

// Save grava a configuração formatada.
func (c Config) Save(path string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	b, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(b, '\n'), 0o644)
}

// DefaultPath é onde o bot procura a configuração: dentro do repo, não no
// perfil do usuário — ver o comentário de baseDir.
func DefaultPath() string {
	if v := os.Getenv("EAFC_CONFIG"); v != "" {
		return v
	}
	return filepath.Join(baseDir, "config.json")
}
