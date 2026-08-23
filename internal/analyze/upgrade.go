package analyze

import (
	"sort"

	"github.com/gscarneiro/eafc-bot/internal/domain"
)

// Upgrade é uma troca concreta sugerida pelo bot: sai um titular, entra
// uma carta do mercado.
type Upgrade struct {
	Slot           domain.Position   `json:"slot"`
	Current        domain.ClubPlayer `json:"current"`
	Candidate      domain.Player     `json:"candidate"`
	CurrentScore   float64           `json:"current_score"`
	CandidateScore float64           `json:"candidate_score"`
	Gain           float64           `json:"gain"`       // ganho na nota da função
	GrossCost      int               `json:"gross_cost"` // preço da carta nova
	Recoup         int               `json:"recoup"`     // o que você levanta vendendo o atual
	NetCost        int               `json:"net_cost"`   // desembolso real
	Profit         int               `json:"profit"`     // sobra, quando o titular vale mais que o reforço
	Efficiency     float64           `json:"efficiency"` // pontos de ganho por 10k coins
	Affordable     bool              `json:"affordable"`
	// Unpriced marca a carta cujo preço não foi possível obter. Ela entra
	// na lista ranqueada só por ganho, e o relatório precisa deixar claro
	// que o custo é desconhecido em vez de mostrar zero.
	Unpriced  bool     `json:"unpriced"`
	Rationale []string `json:"rationale"` // por que essa troca, em texto
}

// UpgradeOptions controla o que conta como sugestão aceitável.
type UpgradeOptions struct {
	MinGain             float64 // ganho mínimo na nota para valer a pena listar
	MaxPerSlot          int     // quantas opções por posição
	IncludeUnaffordable bool    // listar alvos fora do orçamento como "meta"
	Budget              int     // coins disponíveis
	AllowOutOfPos       bool    // aceitar cartas que precisam de card de posição
	// AllowUnpriced deixa entrar carta sem preço conhecido. É o que mantém
	// o relatório útil quando a fonte de dados não entrega cotação — sem
	// isso, "preço zero" reprova tudo e a seção de trocas sai vazia.
	AllowUnpriced bool
}

// DefaultUpgradeOptions são padrões conservadores: só sugere troca que
// muda alguma coisa de verdade em campo.
func DefaultUpgradeOptions(budget int) UpgradeOptions {
	return UpgradeOptions{
		MinGain:             2.0,
		MaxPerSlot:          3,
		IncludeUnaffordable: true,
		Budget:              budget,
		AllowOutOfPos:       false,
	}
}

// UpgradeFunnel conta, carta a carta do mercado, ONDE cada candidata foi
// reprovada. Sem isso uma lista vazia só sabe dizer "não achei nada" e quem
// lê fica adivinhando qual botão mexer — foi exatamente o que a mensagem
// antiga da UI fazia ao culpar o orçamento, que nem chega a filtrar
// (UpgradeOptions.IncludeUnaffordable deixa o inacessível na lista, só
// marcado como Affordable=false).
type UpgradeFunnel struct {
	Considered int `json:"considered"` // len(market)
	Owned      int `json:"owned"`      // você já tem a carta
	SBCOnly    int `json:"sbc_only"`   // exclusiva de SBC, não dá para comprar no mercado
	Unpriced   int `json:"unpriced"`   // sem cotação, com AllowUnpriced desligado
	// OutOfPosition conta carta que não joga em NENHUM slot titular seu —
	// com AllowOutOfPos ligado essa gaveta fica sempre em zero, porque toda
	// carta "cabe" (a penalidade de posição já sai no Score()).
	OutOfPosition int `json:"out_of_position"`
	BelowMinGain  int `json:"below_min_gain"` // jogaria em algum slot, mas não bateu MinGain em nenhum
	// Suggested conta carta que passou do corte em pelo menos um slot — pode
	// ser MAIOR que len(upgrades) devolvido, porque MaxPerSlot ainda decide
	// quais aparecem na lista final.
	Suggested int `json:"suggested"`

	MinGain float64 `json:"min_gain"`
	// BestGain é o maior ganho entre os pares carta×slot REPROVADOS (abaixo
	// de MinGain), incluindo negativo — mede o quão perto (ou longe) o
	// mercado chegou de bater algum titular. Só considera pares em que a
	// carta joga naquela posição; comparar Score() fora de posição não diz
	// nada, porque a penalidade já distorce o número.
	BestGain float64         `json:"best_gain"`
	BestSlot domain.Position `json:"best_slot"`
	BestName string          `json:"best_name"`
	HasBest  bool            `json:"has_best"`
}

// FindUpgrades varre o mercado atrás de trocas melhores para cada titular.
// market deve conter as cartas candidatas já com preço. O UpgradeFunnel
// devolvido junto é o que alimenta a tela "nenhuma sugestão hoje" — ver o
// comentário de UpgradeFunnel.
func FindUpgrades(club domain.Club, market []domain.Player, opt UpgradeOptions) ([]Upgrade, UpgradeFunnel) {
	owned := make(map[int64]bool, len(club.Players))
	for _, p := range club.Players {
		owned[p.ID] = true
	}

	funnel := UpgradeFunnel{Considered: len(market), MinGain: opt.MinGain}

	// Pré-filtro por carta, fora do laço de slot: "já tenho" / "só SBC" /
	// "sem preço" não dependem de posição, então cada candidata do mercado
	// merece UMA resposta, não uma por titular — do contrário essas três
	// checagens rodariam uma vez por titular (11x hoje) e o funil contaria a
	// mesma carta várias vezes.
	type poolCard struct {
		player   domain.Player
		unpriced bool
	}
	pool := make([]poolCard, 0, len(market))
	for _, cand := range market {
		switch {
		case owned[cand.ID]:
			funnel.Owned++
		case cand.Price.IsSBC:
			funnel.SBCOnly++
		case cand.Price.Coins <= 0 && !opt.AllowUnpriced:
			funnel.Unpriced++
		default:
			pool = append(pool, poolCard{player: cand, unpriced: cand.Price.Coins <= 0})
		}
	}
	fitsSomeSlot := make([]bool, len(pool))
	passedGain := make([]bool, len(pool))

	var out []Upgrade

	// Varre o XI titular por SLOT FÍSICO, não por posição lógica: uma
	// formação como 4-4-1-1 repete CB e CM, e club.Starter(pos) só devolve
	// um — o pior dos dois. Iterando Squad.Starters direto, os dois CB são
	// comparados contra o mercado independentemente, e o mais forte também
	// pode receber sugestão de reforço.
	for _, squadSlot := range club.Squad.Starters {
		current, ok := club.PlayerByID(squadSlot.PlayerID)
		if !ok {
			continue
		}
		slot := squadSlot.Position
		curScore := Score(current.Player, slot)
		recoup := current.SellValue()

		var perSlot []Upgrade
		for i, pc := range pool {
			cand := pc.player
			if !opt.AllowOutOfPos && !cand.PlaysAt(slot) {
				continue
			}
			fitsSomeSlot[i] = true

			candScore := Score(cand, slot)
			gain := candScore - curScore
			if gain < opt.MinGain {
				if !funnel.HasBest || gain > funnel.BestGain {
					funnel.HasBest = true
					funnel.BestGain = gain
					funnel.BestSlot = slot
					funnel.BestName = cand.Display()
				}
				continue
			}
			passedGain[i] = true

			net := cand.Price.Coins - recoup
			profit := 0
			if pc.unpriced {
				// Sem cotação não há como calcular desembolso nem
				// eficiência. Fingir um número aqui seria pior que admitir.
				perSlot = append(perSlot, Upgrade{
					Slot:           slot,
					Current:        current,
					Candidate:      cand,
					CurrentScore:   curScore,
					CandidateScore: candScore,
					Gain:           gain,
					Recoup:         recoup,
					Affordable:     true,
					Unpriced:       true,
					Rationale:      explain(current.Player, cand, slot),
				})
				continue
			}
			if net < 0 {
				profit, net = -net, 0
			}
			affordable := cand.Price.Coins <= opt.Budget+recoup
			if !affordable && !opt.IncludeUnaffordable {
				continue
			}

			// Piso de 2.000 moedas no denominador. Sem ele, qualquer troca
			// que se paga sozinha vira eficiência infinita e um ganho de
			// +2 grátis passa na frente de um +12 por 40k — que é a troca
			// que realmente muda o time.
			const effFloor = 2000.0
			denom := float64(net)
			if denom < effFloor {
				denom = effFloor
			}
			eff := gain / (denom / 10000.0)

			perSlot = append(perSlot, Upgrade{
				Slot:           slot,
				Current:        current,
				Candidate:      cand,
				CurrentScore:   curScore,
				CandidateScore: candScore,
				Gain:           gain,
				GrossCost:      cand.Price.Coins,
				Recoup:         recoup,
				NetCost:        net,
				Profit:         profit,
				Efficiency:     eff,
				Affordable:     affordable,
				Rationale:      explain(current.Player, cand, slot),
			})
		}

		// Melhor custo-benefício primeiro, não o maior ganho bruto:
		// três trocas baratas costumam valer mais que uma cara.
		sort.Slice(perSlot, func(i, j int) bool { return lessUpgrade(perSlot[i], perSlot[j]) })
		if len(perSlot) > opt.MaxPerSlot {
			perSlot = perSlot[:opt.MaxPerSlot]
		}
		out = append(out, perSlot...)
	}

	for i := range pool {
		switch {
		case !fitsSomeSlot[i]:
			funnel.OutOfPosition++
		case !passedGain[i]:
			funnel.BelowMinGain++
		default:
			funnel.Suggested++
		}
	}

	sort.Slice(out, func(i, j int) bool { return lessUpgrade(out[i], out[j]) })
	return out, funnel
}

// lessUpgrade ordena com uma regra só, usada nos dois níveis: primeiro o
// que cabe no bolso, depois o que tem preço conhecido (dá para raciocinar
// sobre custo), e aí eficiência entre os precificados ou ganho bruto entre
// os sem preço — porque comparar eficiência com desconhecido não faz
// sentido nenhum.
func lessUpgrade(a, b Upgrade) bool {
	if a.Affordable != b.Affordable {
		return a.Affordable
	}
	if a.Unpriced != b.Unpriced {
		return !a.Unpriced
	}
	if a.Unpriced {
		return a.Gain > b.Gain
	}
	return a.Efficiency > b.Efficiency
}

// explain gera as razões em texto de por que a carta nova é melhor,
// comparando atributo a atributo e olhando PlayStyles decisivos.
func explain(cur, cand domain.Player, slot domain.Position) []string {
	rw, ok := WeightsFor(slot)
	if !ok {
		return nil
	}

	type diff struct {
		attr   string
		label  string
		delta  int
		weight float64
	}
	labels := map[string]string{
		"pac": "Ritmo", "sho": "Chute", "pas": "Passe",
		"dri": "Drible", "def": "Defesa", "phy": "Físico",
	}
	if slot.IsGK() {
		labels = map[string]string{
			"pac": "Elasticidade", "sho": "Manejo", "pas": "Chute",
			"dri": "Reflexos", "def": "Velocidade", "phy": "Posicionamento",
		}
	}

	var diffs []diff
	// attrOrder, não `range rw.Weights` direto: Go aleatoriza a ordem de
	// iteração de mapa de propósito, e SliceStable só preserva ordem de
	// empate se a ordem de ENTRADA já for determinística (ver o comentário
	// de attrOrder em roles.go — o mesmo bug que causava flakiness em
	// TestPlayStylePlusDesempata também deixava a lista de "motivos" de uma
	// sugestão sair em ordem diferente entre execuções idênticas).
	for _, attr := range attrOrder {
		d := cand.Attributes.Get(attr) - cur.Attributes.Get(attr)
		if d != 0 {
			diffs = append(diffs, diff{attr, labels[attr], d, rw.Weights[attr]})
		}
	}
	// Ordena pelo impacto ponderado: a maior diferença que IMPORTA na
	// função. Stable porque a ordem de entrada (attrOrder) já é
	// determinística — um empate mantém a ordem de attrOrder.
	sort.SliceStable(diffs, func(i, j int) bool {
		return float64(abs(diffs[i].delta))*diffs[i].weight > float64(abs(diffs[j].delta))*diffs[j].weight
	})

	// Ganhos primeiro: a linha justifica a troca, então liderar por uma
	// perda ("Físico -6 · Ritmo +4") lê como argumento contra ela.
	var out []string
	var worst diff
	for _, d := range diffs {
		if d.delta > 0 {
			if len(out) < 3 {
				out = append(out, labels_fmt(d.label, "+", d.delta))
			}
			continue
		}
		// Guarda só a pior perda ponderada, como ressalva no fim.
		if worst.label == "" ||
			float64(-d.delta)*d.weight > float64(-worst.delta)*worst.weight {
			worst = d
		}
	}

	// PlayStyles decisivos que a carta nova tem e a atual não. Junta antes
	// de ordenar (peso desc., nome asc. como desempate total) em vez de
	// acrescentar direto no `for range rw.KeyPlayStyles` — do contrário a
	// ordem dos "ganha X" na saída dependeria da iteração aleatorizada do
	// mapa (mesma causa do comentário de attrOrder em roles.go).
	type gain struct {
		name string
		w    float64
	}
	var gains []gain
	for name, w := range rw.KeyPlayStyles {
		if w < 2.0 {
			continue
		}
		if cand.HasPlayStyle(name, false) && !cur.HasPlayStyle(name, false) {
			gains = append(gains, gain{name, w})
		}
	}
	sort.Slice(gains, func(i, j int) bool {
		if gains[i].w != gains[j].w {
			return gains[i].w > gains[j].w
		}
		return gains[i].name < gains[j].name
	})
	for _, g := range gains {
		suffix := ""
		if cand.HasPlayStyle(g.name, true) {
			suffix = "+"
		}
		out = append(out, "ganha "+g.name+suffix)
	}

	// A ressalva fecha a linha, para o trade-off ficar visível sem
	// enfraquecer o argumento.
	if worst.label != "" {
		out = append(out, "custa "+labels_fmt(worst.label, "", worst.delta))
	}
	return out
}

func labels_fmt(label, sign string, delta int) string {
	return label + " " + sign + itoa(delta)
}

func itoa(v int) string {
	if v == 0 {
		return "0"
	}
	neg := v < 0
	if neg {
		v = -v
	}
	var buf [12]byte
	i := len(buf)
	for v > 0 {
		i--
		buf[i] = byte('0' + v%10)
		v /= 10
	}
	if neg {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}

func abs(v int) int {
	if v < 0 {
		return -v
	}
	return v
}

// WeakestLinks aponta os titulares que mais destoam do resto do time.
// É por onde começar quando o orçamento é curto.
func WeakestLinks(club domain.Club, n int) []struct {
	Slot     domain.Position
	Player   domain.ClubPlayer
	Score    float64
	GapToAvg float64
} {
	type entry = struct {
		Slot     domain.Position
		Player   domain.ClubPlayer
		Score    float64
		GapToAvg float64
	}
	var all []entry

	// Slot físico, não posição lógica — mesmo motivo de FindUpgrades: uma
	// formação com posição repetida (dois CB) precisa dos dois na lista,
	// senão um deles nunca aparece como "elo mais fraco".
	haveGG := true
	for _, squadSlot := range club.Squad.Starters {
		p, ok := club.PlayerByID(squadSlot.PlayerID)
		if !ok {
			continue
		}
		slot := squadSlot.Position
		all = append(all, entry{Slot: slot, Player: p, Score: Score(p.Player, slot)})
		if p.GGRating <= 0 {
			haveGG = false
		}
	}
	if len(all) == 0 {
		return nil
	}

	// "Mais fraco" usa o GG Rating do fut.gg quando o XI inteiro tem essa
	// nota — é o número que a pessoa já confere no site, não um cálculo
	// deste bot. Score() (a fórmula própria, com pesos por função e bônus
	// de PlayStyle) continua decidindo O QUE FAZER a respeito — quais
	// trocas de mercado valem a pena — mas não decide mais QUEM é o mais
	// fraco: essas são perguntas diferentes, e só a segunda é opinião
	// nossa. Sem GG Rating em algum titular (fonte que não é o GG Club:
	// csv, chrome), cai pro Score() como antes.
	if haveGG {
		var sum float64
		for _, e := range all {
			sum += e.Player.GGRating
		}
		avg := sum / float64(len(all))
		for i := range all {
			all[i].GapToAvg = all[i].Player.GGRating - avg
		}
		sort.Slice(all, func(i, j int) bool { return all[i].Player.GGRating < all[j].Player.GGRating })
	} else {
		var sum float64
		for _, e := range all {
			sum += e.Score
		}
		avg := sum / float64(len(all))
		for i := range all {
			all[i].GapToAvg = all[i].Score - avg
		}
		sort.Slice(all, func(i, j int) bool { return all[i].GapToAvg < all[j].GapToAvg })
	}

	if len(all) > n {
		all = all[:n]
	}
	return all
}
