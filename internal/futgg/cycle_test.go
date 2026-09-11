package futgg

import "testing"

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
