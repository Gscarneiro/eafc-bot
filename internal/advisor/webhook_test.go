package advisor

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestNewWebhookAceitaHTTPSERestringeHTTPRemoto(t *testing.T) {
	if client, err := New("https://agent.example.test/analyze", "token"); err != nil || client == nil {
		t.Fatalf("HTTPS: client=%v err=%v", client, err)
	}
	if _, err := New("http://agent.example.test/analyze", ""); err == nil {
		t.Fatal("HTTP remoto deveria exigir HTTPS")
	}
	if client, err := New("http://127.0.0.1:8787/analyze", ""); err != nil || client == nil {
		t.Fatalf("HTTP local: client=%v err=%v", client, err)
	}
}

func TestValidateResultExigeVeredictSummaryEFonteHTTPS(t *testing.T) {
	valid := AnalysisResult{
		Verdict: "situacional", Summary: "depende do elenco",
		Sources: []Source{{Title: "fut.gg", URL: "https://www.fut.gg/evolutions/"}},
	}
	if err := ValidateResult(valid); err != nil {
		t.Fatalf("resultado válido: %v", err)
	}
	cases := []AnalysisResult{
		{Verdict: "talvez", Summary: "x", Sources: valid.Sources},
		{Verdict: "situacional", Sources: valid.Sources},
		{Verdict: "situacional", Summary: "x"},
		{Verdict: "situacional", Summary: "x", Sources: []Source{{Title: "local", URL: "http://localhost:1"}}},
	}
	for i, candidate := range cases {
		if err := ValidateResult(candidate); err == nil {
			t.Errorf("caso %d deveria falhar", i)
		}
	}
}

func TestWebhookEnviaContratoEValidaResposta(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.Header.Get("Authorization") != "Bearer segredo" {
			t.Errorf("request = %s auth=%q", r.Method, r.Header.Get("Authorization"))
		}
		body, _ := io.ReadAll(r.Body)
		if string(body) != `{"contract_version":"evolution-analysis.v1"}` {
			t.Errorf("payload = %s", body)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"verdict":"recomendada","summary":"boa","sources":[{"title":"fut.gg","url":"https://www.fut.gg/evolutions/"}]}`)
	}))
	defer server.Close()

	client, err := New(server.URL, "segredo")
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	result, err := client.Analyze(context.Background(), []byte(`{"contract_version":"evolution-analysis.v1"}`))
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}
	if result.Verdict != "recomendada" {
		t.Fatalf("verdict = %q", result.Verdict)
	}
}
