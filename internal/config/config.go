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

	"github.com/gscarneiro/eafc-bot/internal/chemistry"
	"github.com/gscarneiro/eafc-bot/internal/futgg"
)

// Config é o arquivo eafc-bot.json.
type Config struct {
	// GamerTag é o nome do SEU PERFIL NO FUT.GG — não a sua gamertag da EA.
	// São identidades diferentes: o fut.gg deixa você escolher um nome de
	// perfil ao sincronizar o GG Club, e é esse nome que aparece na URL
	// (fut.gg/gg-club/<nome>/). Pode colar a URL inteira aqui; ela é
	// normalizada em Load().
	GamerTag  string        `json:"gamer_tag"`
	Platform  string        `json:"platform"`
	DataDir   string        `json:"data_dir"`
	Postgres  PostgresConf  `json:"postgres"`
	FutGG     futgg.Config  `json:"futgg"`
	Market    MarketConf    `json:"market"`
	Report    ReportConf    `json:"report"`
	Serve     ServeConf     `json:"serve"`
	Chemistry ChemistryConf `json:"chemistry"`
}

// ChemistryConf é a regra de entrosamento que o bot usa para pontuar uma
// escalação. Model é o NOME de um modelo de internal/chemistry, não uma
// tabela de limiares: diferente dos endpoints (que o `autoconfig` DESCOBRE
// da resposta do site), regra de química é inferida contra o que o próprio
// jogo reporta — quem sabe os limiares é o bot, não o usuário. Por isso só
// o nome mora aqui, e só Weight é editável pela UI.
type ChemistryConf struct {
	Model string `json:"model"`

	// Weight é quantos pontos de GG Rating valem 1 ponto de entrosamento na
	// hora de escolher a escalação. 0 = química só é exibida, nunca escolhe.
	// O padrão 0.25 é equilibrado por construção: os 33 pontos inteiros
	// valem 8,25 GG num XI que soma ~990, então a química arredonda a
	// decisão entre cartas próximas e nunca justifica sentar uma carta
	// claramente melhor.
	Weight float64 `json:"weight"`
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

	// FastRefreshMinutes é o intervalo do ciclo de coleta LEVE (momentum,
	// custo de solução de SBC) — bem mais barato que a coleta completa
	// (`runJob`), então vale rodar bem mais vezes por dia: o fut.gg
	// recalcula momentum a cada poucos minutos do lado deles, e a coleta
	// diária sozinha ficaria sempre um dia atrás desse sinal. Zero ou
	// negativo desliga o ciclo rápido (scheduler.FastTicker).
	FastRefreshMinutes int `json:"fast_refresh_minutes"`
	// MomentumWindowHours é a janela pedida à rota de momentum — a rota
	// aceita 6, 12 ou 24 (testado ao vivo em 22/08/2026).
	MomentumWindowHours int `json:"momentum_window_hours"`
	// EvolutionFavorites é uma lista separada por vírgulas para manter Config
	// comparável nos testes de edição atômica e simples no JSON de configuração.
	EvolutionFavorites string `json:"evolution_favorites"`
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
	// Reserve é quanto NUNCA entra em orçamento de compra — moedas guardadas
	// para SBC, evolução ou objetivo que você já decidiu separar. Alimenta
	// domain.Club.Capital diretamente; zero (o padrão) preserva o
	// comportamento anterior a este campo existir.
	Reserve int `json:"reserve"`
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

// UISettings é o subconjunto seguro do config.json que pode ser editado pela
// interface. Credenciais, rotas descobertas, diretórios e banco continuam
// deliberadamente fora deste contrato: a UI é um painel de decisão, não um
// editor de infraestrutura.
type UISettings struct {
	Market    UISettingsMarket    `json:"market"`
	Report    UISettingsReport    `json:"report"`
	Serve     UISettingsServe     `json:"serve"`
	Chemistry UISettingsChemistry `json:"chemistry"`
}

// UISettingsChemistry expõe só o PESO. O modelo (chemistry.model) fica de
// fora de propósito: a UI é painel de decisão, não editor de regra de jogo —
// e é justamente por só parte do bloco ser editável que SaveEditable precisa
// mesclar por chave em vez de substituir o bloco inteiro.
type UISettingsChemistry struct {
	Weight float64 `json:"weight"`
}

type UISettingsMarket struct {
	MinRating   int `json:"min_rating"`
	MaxRating   int `json:"max_rating"`
	MaxPrice    int `json:"max_price"`
	Pages       int `json:"pages"`
	PerPage     int `json:"per_page"`
	ExtraBudget int `json:"extra_budget"`
	Reserve     int `json:"reserve"`
}

type UISettingsReport struct {
	MinGain        float64 `json:"min_gain"`
	TrendWindowHrs int     `json:"trend_window_hours"`
	AllowOutOfPos  bool    `json:"allow_out_of_position"`
	AllowUnpriced  bool    `json:"allow_unpriced"`
}

type UISettingsServe struct {
	DailyAt             string `json:"daily_at"`
	StaleAfterHours     int    `json:"stale_after_hours"`
	RetentionDays       int    `json:"retention_days"`
	CardsMinRating      int    `json:"cards_min_rating"`
	FastRefreshMinutes  int    `json:"fast_refresh_minutes"`
	MomentumWindowHours int    `json:"momentum_window_hours"`
	EvolutionFavorites  string `json:"evolution_favorites"`
}

// Editable devolve apenas os valores que a tela pode mostrar e alterar.
func (c Config) Editable() UISettings {
	return UISettings{
		Market:    UISettingsMarket{MinRating: c.Market.MinRating, MaxRating: c.Market.MaxRating, MaxPrice: c.Market.MaxPrice, Pages: c.Market.Pages, PerPage: c.Market.PerPage, ExtraBudget: c.Market.ExtraBudget, Reserve: c.Market.Reserve},
		Report:    UISettingsReport{MinGain: c.Report.MinGain, TrendWindowHrs: c.Report.TrendWindowHrs, AllowOutOfPos: c.Report.AllowOutOfPos, AllowUnpriced: c.Report.AllowUnpriced},
		Serve:     UISettingsServe{DailyAt: c.Serve.DailyAt, StaleAfterHours: c.Serve.StaleAfterHours, RetentionDays: c.Serve.RetentionDays, CardsMinRating: c.Serve.CardsMinRating, FastRefreshMinutes: c.Serve.FastRefreshMinutes, MomentumWindowHours: c.Serve.MomentumWindowHours, EvolutionFavorites: c.Serve.EvolutionFavorites},
		Chemistry: UISettingsChemistry{Weight: c.Chemistry.Weight},
	}
}

// ApplyEditable aplica e valida uma edição atômica. O receiver só muda depois
// que a configuração inteira continua válida, para um formulário inválido não
// deixar o scheduler ou o próximo job em estado parcial.
func (c *Config) ApplyEditable(v UISettings) error {
	previous := *c
	c.Market.MinRating, c.Market.MaxRating, c.Market.MaxPrice = v.Market.MinRating, v.Market.MaxRating, v.Market.MaxPrice
	c.Market.Pages, c.Market.PerPage, c.Market.ExtraBudget = v.Market.Pages, v.Market.PerPage, v.Market.ExtraBudget
	c.Market.Reserve = v.Market.Reserve
	c.Report.MinGain, c.Report.TrendWindowHrs = v.Report.MinGain, v.Report.TrendWindowHrs
	c.Report.AllowOutOfPos, c.Report.AllowUnpriced = v.Report.AllowOutOfPos, v.Report.AllowUnpriced
	c.Serve.DailyAt, c.Serve.StaleAfterHours, c.Serve.RetentionDays = v.Serve.DailyAt, v.Serve.StaleAfterHours, v.Serve.RetentionDays
	c.Serve.CardsMinRating, c.Serve.FastRefreshMinutes, c.Serve.MomentumWindowHours = v.Serve.CardsMinRating, v.Serve.FastRefreshMinutes, v.Serve.MomentumWindowHours
	c.Serve.EvolutionFavorites = v.Serve.EvolutionFavorites
	c.Chemistry.Weight = v.Chemistry.Weight
	if err := c.Validate(); err != nil {
		*c = previous
		return err
	}
	return nil
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
	// Espelha Platform: Client.SBCs escolhe entre o preço de solução de
	// console e de PC com base nisso, e futgg.Config é um pacote genérico
	// que não lê Config.Platform sozinho.
	fcfg.Platform = "ps5"

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
			Port:                4173,
			DailyAt:             "05:00",
			StaleAfterHours:     20,
			RetentionDays:       30,
			CardsMinRating:      88,
			FastRefreshMinutes:  60,
			MomentumWindowHours: 24,
		},
		Chemistry: ChemistryConf{Model: chemistry.ModeloPadrao().Nome, Weight: 0.25},
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
	// Re-espelha Platform depois do unmarshal: um config.json que só seta
	// "platform" no topo (o normal — ninguém escreve "futgg":{"platform":
	// ...} à mão) deixaria cfg.FutGG.Platform preso no valor de Default(),
	// já que json.Unmarshal só sobrescreve o que o arquivo de fato traz.
	cfg.FutGG.Platform = cfg.Platform
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
	if c.Market.MinRating < 1 || c.Market.MaxRating > 99 || c.Market.MinRating > c.Market.MaxRating {
		return fmt.Errorf("market.min_rating/max_rating inválidos — use uma faixa entre 1 e 99")
	}
	if c.Market.MaxPrice < 0 || c.Market.Pages < 1 || c.Market.PerPage < 1 || c.Market.ExtraBudget < 0 || c.Market.Reserve < 0 {
		return fmt.Errorf("limites do mercado inválidos — preço, páginas, cartas por página, orçamento extra e reserva não podem ser negativos")
	}
	if c.Report.MinGain < 0 || c.Report.TrendWindowHrs < 1 {
		return fmt.Errorf("report.min_gain/trend_window_hours inválidos — use ganho não negativo e uma janela positiva")
	}
	if c.Serve.StaleAfterHours < 1 || c.Serve.RetentionDays < 1 || c.Serve.CardsMinRating < 1 || c.Serve.CardsMinRating > 99 || c.Serve.MomentumWindowHours < 1 || c.Serve.FastRefreshMinutes < 0 {
		return fmt.Errorf("agenda inválida — retenção, atraso, overall de cartas e janela de momentum devem ser positivos")
	}
	if c.Chemistry.Weight < 0 || c.Chemistry.Weight > 5 {
		return fmt.Errorf("chemistry.weight %.2f fora da faixa — use de 0 (química só exibida) a 5 (química manda)", c.Chemistry.Weight)
	}
	if _, err := chemistry.Escolher(c.Chemistry.Model); err != nil {
		return err
	}
	return nil
}

// ChemistryModel resolve o modelo configurado. Nome vazio (config antigo, ou
// servidor de teste) cai no padrão em vez de falhar — Validate é quem recusa
// nome inválido, na hora de carregar.
func (c Config) ChemistryModel() chemistry.Modelo {
	m, err := chemistry.Escolher(c.Chemistry.Model)
	if err != nil {
		return chemistry.ModeloPadrao()
	}
	return m
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

// SaveEditable altera somente os três blocos permitidos pela UI. Ler o JSON
// original antes de gravar é importante: valores vindos de ambiente (cookie,
// DSN e gamer tag) não podem ser reserializados acidentalmente para o arquivo
// só porque uma preferência visual foi salva.
func (c Config) SaveEditable(path string, v UISettings) error {
	var raw map[string]any
	if b, err := os.ReadFile(path); err == nil {
		if err := json.Unmarshal(b, &raw); err != nil {
			return fmt.Errorf("lendo %s para atualização parcial: %w", path, err)
		}
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("abrindo %s para atualização parcial: %w", path, err)
	}
	if raw == nil {
		raw = make(map[string]any)
	}
	// Merge por CHAVE, não substituição do bloco inteiro. `raw["report"] =
	// v.Report` descartava toda chave do bloco que UISettings não carrega —
	// report.output_dir já sumia assim, mascarado porque Default() recoloca o
	// mesmo valor. Com um bloco onde só parte é editável (chemistry: weight
	// pela UI, model não) isso deixaria de ser invisível e viraria perda de
	// configuração de verdade.
	mergeBloco(raw, "market", v.Market)
	mergeBloco(raw, "report", v.Report)
	mergeBloco(raw, "serve", v.Serve)
	mergeBloco(raw, "chemistry", v.Chemistry)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	b, err := json.MarshalIndent(raw, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(b, '\n'), 0o644)
}

// mergeBloco sobrepõe em raw[nome] só as chaves que `bloco` de fato carrega,
// preservando as que já estavam no arquivo. Passa pelo JSON porque é ele que
// define os nomes das chaves (as tags de UISettings*), e não os nomes dos
// campos Go.
func mergeBloco(raw map[string]any, nome string, bloco any) {
	b, err := json.Marshal(bloco)
	if err != nil {
		return
	}
	var novo map[string]any
	if err := json.Unmarshal(b, &novo); err != nil {
		return
	}
	atual, _ := raw[nome].(map[string]any)
	if atual == nil {
		raw[nome] = novo
		return
	}
	for k, v := range novo {
		atual[k] = v
	}
	raw[nome] = atual
}

// RedactSecrets apaga qualquer ocorrência literal de um segredo conhecido
// (DSN do Postgres, cookie de sessão do fut.gg) de uma mensagem antes dela
// chegar ao console ou à API. Existe porque o driver do Postgres, em alguns
// erros de conexão ou de DSN malformada, ecoa a string de conexão inteira de
// volta — e esse erro atravessa openStore/daemon.run até virar texto visível
// (stderr, ou JobStatus.LastError exposto por /api/job). Redigir aqui, no
// ponto onde a mensagem se torna visível, cobre qualquer origem do eco sem
// precisar confiar que cada driver nunca vai vazar a DSN.
func (c Config) RedactSecrets(msg string) string {
	for _, secret := range []string{c.Postgres.DSN, c.FutGG.SessionCookie} {
		if secret == "" {
			continue
		}
		msg = strings.ReplaceAll(msg, secret, "[redigido]")
	}
	return msg
}

// DefaultPath é onde o bot procura a configuração: dentro do repo, não no
// perfil do usuário — ver o comentário de baseDir.
func DefaultPath() string {
	if v := os.Getenv("EAFC_CONFIG"); v != "" {
		return v
	}
	return filepath.Join(baseDir, "config.json")
}
