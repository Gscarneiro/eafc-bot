package store

import (
	"testing"
	"time"

	"github.com/gscarneiro/eafc-bot/internal/domain"
)

func TestJSONStorePathsSalvosIsolaCicloEFazUpsertERemocao(t *testing.T) {
	st, err := NewJSON(t.TempDir())
	if err != nil {
		t.Fatalf("NewJSON: %v", err)
	}
	path := domain.SavedEvolutionPath{
		ID: "path-1", PathID: "path-1", Cycle: "26", CardKey: "item:1",
		Path: domain.EvolutionPath{Steps: []domain.Player{{}, {ID: 1}}}, SavedAt: time.Now(),
	}
	if err := st.SaveEvolutionPath(t.Context(), path); err != nil {
		t.Fatalf("SaveEvolutionPath: %v", err)
	}
	path.VersionHash = "mudou"
	if err := st.SaveEvolutionPath(t.Context(), path); err != nil {
		t.Fatalf("upsert: %v", err)
	}
	other := path
	other.Cycle = "27"
	other.ID, other.PathID = "path-2", "path-2"
	if err := st.SaveEvolutionPath(t.Context(), other); err != nil {
		t.Fatalf("outro ciclo: %v", err)
	}

	got, err := st.ListSavedEvolutionPaths(t.Context(), "26")
	if err != nil || len(got) != 1 || got[0].VersionHash != "mudou" {
		t.Fatalf("ciclo 26 = %+v, erro=%v", got, err)
	}
	if err := st.DeleteSavedEvolutionPath(t.Context(), "26", "path-1"); err != nil {
		t.Fatalf("DeleteSavedEvolutionPath: %v", err)
	}
	got, err = st.ListSavedEvolutionPaths(t.Context(), "26")
	if err != nil || len(got) != 0 {
		t.Fatalf("remoção ciclo 26 = %+v, erro=%v", got, err)
	}
	got, err = st.ListSavedEvolutionPaths(t.Context(), "27")
	if err != nil || len(got) != 1 || got[0].ID != "path-2" {
		t.Fatalf("ciclo 27 = %+v, erro=%v", got, err)
	}
}
