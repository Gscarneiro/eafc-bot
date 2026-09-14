package futgg

import (
	"strings"
	"testing"
)

func TestURLPreencheCicloConfiguradoSemCaminhoFixo(t *testing.T) {
	c := New(Config{
		BaseURL: "https://example.invalid", Cycle: "27",
		Endpoints: map[string]string{"evolution_paths": "/api/fut/evolutions/v2/{cycle}/paths/v2/{id}/"},
	})
	got, err := c.URL("evolution_paths", map[string]string{"id": "123"})
	if err != nil {
		t.Fatal(err)
	}
	want := "https://example.invalid/api/fut/evolutions/v2/27/paths/v2/123/"
	if got != want {
		t.Fatalf("URL = %q; queria %q", got, want)
	}
}

func TestEndpointPadraoDeEvolucaoNaoFixaOCiclo(t *testing.T) {
	cfg := DefaultConfig()
	if got := cfg.Endpoints["evolution_paths"]; got != "/api/fut/evolutions/v2/{cycle}/paths/v2/{id}/" {
		t.Fatalf("endpoint padrão ainda fixa ciclo: %q", got)
	}
}

func TestClienteMigraSomenteODefaultLegadoDoFC26(t *testing.T) {
	c := New(Config{
		BaseURL: "https://example.invalid", Cycle: "27",
		Endpoints: map[string]string{"evolution_paths": "/api/fut/evolutions/v2/26/paths/v2/{id}/"},
	})
	got, err := c.URL("evolution_paths", map[string]string{"id": "9"})
	if err != nil {
		t.Fatal(err)
	}
	if got != "https://example.invalid/api/fut/evolutions/v2/27/paths/v2/9/" {
		t.Fatalf("default legado não acompanhou o ciclo 27: %q", got)
	}
}

// TestEndpointsPadraoUsamPlaceholderDeCicloEmVezDeLiteral varre TODOS os
// endpoints de DefaultConfig, não só evolution_paths: um ciclo literal
// escondido em qualquer um deles é exatamente o bug que fazia virar
// futgg.cycle para "27" mover só uma rota e deixar o resto lendo FC 26 (ou
// batendo num caminho que o site já abandonou).
func TestEndpointsPadraoUsamPlaceholderDeCicloEmVezDeLiteral(t *testing.T) {
	for logical, path := range DefaultConfig().Endpoints {
		if strings.Contains(path, "/26/") || strings.Contains(path, "/27/") {
			t.Errorf("endpoint %q tem ciclo literal: %q — devia usar {cycle}", logical, path)
		}
	}
}

// TestConfigGravadoComCicloLiteralMigraParaPlaceholder espelha o que um
// .eafc-bot/config.json real tinha gravado em 13/09/2026: para cada um dos
// endpoints particionados por ciclo, tanto o primeiro default do repo (que
// nunca existiu de verdade no site) quanto o seguinte (já correto, mas fixo
// no ciclo 26) têm que migrar para o template com {cycle}.
func TestConfigGravadoComCicloLiteralMigraParaPlaceholder(t *testing.T) {
	casos := map[string]struct{ antigo, novo string }{
		"players":         {"/api/fut/players/", "/api/fut/players/v2/{cycle}/"},
		"evolutions":      {"/api/fut/evolutions/", "/api/fut/evolutions/v2/{cycle}/v3/all/"},
		"evolution":       {"/api/fut/evolutions/{slug}/", "/api/fut/evolutions/v2/{cycle}/{slug}/v2/"},
		"sbcs":            {"/api/fut/sbc/sets/", "/api/fut/sbc/{cycle}/"},
		"objectives":      {"/api/fut/objectives/", "/api/fut/objectives/{cycle}/groups/"},
		"evolution_paths": {"/api/fut/evolutions/v2/26/paths/v2/{id}/", "/api/fut/evolutions/v2/{cycle}/paths/v2/{id}/"},
	}
	for logical, c := range casos {
		cfg := Config{BaseURL: "https://example.invalid", Cycle: "27", Endpoints: map[string]string{logical: c.antigo}}
		got := New(cfg).Config().Endpoints[logical]
		if got != c.novo {
			t.Errorf("%s: %q migrou para %q, esperava %q", logical, c.antigo, got, c.novo)
		}
	}

	// A outra forma legada de "players" e "evolutions" — já correta na
	// rota, só fixa no ciclo 26 — também tem que migrar.
	for logical, c := range map[string]struct{ antigo, novo string }{
		"players":    {"/api/fut/players/v2/26/", "/api/fut/players/v2/{cycle}/"},
		"evolutions": {"/api/fut/evolutions/v2/26/v3/all/", "/api/fut/evolutions/v2/{cycle}/v3/all/"},
	} {
		cfg := Config{BaseURL: "https://example.invalid", Cycle: "27", Endpoints: map[string]string{logical: c.antigo}}
		got := New(cfg).Config().Endpoints[logical]
		if got != c.novo {
			t.Errorf("%s: %q migrou para %q, esperava %q", logical, c.antigo, got, c.novo)
		}
	}
}

// TestRotaCustomizadaSobreviveAMigracao trava o critério de igualdade
// exata: a migração só troca um valor que reconhece como default antigo. Uma
// rota que o usuário editou na mão — mesmo que pareça com uma delas — não
// pode ser sobrescrita.
func TestRotaCustomizadaSobreviveAMigracao(t *testing.T) {
	custom := "/minha/rota/manual/{cycle}/pagina/"
	cfg := Config{BaseURL: "https://example.invalid", Cycle: "27", Endpoints: map[string]string{"players": custom}}
	got := New(cfg).Config().Endpoints["players"]
	if got != custom {
		t.Errorf("rota customizada foi sobrescrita: %q", got)
	}
}
