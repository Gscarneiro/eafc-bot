package futgg

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gscarneiro/eafc-bot/internal/domain"
)

// O elenco do GG Club fala uma língua diferente da listagem de mercado, e
// cada diferença abaixo já custou o clube inteiro em algum momento:
//
//   - a carta mora em "playerDef", não no objeto de fora;
//   - "id" é a string "2567219-919095915063" e o número está em "eaId";
//   - "position" é 14, não "CM";
//   - os seis atributos vêm soltos como "facePace", não em faceStatsV2;
//   - quem está no time titular é marcado por "isInActiveSquad".
const clubPlayerReal = `{"data":[
 {"userId":2567219,"id":"2567219-919095915063","eaId":168027413,
  "isUntradeable":true,"isInActiveSquad":true,"kitNumber":16,"goals":37,
  "addedToClubAt":"2026-08-15T02:49:58Z",
  "playerDef":{"id":"2567219-919095915063","eaId":168027413,"slug":"26-168027413",
   "commonName":"Vitinha","firstName":"Vítor","lastName":"Machado Ferreira",
   "overall":99,"position":14,"alternativePositionIds":[16,10,18,12],
   "facePace":96,"faceShooting":95,"facePassing":99,
   "faceDribbling":99,"faceDefending":92,"facePhysicality":90,
   "skillMoves":5,"weakFoot":5,"rarityName":"FUTTIES"}}],
 "next":null,"currentPage":1,"total":1}`

func TestClubeLeOElencoPublicoDoGGClub(t *testing.T) {
	c := &Client{cfg: Config{
		BaseURL:   "https://www.fut.gg",
		Cycle:     "26",
		Endpoints: map[string]string{"club": "/api/gg-club/{gamertag}/players/"},
	}}

	var wrapper node
	if err := jsonUnmarshalNode([]byte(clubPlayerReal), &wrapper); err != nil {
		t.Fatal(err)
	}
	nodes := wrapper.nodes("data")
	if len(nodes) != 1 {
		t.Fatalf("leu %d registros", len(nodes))
	}

	cp := mapClubPlayer(nodes[0], "26", c.lensFor("players"))

	if cp.ID != 168027413 {
		t.Errorf("id %d — parou no \"id\" em texto em vez de cair no eaId?", cp.ID)
	}
	if cp.Player.Display() != "Vitinha" {
		t.Errorf("nome %q", cp.Player.Display())
	}
	if cp.Player.Rating != 99 {
		t.Errorf("overall %d", cp.Player.Rating)
	}
	if cp.Player.Position != domain.CM {
		t.Errorf("posição %q — o 14 não virou CM", cp.Player.Position)
	}
	want := domain.Attributes{Pace: 96, Shooting: 95, Passing: 99,
		Dribbling: 99, Defending: 92, Physical: 90}
	if cp.Player.Attributes != want {
		t.Errorf("atributos %+v, esperava %+v", cp.Player.Attributes, want)
	}
	if !cp.InSquad {
		t.Error("isInActiveSquad não virou InSquad")
	}
	if !cp.Untradeable {
		t.Error("isUntradeable não virou Untradeable")
	}
	// As alternativas também vêm como id.
	if len(cp.Player.AltPositions) != 4 || !cp.Player.PlaysAt(domain.CAM) {
		t.Errorf("posições alternativas %v", cp.Player.AltPositions)
	}
}

// A tabela de ids de posição é da EA e não muda entre ciclos. O LWB é o que
// pega: é 8, e não 6, que é o que a sequência sugeriria.
func TestIdsDePosicaoDaEA(t *testing.T) {
	casos := map[int]domain.Position{
		0: domain.GK, 2: domain.RWB, 3: domain.RB, 5: domain.CB, 7: domain.LB,
		8: domain.LWB, 10: domain.CDM, 12: domain.RM, 14: domain.CM,
		16: domain.LM, 18: domain.CAM, 21: domain.CF, 23: domain.RW,
		25: domain.ST, 27: domain.LW,
	}
	for id, want := range casos {
		got, ok := domain.PositionFromID(id)
		if !ok || got != want {
			t.Errorf("id %d virou %q (ok=%v), esperava %q", id, got, ok, want)
		}
	}
	if _, ok := domain.PositionFromID(6); ok {
		t.Error("6 não é posição nenhuma — LWB é 8")
	}
}

// Um número que não é número não pode parar a busca: era isso que fazia todo
// jogador do clube entrar com id 0 e ser descartado.
func TestIntPulaValorQueNaoConverte(t *testing.T) {
	n := node{"id": "2567219-919095915063", "eaId": 168027413.0}
	if got := n.i64("id", "eaId"); got != 168027413 {
		t.Errorf("i64 devolveu %d, esperava cair no eaId", got)
	}
	// Zero de verdade continua sendo zero, e não faz cair para o próximo.
	z := node{"coinCost": 0.0, "priceCoins": 999.0}
	if got := z.int("coinCost", "priceCoins"); got != 0 {
		t.Errorf("int devolveu %d — zero legítimo não pode virar fallback", got)
	}
}

func TestWithPageNaoMexeNaPrimeiraPagina(t *testing.T) {
	casos := map[int]string{
		1: "https://x/api/gg-club/eu/players/",
		2: "https://x/api/gg-club/eu/players/?page=2",
	}
	for page, want := range casos {
		if got := withPage("https://x/api/gg-club/eu/players/", page); got != want {
			t.Errorf("página %d: %q, esperava %q", page, got, want)
		}
	}
	if got := withPage("https://x/a/?limit=30", 3); got != "https://x/a/?limit=30&page=3" {
		t.Errorf("com query existente: %q", got)
	}
}

// O engano mais comum é usar a gamertag da EA em vez do nome do perfil no
// fut.gg — são identidades diferentes, e o fut.gg responde 404 sem dizer
// isso. O erro que o bot devolve precisa ensinar, não só relatar o status.
func TestClubeInexistenteExplicaOEnganoComum(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte(`{"detail":"Not found."}`))
	}))
	defer srv.Close()

	c := New(Config{
		BaseURL:   srv.URL,
		Cycle:     "26",
		Endpoints: map[string]string{"club": "/api/gg-club/{gamertag}/players/"},
	})

	_, err := c.Club(context.Background(), "BilingualB22")
	if err == nil {
		t.Fatal("esperava erro para perfil inexistente")
	}
	msg := err.Error()
	for _, want := range []string{"BilingualB22", "404", "gamertag da EA", "fut.gg"} {
		if !strings.Contains(msg, want) {
			t.Errorf("mensagem de erro não menciona %q: %s", want, msg)
		}
	}
	// O genérico "não encontrado (404)" não ensina nada — a mensagem tem
	// que ser mais longa que isso.
	if len(msg) < 60 {
		t.Errorf("mensagem curta demais para explicar o engano: %q", msg)
	}
}

// 404 na PRIMEIRA página é perfil inexistente. 404 numa página seguinte é
// só o fim da paginação — não pode virar o mesmo erro nem derrubar o que já
// foi lido.
func TestClube404NaSegundaPaginaNaoEEnganoDePerfil(t *testing.T) {
	pagina1 := `{"data":[
 {"userId":2567219,"id":"2567219-919095915063","eaId":168027413,
  "playerDef":{"eaId":168027413,"commonName":"Vitinha","overall":99,"position":14}}],
 "next":2,"currentPage":1,"total":2}`

	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		w.Header().Set("Content-Type", "application/json")
		if strings.Contains(r.URL.RawQuery, "page=2") {
			w.WriteHeader(http.StatusNotFound)
			w.Write([]byte(`{"detail":"Not found."}`))
			return
		}
		w.Write([]byte(pagina1))
	}))
	defer srv.Close()

	c := New(Config{
		BaseURL:   srv.URL,
		Cycle:     "26",
		Endpoints: map[string]string{"club": "/api/gg-club/{gamertag}/players/"},
	})

	club, err := c.Club(context.Background(), "BilingualBee")
	if err != nil {
		t.Fatalf("não devia falhar por 404 na segunda página: %v", err)
	}
	if len(club.Players) != 1 {
		t.Errorf("perdeu o que já tinha lido na primeira página: %d jogadores", len(club.Players))
	}
	if calls < 2 {
		t.Fatalf("esperava pelo menos 2 chamadas, teve %d", calls)
	}
}

// activeSquadReal é o formato de /active-squad/ que o fut.gg entrega: 11
// slots "FIELD" (o XI) e o resto "SUBSTITUTE" (banco + reservas), que não
// nos interessam aqui. O slot 4 tem "positionOverride":7 (carta de LB); os
// demais têm null e caem na posição natural do jogador no elenco.
func activeSquadFixture() string {
	var entries []string
	for i := 0; i < 11; i++ {
		override := "null"
		if i == 4 {
			override = "7"
		}
		entries = append(entries, fmt.Sprintf(
			`{"group":"FIELD","positionIdx":%d,"playerEaId":%d,"positionOverride":%s}`,
			i, 1000+i, override))
	}
	for i := 0; i < 2; i++ {
		entries = append(entries, fmt.Sprintf(
			`{"group":"SUBSTITUTE","positionIdx":%d,"playerEaId":%d,"positionOverride":null}`,
			i, 2000+i))
	}
	// O payload real embrulha em "data" DUAS vezes: o de fora carrega o
	// usuário e repete o título; activeGroupPositions só existe no "data"
	// de dentro. É exatamente esse nível errado que fazia o título aparecer
	// (o de fora também tem um) enquanto a escalação sumia em silêncio.
	return `{"data":{"title":"Carneiro FC","user":{"id":1},` +
		`"data":{"title":"Carneiro FC","activeFormationId":"18","activeGroupPositions":[` +
		strings.Join(entries, ",") + `]}}}`
}

func TestActiveSquadSoPegaOsOnzeDoField(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(activeSquadFixture()))
	}))
	defer srv.Close()

	c := New(Config{
		BaseURL:   srv.URL,
		Cycle:     "26",
		Endpoints: map[string]string{"club_squad": "/api/gg-club/{gamertag}/active-squad/"},
	})

	// O jogador 1004 é o do positionOverride:7 (LB) — de posição natural
	// diferente, pra provar que a carta de posição vence a natural.
	roster := make([]domain.ClubPlayer, 0, 11)
	for i := 0; i < 11; i++ {
		pos := domain.CM
		if i == 4 {
			pos = domain.ST // natural diferente do override (LB)
		}
		roster = append(roster, domain.ClubPlayer{
			Player:    domain.Player{ID: int64(1000 + i), Position: pos},
			Chemistry: 3,
		})
	}

	sq, err := c.ActiveSquad(context.Background(), "BilingualBee", roster)
	if err != nil {
		t.Fatalf("ActiveSquad: %v", err)
	}
	if sq.Name != "Carneiro FC" {
		t.Errorf("nome %q, esperava \"Carneiro FC\"", sq.Name)
	}
	if sq.Formation != "4-4-1-1" {
		t.Errorf("formação %q, esperava 4-4-1-1", sq.Formation)
	}
	if len(sq.Starters) != 11 {
		t.Fatalf("titulares %d, esperava 11 (SUBSTITUTE não pode entrar)", len(sq.Starters))
	}
	for _, s := range sq.Starters {
		if s.PlayerID >= 2000 {
			t.Errorf("slot %d é um reserva (id %d), não devia estar em Starters", s.Index, s.PlayerID)
		}
	}
	// Slot 4: positionOverride:7 -> LB, não ST (a posição natural).
	var slot4 *domain.SquadSlot
	for i := range sq.Starters {
		if sq.Starters[i].Index == 4 {
			slot4 = &sq.Starters[i]
		}
	}
	if slot4 == nil {
		t.Fatal("slot 4 não apareceu nos titulares")
	}
	if slot4.Position != domain.LB {
		t.Errorf("slot 4: posição %q, esperava LB (positionOverride vence a natural)", slot4.Position)
	}
	// Química: 11 titulares x 3 pontos cada.
	if sq.Chemistry != 33 {
		t.Errorf("química %d, esperava 33 (11 titulares x 3)", sq.Chemistry)
	}
}

// O GG Rating vem com decimais ("92.4") e só existe no elenco do GG Club —
// a listagem de mercado não manda esse campo.
func TestMapPlayerLeGGRating(t *testing.T) {
	n := node{"overall": 98.0, "ggRating": 92.4, "ggRatingPos": 16.0}
	p := mapPlayer(n, "26", lens{})
	if p.GGRating != 92.4 {
		t.Errorf("GGRating = %v, esperava 92.4", p.GGRating)
	}
	if p.GGRatingPos != domain.LM {
		t.Errorf("GGRatingPos = %q, esperava LM (id 16)", p.GGRatingPos)
	}
}

// O id de posição 0 é GK de verdade (ver domain.PositionFromID). Sem
// distinguir "campo ausente" de "campo é 0", todo jogador da listagem de
// mercado — que não manda ggRatingPos nenhum — herdaria GK por acidente.
func TestMapPlayerNaoInventaGKQuandoGGRatingPosAusente(t *testing.T) {
	semCampo := mapPlayer(node{"overall": 90.0}, "26", lens{})
	if semCampo.GGRatingPos != "" {
		t.Errorf("GGRatingPos = %q, esperava vazio (campo nem existia)", semCampo.GGRatingPos)
	}
	if semCampo.GGRating != 0 {
		t.Errorf("GGRating = %v, esperava 0", semCampo.GGRating)
	}

	// null explícito tem que se comportar igual a ausente.
	comNull := mapPlayer(node{"overall": 90.0, "ggRatingPos": nil}, "26", lens{})
	if comNull.GGRatingPos != "" {
		t.Errorf("GGRatingPos com null = %q, esperava vazio", comNull.GGRatingPos)
	}

	// 0 de verdade (GK) continua funcionando quando o campo REALMENTE existe.
	comGK := mapPlayer(node{"overall": 90.0, "ggRatingPos": 0.0}, "26", lens{})
	if comGK.GGRatingPos != domain.GK {
		t.Errorf("GGRatingPos com 0 explícito = %q, esperava GK", comGK.GGRatingPos)
	}
}

// A URL da arte da carta já vem pronta do fut.gg — só precisa achar a chave
// certa entre os formatos que eles usam.
func TestMapPlayerLeImageURL(t *testing.T) {
	n := node{"overall": 91.0, "cardImageUrl": "https://game-assets.fut.gg/x.webp"}
	p := mapPlayer(n, "26", lens{})
	if p.ImageURL != "https://game-assets.fut.gg/x.webp" {
		t.Errorf("ImageURL = %q", p.ImageURL)
	}
}

// O elenco do GG Club (diferente da listagem de mercado) só manda o caminho
// relativo em "cardImagePath", não a URL pronta — sem montar a URL a partir
// do prefixo do CDN, TODA carta do seu clube saía sem imagem no relatório.
func TestMapPlayerMontaImagemAPartirDoPath(t *testing.T) {
	n := node{"overall": 91.0, "cardImagePath": "2026/player-item-card/26-168027413.abc.webp"}
	p := mapPlayer(n, "26", lens{})
	want := "https://game-assets.fut.gg/cdn-cgi/image/quality=85,format=auto,width=300/2026/player-item-card/26-168027413.abc.webp"
	if p.ImageURL != want {
		t.Errorf("ImageURL = %q, esperava %q", p.ImageURL, want)
	}
}

// URL pronta tem prioridade sobre o path — não faz sentido remontar o que
// o fut.gg já entregou pronto.
func TestMapPlayerPrefereURLProntaAoPath(t *testing.T) {
	n := node{"overall": 91.0,
		"cardImageUrl":  "https://ja-pronta.example/x.webp",
		"cardImagePath": "2026/nao-deveria-usar.webp"}
	p := mapPlayer(n, "26", lens{})
	if p.ImageURL != "https://ja-pronta.example/x.webp" {
		t.Errorf("ImageURL = %q, esperava a URL pronta", p.ImageURL)
	}
}

// Club() precisa se garantir sozinho, sem depender de Collect() ter
// carregado a tabela de PlayStyles antes: qualquer chamada a Club() fora de
// Collect() não pode receber playstyles:[0,34,5] cru pra sempre — mapPlayer
// resolve o nome na hora de ler cada carta, não depois.
func TestClubResolvePlayStylesSozinho(t *testing.T) {
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		w.Header().Set("Content-Type", "application/json")
		if strings.Contains(r.URL.Path, "playstyles") {
			w.Write([]byte(`{"data":[{"id":78,"eaId":5,"name":"Incisive Pass"}]}`))
			return
		}
		w.Write([]byte(`{"data":[
			{"userId":1,"id":"1-1","eaId":100,
			 "playerDef":{"eaId":100,"commonName":"Fulano","overall":90,"position":14,
			  "playstylesPlus":[5]}}],
		 "next":null,"currentPage":1,"total":1}`))
	}))
	defer srv.Close()

	c := New(Config{BaseURL: srv.URL, Cycle: "26", Endpoints: map[string]string{
		"club":       "/api/gg-club/{gamertag}/players/",
		"playstyles": "/api/fut/playstyles/",
	}})

	club, err := c.Club(context.Background(), "alguem")
	if err != nil {
		t.Fatal(err)
	}
	if len(club.Players) != 1 {
		t.Fatalf("leu %d jogadores, esperava 1", len(club.Players))
	}
	styles := club.Players[0].PlayStyles
	if len(styles) != 1 || styles[0].Name != "Incisive Pass" {
		t.Errorf("PlayStyles = %+v, esperava [Incisive Pass+] resolvido — não o eaId cru", styles)
	}
}
