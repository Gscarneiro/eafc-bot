package futgg

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestRecuperaProcedenciaPorIdentidadeFisicaSemInferirAusencia(t *testing.T) {
	dir := t.TempDir()
	old := filepath.Join(dir, "old.cache")
	newer := filepath.Join(dir, "new.cache")
	if err := os.WriteFile(old, []byte(`{"data":[{"id":"clube-a","playerDef":{"isFirstOwner":false,"loanDuration":7}},{"id":"outra","playerDef":{"isFirstOwner":true}}]}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(newer, []byte(`{"data":[{"id":"clube-a","playerDef":{"isFirstOwner":true,"loanDuration":0}},{"id":"clube-b","playerDef":{}}]}`), 0o600); err != nil {
		t.Fatal(err)
	}
	stamp := time.Now().Add(-time.Minute)
	if err := os.Chtimes(old, stamp, stamp); err != nil {
		t.Fatal(err)
	}
	states := RecoverClubItemStates(dir, map[string]bool{"clube-a": true, "clube-b": true})
	a, ok := states["clube-a"]
	if !ok || a.FirstOwner == nil || !*a.FirstOwner || a.Loan == nil || *a.Loan {
		t.Fatalf("procedência mais recente = %#v", a)
	}
	if _, ok := states["clube-b"]; ok {
		t.Fatal("ausência de atributos não pode virar estado falso")
	}
}

func TestClienteAcrescentaGalleryEmConfiguracaoAntigaSemTrocarPersonalizacao(t *testing.T) {
	client := New(Config{BaseURL: "https://example.test", Endpoints: map[string]string{"gallery_catalog": "/catalogo-proprio"}})
	if got := client.cfg.Endpoints["gallery_catalog"]; got != "/catalogo-proprio" {
		t.Fatalf("catálogo personalizado foi trocado: %q", got)
	}
	if got := client.cfg.Endpoints["gallery_pool"]; got != "/api/fut/gallery/fc{cycle}/sets/{setId}/pool/" {
		t.Fatalf("pool ausente em configuração antiga: %q", got)
	}
}
