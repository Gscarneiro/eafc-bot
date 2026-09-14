package futgg

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// Collect precisa terminar com um mapa de Capabilities que separa o que deu
// certo (clube, confirmado) do que falhou (mercado, erro) — é o que
// /api/saude expõe sem exigir que quem lê vasculhe a lista plana de Errors
// atrás do nome certo.
func TestCollectPopulaCapabilitiesComErroPorFonte(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case strings.Contains(r.URL.Path, "gg-club"):
			w.Write([]byte(clubPlayerReal))
		case strings.Contains(r.URL.Path, "players"):
			// 404 devolve ErrNotFound sem retry (backoff só entra em 429/5xx) —
			// é o jeito rápido de simular uma fonte que falhou neste teste.
			w.WriteHeader(http.StatusNotFound)
		default:
			w.Write([]byte(`{"data":[]}`))
		}
	}))
	defer srv.Close()

	c := New(Config{
		BaseURL: srv.URL,
		Cycle:   "26",
		Endpoints: map[string]string{
			"club":       "/api/gg-club/{gamertag}/players/",
			"players":    "/api/fut/players/",
			"evolutions": "/api/fut/evolutions/",
			"objectives": "/api/fut/objectives/",
			"sbcs":       "/api/fut/sbc/sets/",
			"news":       "/api/fut/news/",
		},
	})

	snap, err := c.Collect(context.Background(), "BilingualBee", PlayerFilter{Pages: 1})
	if err != nil {
		t.Fatalf("Collect: %v", err)
	}

	clube := snap.Capabilities["clube"]
	if clube.Status != StatusConfirmado || clube.Coverage != 1 {
		t.Fatalf("clube = %+v, esperava confirmado com cobertura 1", clube)
	}
	mercado := snap.Capabilities["mercado"]
	if mercado.Status != StatusErro || mercado.Error == "" {
		t.Fatalf("mercado = %+v, esperava erro não vazio", mercado)
	}
	for _, key := range []string{"evoluções", "objetivos", "SBCs", "notícias"} {
		if got := snap.Capabilities[key]; got.Status != StatusConfirmado {
			t.Errorf("%s = %+v, esperava confirmado (lista vazia é resposta válida, não falha)", key, got)
		}
	}
}

// clubDoCicloAnterior imita o que o elenco público do fut.gg devolve quando
// o clube do ciclo configurado ainda não existe: cartas com "game":"26"
// mesmo pedindo o ciclo 27 — visto ao vivo em 13/09/2026, o mercado FC 27 já
// respondia mas o elenco do GG Club ainda só tinha cartas do FC 26.
const clubDoCicloAnterior = `{"data":[
 {"userId":1,"id":"1-1","eaId":100,
  "playerDef":{"eaId":100,"game":"26","commonName":"Jogador Um","overall":80,"position":14}},
 {"userId":1,"id":"1-2","eaId":200,
  "playerDef":{"eaId":200,"game":"26","commonName":"Jogador Dois","overall":75,"position":25}}],
 "next":null,"currentPage":1,"total":2}`

// TestClubeDeCicloDivergenteExpoeSourceCycle prova que Client.Club detecta,
// sozinho, quando o payload é de um ciclo diferente do configurado — sem
// travar nada: o clube continua vindo cheio, só SourceCycle passa a
// registrar o que a fonte disse de verdade.
func TestClubeDeCicloDivergenteExpoeSourceCycle(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(clubDoCicloAnterior))
	}))
	defer srv.Close()

	c := New(Config{
		BaseURL:   srv.URL,
		Cycle:     "27",
		Endpoints: map[string]string{"club": "/api/gg-club/{gamertag}/players/"},
	})

	club, err := c.Club(context.Background(), "BilingualBee")
	if err != nil {
		t.Fatalf("Club: %v", err)
	}
	if len(club.Players) != 2 {
		t.Fatalf("clube veio com %d jogadores, esperava 2 (divergência de ciclo não pode truncar nada)", len(club.Players))
	}
	if club.Cycle != "27" {
		t.Errorf("Cycle = %q, esperava o CONFIGURADO 27 (é por ele que o store particiona)", club.Cycle)
	}
	if club.SourceCycle != "26" {
		t.Errorf("SourceCycle = %q, esperava 26 (o que a maioria das cartas do payload reportou)", club.SourceCycle)
	}
}

// TestCollectAvisaClubeDeCicloDivergenteSemFalhar prova que Collect
// transforma a divergência acima num aviso em Errors — visível, mas sem
// derrubar o clube nem virar Capabilities["clube"].Status de erro.
func TestCollectAvisaClubeDeCicloDivergenteSemFalhar(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case strings.Contains(r.URL.Path, "gg-club"):
			w.Write([]byte(clubDoCicloAnterior))
		default:
			w.Write([]byte(`{"data":[]}`))
		}
	}))
	defer srv.Close()

	c := New(Config{
		BaseURL: srv.URL,
		Cycle:   "27",
		Endpoints: map[string]string{
			"club":       "/api/gg-club/{gamertag}/players/",
			"players":    "/api/fut/players/",
			"evolutions": "/api/fut/evolutions/",
			"objectives": "/api/fut/objectives/",
			"sbcs":       "/api/fut/sbc/sets/",
			"news":       "/api/fut/news/",
		},
	})

	snap, err := c.Collect(context.Background(), "BilingualBee", PlayerFilter{Pages: 1})
	if err != nil {
		t.Fatalf("Collect: %v", err)
	}
	if len(snap.Club.Players) != 2 {
		t.Fatalf("snapshot perdeu o clube por causa do aviso: %d jogadores", len(snap.Club.Players))
	}
	if clube := snap.Capabilities["clube"]; clube.Status != StatusConfirmado {
		t.Errorf("clube = %+v, divergência de ciclo não é falha de coleta", clube)
	}
	achou := false
	for _, e := range snap.Errors {
		if strings.Contains(e, "ciclo 26") && strings.Contains(e, "configurado para 27") {
			achou = true
		}
	}
	if !achou {
		t.Errorf("Errors não menciona a divergência de ciclo: %v", snap.Errors)
	}
}
