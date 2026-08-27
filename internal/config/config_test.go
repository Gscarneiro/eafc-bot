package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// O usuário pensa em link, não em slug ("já passei o link"). As três formas
// abaixo têm que produzir o mesmo valor — é o que o fut.gg de fato aceita
// na URL, e é o único identificador que a rota do clube reconhece.
func TestNormalizeProfileAceitaOLinkColado(t *testing.T) {
	casos := map[string]string{
		"https://www.fut.gg/gg-club/BilingualBee/": "BilingualBee",
		"fut.gg/gg-club/BilingualBee":              "BilingualBee",
		"BilingualBee":                             "BilingualBee",
		"https://www.fut.gg/gg-club/BilingualBee":  "BilingualBee",
		"www.fut.gg/gg-club/BilingualBee/?x=1":     "BilingualBee",
		"":                                         "",
	}
	for in, want := range casos {
		if got := normalizeProfile(in); got != want {
			t.Errorf("normalizeProfile(%q) = %q, esperava %q", in, got, want)
		}
	}
}

// Ninguém escreve "futgg":{"platform":"PC"} à mão no config.json — só
// "platform":"PC" no topo. json.Unmarshal só sobrescreve o que o arquivo
// de fato traz, então sem o re-espelho explícito em Load, cfg.FutGG.Platform
// ficaria preso no "ps5" de Default() mesmo com outra plataforma no
// arquivo — e Client.SBCs escolheria o preço de solução errado (console x
// PC divergem de verdade, testado ao vivo em 22/08/2026).
func TestLoadEspelhaPlatformParaFutGG(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	b, _ := json.Marshal(map[string]any{"platform": "PC"})
	if err := os.WriteFile(path, b, 0o644); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.FutGG.Platform != "PC" {
		t.Errorf("cfg.FutGG.Platform = %q, esperava \"PC\" (espelhado de cfg.Platform)", cfg.FutGG.Platform)
	}
}

func TestApplyEditableAtualizaAtomico(t *testing.T) {
	cfg := Default()
	next := cfg.Editable()
	next.Report.MinGain = 3.5
	next.Serve.DailyAt = "06:30"
	if err := cfg.ApplyEditable(next); err != nil {
		t.Fatalf("ApplyEditable: %v", err)
	}
	if cfg.Report.MinGain != 3.5 || cfg.Serve.DailyAt != "06:30" {
		t.Fatalf("configuração não aplicada: %+v", cfg.Editable())
	}
}

func TestApplyEditableDescartaConfiguracaoInvalida(t *testing.T) {
	cfg := Default()
	before := cfg.Editable()
	next := before
	next.Market.MinRating = 110
	if err := cfg.ApplyEditable(next); err == nil {
		t.Fatal("ApplyEditable inválido deveria devolver erro")
	}
	if got := cfg.Editable(); got != before {
		t.Fatalf("configuração mudou apesar do erro: antes=%+v depois=%+v", before, got)
	}
}

func TestValidateRecusaReservaNegativa(t *testing.T) {
	cfg := Default()
	cfg.Market.Reserve = -1
	if err := cfg.Validate(); err == nil {
		t.Fatal("esperava erro para market.reserve negativo")
	}
}

func TestApplyEditableAtualizaReserva(t *testing.T) {
	cfg := Default()
	next := cfg.Editable()
	next.Market.Reserve = 50_000
	if err := cfg.ApplyEditable(next); err != nil {
		t.Fatalf("ApplyEditable: %v", err)
	}
	if cfg.Market.Reserve != 50_000 {
		t.Fatalf("Market.Reserve = %d, esperava 50000", cfg.Market.Reserve)
	}
}

// A mensagem que sai pro console/API pode conter a DSN inteira quando o
// driver do Postgres a ecoa de volta num erro de conexão — RedactSecrets é o
// ponto único que impede essa string de aparecer em texto visível.
func TestRedactSecretsApagaDSNECookie(t *testing.T) {
	cfg := Default()
	cfg.Postgres.DSN = "postgres://user:supersecreto@host/db"
	cfg.FutGG.SessionCookie = "session=abc123"

	msg := cfg.RedactSecrets("falha ao conectar em postgres://user:supersecreto@host/db: timeout (cookie session=abc123 rejeitado)")
	if strings.Contains(msg, "supersecreto") || strings.Contains(msg, "abc123") {
		t.Fatalf("segredo vazou na mensagem redigida: %q", msg)
	}
	if !strings.Contains(msg, "[redigido]") {
		t.Fatalf("mensagem não foi redigida: %q", msg)
	}
}

func TestSaveEditablePreservaSegredosEBlocosNaoEditaveis(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	original := []byte(`{"gamer_tag":"perfil","futgg":{"session_cookie":"segredo"},"market":{"min_rating":80},"custom":{"keep":true}}`)
	if err := os.WriteFile(path, original, 0o644); err != nil {
		t.Fatal(err)
	}
	cfg := Default()
	v := cfg.Editable()
	v.Market.MinRating = 90
	if err := cfg.SaveEditable(path, v); err != nil {
		t.Fatalf("SaveEditable: %v", err)
	}
	var got map[string]any
	b, _ := os.ReadFile(path)
	if err := json.Unmarshal(b, &got); err != nil {
		t.Fatal(err)
	}
	if got["gamer_tag"] != "perfil" || got["custom"].(map[string]any)["keep"] != true {
		t.Fatalf("blocos não editáveis foram perdidos: %s", b)
	}
	if got["futgg"].(map[string]any)["session_cookie"] != "segredo" {
		t.Fatalf("segredo foi alterado: %s", b)
	}
	if got["market"].(map[string]any)["min_rating"] != float64(90) {
		t.Fatalf("market não foi atualizado: %s", b)
	}
}

func TestSaveEvolutionProgressPreservaOutrasCartasEBlocos(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	original := []byte(`{"gamer_tag":"perfil","market":{"min_rating":80},"serve":{"evolution_progress":{"outra-carta":["Gold Standard"]}}}`)
	if err := os.WriteFile(path, original, 0o644); err != nil {
		t.Fatal(err)
	}
	cfg := Default()
	if err := cfg.SaveEvolutionProgress(path, "26-1", []string{"Salto"}); err != nil {
		t.Fatalf("SaveEvolutionProgress: %v", err)
	}
	var got map[string]any
	b, _ := os.ReadFile(path)
	if err := json.Unmarshal(b, &got); err != nil {
		t.Fatal(err)
	}
	if got["gamer_tag"] != "perfil" || got["market"].(map[string]any)["min_rating"] != float64(80) {
		t.Fatalf("blocos não relacionados foram perdidos: %s", b)
	}
	progress := got["serve"].(map[string]any)["evolution_progress"].(map[string]any)
	if outra, _ := progress["outra-carta"].([]any); len(outra) != 1 || outra[0] != "Gold Standard" {
		t.Fatalf("progresso de outra carta foi perdido: %s", b)
	}
	if nova, _ := progress["26-1"].([]any); len(nova) != 1 || nova[0] != "Salto" {
		t.Fatalf("progresso novo não foi gravado: %s", b)
	}
}

func TestSaveEvolutionProgressListaVaziaRemoveChave(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	original := []byte(`{"serve":{"evolution_progress":{"26-1":["Salto"]}}}`)
	if err := os.WriteFile(path, original, 0o644); err != nil {
		t.Fatal(err)
	}
	cfg := Default()
	if err := cfg.SaveEvolutionProgress(path, "26-1", nil); err != nil {
		t.Fatalf("SaveEvolutionProgress: %v", err)
	}
	var got map[string]any
	b, _ := os.ReadFile(path)
	if err := json.Unmarshal(b, &got); err != nil {
		t.Fatal(err)
	}
	progress := got["serve"].(map[string]any)["evolution_progress"].(map[string]any)
	if _, ok := progress["26-1"]; ok {
		t.Fatalf("chave deveria ter sido removida com lista vazia: %s", b)
	}
}
