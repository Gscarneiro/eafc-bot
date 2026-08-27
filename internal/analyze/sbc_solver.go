package analyze

import (
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/gscarneiro/eafc-bot/internal/domain"
)

// EstadoPlanoSBC descreve o que o motor conseguiu provar. Os nomes distinguem
// explicitamente falta de prova de uma resposta negativa: limite esgotado nao
// transforma a melhor tentativa em otimo nem em inviavel.
type EstadoPlanoSBC string

const (
	SBCOtimoComprovado       EstadoPlanoSBC = "otimo_comprovado"
	SBCMelhorNoLimite        EstadoPlanoSBC = "melhor_no_limite"
	SBCInviavelComprovado    EstadoPlanoSBC = "inviavel_comprovado"
	SBCRequisitoDesconhecido EstadoPlanoSBC = "requisito_desconhecido"
	SBCDadosIndisponiveis    EstadoPlanoSBC = "dados_indisponiveis"
)

type TipoRequisitoSBC string

const (
	SBCMinNota      TipoRequisitoSBC = "min_nota_time"
	SBCMinOrigem    TipoRequisitoSBC = "min_origem"
	SBCMinVersao    TipoRequisitoSBC = "min_versao"
	SBCMinQuimica   TipoRequisitoSBC = "min_quimica"
	SBCMinJogadores TipoRequisitoSBC = "min_jogadores"
)

// RequisitoSBC preserva texto e resultado normalizado para o certificado.
type RequisitoSBC struct {
	Texto  string           `json:"texto"`
	Tipo   TipoRequisitoSBC `json:"tipo,omitempty"`
	Valor  string           `json:"valor,omitempty"`
	Min    int              `json:"min,omitempty"`
	Valido bool             `json:"valido"`
}

// NormalizarRequisitosSBC reconhece TODAS as linhas. Uma linha nova ou
// ambigua fica invalida e bloqueia o solver; isso impede que um texto livre
// vire uma submissao manual errada por inferencia silenciosa.
func NormalizarRequisitosSBC(lines []string) []RequisitoSBC {
	out := make([]RequisitoSBC, 0, len(lines))
	for _, raw := range lines {
		text := strings.TrimSpace(raw)
		r := RequisitoSBC{Texto: text}
		lower := strings.ToLower(text)
		switch {
		case strings.HasPrefix(lower, "min. team rating:"):
			r.Tipo, r.Valor = SBCMinNota, strings.TrimSpace(text[len("Min. Team Rating:"):])
			r.Min, r.Valido = positiveInt(r.Valor)
		case strings.HasPrefix(lower, "min. team chemistry:"):
			r.Tipo, r.Valor = SBCMinQuimica, strings.TrimSpace(text[len("Min. Team Chemistry:"):])
			r.Min, r.Valido = positiveInt(r.Valor)
		case strings.HasPrefix(lower, "number of players in the squad:"):
			r.Tipo, r.Valor = SBCMinJogadores, strings.TrimSpace(text[len("Number of Players in the Squad:"):])
			r.Min, r.Valido = positiveInt(r.Valor)
		case strings.HasPrefix(lower, "min. "):
			body := strings.TrimSpace(text[len("Min. "):])
			parts := strings.SplitN(body, " ", 3)
			if len(parts) == 3 && strings.HasPrefix(strings.ToLower(parts[1]), "players") {
				r.Min, r.Valido = positiveInt(parts[0])
				rest := strings.TrimSpace(parts[2])
				switch {
				case strings.HasPrefix(strings.ToLower(parts[1]), "players") && strings.HasPrefix(strings.ToLower(rest), "from:"):
					r.Tipo, r.Valor = SBCMinOrigem, strings.TrimSpace(rest[len("from:"):])
					r.Valido = r.Valido && r.Valor != ""
				case strings.HasPrefix(strings.ToLower(parts[1]), "players:") && strings.HasPrefix(strings.ToLower(rest), "any "):
					r.Tipo, r.Valor = SBCMinVersao, strings.TrimSpace(rest[len("Any "):])
					r.Valido = r.Valido && r.Valor != ""
				}
			}
		}
		out = append(out, r)
	}
	return out
}

func positiveInt(v string) (int, bool) {
	n, err := strconv.Atoi(strings.TrimSpace(v))
	return n, err == nil && n > 0
}

type SBCItem struct {
	ID        string        `json:"id"`
	Jogador   domain.Player `json:"jogador"`
	Quimica   int           `json:"quimica"`
	Protegido bool          `json:"protegido"`
	Compra    bool          `json:"compra"`
	Elegivel  bool          `json:"elegivel"`
	Razao     string        `json:"razao,omitempty"`
}

type PlanoSBCOpcoes struct {
	MaxNos     int             `json:"max_nos"`
	Timeout    time.Duration   `json:"timeout"`
	Bloqueados map[string]bool `json:"-"`
	// DadosDeEmprestimoConfirmados so pode ser verdadeiro quando a fonte
	// efetivamente trouxe esse estado por copia. O modelo atual nao traz esse
	// sinal, logo o chamador publico o mantem falso e nao recomenda descarte.
	DadosDeEmprestimoConfirmados bool `json:"-"`
}

func DefaultPlanoSBCOpcoes() PlanoSBCOpcoes {
	return PlanoSBCOpcoes{MaxNos: 50_000, Timeout: 750 * time.Millisecond}
}

type CertificadoSBC struct {
	Requisitos     []RequisitoSBC `json:"requisitos"`
	LinhasCobertas bool           `json:"linhas_cobertas"`
	Valido         bool           `json:"valido"`
	Razoes         []string       `json:"razoes"`
}

type PlanoSBC struct {
	Estado           EstadoPlanoSBC `json:"estado"`
	Itens            []SBCItem      `json:"itens,omitempty"`
	Certificado      CertificadoSBC `json:"certificado"`
	NosVisitados     int            `json:"nos_visitados"`
	LimiteAtingido   bool           `json:"limite_atingido"`
	ExportacaoManual []string       `json:"exportacao_manual,omitempty"`
}

// MontarPlanoSBC encontra uma composicao consultiva e deterministica. Nao ha
// caminho desta funcao para EA: a saida e apenas checklist manual validado.
func MontarPlanoSBC(ch domain.SBCChallenge, club domain.Club, market []domain.Player, opt PlanoSBCOpcoes) PlanoSBC {
	reqs := NormalizarRequisitosSBC(ch.RequirementsText)
	cert := CertificadoSBC{Requisitos: reqs, LinhasCobertas: len(reqs) > 0}
	for _, r := range reqs {
		if !r.Valido {
			return PlanoSBC{Estado: SBCRequisitoDesconhecido, Certificado: cert}
		}
	}
	if len(reqs) == 0 || !opt.DadosDeEmprestimoConfirmados {
		return PlanoSBC{Estado: SBCDadosIndisponiveis, Certificado: cert}
	}
	if opt.MaxNos <= 0 {
		opt.MaxNos = DefaultPlanoSBCOpcoes().MaxNos
	}
	if opt.Timeout <= 0 {
		opt.Timeout = DefaultPlanoSBCOpcoes().Timeout
	}

	items := poolSBC(club, market, opt)
	target := 11
	for _, r := range reqs {
		if r.Tipo == SBCMinJogadores {
			target = r.Min
		}
	}
	if len(items) < target {
		return PlanoSBC{Estado: SBCInviavelComprovado, Certificado: cert}
	}

	deadline := time.Now().Add(opt.Timeout)
	best := []SBCItem(nil)
	visited, stopped := 0, false
	var walk func(int, []SBCItem)
	walk = func(start int, chosen []SBCItem) {
		if stopped {
			return
		}
		visited++
		if visited > opt.MaxNos || time.Now().After(deadline) {
			stopped = true
			return
		}
		if len(chosen) == target {
			if valid, _ := ValidarPlanoSBC(reqs, chosen); valid && betterSBC(chosen, best) {
				best = append([]SBCItem(nil), chosen...)
			}
			return
		}
		if len(items)-start < target-len(chosen) {
			return
		}
		for i := start; i < len(items); i++ {
			walk(i+1, append(chosen, items[i]))
			if stopped {
				return
			}
		}
	}
	walk(0, nil)
	if len(best) == 0 {
		state := SBCInviavelComprovado
		if stopped {
			state = SBCDadosIndisponiveis
		}
		return PlanoSBC{Estado: state, Certificado: cert, NosVisitados: visited, LimiteAtingido: stopped}
	}
	valid, reasons := ValidarPlanoSBC(reqs, best)
	cert.Valido, cert.Razoes = valid, reasons
	state := SBCOtimoComprovado
	if stopped {
		state = SBCMelhorNoLimite
	}
	manual := make([]string, 0, len(best)+1)
	manual = append(manual, "Monte manualmente no jogo com estas copias; o bot nao envia SBC.")
	for _, item := range best {
		manual = append(manual, item.Jogador.Name+" ("+strconv.Itoa(item.Jogador.Rating)+")")
	}
	return PlanoSBC{Estado: state, Itens: best, Certificado: cert, NosVisitados: visited, LimiteAtingido: stopped, ExportacaoManual: manual}
}

func poolSBC(club domain.Club, market []domain.Player, opt PlanoSBCOpcoes) []SBCItem {
	out := make([]SBCItem, 0, len(club.Players)+len(market))
	for _, p := range club.Players {
		if p.ClubItemID == "" {
			continue
		} // identidade fisica incompleta nunca pode ser descartada.
		locked := opt.Bloqueados[p.ClubItemID] || p.InSquad || len(p.EvosApplied) > 0
		out = append(out, SBCItem{ID: p.ClubItemID, Jogador: p.Player, Quimica: p.Chemistry, Protegido: locked, Elegivel: !locked, Razao: reasonSBC(locked, p.InSquad, len(p.EvosApplied) > 0)})
	}
	for _, p := range market {
		if p.Price.Tradeable() {
			out = append(out, SBCItem{ID: "mercado-" + strconv.FormatInt(p.ID, 10), Jogador: p, Compra: true, Elegivel: true})
		}
	}
	sort.Slice(out, func(i, j int) bool {
		a, b := out[i], out[j]
		if a.Compra != b.Compra {
			return !a.Compra
		}
		if a.Protegido != b.Protegido {
			return !a.Protegido
		}
		if a.Jogador.Price.Coins != b.Jogador.Price.Coins {
			return a.Jogador.Price.Coins < b.Jogador.Price.Coins
		}
		if a.Jogador.Rating != b.Jogador.Rating {
			return a.Jogador.Rating < b.Jogador.Rating
		}
		return a.ID < b.ID
	})
	filtered := out[:0]
	for _, item := range out {
		if item.Elegivel {
			filtered = append(filtered, item)
		}
	}
	return filtered
}

func reasonSBC(locked, squad, evo bool) string {
	if squad {
		return "titular protegido"
	}
	if evo {
		return "evolucao preservada"
	}
	if locked {
		return "bloqueio manual"
	}
	return ""
}

// ValidarPlanoSBC recalcula tudo sem confiar no caminho do solver.
func ValidarPlanoSBC(reqs []RequisitoSBC, items []SBCItem) (bool, []string) {
	reasons := []string{}
	seen := map[string]bool{}
	for _, i := range items {
		if i.ID == "" || seen[i.ID] || !i.Elegivel {
			reasons = append(reasons, "copia repetida ou inelegivel: "+i.ID)
		}
		seen[i.ID] = true
	}
	for _, r := range reqs {
		if !r.Valido {
			reasons = append(reasons, "requisito desconhecido: "+r.Texto)
			continue
		}
		switch r.Tipo {
		case SBCMinJogadores:
			if len(items) != r.Min {
				reasons = append(reasons, "quantidade de jogadores insuficiente")
			}
		case SBCMinNota:
			total := 0
			for _, i := range items {
				total += i.Jogador.Rating
			}
			if len(items) == 0 || total/len(items) < r.Min {
				reasons = append(reasons, "nota media insuficiente")
			}
		case SBCMinQuimica:
			total := 0
			for _, i := range items {
				total += i.Quimica
			}
			if total < r.Min {
				reasons = append(reasons, "quimica insuficiente")
			}
		case SBCMinOrigem:
			n := 0
			for _, i := range items {
				p := i.Jogador
				if strings.EqualFold(p.Nation, r.Valor) || strings.EqualFold(p.League, r.Valor) || strings.EqualFold(p.Club, r.Valor) {
					n++
				}
			}
			if n < r.Min {
				reasons = append(reasons, "origem insuficiente: "+r.Valor)
			}
		case SBCMinVersao:
			n := 0
			wanted := strings.Split(strings.ToLower(r.Valor), "/")
			for _, i := range items {
				for _, v := range wanted {
					if strings.Contains(strings.ToLower(i.Jogador.Version), strings.TrimSpace(v)) {
						n++
						break
					}
				}
			}
			if n < r.Min {
				reasons = append(reasons, "versao insuficiente: "+r.Valor)
			}
		}
	}
	return len(reasons) == 0, reasons
}

func betterSBC(a, b []SBCItem) bool {
	if len(b) == 0 {
		return true
	}
	metric := func(x []SBCItem) (int, int, int, int) {
		buy, protected, cost, dup := 0, 0, 0, 0
		ids := map[int64]int{}
		for _, i := range x {
			if i.Compra {
				buy++
			}
			if i.Protegido {
				protected++
			}
			cost += i.Jogador.Price.Coins
			ids[i.Jogador.ID]++
			if ids[i.Jogador.ID] > 1 {
				dup++
			}
		}
		return buy, protected, cost, dup
	}
	ab, ap, ac, ad := metric(a)
	bb, bp, bc, bd := metric(b)
	if ab != bb {
		return ab < bb
	}
	if ap != bp {
		return ap < bp
	}
	if ac != bc {
		return ac < bc
	}
	return ad < bd
}
