package discover

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// httpFetcher é o mínimo que a descoberta pede.
type httpFetcher struct{ t *testing.T }

func (h httpFetcher) GetRaw(ctx context.Context, url string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	return io.ReadAll(resp.Body)
}

// fakeSite imita um SPA hostil de propósito: as rotas NÃO têm os nomes
// óbvios, e os campos usam abreviações que não estão em nenhum alias do
// parser. Se a descoberta acerta aqui, ela acerta quando o fut.gg mudar.
func fakeSite() *httptest.Server {
	mux := http.NewServeMux()

	page := func(w http.ResponseWriter, _ *http.Request) {
		fmt.Fprint(w, `<!doctype html><html><head>
<script src="/_next/static/chunks/framework-abc.js"></script>
<script src="/_next/static/chunks/pages-xyz.js"></script>
</head><body>carregando</body></html>`)
	}
	for _, p := range []string{"/", "/players/", "/evolutions/", "/sbc/", "/objectives/", "/news/", "/gg-club/"} {
		mux.HandleFunc(p, page)
	}

	// Bundle de framework: só ruído, para provar que é descartado.
	mux.HandleFunc("/_next/static/chunks/framework-abc.js", func(w http.ResponseWriter, _ *http.Request) {
		fmt.Fprint(w, `var a="https://www.google-analytics.com/collect";var b="/fonts/inter.woff2";`)
	})

	// Bundle de página: aqui estão as rotas de verdade, escondidas entre
	// chamadas irrelevantes, com nomes que ninguém adivinharia.
	mux.HandleFunc("/_next/static/chunks/pages-xyz.js", func(w http.ResponseWriter, _ *http.Request) {
		fmt.Fprint(w, "var t=fetch(\"/v3/catalogue/item-index/\");"+
			"var u=fetch(\"/v3/progression/upgrade-paths/\");"+
			"var v=fetch(\"/v3/builder/challenge-sets/\");"+
			"var w=fetch(\"/v3/feed/posts/\");"+
			"var x=fetch(`/v3/catalogue/item-index/${e}/detail/`);"+
			"var y=\"/img/badges/logo.png\";var z=\"/api/telemetry/beacon\";")
	})

	// players: "ovr" e "pos" no lugar de overall/position, preço em "bin".
	mux.HandleFunc("/v3/catalogue/item-index/", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"meta":{"total":3},"records":[
{"resId":50512345,"label":"Mbappé","ovr":91,"pos":"ST","bin":845000,
 "spd":97,"fin":92,"vis":80,"ctrl":93,"tkl":36,"str":78,"tier":"TOTS","sq":5,"wf":4},
{"resId":50598765,"label":"Saliba","ovr":86,"pos":"CB","bin":46000,
 "spd":84,"fin":40,"vis":65,"ctrl":72,"tkl":87,"str":84,"tier":"Gold Rare","sq":2,"wf":3},
{"resId":50533221,"label":"Wirtz","ovr":87,"pos":"CAM","bin":74000,
 "spd":82,"fin":82,"vis":87,"ctrl":89,"tkl":45,"str":68,"tier":"Gold Rare","sq":4,"wf":4}]}`)
	})

	// evolutions: chamado de "upgrade-paths", níveis em "stages".
	mux.HandleFunc("/v3/progression/upgrade-paths/", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"data":[
{"handle":"ponta-explosiva","label":"Ponta Explosiva","costCoins":25000,
 "gates":[{"type":"maxOverall","value":81}],"stages":[{"tier":1,"boosts":[]}],"endsAt":"2026-08-24T12:00:00Z"},
{"handle":"muralha","label":"Muralha","costCoins":50000,
 "gates":[{"type":"maxOverall","value":84}],"stages":[{"tier":1,"boosts":[]}],"endsAt":"2026-08-28T12:00:00Z"}]}`)
	})

	// sbcs: "challenge-sets", custo em "estCost".
	mux.HandleFunc("/v3/builder/challenge-sets/", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"results":[
{"label":"Upgrade 85+","estCost":62000,"segments":[{"n":1}],"prizes":[{"desc":"85+ player"}],"canRepeat":true},
{"label":"Serie A","estCost":18000,"segments":[{"n":1}],"prizes":[{"desc":"Gold pack"}],"canRepeat":false}]}`)
	})

	// news: feed com "heading" e "postedOn".
	mux.HandleFunc("/v3/feed/posts/", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `[
{"heading":"TOTS Serie A anunciado","postedOn":"2026-08-21T10:00:00Z","handle":"tots-serie-a","blurb":"Onze titular hoje às 19h"},
{"heading":"Nova evolução gratuita","postedOn":"2026-08-20T14:00:00Z","handle":"evo-gratis","blurb":"+4 de overall sem custo"}]`)
	})

	// Rota que responde JSON mas não é nada nosso: tem que cair em Unmatched.
	mux.HandleFunc("/api/telemetry/beacon/", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"ok":true,"ts":1755000000}`)
	})

	return httptest.NewServer(mux)
}

func TestDescobreRotasEClassificaPorFormato(t *testing.T) {
	srv := fakeSite()
	defer srv.Close()

	opt := DefaultOptions()
	opt.Verbose = func(f string, a ...any) { t.Logf(f, a...) }

	res, err := Run(context.Background(), httpFetcher{t}, srv.URL, opt)
	if err != nil {
		t.Fatalf("descoberta falhou: %v", err)
	}

	// As quatro rotas com nome irreconhecível têm que ser identificadas
	// pelo formato dos dados.
	esperado := map[string]string{
		"players":    "/v3/catalogue/item-index/",
		"evolutions": "/v3/progression/upgrade-paths/",
		"sbcs":       "/v3/builder/challenge-sets/",
		"news":       "/v3/feed/posts/",
	}
	for kind, path := range esperado {
		f, ok := res.Best[kind]
		if !ok {
			t.Errorf("%s: não foi identificado (achou: %v)", kind, kinds(res))
			continue
		}
		if f.Path != path {
			t.Errorf("%s: rota %q, esperava %q", kind, f.Path, path)
		}
		if f.Score < 0.5 {
			t.Errorf("%s: confiança baixa demais (%.2f)", kind, f.Score)
		}
	}
}

func TestAprendeOsNomesDeCampoReais(t *testing.T) {
	srv := fakeSite()
	defer srv.Close()

	res, err := Run(context.Background(), httpFetcher{t}, srv.URL, DefaultOptions())
	if err != nil {
		t.Fatal(err)
	}
	f, ok := res.Best["players"]
	if !ok {
		t.Fatal("players não identificado")
	}

	// Nenhum destes nomes está nos aliases do parser: só dá para achar
	// olhando o tipo, a faixa e a unicidade dos valores.
	quero := map[string]string{
		"rating":   "ovr",   // inteiro 40..99 + nome batendo
		"position": "pos",   // vocabulário fechado de posições
		"name":     "label", // string única por registro ("tier" se repete)
		"id":       "resId", // inteiro grande e único
		"price":    "bin",   // inteiro de faixa larga
	}
	for logical, real := range quero {
		if got := f.Fields[logical]; got != real {
			t.Errorf("campo %q: aprendeu %q, esperava %q", logical, got, real)
		}
	}

	// "fin", "vis", "ctrl", "tkl" e "str" são cinco inteiros na mesma faixa
	// sem nenhuma pista de nome. A descoberta não tem como saber qual é
	// finalização e qual é desarme — e gravar um palpite aqui faria o bot
	// recomendar trocas erradas calado. O certo é deixar em branco e cair
	// nos aliases do código.
	for _, amb := range []string{"shooting", "passing", "dribbling", "defending"} {
		if got, ok := f.Fields[amb]; ok {
			t.Errorf("campo %q era ambíguo e mesmo assim gravou %q", amb, got)
		}
	}
	if f.Wrapper != "records" {
		t.Errorf("embrulho da lista: %q, esperava \"records\"", f.Wrapper)
	}
}

// Uma rota que devolve JSON sem cara de nada nosso não pode ser
// classificada por engano — falso positivo aqui vira lixo no relatório.
func TestJSONIrrelevanteNaoEClassificado(t *testing.T) {
	samples := []map[string]any{
		{"ok": true, "ts": 1755000000.0},
		{"ok": false, "ts": 1755000060.0},
	}
	if kind, score, _ := Classify(samples); kind != "" {
		t.Fatalf("classificou telemetria como %q (%.2f)", kind, score)
	}
}

// Um jogador continua sendo reconhecido mesmo se TODOS os nomes forem
// irreconhecíveis — é o teste que garante a virada para o FC 27.
func TestReconheceJogadorComNomesTotalmenteEstranhos(t *testing.T) {
	samples := []map[string]any{
		{"k1": 50512345.0, "k2": "Mbappé", "k3": 91.0, "k4": "ST", "k5": 845000.0,
			"k6": 97.0, "k7": 92.0, "k8": 80.0, "k9": 93.0, "k10": 36.0, "k11": 78.0},
		{"k1": 50598765.0, "k2": "Saliba", "k3": 86.0, "k4": "CB", "k5": 46000.0,
			"k6": 84.0, "k7": 40.0, "k8": 65.0, "k9": 72.0, "k10": 87.0, "k11": 84.0},
		{"k1": 50533221.0, "k2": "Wirtz", "k3": 87.0, "k4": "CAM", "k5": 74000.0,
			"k6": 82.0, "k7": 82.0, "k8": 87.0, "k9": 89.0, "k10": 45.0, "k11": 68.0},
	}
	kind, score, fields := Classify(samples)
	if kind != "players" {
		t.Fatalf("classificou como %q (%.2f), esperava players", kind, score)
	}
	// Reconhecer o RECURSO não depende de resolver cada campo. A posição
	// tem vocabulário fechado e o nome é a única string única — esses dois
	// dão para cravar mesmo com as chaves chamadas k1..k11.
	if fields["position"] != "k4" {
		t.Errorf("posição: achou %q, esperava k4", fields["position"])
	}
	if fields["name"] != "k2" {
		t.Errorf("nome: achou %q, esperava k2", fields["name"])
	}
	// O rating é um inteiro 40..99 igual a vários atributos, sem nome que
	// ajude: tem que ficar em branco em vez de virar chute.
	if got, ok := fields["rating"]; ok {
		t.Errorf("rating era ambíguo e mesmo assim gravou %q", got)
	}
}

// Muitos sites servem os dados dentro do próprio HTML. Sem esse plano B
// a descoberta voltaria de mãos vazias justamente nesses casos.
func TestPlanoBLeDadosEmbutidosNaPagina(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, _ *http.Request) {
		fmt.Fprint(w, `<!doctype html><html><body>
<script id="__NEXT_DATA__" type="application/json">
{"props":{"pageProps":{"catalogue":[
{"eaId":50512345,"name":"Mbappé","overall":91,"position":"ST","price":845000},
{"eaId":50598765,"name":"Saliba","overall":86,"position":"CB","price":46000},
{"eaId":50533221,"name":"Wirtz","overall":87,"position":"CAM","price":74000}]}}}
</script></body></html>`)
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	opt := DefaultOptions()
	opt.Mine.SeedPaths = []string{"/"}
	opt.Verbose = func(f string, a ...any) { t.Logf(f, a...) }

	res, err := Run(context.Background(), httpFetcher{t}, srv.URL, opt)
	if err != nil {
		t.Fatal(err)
	}
	f, ok := res.Best["players"]
	if !ok {
		t.Fatalf("não achou os jogadores embutidos (achou: %v)", kinds(res))
	}
	if f.Source != "página" {
		t.Errorf("origem %q, esperava \"página\"", f.Source)
	}
	if !strings.Contains(f.Wrapper, "catalogue") {
		t.Errorf("caminho do payload %q, esperava conter \"catalogue\"", f.Wrapper)
	}
}

func TestNormalizaTemplateLiteral(t *testing.T) {
	casos := map[string]string{
		"/v3/players/${e}/":       "/v3/players/{id}/",
		"/api/evo/${t.slug}/lvl/": "/api/evo/{id}/lvl/",
		"/api/x/?page=2":          "/api/x/",
		"/api//dup/":              "/api/dup/",
	}
	for in, want := range casos {
		if got := normalize(in); got != want {
			t.Errorf("normalize(%q) = %q, esperava %q", in, got, want)
		}
	}
}

func kinds(res *Result) []string {
	var out []string
	for k := range res.Best {
		out = append(out, k)
	}
	return out
}

// O JSON do clube contém uma lista de jogadores dentro. Ler só essa lista
// classificaria a rota como "players" e o clube — que é o dado central do
// bot — nunca seria encontrado. E o objeto de fora, visto de raspão, casa
// com "evolutions" por acaso: a lista de elenco parece uma lista de níveis.
// Só a checagem aninhada resolve: um clube é um objeto que contém cartas
// de jogador de verdade.
func TestClubNaoEConfundidoComListaDeJogadores(t *testing.T) {
	body := []byte(`{"gamerTag":"carneiro22","platform":"ps5","coins":145000,
"roster":[{"resId":50512345,"label":"Mbappé","ovr":91,"pos":"ST","bin":845000,
  "pace":97,"shooting":92,"passing":80,"dribbling":93,"defending":36,"physicality":78},
 {"resId":50598765,"label":"Saliba","ovr":86,"pos":"CB","bin":46000,
  "pace":84,"shooting":40,"passing":65,"dribbling":72,"defending":87,"physicality":84}],
"squad":{"name":"Titular","formation":"4-2-3-1","chemistry":29}}`)

	kind, score, fields, wrapper, n := classifyBody(body)
	if kind != "club" {
		t.Fatalf("classificou como %q (%.2f), esperava club", kind, score)
	}
	if wrapper != "" {
		t.Errorf("o clube é o objeto de topo, não devia ter embrulho (%q)", wrapper)
	}
	if n != 1 {
		t.Errorf("amostras %d, esperava 1", n)
	}
	if fields["players"] != "roster" {
		t.Errorf("lista de jogadores: achou %q, esperava roster", fields["players"])
	}
	if fields["coins"] != "coins" {
		t.Errorf("moedas: achou %q", fields["coins"])
	}
}

// A lista aninhada tem que ser de cartas DE VERDADE. Um objeto qualquer com
// uma lista dentro não pode virar clube.
func TestObjetoComListaQualquerNaoViraClube(t *testing.T) {
	body := []byte(`{"gamerTag":"alguem","coins":500,
"roster":[{"a":1,"b":"x"},{"a":2,"b":"y"}],"squad":{"n":1}}`)
	if kind, _, _, _, _ := classifyBody(body); kind == "club" {
		t.Fatal("virou clube sem ter uma única carta de jogador dentro")
	}
}
