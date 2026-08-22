package futgg

import "testing"

// Um app Next.js do App Router não usa __NEXT_DATA__: ele despeja o
// resultado do servidor em pedaços de self.__next_f.push(). O dado da
// página está ali, no HTML que o servidor já mandou — e um objeto pode
// atravessar a fronteira entre dois pedaços, que é onde a extração
// ingênua perde justamente os registros grandes.
func TestExtraiPayloadDoFlightAtravessandoPedacos(t *testing.T) {
	html := []byte(`<html><body>
<script>self.__next_f.push([1,"2:[\"$\",\"div\",null,{\"player\":{\"eaId\":117633497,\"nam`)
	html = append(html, []byte(`e\":\"Kevin De Bruyne\",\"overall\":99,\"position\":\"CM\",`)...)
	html = append(html, []byte(`\"pace\":95,\"shooting\":96}}]"])</script>
<script>self.__next_f.push([1,",\"outro\":1"])</script>
</body></html>`)...)

	payloads := ExtractEmbedded(html)
	if len(payloads) == 0 {
		t.Fatal("não achou nenhum payload embutido")
	}

	var achou bool
	for _, pl := range payloads {
		for _, o := range pl.Objects {
			if o["eaId"] == float64(117633497) && o["overall"] == float64(99) {
				achou = true
				if o["name"] != "Kevin De Bruyne" {
					t.Errorf("nome veio %q", o["name"])
				}
			}
		}
	}
	if !achou {
		t.Fatal("o objeto do jogador não sobreviveu à junção dos pedaços")
	}
}

// Acento no payload vem como \uXXXX. Sem desescapar direito, "Mbappé"
// vira lixo e não casa com nada do elenco.
func TestDesescapaAcentoEParSubstituto(t *testing.T) {
	casos := map[string]string{
		`Mbappé`:         "Mbappé",
		`Nuñez`:          "Nuñez",
		`a\\b`:           `a\b`,
		`linha\nnova`:    "linha\nnova",
		`😀 ok`:           "😀 ok",
		`aspas \"aqui\"`: `aspas "aqui"`,
	}
	for in, want := range casos {
		if got := unescapeJS(in); got != want {
			t.Errorf("unescapeJS(%q) = %q, esperava %q", in, got, want)
		}
	}
}

func TestExtraiNextDataELdJSON(t *testing.T) {
	html := []byte(`<html><head>
<script type="application/ld+json">{"@type":"Person","name":"X","jobTitle":"Y"}</script>
<script id="__NEXT_DATA__" type="application/json">
{"props":{"pageProps":{"player":{"eaId":1,"overall":91,"position":"ST"}}}}
</script></head></html>`)

	fontes := map[string]bool{}
	for _, pl := range ExtractEmbedded(html) {
		fontes[pl.Source] = true
	}
	if !fontes["__NEXT_DATA__"] {
		t.Error("não leu __NEXT_DATA__")
	}
	if !fontes["ld+json"] {
		t.Error("não leu ld+json")
	}
}

// A URL da página carrega o id do recurso e o nome — é a fonte mais
// confiável dos dois, porque não depende do formato do payload.
func TestLeIdENomeDaURLDaPagina(t *testing.T) {
	id, name, ok := parsePlayerURL("https://www.fut.gg/players/192985-kevin-de-bruyne/26-117633497/")
	if !ok {
		t.Fatal("não conseguiu ler a URL")
	}
	if id != 117633497 {
		t.Errorf("id %d, esperava 117633497", id)
	}
	if name != "Kevin De Bruyne" {
		t.Errorf("nome %q, esperava \"Kevin De Bruyne\"", name)
	}

	if _, _, ok := parsePlayerURL("https://www.fut.gg/evolutions/alguma/"); ok {
		t.Error("uma URL que não é de jogador não deveria ser aceita")
	}
}
