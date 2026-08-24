package query

import (
	"net/url"
	"testing"
)

type linhaTeste struct {
	Nome  string
	Pos   string
	Nota  int
	Preco int
	Ativo bool
}

func schemaTeste() Schema[linhaTeste] {
	return NewSchema("teste", "nota desc,nome asc", 2,
		Field[linhaTeste]{Name: "nome", Kind: String, Search: true, Get: func(v linhaTeste) Value { return StringValue(v.Nome) }},
		Field[linhaTeste]{Name: "posicao", Kind: String, Facet: true, Get: func(v linhaTeste) Value { return StringValue(v.Pos) }},
		Field[linhaTeste]{Name: "nota", Kind: Number, Get: func(v linhaTeste) Value { return IntValue(v.Nota) }},
		Field[linhaTeste]{Name: "preco", Kind: Number, Get: func(v linhaTeste) Value { return IntValue(v.Preco) }},
		Field[linhaTeste]{Name: "ativo", Kind: Boolean, Get: func(v linhaTeste) Value { return BoolValue(v.Ativo) }},
	)
}

func TestFiltroCombinaAndOrComParenteses(t *testing.T) {
	options, err := Parse(url.Values{"$filter": {"(posicao eq 'CB' or posicao eq 'ST') and nota ge 85"}})
	if err != nil {
		t.Fatal(err)
	}
	page, err := Apply(schemaTeste(), []linhaTeste{{"A", "CB", 86, 10, true}, {"B", "ST", 84, 20, true}, {"C", "CM", 90, 30, true}}, options)
	if err != nil {
		t.Fatal(err)
	}
	if len(page.Value) != 1 || page.Value[0].Nome != "A" {
		t.Fatalf("filtro inesperado: %+v", page.Value)
	}
}

func TestCampoDesconhecidoListaOsValidos(t *testing.T) {
	options, err := Parse(url.Values{"$filter": {"inexistente eq 1"}})
	if err != nil {
		t.Fatal(err)
	}
	_, err = Apply(schemaTeste(), nil, options)
	qerr, ok := err.(*Error)
	if !ok || len(qerr.ValidFields) != 5 {
		t.Fatalf("erro sem campos válidos: %T %+v", err, err)
	}
}

func TestBuscaIgnoraAcentoECaixa(t *testing.T) {
	options, err := Parse(url.Values{"$search": {"EVOLUCAO"}})
	if err != nil {
		t.Fatal(err)
	}
	page, err := Apply(schemaTeste(), []linhaTeste{{"Evolução rápida", "ST", 90, 1, true}, {"Outro", "CM", 90, 2, true}}, options)
	if err != nil || len(page.Value) != 1 {
		t.Fatalf("busca não ignorou acento/caixa: %+v %v", page.Value, err)
	}
}

func TestFacetasNaoSomemQuandoOFiltroZeraALista(t *testing.T) {
	options, err := Parse(url.Values{"$filter": {"nota gt 999"}})
	if err != nil {
		t.Fatal(err)
	}
	page, err := Apply(schemaTeste(), []linhaTeste{{"A", "CB", 90, 1, true}, {"B", "ST", 80, 2, true}}, options)
	if err != nil || page.Count != 0 || len(page.Facets["posicao"]) != 2 {
		t.Fatalf("facetas deveriam vir da lista original: %+v", page)
	}
}

func TestOrderByMultiplosCriteriosRespeitaAPrioridade(t *testing.T) {
	options, err := Parse(url.Values{"$orderby": {"nota desc,preco asc"}})
	if err != nil {
		t.Fatal(err)
	}
	page, err := Apply(schemaTeste(), []linhaTeste{{"caro", "CB", 90, 20, true}, {"barato", "ST", 90, 10, true}}, options)
	if err != nil || page.Value[0].Nome != "barato" {
		t.Fatalf("ordenação inesperada: %+v", page.Value)
	}
}

func TestTopAcimaDoTetoEhCortado(t *testing.T) {
	options, err := Parse(url.Values{"$top": {"99"}})
	if err != nil {
		t.Fatal(err)
	}
	page, err := Apply(schemaTeste(), []linhaTeste{{"A", "CB", 1, 1, true}, {"B", "ST", 2, 2, true}, {"C", "CM", 3, 3, true}}, options)
	if err != nil || page.Top != 2 || len(page.Value) != 2 {
		t.Fatalf("teto não aplicado: %+v", page)
	}
}

func TestSkipAlemDoFimDevolveListaVaziaNaoErro(t *testing.T) {
	options, err := Parse(url.Values{"$skip": {"99"}})
	if err != nil {
		t.Fatal(err)
	}
	page, err := Apply(schemaTeste(), []linhaTeste{{"A", "CB", 1, 1, true}}, options)
	if err != nil || len(page.Value) != 0 || page.Count != 1 {
		t.Fatalf("skip inválido: %+v %v", page, err)
	}
}

func TestContainsNaoAceitaCampoNoSegundoArgumento(t *testing.T) {
	_, err := Parse(url.Values{"$filter": {"contains(nome, posicao)"}})
	if err == nil {
		t.Fatal("campo no segundo argumento deveria ser rejeitado")
	}
}
