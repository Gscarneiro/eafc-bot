package discover

import "testing"

// carta é uma carta de jogador plausível, para montar as cargas abaixo.
const carta = `{"id":144344,"overall":91,"position":"ST","commonName":"Mbappé",
 "price":845000,"skillMoves":5,"weakFoot":4,
 "faceStatsV2":{"facePace":97,"faceShooting":92,"facePassing":80,
   "faceDribbling":93,"faceDefending":36,"facePhysicality":78}}`

// O fut.gg devolve as evoluções embrulhadas com a carta do jogador. Por fora
// a lista é de cartas — e classificava como "players", com 90% de confiança,
// fazendo o endpoint de evoluções sumir. As duas leituras são legítimas e as
// duas têm que sair.
func TestRespostaPodeServirDoisTiposEmProfundidadesDiferentes(t *testing.T) {
	body := []byte(`{"data":[
 {"overall":91,"position":"ST","commonName":"Mbappé","id":144344,"price":845000,
  "evolution":{"id":2459,"slug":"2459-futties-finale","name":"FUTTIES Finale",
   "coinsCost":50000,"pointsCost":150,"endTime":"2026-08-30T17:00:00Z",
   "levels":[{"level":1}],"requirements":[{"label":"Overall","value":"Max. 97"}]}},
 {"overall":86,"position":"CB","commonName":"Saliba","id":144399,"price":46000,
  "evolution":{"id":2460,"slug":"2460-muralha","name":"Muralha",
   "coinsCost":25000,"pointsCost":0,"endTime":"2026-08-28T17:00:00Z",
   "levels":[{"level":1}],"requirements":[{"label":"Overall","value":"Max. 88"}]}}]}`)

	all := classifyAll(body)

	evo, ok := all["evolutions"]
	if !ok {
		t.Fatalf("não achou as evoluções um nível abaixo (achou: %v)", sortedClassified(all))
	}
	if evo.wrapper != "data[].evolution" {
		t.Errorf("embrulho %q, esperava \"data[].evolution\"", evo.wrapper)
	}
	if evo.fields["coin_cost"] != "coinsCost" {
		t.Errorf("custo em moedas: aprendeu %q", evo.fields["coin_cost"])
	}
	// A leitura de fora continua sendo jogadores — e é assim que deve ser.
	if _, ok := all["players"]; !ok {
		t.Error("a leitura de fora deixou de ser jogadores")
	}
}

// A assinatura de evoluções acumulava evidência em cima de QUALQUER objeto
// com um nome, dois arrays e um inteiro — e assim ganhava de assinaturas
// específicas na própria rota delas. Uma notícia tem "categories" e "tags";
// nenhuma das duas é um nível de evolução.
func TestNoticiaNaoViraEvolucaoPorTerArrays(t *testing.T) {
	samples := []map[string]any{
		{"id": 1235.0, "title": "How to finish the Callum Wilson objective",
			"slug": "how-to-finish-callum-wilson", "date": "2026-08-13",
			"intro":      "Unlock FUTTIES Callum Wilson without wasting coins",
			"categories": []any{map[string]any{"id": 4.0, "name": "Guides"}},
			"tags":       []any{"Pre-Season", "Wilson"}},
		{"id": 1236.0, "title": "Evolutions changing in EA FC 27",
			"slug": "evolutions-changing-fc27", "date": "2026-08-12",
			"intro":      "Pathways, previews and everything else announced",
			"categories": []any{map[string]any{"id": 5.0, "name": "News"}},
			"tags":       []any{"FC27"}},
	}
	kind, score, _ := Classify(samples)
	if kind != "news" {
		t.Fatalf("classificou como %q (%.2f), esperava news", kind, score)
	}
}

// "Contém cartas de jogador" descreve metade da API do fut.gg: /hotness/,
// /live-hub/, evoluções em alta. Um clube é mais que isso — tem moedas,
// escalação, plataforma. Sem essa exigência o autoconfig gravava /hotness/
// como se fosse o clube do usuário.
func TestListaDeCartasEmbrulhadaNaoViraClube(t *testing.T) {
	body := []byte(`{"data":{"items":[` + carta + `,` + carta + `]}}`)
	if kind, score, _, _, _ := classifyBody(body); kind == "club" {
		t.Fatalf("virou clube (%.2f) sem moedas, escalação nem gamertag", score)
	}
}

// O clube de verdade continua sendo reconhecido: o que o separa é o resto
// dos campos, não a lista de cartas.
func TestClubeDeVerdadeContinuaSendoReconhecido(t *testing.T) {
	body := []byte(`{"gamerTag":"carneiro22","platform":"ps5","coins":145000,
"squad":{"name":"Titular","formation":"4-2-3-1"},
"players":[` + carta + `,` + carta + `]}`)
	kind, score, fields, _, _ := classifyBody(body)
	if kind != "club" {
		t.Fatalf("classificou como %q (%.2f), esperava club", kind, score)
	}
	if fields["coins"] != "coins" {
		t.Errorf("moedas: aprendeu %q", fields["coins"])
	}
}

// Uma lista qualquer não é evidência de nada: todo JSON tem listas. Sem o
// NameRequired, "focus" e "characteristics" de uma rota de funções táticas
// viravam os níveis e os requisitos de uma evolução.
func TestArraySemNomeNaoSustentaAssinatura(t *testing.T) {
	samples := []map[string]any{
		{"name": "Wide Back", "cleanName": "Wide Back", "id": 293.0,
			"focus": []any{map[string]any{"x": 1.0}}, "chemistry": []any{"a", "b"}},
		{"name": "Falso Nove", "cleanName": "Falso Nove", "id": 294.0,
			"focus": []any{map[string]any{"x": 2.0}}, "chemistry": []any{"c"}},
	}
	if kind, score, _ := Classify(samples); kind == "evolutions" {
		t.Fatalf("virou evolução (%.2f) só por ter dois arrays e um inteiro", score)
	}
}
