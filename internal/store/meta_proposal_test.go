package store

import (
	"context"
	"testing"

	"github.com/gscarneiro/eafc-bot/internal/domain"
)

func TestPropostaMetaPreservaPacoteImutavelAoMudarStatus(t *testing.T) {
	st, err := NewJSON(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	proposal := domain.PropostaMeta{
		ID: "meta-1", Pacote: "sha256:abc", Ciclo: "27", Patch: "TU-1",
		Plataformas: []string{"ps5"}, Perfil: "meta_competitivo", VersaoCandidata: "2026.09.2",
		CriadaEm: "2026-09-10T00:00:00Z", AtualizadaEm: "2026-09-10T00:00:00Z", Status: "proposta",
		Fatos: []domain.EvidenciaMeta{{Descricao: "teste reproduzível", Fonte: "https://example.invalid", Data: "2026-09-10"}},
	}
	if err := st.SaveMetaProposal(context.Background(), proposal); err != nil {
		t.Fatal(err)
	}
	proposal.Status = "aprovada"
	if err := st.SaveMetaProposal(context.Background(), proposal); err != nil {
		t.Fatal(err)
	}
	proposal.Pacote = "sha256:alterado"
	if err := st.SaveMetaProposal(context.Background(), proposal); err == nil {
		t.Fatal("esperava recusa ao trocar o pacote imutável")
	}
	entries, err := st.ListMetaProposals(context.Background(), "27")
	if err != nil || len(entries) != 1 || entries[0].Status != "aprovada" {
		t.Fatalf("propostas = %+v, erro = %v", entries, err)
	}
}
