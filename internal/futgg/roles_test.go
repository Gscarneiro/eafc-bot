package futgg

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gscarneiro/eafc-bot/internal/domain"
)

// Os valores abaixo (id 128, 114) e os nomes/posições que eles resolvem
// (Wide Midfielder em LM, Holding em CDM) são os que confirmei ao vivo
// contra /api/fut/roles/ e a carta do Vitinha. rolesPlus e rolesPlusPlus
// usam ESPAÇOS DE ID DIFERENTES (plusEaId vs plusPlusEaId) — é por isso que
// a tabela guarda os dois separados.
func TestRolesCruzaIdComNomeEPosicao(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"data":[
			{"id":1,"name":"Wide Midfielder","position":16,"plusEaId":28,"plusPlusEaId":128},
			{"id":2,"name":"Holding","position":10,"plusEaId":14,"plusPlusEaId":114}
		]}`))
	}))
	defer srv.Close()

	c := New(Config{BaseURL: srv.URL, Cycle: "26", Endpoints: map[string]string{"roles": "/api/fut/roles/"}})
	rt := c.Roles(context.Background())

	got, ok := rt.PlusPlus[128]
	if !ok || got.Name != "Wide Midfielder" || got.Position != domain.LM {
		t.Errorf("PlusPlus[128] = %+v (ok=%v), esperava Wide Midfielder/LM", got, ok)
	}
	got, ok = rt.PlusPlus[114]
	if !ok || got.Name != "Holding" || got.Position != domain.CDM {
		t.Errorf("PlusPlus[114] = %+v (ok=%v), esperava Holding/CDM", got, ok)
	}
	if _, ok := rt.Plus[28]; !ok {
		t.Error("Plus[28] (plusEaId da Wide Midfielder) não foi indexado")
	}
	// plusEaId e plusPlusEaId são espaços diferentes: o id 128 (plusPlus)
	// não pode aparecer também em Plus.
	if _, ok := rt.Plus[128]; ok {
		t.Error("128 é plusPlusEaId, não devia estar em Plus")
	}
}

// Endpoint não configurado não pode travar quem chama — mesmo princípio de
// ensurePlayStyles: falha em silêncio, tabela vazia. (Sem registrar "roles"
// em Endpoints, c.URL() erra antes de qualquer rede — evita depender do
// backoff de rede real, que passa de 10s por causa das retentativas.)
func TestRolesFalhaEmSilencioSemEndpoint(t *testing.T) {
	c := New(Config{BaseURL: "https://www.fut.gg", Cycle: "26", Endpoints: map[string]string{}})
	rt := c.Roles(context.Background())
	if rt.Plus == nil || rt.PlusPlus == nil {
		t.Error("Roles() devia devolver mapas vazios, não nil, mesmo sem endpoint")
	}
}
