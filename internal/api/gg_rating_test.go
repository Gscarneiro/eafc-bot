package api

import (
	"math"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gscarneiro/eafc-bot/internal/analyze"
	"github.com/gscarneiro/eafc-bot/internal/domain"
	"github.com/gscarneiro/eafc-bot/internal/store"
)

func snapshotComPlanosDeNotasAntigas() store.Snapshot {
	snap := fixtureSnapshotComGauntletDeSobra()
	// Completa oito cartas por posição para o formato padrão de quatro rodadas.
	for i, slot := range snap.Club.Squad.Starters {
		for j := 0; j < 2; j++ {
			p := snap.Club.Players[i*6]
			p.ID = int64(1000 + i*2 + j)
			p.BasePlayerEaID = p.ID
			p.GGRating = 66 + float64(j)
			p.GGRatingPos = slot.Position
			snap.Club.Players = append(snap.Club.Players, p)
		}
	}
	snap.GauntletPlan = analyze.BuildGauntletPlan(snap.Club)
	snap.SquadPlan = analyze.OptimizeSquad(snap.Club)
	snap.GauntletPlan.RatingVersion = 1
	snap.SquadPlan.RatingVersion = 1
	// A cópia tem nota maior, mas o plano salvo foi calculado só com o metarank.
	snap.Club.Players[1].GGRatings = map[domain.Position]float64{domain.GK: 61}
	snap.Club.Players[1].GGRating = 99.74
	return snap
}

func TestGauntletRecalculaEscolhasDePlanoComNotasAntigas(t *testing.T) {
	snap := snapshotComPlanosDeNotasAntigas()
	srv, _ := newTestServerWithSnapshot(t, snap)
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/gauntlet", nil))
	got := decodeJSON[GauntletResponse](t, w)
	if got.Status != "ok" {
		t.Fatalf("plano indisponível: %s", got.Reason)
	}
	var achou bool
	for _, round := range got.Rounds {
		var total float64
		for _, a := range round.Starters {
			total += a.Rating
			if a.Player.ID == snap.Club.Players[1].ID && a.Rating == 99.74 {
				achou = true
			}
		}
		if math.Abs(round.TotalRating-total) > 1e-9 || math.Abs(round.AverageRating-total/11) > 1e-9 {
			t.Fatalf("totais antigos na rodada %d", round.Round)
		}
	}
	if !achou {
		t.Fatal("plano antigo não foi refeito para escalar a carta com nota 99.74")
	}
}

func TestMeuTimeRecalculaEscolhasDePlanoComNotasAntigas(t *testing.T) {
	snap := snapshotComPlanosDeNotasAntigas()
	srv, _ := newTestServerWithSnapshot(t, snap)
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/time", nil))
	got := decodeJSON[TimeResponse](t, w)
	esperado := (99.74 + 10*67) / 11
	if math.Abs(got.Optimization.SuggestedAverage-esperado) > 1e-9 || math.Abs(got.Optimization.Gain-(esperado-60)) > 1e-9 {
		t.Fatalf("média ou ganho antigos: %+v", got.Optimization)
	}
	for _, m := range got.Optimization.Moves {
		if m.Suggested.Player.ID == snap.Club.Players[1].ID && m.SuggestedGGRating == 99.74 {
			return
		}
	}
	t.Fatal("plano antigo não foi refeito para sugerir a carta com nota 99.74")
}

func TestMeuTimeReutilizaPlanoComVersaoAtualDasNotas(t *testing.T) {
	snap := snapshotComPlanosDeNotasAntigas()
	snap.SquadPlan.RatingVersion = domain.GGRatingVersion
	snap.SquadPlan.SuggestedAverage = 17
	srv, _ := newTestServerWithSnapshot(t, snap)
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/time", nil))
	got := decodeJSON[TimeResponse](t, w)
	if got.Optimization.SuggestedAverage != 17 {
		t.Fatal("plano com versão atual foi recalculado desnecessariamente")
	}
}

func TestPlanejadorEntregaNotaCorrigidaParaOCampo(t *testing.T) {
	snap := snapshotComPlanosDeNotasAntigas()
	srv, _ := newTestServerWithSnapshot(t, snap)
	got := decodeJSON[SquadPlanResponse](t, postSquadPlan(t, srv, ""))
	if len(got.Scenarios) == 0 {
		t.Fatal("nenhum cenário calculado")
	}
	for _, sc := range got.Scenarios {
		if sc.Starters[0].Player.ID != snap.Club.Players[1].ID || sc.Starters[0].Rating == nil || *sc.Starters[0].Rating != 99.74 {
			t.Fatalf("titular incorreto: %+v", sc.Starters[0])
		}
	}
}

func TestMeuTimeEReservasIgnoramMetarankDeSnapshotAntigo(t *testing.T) {
	titular := domain.ClubPlayer{Player: domain.Player{
		ID: 267234, Name: "Kerolin Nicoli", Rating: 85, Position: domain.ST,
		GGRating: 84, GGRatingPos: domain.RM, GGRatings: map[domain.Position]float64{domain.ST: 82.79},
	}}
	reserva := domain.ClubPlayer{Player: domain.Player{
		ID: 211110, Name: "Paulo Dybala", Rating: 85, Position: domain.CAM, AltPositions: []domain.Position{domain.ST},
		GGRating: 83.95, GGRatingPos: domain.CAM, GGRatings: map[domain.Position]float64{domain.ST: 86.28},
	}}
	snap := fixtureSnapshot()
	snap.Club.Players = []domain.ClubPlayer{titular, reserva}
	snap.Club.Squad.Starters = []domain.SquadSlot{{Index: 10, Position: domain.ST, PlayerID: titular.ID}}
	snap.SquadSwaps = []analyze.SquadSwap{{Index: 10, Slot: domain.ST, Current: titular, Candidate: reserva,
		CurrentRating: 82.79, CandidateRating: 86.28, GGRatingGap: 3.49}}
	srv, _ := newTestServerWithSnapshot(t, snap)
	for _, endpoint := range []string{"/api/time", "/api/elenco/reservas"} {
		t.Run(endpoint, func(t *testing.T) {
			w := httptest.NewRecorder()
			srv.Handler().ServeHTTP(w, httptest.NewRequest(http.MethodGet, endpoint, nil))
			if w.Code != http.StatusOK {
				t.Fatalf("status = %d: %s", w.Code, w.Body.String())
			}
			var banco []RosterCard
			if endpoint == "/api/time" {
				banco = decodeJSON[TimeResponse](t, w).Bench
			} else {
				banco = decodeJSON[struct {
					Value []RosterCard `json:"value"`
				}](t, w).Value
			}
			if len(banco) != 1 || banco[0].Player.ID != reserva.ID || banco[0].Player.GGRating != 83.95 {
				t.Fatalf("banco não preservou Dybala e seu GG Rating publicado: %+v", banco)
			}
			if leitura := banco[0].Leitura; leitura != nil && (leitura.Kind == "promover" || leitura.Promocao != nil) {
				t.Fatalf("promoção antiga de metarank vazou na resposta: %+v", leitura)
			}
		})
	}
}
