package domain

import (
	"errors"
	"testing"
)

// card constrói uma carta de teste. O ID fica FIXO de propósito — dentro de
// um caminho de evolução de verdade, o eaId não muda do início ao fim
// (confirmado em internal/futgg/evopaths_test.go): é a mesma cópia de
// clube evoluindo, nunca um item novo. Usar o mesmo ID em toda carta de
// teste prova que o grafo não depende de Card.ID pra identidade de nó —
// quem distingue nós é EvolutionNode.ID.
func card(rating int, cycle string) Player {
	return Player{ID: 999, Rating: rating, Cycle: cycle}
}

// branchGraph tem uma bifurcação na raiz: "raiz" -> "a" e "raiz" -> "b",
// nenhuma delas se reencontra.
func branchGraph(lab bool) EvolutionGraph {
	return EvolutionGraph{
		Cycle:  "26",
		RootID: "raiz",
		Nodes: map[string]EvolutionNode{
			"raiz": {ID: "raiz", Card: card(80, "26")},
			"a":    {ID: "a", Card: card(85, "26")},
			"b":    {ID: "b", Card: card(86, "26")},
		},
		Transitions: []EvolutionTransition{
			{From: "raiz", To: "a", Evolution: "Ramo A", Lab: lab},
			{From: "raiz", To: "b", Evolution: "Ramo B", Lab: lab},
		},
	}
}

func TestEvolutionGraphIsBranchQuandoNoTemMaisDeUmaTransicaoDeSaida(t *testing.T) {
	g := branchGraph(false)
	if !g.IsBranch("raiz") {
		t.Error("nó raiz tem duas transições de saída, deveria ser branch")
	}
	if g.IsBranch("a") {
		t.Error("nó a não tem transição de saída nenhuma, não deveria ser branch")
	}
}

// diamondGraph bifurca na raiz (-> a e -> b) e os dois ramos se reencontram
// no nó "final".
func diamondGraph() EvolutionGraph {
	return EvolutionGraph{
		Cycle:  "26",
		RootID: "raiz",
		Nodes: map[string]EvolutionNode{
			"raiz":  {ID: "raiz", Card: card(80, "26")},
			"a":     {ID: "a", Card: card(85, "26")},
			"b":     {ID: "b", Card: card(86, "26")},
			"final": {ID: "final", Card: card(90, "26")},
		},
		Transitions: []EvolutionTransition{
			{From: "raiz", To: "a", Evolution: "A", CoinsCost: 100},
			{From: "raiz", To: "b", Evolution: "B", CoinsCost: 200},
			{From: "a", To: "final", Evolution: "C", CoinsCost: 10},
			{From: "b", To: "final", Evolution: "D", CoinsCost: 20},
		},
	}
}

func TestEvolutionGraphIsRejoinQuandoDoisCaminhosConvergemNoMesmoNo(t *testing.T) {
	g := diamondGraph()
	if !g.IsRejoin("final") {
		t.Error("nó final recebe dois ramos, deveria ser rejoin")
	}
	if g.IsRejoin("a") {
		t.Error("nó a recebe um ramo só, não deveria ser rejoin")
	}

	paths := g.LinearPaths()
	if len(paths) != 2 {
		t.Fatalf("esperava 2 caminhos raiz->folha (um por ramo), achou %d", len(paths))
	}
	total := 0
	for _, p := range paths {
		total += p.CoinsCost
	}
	// 100+10 (via nó a) + 200+20 (via nó b) = 330 — cada ramo soma o custo
	// do trecho compartilhado (a chegada em "final") de forma
	// independente, sem deduplicar: são caminhos DIFERENTES, mesmo
	// terminando no mesmo nó.
	if total != 330 {
		t.Errorf("soma dos custos dos dois ramos = %d, esperava 330", total)
	}
}

func TestEvolutionGraphTransicaoLabNaoAlteraMecanicaDeTravessia(t *testing.T) {
	g := branchGraph(true)
	if !g.IsBranch("raiz") {
		t.Error("Lab não deveria mudar a detecção de branch")
	}
	paths := g.LinearPaths()
	if len(paths) != 2 {
		t.Fatalf("esperava 2 caminhos (um por escolha do Lab), achou %d", len(paths))
	}
	for _, tr := range g.Transitions {
		if !tr.Lab {
			t.Errorf("transição %q perdeu a flag Lab", tr.Evolution)
		}
	}
}

func TestEvolutionGraphRepeticaoGeraNoNovoNuncaCicloLiteral(t *testing.T) {
	// "Aplicar a evolução de novo" produz um ESTADO novo (nó "nivel2"),
	// nunca volta pro nó "raiz" ou "nivel1" — é assim que repetição é
	// modelada, sem ciclo, mesmo com o Card.ID igual em toda a cadeia.
	g := EvolutionGraph{
		Cycle:  "26",
		RootID: "raiz",
		Nodes: map[string]EvolutionNode{
			"raiz":   {ID: "raiz", Card: card(80, "26")},
			"nivel1": {ID: "nivel1", Card: card(85, "26")},
			"nivel2": {ID: "nivel2", Card: card(90, "26")},
		},
		Transitions: []EvolutionTransition{
			{From: "raiz", To: "nivel1", Evolution: "Repetível", Repeatable: true},
			{From: "nivel1", To: "nivel2", Evolution: "Repetível", Repeatable: true},
		},
	}
	if err := g.Validate(); err != nil {
		t.Fatalf("grafo de repetição sem ciclo literal não deveria falhar Validate: %v", err)
	}
	paths := g.LinearPaths()
	if len(paths) != 1 || len(paths[0].Chain) != 2 {
		t.Fatalf("esperava 1 caminho com 2 etapas repetidas, achou %+v", paths)
	}
}

func TestEvolutionGraphValidateRejeitaCicloComoPayloadDesconhecido(t *testing.T) {
	g := EvolutionGraph{
		Cycle:  "26",
		RootID: "raiz",
		Nodes: map[string]EvolutionNode{
			"raiz": {ID: "raiz", Card: card(80, "26")},
			"a":    {ID: "a", Card: card(85, "26")},
		},
		Transitions: []EvolutionTransition{
			{From: "raiz", To: "a", Evolution: "A"},
			{From: "a", To: "raiz", Evolution: "Volta"}, // ciclo
		},
	}
	err := g.Validate()
	if !errors.Is(err, ErrEvolutionGraphCycle) {
		t.Fatalf("esperava ErrEvolutionGraphCycle, achou %v", err)
	}
}

func TestEvolutionGraphValidateRejeitaTransicaoParaNoInexistente(t *testing.T) {
	g := EvolutionGraph{
		Cycle:  "26",
		RootID: "raiz",
		Nodes: map[string]EvolutionNode{
			"raiz": {ID: "raiz", Card: card(80, "26")},
		},
		Transitions: []EvolutionTransition{
			{From: "raiz", To: "fantasma", Evolution: "Fantasma"},
		},
	}
	err := g.Validate()
	if !errors.Is(err, ErrEvolutionGraphDanglingEdge) {
		t.Fatalf("esperava ErrEvolutionGraphDanglingEdge, achou %v", err)
	}
}

func TestEvolutionGraphValidateRejeitaRootIDInexistente(t *testing.T) {
	g := EvolutionGraph{
		Cycle:  "26",
		RootID: "raiz",
		Nodes: map[string]EvolutionNode{
			"a": {ID: "a", Card: card(85, "26")},
		},
	}
	err := g.Validate()
	if !errors.Is(err, ErrEvolutionGraphNoRoot) {
		t.Fatalf("esperava ErrEvolutionGraphNoRoot, achou %v", err)
	}
}

func TestEvolutionGraphValidateRejeitaNoDeCicloDiferente(t *testing.T) {
	g := EvolutionGraph{
		Cycle:  "26",
		RootID: "raiz",
		Nodes: map[string]EvolutionNode{
			"raiz": {ID: "raiz", Card: card(80, "26")},
			"a":    {ID: "a", Card: card(85, "27")}, // ciclo de jogo diferente
		},
		Transitions: []EvolutionTransition{
			{From: "raiz", To: "a", Evolution: "A"},
		},
	}
	err := g.Validate()
	if !errors.Is(err, ErrEvolutionGraphMixedCycle) {
		t.Fatalf("esperava ErrEvolutionGraphMixedCycle, achou %v", err)
	}
}

// chainGraph é uma cadeia linear de duas etapas: raiz -> meio -> final.
func chainGraph() EvolutionGraph {
	return EvolutionGraph{
		Cycle:  "26",
		RootID: "raiz",
		Nodes: map[string]EvolutionNode{
			"raiz":  {ID: "raiz", Card: card(80, "26")},
			"meio":  {ID: "meio", Card: card(85, "26")},
			"final": {ID: "final", Card: card(90, "26")},
		},
		Transitions: []EvolutionTransition{
			{From: "raiz", To: "meio", Evolution: "Primeira", CoinsCost: 1000, TrainingTime: "3 dias"},
			{From: "meio", To: "final", Evolution: "Segunda", CoinsCost: 2000, TrainingTime: "5 dias", IsExpired: true},
		},
	}
}

func TestLinearPathsSomaCustoEExpiraSeQualquerTrechoDoRamoExpirar(t *testing.T) {
	paths := chainGraph().LinearPaths()
	if len(paths) != 1 {
		t.Fatalf("esperava 1 caminho, achou %d", len(paths))
	}
	p := paths[0]
	if p.CoinsCost != 3000 {
		t.Errorf("custo somado = %d, esperava 3000", p.CoinsCost)
	}
	if !p.IsExpired {
		t.Error("segunda etapa expirou, o caminho inteiro deveria vir marcado como expirado")
	}
	if len(p.Steps) != 3 {
		t.Errorf("esperava 3 cartas na cadeia (raiz + 2 etapas), achou %d", len(p.Steps))
	}
}

func TestLinearPathsJuntaTrainingTimeDosTrechosComMaisDeUmaEtapa(t *testing.T) {
	paths := chainGraph().LinearPaths()
	if len(paths) != 1 {
		t.Fatalf("esperava 1 caminho, achou %d", len(paths))
	}
	want := "3 dias + 5 dias"
	if got := paths[0].TrainingTime; got != want {
		t.Errorf("TrainingTime = %q, esperava %q (nunca somado como duração)", got, want)
	}
}

func TestLinearPathsProgressoParcialRepontaRootParaNoIntermediario(t *testing.T) {
	g := chainGraph()

	full := g.LinearPaths()
	if len(full) != 1 || full[0].CoinsCost != 3000 {
		t.Fatalf("caminho completo deveria custar 3000, achou %+v", full)
	}

	// Usuário já concluiu a primeira etapa: a cópia dele está no nó
	// "meio", não mais no "raiz". Repontar RootID simula esse progresso
	// parcial.
	g.RootID = "meio"
	remaining := g.LinearPaths()
	if len(remaining) != 1 {
		t.Fatalf("esperava 1 caminho restante, achou %d", len(remaining))
	}
	if remaining[0].CoinsCost != 2000 {
		t.Errorf("custo restante = %d, esperava 2000 (só a etapa que falta)", remaining[0].CoinsCost)
	}
	if len(remaining[0].Steps) != 2 {
		t.Errorf("esperava 2 cartas (nó atual + final), achou %d", len(remaining[0].Steps))
	}
}

func TestLinearPathsRaizSemTransicaoDevolveVazio(t *testing.T) {
	g := EvolutionGraph{
		Cycle:  "26",
		RootID: "raiz",
		Nodes: map[string]EvolutionNode{
			"raiz": {ID: "raiz", Card: card(80, "26")},
		},
	}
	paths := g.LinearPaths()
	if len(paths) != 0 {
		t.Errorf("carta já no teto (sem transição nenhuma) não é um caminho, achou %d", len(paths))
	}
}
