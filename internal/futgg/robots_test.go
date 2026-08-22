package futgg

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

// checkRobots nunca bloqueia a leitura — só conta e expõe em Stats o que os
// endpoints JÁ configurados pisam no Disallow do site, para essa escolha
// nunca ficar silenciosa (ver o comentário de robots.go).
func TestCheckRobotsContaRotasConfiguradasBloqueadas(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/robots.txt" {
			w.Write([]byte("User-agent: *\nDisallow: /api/*\nDisallow: /gg-club/\nAllow: /gg-club/$\n"))
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	c := New(Config{
		BaseURL: srv.URL,
		Cycle:   "26",
		Endpoints: map[string]string{
			"club":    "/api/gg-club/{gamertag}/players/", // bloqueada
			"players": "/api/fut/players/v2/26/",          // bloqueada
			"page":    "/players/26-168027413/",           // fora do robots
		},
	})

	c.checkRobots(context.Background())

	if got := c.Stats().RobotsBypassed; got != 2 {
		t.Fatalf("esperava 2 rotas configuradas contadas como bloqueadas, veio %d", got)
	}
}

func TestCheckRobotsSemRobotsTxtLegivelNaoQuebra(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	c := New(Config{
		BaseURL:   srv.URL,
		Cycle:     "26",
		Endpoints: map[string]string{"club": "/api/gg-club/{gamertag}/players/"},
	})

	c.checkRobots(context.Background()) // não deve travar nem entrar em pânico

	if got := c.Stats().RobotsBypassed; got != 0 {
		t.Fatalf("sem robots.txt legível, esperava RobotsBypassed=0, veio %d", got)
	}
}
