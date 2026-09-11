package main

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/gscarneiro/eafc-bot/internal/config"
	"github.com/gscarneiro/eafc-bot/internal/domain"
	"github.com/gscarneiro/eafc-bot/internal/store"
)

func TestPerfilAtivarEReverterPreservaEscolhaAnterior(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	cfg := config.Default()
	cfg.DataDir = filepath.Join(t.TempDir(), "dados")
	if err := cfg.Save(path); err != nil {
		t.Fatal(err)
	}
	st, err := store.NewJSON(cfg.DataDir)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC().Format(time.RFC3339)
	proposal := domain.PropostaMeta{
		ID: "meta-posse", Pacote: "sha256:posse", Ciclo: cfg.FutGG.Cycle, Patch: "TU-1",
		Plataformas: []string{"ps5"}, Perfil: "posse", VersaoCandidata: "2026.09.2",
		CriadaEm: now, AtualizadaEm: now, Status: "aprovada",
		Fatos: []domain.EvidenciaMeta{{Descricao: "regressão aprovada", Fonte: "teste local", Data: "2026-09-10"}},
	}
	if err := st.SaveMetaProposal(context.Background(), proposal); err != nil {
		t.Fatal(err)
	}
	if err := cmdPerfilAtivar(context.Background(), []string{"-config", path, "-perfil", "posse"}); err != nil {
		t.Fatalf("ativando perfil: %v", err)
	}
	after, err := config.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if !after.Evaluation.UseBot || after.Evaluation.Profile != "posse" {
		t.Fatalf("perfil ativo inesperado: %+v", after.Evaluation)
	}
	if err := cmdPerfilReverter(context.Background(), []string{"-config", path}); err != nil {
		t.Fatalf("revertendo perfil: %v", err)
	}
	reverted, err := config.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if reverted.Evaluation.UseBot || reverted.Evaluation.ExternalSource != "futgg" {
		t.Fatalf("preferência anterior não foi restaurada: %+v", reverted.Evaluation)
	}
	history, err := loadProfileHistory(profileHistoryPath(path))
	if err != nil {
		t.Fatal(err)
	}
	if len(history.Eventos) != 2 || len(history.Pilha) != 0 {
		t.Fatalf("histórico de ativação inesperado: %+v", history)
	}
}

func TestPerfilExperimentalNaoAtivaSemPropostaAprovada(t *testing.T) {
	base := t.TempDir()
	path := filepath.Join(base, "config.json")
	cfg := config.Default()
	cfg.DataDir = filepath.Join(base, "dados")
	if err := cfg.Save(path); err != nil {
		t.Fatal(err)
	}
	if err := cmdPerfilAtivar(context.Background(), []string{"-config", path, "-perfil", "posse"}); err == nil {
		t.Fatal("esperava recusa sem proposta aprovada")
	}
	after, err := config.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if after.Evaluation.UseBot {
		t.Fatal("perfil foi ativado apesar da ausência de aprovação")
	}
}

func TestPerfilProporEAprovarPersistePacoteAuditavel(t *testing.T) {
	base := t.TempDir()
	path := filepath.Join(base, "config.json")
	cfg := config.Default()
	cfg.DataDir = filepath.Join(base, "dados")
	if err := cfg.Save(path); err != nil {
		t.Fatal(err)
	}
	args := []string{
		"-config", path, "-perfil", "meta_competitivo", "-patch", "TU-2",
		"-mecanicas", "pressão,aceleração", "-impacto", "reordenar atacantes",
		"-fato", "nota oficial publicada|https://example.invalid/tu2|2026-09-10|todas as plataformas",
		"-hipotese", "explosivo ganhou valor|teste interno|2026-09-10|atacantes",
		"-mudanca-codigo", "modelar nova mecânica|codex/meta-aceleracao|internal/analyze/evaluation.go",
	}
	if err := cmdPerfilPropor(context.Background(), args); err != nil {
		t.Fatal(err)
	}
	st, err := store.NewJSON(cfg.DataDir)
	if err != nil {
		t.Fatal(err)
	}
	proposals, err := st.ListMetaProposals(context.Background(), cfg.FutGG.Cycle)
	if err != nil || len(proposals) != 1 {
		t.Fatalf("propostas = %+v, erro = %v", proposals, err)
	}
	proposal := proposals[0]
	if proposal.Pacote == "" || len(proposal.Fatos) != 1 || len(proposal.Hipoteses) != 1 || len(proposal.MudancasCodigo) != 1 {
		t.Fatalf("pacote incompleto: %+v", proposal)
	}
	if err := cmdPerfilDecidir(context.Background(), []string{"-config", path, "-proposta", proposal.ID}, "aprovada"); err != nil {
		t.Fatal(err)
	}
	proposals, err = st.ListMetaProposals(context.Background(), cfg.FutGG.Cycle)
	if err != nil || proposals[0].Status != "aprovada" {
		t.Fatalf("aprovação não persistiu: %+v, erro = %v", proposals, err)
	}
}
