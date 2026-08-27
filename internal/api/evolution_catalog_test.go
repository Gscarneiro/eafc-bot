package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/gscarneiro/eafc-bot/internal/advisor"
	"github.com/gscarneiro/eafc-bot/internal/domain"
	"github.com/gscarneiro/eafc-bot/internal/store"
)

func TestHandleEvolutionCatalogRespeitaTaxonomiaExclusiva(t *testing.T) {
	snap := fixtureSnapshot()
	snap.Evolutions = []domain.Evolution{
		{ID: "normal", Name: "Normal", CoinCost: 10_000},
		{ID: "reward", Name: "Reward com PS+", IsRewardEvolution: true, ObjectiveGroupName: "Objetivo", Levels: []domain.EvoLevel{{Upgrades: []domain.EvoUpgrade{{Kind: "playstyle", PlayStyle: domain.PlayStyle{Name: "Rapid", Plus: true}}}}}},
		{ID: "lab", Name: "Lab", CategorySlug: "playstyle-lab", CategoryName: "PlayStyles Lab", Levels: []domain.EvoLevel{{Upgrades: []domain.EvoUpgrade{{Kind: "playstyle", PlayStyle: domain.PlayStyle{Name: "Tiki Taka", Plus: true}}}}}},
	}
	srv, _ := newTestServerWithSnapshot(t, snap)
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/evolucoes/catalogo", nil))
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d: %s", w.Code, w.Body.String())
	}
	got := decodeJSON[evolutionCatalogResponse](t, w)
	byCategory := map[string]int{}
	for _, category := range got.Summary.Categories {
		byCategory[category.Key] = category.Count
	}
	if byCategory[domain.EvolutionCategoryRewards] != 1 {
		t.Fatalf("rewards = %d, want 1; resumo=%+v", byCategory[domain.EvolutionCategoryRewards], got.Summary)
	}
	if byCategory[domain.EvolutionCategoryPlayStylesPlus] != 1 {
		t.Fatalf("playstyles+ = %d, want 1; resumo=%+v", byCategory[domain.EvolutionCategoryPlayStylesPlus], got.Summary)
	}
	for _, item := range got.Value {
		if item.Evolution.ID == "reward" && item.Category != domain.EvolutionCategoryRewards {
			t.Fatalf("recompensa foi categorizada como %q", item.Category)
		}
	}
}

func TestHandleEvolutionCatalogDetailProjetaCartaEReutilizaAgente(t *testing.T) {
	snap := fixtureSnapshot()
	value := 72
	snap.Club.Players[0].DetailedAttributes = &domain.DetailedAttributes{Finishing: &value}
	snap.Evolutions = []domain.Evolution{{
		ID: "evo", Slug: "evo", Name: "Evo", Requirements: nil,
		Levels: []domain.EvoLevel{{Upgrades: []domain.EvoUpgrade{{Kind: "sub_attribute", Attr: "finishing", Amount: 5}, {Kind: "playstyle", PlayStyle: domain.PlayStyle{Name: "Rapid", Plus: true}}}}},
	}}
	srv, _ := newTestServerWithSnapshot(t, snap)
	analysisDone := make(chan struct{})
	srv.EvolutionAdvisor = advisor.Func(func(_ context.Context, _ []byte) (advisor.AnalysisResult, error) {
		defer close(analysisDone)
		return advisor.AnalysisResult{Verdict: "recomendada", Summary: "boa", Sources: []advisor.Source{{Title: "fut.gg", URL: "https://www.fut.gg/evolutions/"}}}, nil
	})
	key := "card:1:0"
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/evolucoes/catalogo/evo?player_key="+url.QueryEscape(key), nil))
	if w.Code != http.StatusOK {
		t.Fatalf("detalhe status = %d: %s", w.Code, w.Body.String())
	}
	got := decodeJSON[EvolutionCatalogDetailResponse](t, w)
	if got.Projection == nil {
		t.Fatalf("projeção ausente")
	}
	if len(got.Projection.PlayStyles) != 1 || got.Projection.PlayStyles[0].Status != "adicionado" {
		t.Fatalf("playstyles = %+v", got.Projection.PlayStyles)
	}
	var finishing *EvolutionNumberChange
	for i := range got.Projection.DetailedChanges {
		if got.Projection.DetailedChanges[i].Key == "finishing" {
			finishing = &got.Projection.DetailedChanges[i]
			break
		}
	}
	if finishing == nil || finishing.Delta != 5 || !finishing.Available {
		t.Fatalf("subatributo finalização = %+v", finishing)
	}

	request := httptest.NewRequest(http.MethodPost, "/api/evolucoes/catalogo/evo/analises", strings.NewReader(`{"player_key":"`+key+`"}`))
	w = httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, request)
	if w.Code != http.StatusAccepted {
		t.Fatalf("análise status = %d: %s", w.Code, w.Body.String())
	}
	queued := decodeJSON[EvolutionAnalysisResponse](t, w)
	if queued.Analysis.ID == "" || queued.Analysis.InputHash == "" {
		t.Fatalf("fila sem identidade/hash: %+v", queued.Analysis)
	}
	select {
	case <-analysisDone:
	case <-time.After(time.Second):
		t.Fatal("agente não terminou no tempo do teste")
	}
	backend := srv.Store.(store.EvolutionAnalysisStore)
	deadline := time.Now().Add(time.Second)
	for {
		entries, err := backend.ListEvolutionAnalyses(t.Context(), "26", queued.Analysis.InputHash)
		if err == nil && len(entries) > 0 && entries[0].Status == "completed" {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("resultado do agente não foi persistido no tempo do teste")
		}
		time.Sleep(5 * time.Millisecond)
	}
}

func TestProjectEvolutionAvisaPlayStyleJaExistente(t *testing.T) {
	player := domain.Player{PlayStyles: []domain.PlayStyle{{Name: "Rapid", Plus: true}}}
	evo := domain.Evolution{Levels: []domain.EvoLevel{{Upgrades: []domain.EvoUpgrade{{Kind: "playstyle", PlayStyle: domain.PlayStyle{Name: "Rapid", Plus: true}}}}}}
	projection := projectEvolution(player, evo)
	if len(projection.PlayStyles) != 1 || !projection.PlayStyles[0].Existing || projection.PlayStyles[0].Status != "mantido" {
		t.Fatalf("playstyle = %+v", projection.PlayStyles)
	}
	found := false
	for _, warning := range projection.Warnings {
		if strings.Contains(warning, "já possui Rapid+") {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("warnings = %v, esperava aviso de duplicata", projection.Warnings)
	}
}

func TestProjectEvolutionNaoAvisaPlayStylesNaoAlterados(t *testing.T) {
	player := domain.Player{PlayStyles: []domain.PlayStyle{{Name: "Quick Step"}}}
	evo := domain.Evolution{Levels: []domain.EvoLevel{{Upgrades: []domain.EvoUpgrade{{Kind: "attribute", Attr: "pac", Amount: 2}}}}}
	projection := projectEvolution(player, evo)
	if len(projection.PlayStyles) != 0 {
		t.Fatalf("playstyles não alterados = %+v", projection.PlayStyles)
	}
	for _, warning := range projection.Warnings {
		if strings.Contains(warning, "Quick Step") {
			t.Fatalf("aviso indevido = %q", warning)
		}
	}
}
