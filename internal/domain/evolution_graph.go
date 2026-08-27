package domain

import (
	"errors"
	"fmt"
	"strings"
	"time"
)

// EvolutionNode é um estado possível de uma carta no grafo de evolução. ID é
// sintético e só precisa ser único DENTRO do grafo — não é Card.ID: uma
// evolução muda o overall/atributos da MESMA cópia, ela nunca ganha um
// Player.ID novo (confirmado ao vivo: dentro de um único caminho retornado
// por EvolutionPaths, todo Step traz o mesmo eaId, do início ao fim — é o
// mesmo item de clube evoluindo, não um item novo). Usar Card.ID como
// identidade de nó colapsaria toda transição num self-loop.
type EvolutionNode struct {
	ID   string `json:"id"`
	Card Player `json:"card"`
}

// EvolutionTransition é uma aresta do grafo: sai de From e chega em To
// (EvolutionNode.ID) aplicando uma ou mais evoluções em sequência.
type EvolutionTransition struct {
	From string `json:"from"`
	To   string `json:"to"`

	// Evolution é o(s) nome(s) da(s) evolução(ões) aplicadas nesta
	// transição, unidos por " → " quando ela resume mais de uma etapa —
	// mesmo vocabulário de EvolutionPath.Chain.
	Evolution string `json:"evolution"`

	CoinsCost  int `json:"coin_cost"`
	PointsCost int `json:"point_cost"`
	// ExpiresAt fica zero nesta entrega: nenhum produtor real preenche um
	// prazo ABSOLUTO ainda — o payload de paths só confirma IsExpired
	// (bool) e TrainingTime (texto formatado). Existe para quando
	// domain.Evolution.ExpiresAt (outro endpoint) puder ser cruzado por
	// nome contra Evolution acima.
	ExpiresAt    time.Time `json:"expires_at"`
	IsExpired    bool      `json:"is_expired"`
	TrainingTime string    `json:"training_time"` // já formatado ("3 days"); nunca parseado/somado como duração

	// Repeatable/Lab são metadado descritivo pedido pelo Gate desta fase.
	// Nenhum produtor real ainda preenche — ficam false até um adapter
	// confirmar o formato do FC 27.
	Repeatable bool `json:"repeatable"`
	Lab        bool `json:"lab"`

	// Source preserva o EvolutionPath original inteiro (Steps
	// intermediários inclusos) quando esta transição resume um payload
	// linear de hoje (ver LinearGraph). nil para transições construídas à
	// mão (fixtures).
	Source *EvolutionPath `json:"source,omitempty"`
}

// EvolutionGraph é a evolução de UM jogador-base, a partir da cópia que o
// usuário tem hoje. RootID é o nó dessa carta — repontar RootID pra um nó
// interior é como "progresso parcial" é representado nesta entrega (sem
// persistência ainda: ver docs/planos/copiloto/03-plano-evolucao-e-workbench.md).
type EvolutionGraph struct {
	Cycle       string                   `json:"cycle"` // "26"/"27", de current.Cycle — Validate rejeita nó de outro ciclo
	RootID      string                   `json:"root_id"`
	Nodes       map[string]EvolutionNode `json:"nodes"`
	Transitions []EvolutionTransition    `json:"transitions"`
}

// Node devolve o nó de um ID, se existir no grafo.
func (g EvolutionGraph) Node(id string) (EvolutionNode, bool) {
	n, ok := g.Nodes[id]
	return n, ok
}

// From lista as transições que SAEM do nó — mais de uma é um branch.
func (g EvolutionGraph) From(id string) []EvolutionTransition {
	var out []EvolutionTransition
	for _, t := range g.Transitions {
		if t.From == id {
			out = append(out, t)
		}
	}
	return out
}

// Into lista as transições que CHEGAM no nó — mais de uma é um rejoin.
func (g EvolutionGraph) Into(id string) []EvolutionTransition {
	var out []EvolutionTransition
	for _, t := range g.Transitions {
		if t.To == id {
			out = append(out, t)
		}
	}
	return out
}

// IsBranch diz se o nó se bifurca: mais de um caminho possível a partir dele.
func (g EvolutionGraph) IsBranch(id string) bool {
	return len(g.From(id)) > 1
}

// IsRejoin diz se o nó é ponto de encontro: mais de um ramo chega nele.
func (g EvolutionGraph) IsRejoin(id string) bool {
	return len(g.Into(id)) > 1
}

var (
	// ErrEvolutionGraphNoRoot é devolvido quando RootID não aponta pra
	// nenhum nó do grafo.
	ErrEvolutionGraphNoRoot = errors.New("grafo de evolução: RootID não existe em Nodes")
	// ErrEvolutionGraphDanglingEdge é devolvido quando uma transição
	// referencia um nó (From ou To) que não existe.
	ErrEvolutionGraphDanglingEdge = errors.New("grafo de evolução: transição referencia nó inexistente")
	// ErrEvolutionGraphCycle é devolvido quando o grafo tem um ciclo
	// alcançável a partir da raiz — inclusive self-loop.
	ErrEvolutionGraphCycle = errors.New("grafo de evolução: ciclo detectado a partir da raiz")
	// ErrEvolutionGraphMixedCycle é devolvido quando um nó pertence a um
	// ciclo de jogo (Player.Cycle) diferente do declarado em EvolutionGraph.Cycle.
	ErrEvolutionGraphMixedCycle = errors.New("grafo de evolução: nó de ciclo de jogo diferente do grafo")
)

// Validate faz as checagens fail-closed do grafo: RootID existe, toda
// transição referencia nó existente, nenhum ciclo é alcançável a partir da
// raiz (self-loop incluso) e nenhum nó pertence a um ciclo de jogo
// diferente do grafo. É a fronteira que barra "payload desconhecido": um
// decoder futuro que não entendeu o formato deve falhar aqui, não produzir
// um grafo torto que alguém tenta atravessar.
func (g EvolutionGraph) Validate() error {
	if _, ok := g.Nodes[g.RootID]; !ok {
		return ErrEvolutionGraphNoRoot
	}
	for _, t := range g.Transitions {
		fromNode, ok := g.Nodes[t.From]
		if !ok {
			return ErrEvolutionGraphDanglingEdge
		}
		toNode, ok := g.Nodes[t.To]
		if !ok {
			return ErrEvolutionGraphDanglingEdge
		}
		if g.Cycle != "" {
			if fromNode.Card.Cycle != "" && fromNode.Card.Cycle != g.Cycle {
				return ErrEvolutionGraphMixedCycle
			}
			if toNode.Card.Cycle != "" && toNode.Card.Cycle != g.Cycle {
				return ErrEvolutionGraphMixedCycle
			}
		}
	}

	// DFS de ancestrais a partir da raiz: se alcançarmos um nó que já está
	// na pilha de ancestrais do caminho atual, é um ciclo.
	byFrom := map[string][]EvolutionTransition{}
	for _, t := range g.Transitions {
		byFrom[t.From] = append(byFrom[t.From], t)
	}
	onStack := map[string]bool{}
	var visit func(id string) error
	visit = func(id string) error {
		if onStack[id] {
			return ErrEvolutionGraphCycle
		}
		onStack[id] = true
		for _, t := range byFrom[id] {
			if err := visit(t.To); err != nil {
				return err
			}
		}
		onStack[id] = false
		return nil
	}
	return visit(g.RootID)
}

// LinearPaths enumera toda caminhada raiz→folha (folha = nó sem transição de
// saída) como EvolutionPath: Steps são as cartas no caminho, Chain são os
// nomes de Evolution de cada transição, CoinsCost/PointsCost somados,
// IsExpired é true se QUALQUER transição do caminho expirou, e TrainingTime
// junta os textos das transições com " + " (nunca soma como duração — o
// fut.gg não documenta esse formato). Não filtra expirado nem ganho — isso
// continua decisão de quem consome (cards.bestPaths, hoje). Raiz sem
// transição de saída devolve slice vazio. Assume grafo já validado
// (Validate já rodou na fronteira); não detecta ciclo sozinha.
func (g EvolutionGraph) LinearPaths() []EvolutionPath {
	byFrom := map[string][]EvolutionTransition{}
	for _, t := range g.Transitions {
		byFrom[t.From] = append(byFrom[t.From], t)
	}

	var out []EvolutionPath
	root, ok := g.Nodes[g.RootID]
	if !ok {
		return nil
	}

	// steps/chain/times são clonados a cada passo (nunca estendidos via
	// append in-place) para que ramos irmãos nunca compartilhem backing
	// array — cada chamada recursiva enxerga sua própria cópia, sem
	// depender da ordem em que os ramos são visitados.
	var walk func(id string, steps []Player, chain []string, coins, points int, expired bool, times []string)
	walk = func(id string, steps []Player, chain []string, coins, points int, expired bool, times []string) {
		outgoing := byFrom[id]
		if len(outgoing) == 0 {
			if len(steps) < 2 {
				return // raiz sem transição nenhuma não é um caminho
			}
			out = append(out, EvolutionPath{
				Steps:        steps,
				Chain:        chain,
				CoinsCost:    coins,
				PointsCost:   points,
				IsExpired:    expired,
				TrainingTime: strings.Join(times, " + "),
			})
			return
		}
		for _, t := range outgoing {
			nextNode, ok := g.Nodes[t.To]
			if !ok {
				continue // dangling: grafo não validado, não afirma nada
			}
			nextSteps := append(append([]Player(nil), steps...), nextNode.Card)
			nextChain := append(append([]string(nil), chain...), t.Evolution)
			nextTimes := times
			if t.TrainingTime != "" {
				nextTimes = append(append([]string(nil), times...), t.TrainingTime)
			}
			walk(t.To, nextSteps, nextChain,
				coins+t.CoinsCost,
				points+t.PointsCost,
				expired || t.IsExpired,
				nextTimes,
			)
		}
	}
	walk(g.RootID, []Player{root.Card}, nil, 0, 0, false, nil)
	return out
}

// LinearGraph converte o retorno confirmado de hoje (futgg.Client.EvolutionPaths)
// num grafo. A raiz é SEMPRE current — nunca path.Steps[0], que a API
// sempre devolve sem GG Rating (ver EvolutionPath.Initial). Cada
// EvolutionPath válido (len(Steps) >= 2 — sem isso não há aresta pra
// formar) vira UMA transição raiz→final carregando o custo/prazo agregado
// que a API realmente confirma, com o path original preservado em Source.
// Não inventa nó nem custo intermediário: o payload de hoje só confirma o
// agregado da cadeia inteira, não uma quebra por evolução. O nó final usa
// um ID sintético ("path-N") em vez de Card.ID — a mesma cópia mantém o
// eaId do início ao fim de um caminho de evolução (ver EvolutionNode), então
// Card.ID não serve pra distinguir raiz de final.
func LinearGraph(current Player, paths []EvolutionPath) EvolutionGraph {
	const rootID = "root"
	g := EvolutionGraph{
		Cycle:  current.Cycle,
		RootID: rootID,
		Nodes:  map[string]EvolutionNode{rootID: {ID: rootID, Card: current}},
	}
	for i := range paths {
		p := paths[i]
		if len(p.Steps) < 2 {
			continue
		}
		final := p.Final()
		nodeID := fmt.Sprintf("path-%d", i)
		g.Nodes[nodeID] = EvolutionNode{ID: nodeID, Card: final}
		g.Transitions = append(g.Transitions, EvolutionTransition{
			From:         rootID,
			To:           nodeID,
			Evolution:    strings.Join(p.Chain, " → "),
			CoinsCost:    p.CoinsCost,
			PointsCost:   p.PointsCost,
			IsExpired:    p.IsExpired,
			TrainingTime: p.TrainingTime,
			Source:       &p,
		})
	}
	return g
}
