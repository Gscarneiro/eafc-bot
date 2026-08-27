package api

import (
	"bytes"
	"github.com/gscarneiro/eafc-bot/internal/domain"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestPlanoSBCNaoRecomendaQuandoEmprestimoNaoFoiComprovado(t *testing.T) {
	snap := fixtureSnapshot()
	snap.SBCs = []domain.SBC{{ID: "s", Challenges: []domain.SBCChallenge{{RequirementsText: []string{"Min. Team Rating: 85"}}}}}
	srv, _ := newTestServerWithSnapshot(t, snap)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/planos/sbc", bytes.NewBufferString(`{"sbc_id":"s","challenge":0}`))
	srv.Handler().ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status=%d %s", w.Code, w.Body.String())
	}
	if !bytes.Contains(w.Body.Bytes(), []byte(`"dados_indisponiveis"`)) {
		t.Fatalf("resposta=%s", w.Body.String())
	}
}
