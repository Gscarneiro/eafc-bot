package futgg

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gscarneiro/eafc-bot/internal/domain"
)

// Os labels abaixo são os que aparecem nos 30 requisitos reais das
// evoluções ativas em 2026-08-21. O formato do fut.gg é
// {"label":"Overall","value":"Max. 97"} — um par de campos que nenhum dos
// formatos antigos (description/type/operator) previa, e por isso tinha um
// Kind vazio para TODO requisito, derrubando toda evolução.
func TestMapRequirementLeOFormatoRealDoFutGG(t *testing.T) {
	casos := []struct {
		label, value string
		wantKind     string
		wantInt      int
		wantStrings  []string
	}{
		{"Overall", "Max. 97", "max_overall", 97, nil},
		{"Overall", "Max. 74", "max_overall", 74, nil},
		{"Max PS", "10", "max_playstyles", 10, nil},
		{"Max PS+", "4", "max_playstyles_plus", 4, nil},
		{"Min PS+", "4", "min_playstyles_plus", 4, nil},
		{"Position", "ST, LW, RW", "position", 0, []string{"ST", "LW", "RW"}},
		{"Position", "CB", "position", 0, []string{"CB"}},
		{"Excluded Position", "CB", "excluded_position", 0, []string{"CB"}},
		{"Rarity", "FUTTIES, FUTTIES Hero, FUTTIES ICON", "rarity", 0,
			[]string{"FUTTIES", "FUTTIES Hero", "FUTTIES ICON"}},
	}
	for _, c := range casos {
		n := node{"label": c.label, "value": c.value}
		got := mapRequirement(n)
		if got.Kind != c.wantKind {
			t.Errorf("label=%q value=%q: Kind=%q, esperava %q", c.label, c.value, got.Kind, c.wantKind)
			continue
		}
		if got.IntValue != c.wantInt {
			t.Errorf("label=%q value=%q: IntValue=%d, esperava %d", c.label, c.value, got.IntValue, c.wantInt)
		}
		if len(c.wantStrings) > 0 {
			if len(got.Strings) != len(c.wantStrings) {
				t.Errorf("label=%q value=%q: Strings=%v, esperava %v", c.label, c.value, got.Strings, c.wantStrings)
				continue
			}
			for i, want := range c.wantStrings {
				if got.Strings[i] != want {
					t.Errorf("label=%q value=%q: Strings[%d]=%q, esperava %q", c.label, c.value, i, got.Strings[i], want)
				}
			}
		}
	}
}

// "Max Pos." aparece nos dados reais mas não deu para confirmar o que
// significa. Fica sem Kind de propósito — meets() trata isso como
// bloqueante, e uma evolução com esse requisito é descartada em vez de
// recomendada com base num palpite.
func TestMapRequirementNaoInventaSemanticaParaMaxPos(t *testing.T) {
	got := mapRequirement(node{"label": "Max Pos.", "value": "5"})
	if got.Kind != "" && got.Kind != "unknown" {
		t.Errorf("Max Pos. virou Kind %q — deveria continuar sem interpretação", got.Kind)
	}
}

// O caminho antigo (description/type/operator) continua funcionando quando
// não há "label" nenhum — é o formato que outra API do fut.gg (ou uma
// futura) poderia usar.
func TestMapRequirementCaminhoAntigoContinuaFuncionando(t *testing.T) {
	n := node{"type": "max_overall", "operator": "lte", "value": 85.0,
		"description": "Max Overall: 85"}
	got := mapRequirement(n)
	if got.Kind != "max_overall" || got.IntValue != 85 {
		t.Errorf("Kind=%q IntValue=%d, esperava max_overall/85", got.Kind, got.IntValue)
	}
}

// levelsReal é o formato de upgrade que o fut.gg realmente entrega: chave
// "upgrade" (não "type"), e dois atributos aparentemente conflitantes no
// mesmo nível — "face_passing" (um dos 6 atributos da carta) e
// "attribute_short_passing" (um dos 35 sub-atributos). Contar os dois juntos
// dobra o ganho de passe estimado.
const levelUpgradesReal = `[
 {"upgrade":"face_shooting","value":10,"maxValue":96},
 {"upgrade":"attribute_short_passing","value":8,"maxValue":90},
 {"upgrade":"weak_foot","value":4,"maxValue":null},
 {"upgrade":"skill_moves","value":4,"maxValue":null},
 {"upgrade":"play_style_plus","value":5,"maxValue":5},
 {"upgrade":"overall","value":5,"maxValue":98}
]`

func TestMapUpgradeLeAChaveUpgradeENaoDobraSubAtributos(t *testing.T) {
	c := &Client{cfg: Config{FieldMaps: map[string]map[string]string{}}}
	c.psTable = map[int]string{5: "Incisive Pass"}
	l := c.lensFor("evolutions")

	nodes := decodeNodeArray(t, levelUpgradesReal)
	ups := make([]domain.EvoUpgrade, len(nodes))
	for i, n := range nodes {
		ups[i] = mapUpgrade(n, l)
	}

	// face_shooting: um dos 6 atributos da carta, com teto declarado.
	if ups[0].Kind != "attribute" || ups[0].Attr != "sho" || ups[0].Amount != 10 || ups[0].MaxValue != 96 {
		t.Errorf("face_shooting virou %+v", ups[0])
	}
	// attribute_short_passing: sub-atributo, tem que ficar de fora do
	// cálculo dos 6 — não pode virar Kind "attribute" com Attr "pas" (que
	// dobraria em cima de um eventual face_passing no mesmo nível).
	if ups[1].Kind != "ignored" {
		t.Errorf("attribute_short_passing virou %q, esperava \"ignored\"", ups[1].Kind)
	}
	if ups[2].Kind != "weak_foot" || ups[2].Amount != 4 {
		t.Errorf("weak_foot virou %+v", ups[2])
	}
	if ups[3].Kind != "skill_moves" || ups[3].Amount != 4 {
		t.Errorf("skill_moves virou %+v", ups[3])
	}
	// play_style_plus: o "value" é o eaId (5), resolvido pela tabela.
	if ups[4].Kind != "playstyle" {
		t.Fatalf("play_style_plus virou Kind %q, esperava \"playstyle\"", ups[4].Kind)
	}
	if ups[4].PlayStyle.Name != "Incisive Pass" || !ups[4].PlayStyle.Plus {
		t.Errorf("play_style_plus resolveu para %+v, esperava Incisive Pass/Plus", ups[4].PlayStyle)
	}
	if ups[5].Kind != "overall" || ups[5].Amount != 5 || ups[5].MaxValue != 98 {
		t.Errorf("overall virou %+v", ups[5])
	}
}

// O teto que o fut.gg declara ("+10, até 96") tem que ser respeitado na
// projeção da carta final — sem ele o bot superestima o ganho e ranqueia
// uma evolução acima de outra por engano.
func TestApplyRespeitaOTetoDeclarado(t *testing.T) {
	p := domain.Player{Rating: 91, Attributes: domain.Attributes{Shooting: 92}}
	evo := domain.Evolution{Levels: []domain.EvoLevel{{Upgrades: []domain.EvoUpgrade{
		{Kind: "attribute", Attr: "sho", Amount: 10, MaxValue: 96},
		{Kind: "overall", Amount: 5, MaxValue: 94},
	}}}}
	out := evo.Apply(p)
	if out.Attributes.Shooting != 96 {
		t.Errorf("finalização %d, esperava travar em 96 (92+10=102 sem teto)", out.Attributes.Shooting)
	}
	if out.Rating != 94 {
		t.Errorf("overall %d, esperava travar em 94 (91+5=96 sem teto)", out.Rating)
	}
}

// decodeNodeArray é o mínimo para virar uma lista de node num teste sem
// depender de decodeList/wrappers.
func decodeNodeArray(t *testing.T, raw string) []node {
	t.Helper()
	var nodes []node
	if err := json.Unmarshal([]byte(raw), &nodes); err != nil {
		t.Fatalf("fixture inválida: %v", err)
	}
	return nodes
}

// eaId 0 é um PlayStyle legítimo do fut.gg ("Finesse Shot"). Uma checagem
// "eaId > 0" para filtrar entradas malformadas dropava esse exatamente, e
// qualquer carta com Finesse Shot ficava com "0" no lugar do nome.
func TestEnsurePlayStylesNaoDescartaEaIdZero(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"data":[
			{"id":75,"eaId":0,"name":"Finesse Shot"},
			{"id":78,"eaId":5,"name":"Incisive Pass"}
		]}`))
	}))
	defer srv.Close()

	c := New(Config{
		BaseURL:   srv.URL,
		Cycle:     "26",
		Endpoints: map[string]string{"playstyles": "/api/fut/playstyles/"},
	})
	c.ensurePlayStyles(context.Background())

	if got := c.psTable[0]; got != "Finesse Shot" {
		t.Errorf("eaId 0 = %q, esperava \"Finesse Shot\"", got)
	}
	if got := c.psTable[5]; got != "Incisive Pass" {
		t.Errorf("eaId 5 = %q, esperava \"Incisive Pass\"", got)
	}
}
