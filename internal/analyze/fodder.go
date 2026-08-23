package analyze

import (
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/gscarneiro/eafc-bot/internal/domain"
)

// CostTrend é o resumo de tendência de custo que FindFodderDemand recebe
// — mesmo formato de store.PriceTrend (ChangePct/Samples), redeclarado
// AQUI de propósito: internal/store já importa internal/analyze
// (store.Snapshot.Upgrades etc.), então analyze não pode importar store
// sem criar ciclo de import. Quem chama (cmd/eafcbot) converte de
// store.PriceTrend pra este tipo.
type CostTrend struct {
	ChangePct float64
	Samples   int
}

// Fases do ciclo de demanda de um desafio de SBC — nomes exportados pra
// quem monta a UI/relatório não precisar repetir a string crua.
const (
	// PhaseRecent: poucas amostras de custo ainda — cedo demais pra saber
	// se está subindo ou descendo.
	PhaseRecent = "recente"
	// PhasePeak: custo subindo forte. A pesquisa de mercado inverte a
	// leitura ingênua — a demanda pica logo no LANÇAMENTO do SBC, não
	// perto da expiração — então esta fase é "não compre agora", não um
	// gatilho de compra.
	PhasePeak = "pico"
	// PhaseCooling: custo caindo — janela de acumular fodder mais barato.
	PhaseCooling = "esfriando"
	PhaseStable  = "estavel"
	// PhaseDump: perto de expirar. Conselho da pesquisa é vender o fodder
	// que já tem 3-5 dias ANTES da expiração — o crash pós-expiração é
	// consistentemente documentado — não comprar mais nesta janela.
	PhaseDump = "esvaziar"
	// PhaseExpired: já passou da expiração. Provável janela de
	// reabastecer fodder mais barato, mas snap.SBCs normalmente já não
	// lista SBC expirado — fase mantida por defesa, não por expectativa
	// de uso frequente.
	PhaseExpired = "expirado"
)

// ParsedSBCRequirement é o requisito de UM challenge, quando o parser
// best-effort reconhece um dos padrões que o fut.gg publica — visto ao
// vivo em 22/08/2026: "Min. N Players from: X", "Min. Team Rating: N",
// "Min. N Players: Any X". Requisito não reconhecido não vira este tipo;
// fica só como texto cru em FodderSignal.Requirement — mesmo espírito de
// "na dúvida, não afirma" que internal/futgg/map.go's
// parseRequirementText já segue pra evolução.
type ParsedSBCRequirement struct {
	// Kind: "min_from" (nação/liga/clube — o texto não distingue qual),
	// "min_team_rating" (média de squad — PoolSize não se aplica, é
	// requisito de squad, não de carta isolada), ou "min_rarity_count"
	// (contagem de cartas de uma raridade/lista de raridades).
	Kind  string `json:"kind"`
	Value string `json:"value"` // nome da nação/liga/clube, número da nota mínima, ou lista de raridades
	Min   int    `json:"min"`   // N de "Min. N Players..." — 0 quando não se aplica
}

// FodderSignal é uma faixa de requisito de SBC sinalizada como
// interessante — não uma carta específica: v1 fica no sinal agregado
// (custo subindo/descendo, fase, tamanho do pool elegível), sem cruzar
// com o elenco do próprio usuário — isso fica pra uma versão futura.
type FodderSignal struct {
	SBCID       string `json:"sbc_id"`
	SBCName     string `json:"sbc_name"`
	Challenge   string `json:"challenge"`
	Requirement string `json:"requirement"` // texto cru do fut.gg, todas as linhas
	// Parsed é o primeiro requisito reconhecido, quando o parser
	// reconheceu algum padrão — nil quando não reconheceu nenhum; a tela
	// mostra Requirement (texto cru) nesse caso, sem chutar.
	Parsed *ParsedSBCRequirement `json:"parsed,omitempty"`
	// PoolSize é quantas cartas do mercado bateriam Parsed — só
	// computado pra Kind=="min_from" (comparação direta de texto); -1
	// nos outros casos (não computado, não "zero cartas").
	PoolSize      int       `json:"pool_size"`
	CostCoins     int       `json:"cost_coins"` // custo da solução mais barata hoje, já resolvido pelo fut.gg
	CostChangePct float64   `json:"cost_change_pct"`
	Phase         string    `json:"phase"`
	Repeatable    bool      `json:"repeatable"`
	ExpiresAt     time.Time `json:"expires_at"`
	Rationale     []string  `json:"rationale"`
}

// FodderDemandOptions controla os limiares de classificação de fase.
type FodderDemandOptions struct {
	// DumpWithin é a janela antes da expiração em que a fase vira
	// PhaseDump — pesquisa de mercado recomenda vender 3-5 dias antes.
	DumpWithin time.Duration
	// PeakChangePct é o piso de alta no custo pra classificar como pico
	// de demanda (não comprar). CoolingChangePct é o teto de queda pra
	// classificar como esfriando (janela de acumular).
	PeakChangePct    float64
	CoolingChangePct float64
}

// DefaultFodderDemandOptions são padrões conservadores.
func DefaultFodderDemandOptions() FodderDemandOptions {
	return FodderDemandOptions{DumpWithin: 72 * time.Hour, PeakChangePct: 10, CoolingChangePct: -10}
}

// FindFodderDemand cruza os SBCs ativos com a tendência de custo de cada
// challenge (ver store.SaveSBCCost/SBCCostTrend) pra sinalizar qual faixa
// de requisito está esquentando. costTrends é indexado por challengeKey
// (mesma função, mesmo formato de store.SBCChallengeKey — ver o
// comentário de challengeKey abaixo).
func FindFodderDemand(sbcs []domain.SBC, market []domain.Player, costTrends map[string]CostTrend, opt FodderDemandOptions) []FodderSignal {
	var out []FodderSignal
	for _, sbc := range sbcs {
		for idx, ch := range sbc.Challenges {
			if ch.CheapestSolutionCoins <= 0 {
				continue // fut.gg ainda não resolveu a solução mais barata pra esta
			}

			trend, hasTrend := costTrends[challengeKey(sbc.ID, idx, ch.Name)]
			hasTrend = hasTrend && trend.Samples >= 2
			phase, note := classifyPhase(sbc, trend, hasTrend, opt)

			parsed := parseSBCRequirementText(ch.RequirementsText)
			var p *ParsedSBCRequirement
			pool := -1
			if len(parsed) > 0 {
				p = &parsed[0] // v1: só o primeiro requisito reconhecido do challenge
				pool = poolSize(*p, market)
			}

			rationale := []string{note}
			if sbc.Repeatable {
				rationale = append(rationale, "SBC repetível — mais chance de virar piso de preço permanente pra esta faixa, não só um pico passageiro")
			}

			out = append(out, FodderSignal{
				SBCID: sbc.ID, SBCName: sbc.Name, Challenge: ch.Name,
				Requirement:   strings.Join(ch.RequirementsText, " · "),
				Parsed:        p,
				PoolSize:      pool,
				CostCoins:     ch.CheapestSolutionCoins,
				CostChangePct: trend.ChangePct,
				Phase:         phase,
				Repeatable:    sbc.Repeatable,
				ExpiresAt:     sbc.ExpiresAt,
				Rationale:     rationale,
			})
		}
	}

	sort.Slice(out, func(i, j int) bool {
		ri, rj := phaseUrgency(out[i].Phase), phaseUrgency(out[j].Phase)
		if ri != rj {
			return ri < rj
		}
		return out[i].CostChangePct > out[j].CostChangePct
	})
	return out
}

// phaseUrgency ordena a fase que pede ação primeiro: esvaziar (venda
// antes do crash) e pico (não compre) são as que mais importam ver de
// cara; estável é a menos acionável.
func phaseUrgency(phase string) int {
	switch phase {
	case PhaseDump:
		return 0
	case PhasePeak:
		return 1
	case PhaseCooling:
		return 2
	case PhaseRecent:
		return 3
	case PhaseExpired:
		return 5
	default: // PhaseStable
		return 4
	}
}

func classifyPhase(sbc domain.SBC, trend CostTrend, hasTrend bool, opt FodderDemandOptions) (phase string, note string) {
	if !sbc.ExpiresAt.IsZero() && !sbc.ExpiresAt.After(time.Now()) {
		return PhaseExpired, "expirado — provável janela de reabastecer fodder mais barato"
	}
	if sbc.Expiring(opt.DumpWithin) {
		return PhaseDump, "perto de expirar — a demanda historicamente derrete logo depois da expiração; considere vender o fodder que já tem, não comprar mais"
	}
	if !hasTrend {
		return PhaseRecent, "ainda sem tendência de custo (poucas amostras) — cedo pra saber se está subindo ou descendo"
	}
	switch {
	case trend.ChangePct >= opt.PeakChangePct:
		return PhasePeak, "custo da solução mais barata subindo forte — provável pico de demanda logo após o lançamento, não é hora de comprar fodder"
	case trend.ChangePct <= opt.CoolingChangePct:
		return PhaseCooling, "custo da solução mais barata caindo — pode ser janela de acumular fodder mais barato"
	default:
		return PhaseStable, "custo estável"
	}
}

// challengeKey espelha store.SBCChallengeKey — duplicado aqui de
// propósito pelo mesmo motivo de CostTrend (ciclo de import). As duas
// funções TÊM que ficar idênticas; mudar uma sem mudar a outra quebra o
// cruzamento entre o que SaveSBCCost grava e o que FindFodderDemand lê.
func challengeKey(sbcID string, idx int, name string) string {
	if name != "" {
		return sbcID + "#" + name
	}
	return sbcID + "#" + strconv.Itoa(idx)
}

// parseSBCRequirementText tenta reconhecer cada linha de requisito
// independentemente — um challenge pode ter mais de uma linha.
func parseSBCRequirementText(lines []string) []ParsedSBCRequirement {
	var out []ParsedSBCRequirement
	for _, line := range lines {
		if p, ok := parseSBCRequirementLine(line); ok {
			out = append(out, p)
		}
	}
	return out
}

func parseSBCRequirementLine(line string) (ParsedSBCRequirement, bool) {
	line = strings.TrimSpace(line)

	const ratingPrefix = "Min. Team Rating:"
	if strings.HasPrefix(line, ratingPrefix) {
		return ParsedSBCRequirement{
			Kind:  "min_team_rating",
			Value: strings.TrimSpace(strings.TrimPrefix(line, ratingPrefix)),
		}, true
	}

	const minPrefix = "Min. "
	if !strings.HasPrefix(line, minPrefix) {
		return ParsedSBCRequirement{}, false
	}
	fields := strings.SplitN(strings.TrimPrefix(line, minPrefix), " ", 3)
	// O dois-pontos gruda em palavras DIFERENTES nos dois templates vistos
	// ao vivo — "N Players from: X" (dois-pontos depois de "from") vs
	// "N Players: Any X" (dois-pontos direto em "Players") — por isso o
	// HasPrefix em vez de comparar fields[1] com um valor fixo só.
	if len(fields) < 2 || !strings.HasPrefix(fields[1], "Players") {
		return ParsedSBCRequirement{}, false
	}
	n, err := strconv.Atoi(fields[0])
	if err != nil {
		return ParsedSBCRequirement{}, false
	}
	var rest string
	if len(fields) >= 3 {
		rest = fields[2]
	}
	switch {
	case fields[1] == "Players" && strings.HasPrefix(rest, "from:"):
		return ParsedSBCRequirement{
			Kind: "min_from", Min: n,
			Value: strings.TrimSpace(strings.TrimPrefix(rest, "from:")),
		}, true
	case fields[1] == "Players:" && strings.HasPrefix(rest, "Any "):
		return ParsedSBCRequirement{
			Kind: "min_rarity_count", Min: n,
			Value: strings.TrimPrefix(rest, "Any "),
		}, true
	}
	return ParsedSBCRequirement{}, false
}

// poolSize conta quantas cartas do mercado bateriam um requisito
// reconhecido — só pra "min_from" (nação/liga/clube), que é comparação
// direta de texto. "min_team_rating" é requisito de MÉDIA DE SQUAD, não
// de uma carta isolada; "min_rarity_count" (ex.: "Any TOTW/TOTS/FOF") não
// tem uma comparação confiável contra domain.Player.Version sem arriscar
// falso negativo/positivo (abreviação vs. nome completo da raridade) — os
// dois ficam em -1 (não computado) em vez de um número que pareceria
// preciso sem ser.
func poolSize(req ParsedSBCRequirement, market []domain.Player) int {
	if req.Kind != "min_from" {
		return -1
	}
	n := 0
	for _, p := range market {
		if strings.EqualFold(p.Nation, req.Value) ||
			strings.EqualFold(p.League, req.Value) ||
			strings.EqualFold(p.Club, req.Value) {
			n++
		}
	}
	return n
}
