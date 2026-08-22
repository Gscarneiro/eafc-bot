package futgg

import "testing"

// O fut.gg serve as evoluções embrulhadas junto com a carta do jogador. Lida
// de fora, a lista é de CARTAS; a evolução só existe um nível abaixo, em
// "evolution". O autoconfig grava esse caminho como "data[].evolution", e é
// aqui que ele tem que ser seguido — senão o parser lê cartas achando que
// são evoluções e o briefing recomenda evolução que não existe.
const evoAninhado = `{"currentPage":1,"totalPages":8,"data":[
 {"overall":91,"position":"ST","commonName":"Mbappé",
  "evolution":{"id":2459,"slug":"2459-futties-finale","name":"FUTTIES Finale",
   "coinsCost":50000,"pointsCost":150,"endTime":"2026-08-30T17:00:00Z",
   "requirementsText":[{"label":"Overall","value":"Max. 97"}],
   "levels":[{"level":1,"upgrades":[{"type":"pace","value":3}]}]}},
 {"overall":86,"position":"CB","commonName":"Saliba",
  "evolution":{"id":2460,"slug":"2460-muralha","name":"Muralha",
   "coinsCost":25000,"pointsCost":0,"endTime":"2026-08-28T17:00:00Z",
   "requirementsText":[{"label":"Overall","value":"Max. 88"}],
   "levels":[{"level":1,"upgrades":[{"type":"defending","value":4}]}]}}]}`

func TestEmbrulhoAninhadoDesceAteORecurso(t *testing.T) {
	c := &Client{cfg: Config{
		BaseURL:  "https://www.fut.gg",
		Cycle:    "26",
		Wrappers: map[string]string{"evolutions": "data[].evolution"},
		FieldMaps: map[string]map[string]string{"evolutions": {
			"coin_cost": "coinsCost", "point_cost": "pointsCost",
			"expires_at": "endTime", "requirements": "requirementsText",
		}},
	}}

	nodes, err := c.decodeList([]byte(evoAninhado), "evolutions")
	if err != nil {
		t.Fatalf("decodeList: %v", err)
	}
	if len(nodes) != 2 {
		t.Fatalf("desceu para %d nós, esperava 2", len(nodes))
	}

	e := mapEvolution(nodes[0], "26", "https://www.fut.gg", c.lensFor("evolutions"))
	if e.Name != "FUTTIES Finale" {
		t.Errorf("nome %q — leu a carta em vez da evolução?", e.Name)
	}
	if e.CoinCost != 50000 {
		t.Errorf("custo %d, esperava 50000", e.CoinCost)
	}
	if len(e.Levels) != 1 {
		t.Errorf("níveis %d, esperava 1", len(e.Levels))
	}
	if len(e.Requirements) != 1 {
		t.Errorf("requisitos %d, esperava 1", len(e.Requirements))
	}
	if e.ExpiresAt.IsZero() {
		t.Error("expiração não foi lida")
	}
	if e.URL != "https://www.fut.gg/evolutions/2459-futties-finale/" {
		t.Errorf("URL %q", e.URL)
	}
}

// Sem o embrulho aninhado o parser cai na detecção genérica e lê a CARTA,
// não a evolução. É a prova de que o caminho aprendido é o que faz a
// diferença — e de que perdê-lo não passa despercebido.
func TestSemOEmbrulhoAninhadoLeriaACarta(t *testing.T) {
	c := &Client{cfg: Config{BaseURL: "https://www.fut.gg", Cycle: "26"}}
	nodes, err := c.decodeList([]byte(evoAninhado), "evolutions")
	if err != nil {
		t.Fatal(err)
	}
	if got := nodes[0].str("name", "commonName"); got == "FUTTIES Finale" {
		t.Fatal("sem o embrulho aninhado não deveria chegar na evolução")
	}
}

// Embrulho simples continua funcionando como antes.
func TestEmbrulhoSimplesContinuaValendo(t *testing.T) {
	c := &Client{cfg: Config{Wrappers: map[string]string{"news": "data"}}}
	nodes, err := c.decodeList([]byte(`{"data":[{"title":"a"},{"title":"b"}]}`), "news")
	if err != nil {
		t.Fatal(err)
	}
	if len(nodes) != 2 || nodes[1].str("title") != "b" {
		t.Fatalf("leu %d nós: %v", len(nodes), nodes)
	}
}

// Embrulho aprendido que não bate mais com a resposta não pode derrubar a
// coleta: cai para a detecção genérica, que ainda acha a lista.
func TestEmbrulhoObsoletoCaiParaADetecaoGenerica(t *testing.T) {
	c := &Client{cfg: Config{Wrappers: map[string]string{"news": "data[].naoExiste"}}}
	nodes, err := c.decodeList([]byte(`{"data":[{"title":"a"}]}`), "news")
	if err != nil {
		t.Fatal(err)
	}
	if len(nodes) != 1 || nodes[0].str("title") != "a" {
		t.Fatalf("leu %d nós: %v", len(nodes), nodes)
	}
}
