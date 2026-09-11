package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gscarneiro/eafc-bot/internal/analyze"
	"github.com/gscarneiro/eafc-bot/internal/domain"
)

func editorSlotsDoSnapshot(snapSlots []domain.SquadSlot) []domain.VagaPlanoElenco {
	result := make([]domain.VagaPlanoElenco, 0, len(snapSlots))
	for _, slot := range snapSlots {
		result = append(result, domain.VagaPlanoElenco{
			Index: slot.Index, Posicao: slot.Position,
			Carta: domain.ReferenciaCartaElenco{PlayerID: slot.PlayerID},
		})
	}
	return result
}

func TestEditorDeElencoSalvaReferenciaEAvaliaXI(t *testing.T) {
	snap := fixtureSnapshotComGauntletDeSobra()
	srv, _ := newTestServerWithSnapshot(t, snap)
	slots := editorSlotsDoSnapshot(snap.Club.Squad.Starters)
	body, err := json.Marshal(PlanoElencoInput{
		Nome: "Principal", Formacao: snap.Club.Squad.Formation, OrigemFormacao: "manual", Vagas: slots,
	})
	if err != nil {
		t.Fatal(err)
	}

	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/api/planos/elenco/salvos", bytesReader(body)))
	if w.Code != http.StatusOK {
		t.Fatalf("salvando plano: status %d: %s", w.Code, w.Body.String())
	}
	saved := decodeJSON[planoElencoSalvoView](t, w)
	if saved.Plano.ID == "" || saved.Plano.Revisao != 1 {
		t.Fatalf("plano salvo inesperado: %+v", saved.Plano)
	}
	for _, slot := range saved.Plano.Vagas {
		if slot.Carta.PlayerID == 0 {
			t.Fatalf("vaga %d não preservou player_id", slot.Index)
		}
	}

	w = httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/api/planos/elenco/salvos/"+saved.Plano.ID+"/referencia", nil))
	if w.Code != http.StatusOK {
		t.Fatalf("aplicando referência: status %d: %s", w.Code, w.Body.String())
	}
	reference := decodeJSON[planoElencoSalvoView](t, w)
	if !reference.Plano.Referencia || reference.Plano.Revisao != 2 {
		t.Fatalf("referência não foi aplicada com revisão nova: %+v", reference.Plano)
	}

	evaluationBody, err := json.Marshal(struct {
		Formacao string                   `json:"formacao"`
		Vagas    []domain.VagaPlanoElenco `json:"vagas"`
	}{Formacao: snap.Club.Squad.Formation, Vagas: slots})
	if err != nil {
		t.Fatal(err)
	}
	w = httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/api/editor/elenco/avaliar", bytesReader(evaluationBody)))
	if w.Code != http.StatusOK {
		t.Fatalf("avaliando rascunho: status %d: %s", w.Code, w.Body.String())
	}
	evaluation := decodeJSON[AvaliacaoEditorElencoResponse](t, w)
	if evaluation.Status != "ok" || evaluation.Cobertura != 11 || evaluation.EloMaisFraco == nil {
		t.Fatalf("avaliação não descreveu o XI completo: %+v", evaluation)
	}
	if evaluation.EloMaisFraco.Nota <= 0 || evaluation.EloMaisFraco.Posicao == "" {
		t.Fatalf("elo mais fraco sem nota posicional: %+v", evaluation.EloMaisFraco)
	}
}

func TestAvaliacaoDoEditorIdentificaOConteudoDoRascunho(t *testing.T) {
	snap := fixtureSnapshotComGauntletDeSobra()
	srv, _ := newTestServerWithSnapshot(t, snap)
	slots := editorSlotsDoSnapshot(snap.Club.Squad.Starters)
	avaliar := func() AvaliacaoEditorElencoResponse {
		body, err := json.Marshal(struct {
			Formacao string                   `json:"formacao"`
			Vagas    []domain.VagaPlanoElenco `json:"vagas"`
		}{Formacao: snap.Club.Squad.Formation, Vagas: slots})
		if err != nil {
			t.Fatal(err)
		}
		w := httptest.NewRecorder()
		srv.Handler().ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/api/editor/elenco/avaliar", bytesReader(body)))
		if w.Code != http.StatusOK {
			t.Fatalf("avaliando rascunho: %d: %s", w.Code, w.Body.String())
		}
		return decodeJSON[AvaliacaoEditorElencoResponse](t, w)
	}
	primeira := avaliar()
	slots[10].Funcao = "Atacante móvel"
	segunda := avaliar()
	if primeira.RevisaoPlano == segunda.RevisaoPlano {
		t.Fatalf("rascunhos diferentes receberam a mesma revisão %q", primeira.RevisaoPlano)
	}
}

func TestEditorDeElencoRecusaPlayerIDAmbiguo(t *testing.T) {
	snap := fixtureSnapshotComGauntletDeSobra()
	first := snap.Club.Players[0]
	second := first
	first.ClubItemID, second.ClubItemID = "copia-a", "copia-b"
	snap.Club.Players[0] = first
	snap.Club.Players = append(snap.Club.Players, second)
	srv, _ := newTestServerWithSnapshot(t, snap)
	slots := editorSlotsDoSnapshot(snap.Club.Squad.Starters)
	body, err := json.Marshal(PlanoElencoInput{Nome: "Ambíguo", Formacao: snap.Club.Squad.Formation, OrigemFormacao: "manual", Vagas: slots})
	if err != nil {
		t.Fatal(err)
	}
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/api/planos/elenco/salvos", bytesReader(body)))
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status %d; queria 400 para cópia ambígua: %s", w.Code, w.Body.String())
	}
}

func TestEditorDeElencoRecusaAtualizacaoDeRevisaoAntiga(t *testing.T) {
	snap := fixtureSnapshotComGauntletDeSobra()
	srv, _ := newTestServerWithSnapshot(t, snap)
	input := PlanoElencoInput{
		Nome: "Concorrente", Formacao: snap.Club.Squad.Formation, OrigemFormacao: "confirmada",
		Vagas: editorSlotsDoSnapshot(snap.Club.Squad.Starters),
	}
	body, err := json.Marshal(input)
	if err != nil {
		t.Fatal(err)
	}
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/api/planos/elenco/salvos", bytesReader(body)))
	if w.Code != http.StatusOK {
		t.Fatalf("criando plano: %d: %s", w.Code, w.Body.String())
	}
	saved := decodeJSON[planoElencoSalvoView](t, w).Plano

	input.RevisaoEsperada = saved.Revisao
	body, _ = json.Marshal(input)
	w = httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, httptest.NewRequest(http.MethodPut, "/api/planos/elenco/salvos/"+saved.ID, bytesReader(body)))
	if w.Code != http.StatusOK {
		t.Fatalf("salvando revisão atual: %d: %s", w.Code, w.Body.String())
	}

	w = httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, httptest.NewRequest(http.MethodPut, "/api/planos/elenco/salvos/"+saved.ID, bytesReader(body)))
	if w.Code != http.StatusConflict {
		t.Fatalf("status = %d; queria 409 para a revisão antiga: %s", w.Code, w.Body.String())
	}
}

func TestReferenciaDoEditorRecalculaMercadoComONovoTitular(t *testing.T) {
	snap := fixtureSnapshotComGauntletDeSobra()
	// A carta de mercado vence os dois goleiros. A identidade do Current prova
	// que a recomendação usou o plano de referência, não o XI do snapshot.
	snap.Market = []domain.Player{{
		ID: 999, Name: "GK de mercado", Position: domain.GK, GGRating: 90, GGRatingPos: domain.GK,
		Price: domain.Price{Coins: 1_000},
	}}
	srv, _ := newTestServerWithSnapshot(t, snap)
	slots := editorSlotsDoSnapshot(snap.Club.Squad.Starters)
	slots[0].Carta = domain.ReferenciaCartaElenco{PlayerID: snap.Club.Players[1].ID}
	body, err := json.Marshal(PlanoElencoInput{
		Nome: "GK alternativo", Formacao: snap.Club.Squad.Formation, OrigemFormacao: "manual", EstiloJogo: "posse", Vagas: slots,
	})
	if err != nil {
		t.Fatal(err)
	}
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/api/planos/elenco/salvos", bytesReader(body)))
	if w.Code != http.StatusOK {
		t.Fatalf("salvando plano: status %d: %s", w.Code, w.Body.String())
	}
	saved := decodeJSON[planoElencoSalvoView](t, w)
	w = httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/api/planos/elenco/salvos/"+saved.Plano.ID+"/referencia", nil))
	if w.Code != http.StatusOK {
		t.Fatalf("aplicando referência: status %d: %s", w.Code, w.Body.String())
	}

	w = httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/mercado", nil))
	if w.Code != http.StatusOK {
		t.Fatalf("consultando mercado: status %d: %s", w.Code, w.Body.String())
	}
	mercado := decodeJSON[mercadoCollectionResponse](t, w)
	if len(mercado.Upgrades) != 1 {
		t.Fatalf("upgrades = %+v, esperava a melhoria do goleiro", mercado.Upgrades)
	}
	if got, want := mercado.Upgrades[0].Current.ID, snap.Club.Players[1].ID; got != want {
		t.Fatalf("upgrade comparou titular %d; queria o do plano de referência %d", got, want)
	}
	if got := mercado.Upgrades[0].CurrentEvaluation.Contexto.EstiloJogo; got != "posse" {
		t.Fatalf("contexto da referência = %q; queria estilo do plano posse", got)
	}
	w = httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/editor/elenco", nil))
	if w.Code != http.StatusOK {
		t.Fatalf("abrindo editor: status %d: %s", w.Code, w.Body.String())
	}
	editor := decodeJSON[SquadEditorResponse](t, w)
	if len(editor.Titulares) == 0 || editor.Titulares[0].Player.ID != snap.Club.Players[1].ID {
		t.Fatalf("editor não mostrou a referência ativa: %+v", editor.Titulares)
	}
}

func TestPlanoMantemCopiasFisicasIguaisEmVagasDistintas(t *testing.T) {
	base := domain.Club{Players: []domain.ClubPlayer{
		{Player: domain.Player{ID: 7, Name: "Cópia A", Position: domain.CB}, ClubItemID: "item-a"},
		{Player: domain.Player{ID: 7, Name: "Cópia B", Position: domain.CB}, ClubItemID: "item-b"},
	}}
	plan := domain.PlanoElencoSalvo{Vagas: []domain.VagaPlanoElenco{
		{Index: 0, Posicao: domain.CB, Carta: domain.ReferenciaCartaElenco{PlayerID: 7, ClubItemID: "item-a"}},
		{Index: 1, Posicao: domain.CB, Carta: domain.ReferenciaCartaElenco{PlayerID: 7, ClubItemID: "item-b"}},
	}}
	club, _, err := clubeParaPlano(base, plan)
	if err != nil {
		t.Fatalf("montando plano com cópias: %v", err)
	}
	for index, want := range []string{"item-a", "item-b"} {
		got, ok := club.PlayerForSlot(club.Squad.Starters[index])
		if !ok || got.ClubItemID != want {
			t.Fatalf("vaga %d resolveu %+v; queria a cópia %q", index, got, want)
		}
	}
}

func TestPlanoProtegeBancoDeSugestaoDeVenda(t *testing.T) {
	snap := fixtureSnapshotComGauntletDeSobra()
	plan := domain.PlanoElencoSalvo{
		Vagas: editorSlotsDoSnapshot(snap.Club.Squad.Starters),
		Banco: []domain.ReferenciaCartaElenco{{PlayerID: snap.Club.Players[1].ID}},
	}
	club, _, err := clubeParaPlano(snap.Club, plan)
	if err != nil {
		t.Fatalf("montando plano: %v", err)
	}
	protected := snap.Club.Players[1]
	if !club.IsProtected(protected) {
		t.Fatal("reserva do plano deveria estar protegida")
	}
	candidates, funnel := analyze.FindSellCandidates(club, nil, nil, analyze.DefaultSellOptions())
	for _, candidate := range candidates {
		if candidate.Player.IdentityKey() == protected.IdentityKey() {
			t.Fatalf("reserva protegida apareceu para venda: %+v", candidate)
		}
	}
	if funnel.Protected != 1 {
		t.Fatalf("protegidas = %d; queria 1", funnel.Protected)
	}
}

func TestEditorAceitaAlvoDeMercadoSemTransformarEmPosse(t *testing.T) {
	snap := fixtureSnapshotComGauntletDeSobra()
	alvo := domain.Player{ID: 990, Name: "Alvo ST", CommonName: "Alvo ST", Position: domain.ST, Rating: 91, GGRating: 93, GGRatingPos: domain.ST, Price: domain.Price{Coins: 120_000}}
	snap.Market = []domain.Player{alvo}
	snap.Upgrades = []analyze.Upgrade{{Slot: domain.ST, Candidate: alvo, GrossCost: alvo.Price.Coins}}
	srv, _ := newTestServerWithSnapshot(t, snap)

	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/editor/elenco", nil))
	if w.Code != http.StatusOK {
		t.Fatalf("abrindo editor: %d: %s", w.Code, w.Body.String())
	}
	editor := decodeJSON[SquadEditorResponse](t, w)
	if len(editor.Alvos) != 1 || editor.Alvos[0].Referencia.Origem != "mercado" {
		t.Fatalf("alvos do editor = %+v", editor.Alvos)
	}

	slots := editorSlotsDoSnapshot(snap.Club.Squad.Starters)
	slots[10].Carta = editor.Alvos[0].Referencia
	body, err := json.Marshal(PlanoElencoInput{Nome: "Com alvo", Formacao: snap.Club.Squad.Formation, OrigemFormacao: "confirmada", Vagas: slots})
	if err != nil {
		t.Fatal(err)
	}
	w = httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/api/planos/elenco/salvos", bytesReader(body)))
	if w.Code != http.StatusOK {
		t.Fatalf("salvando alvo: %d: %s", w.Code, w.Body.String())
	}
	saved := decodeJSON[planoElencoSalvoView](t, w)
	if saved.Plano.Vagas[10].Carta.Origem != "mercado" {
		t.Fatalf("plano perdeu a origem do alvo: %+v", saved.Plano.Vagas[10].Carta)
	}

	plan := domain.PlanoElencoSalvo{Vagas: slots, Banco: nil, Formacao: snap.Club.Squad.Formation}
	club, jogadores, err := clubeParaPlanoSnapshot(snap, plan)
	if err != nil {
		t.Fatal(err)
	}
	if len(jogadores) != 11 || !jogadores[10].AlvoPlano {
		t.Fatalf("alvo virou posse ou sumiu da simulação: %+v", jogadores)
	}
	_, funnel := analyze.FindUpgrades(club, snap.Market, analyze.DefaultUpgradeOptions(200_000))
	if funnel.Owned != 0 {
		t.Fatalf("alvo de mercado contou como carta possuída: %+v", funnel)
	}
}

func TestEditorPreservaEvolucaoEspecificaNoAlvo(t *testing.T) {
	snap := fixtureSnapshotComGauntletDeSobra()
	base := snap.Club.Players[10]
	result := base.Player
	result.Rating, result.GGRating, result.GGRatingPos = base.Rating+2, base.GGRating+2, domain.ST
	evo := domain.Evolution{ID: "evo-st", Name: "Evolução ST", CoinCost: 25_000}
	snap.EvoMatches = []analyze.EvoMatch{{Evolution: evo, Player: base, Result: result, Cost: evo.CoinCost}}
	alvos := alvosDoEditor(snap)
	if len(alvos) != 1 || alvos[0].Referencia.EvolucaoID != evo.ID || alvos[0].Referencia.Origem != "evolucao" {
		t.Fatalf("alvo de evolução = %+v", alvos)
	}
	resolved, err := jogadorDoSnapshot(snap, alvos[0].Referencia)
	if err != nil || !resolved.AlvoPlano || resolved.Rating != result.Rating {
		t.Fatalf("resolvendo alvo: jogador=%+v erro=%v", resolved, err)
	}
}

func TestTrocaDeAvaliadorRecalculaMercadoSemNovaColeta(t *testing.T) {
	snap := fixtureSnapshot()
	snap.Avaliacao = domain.ContextoAvaliacao{Fonte: domain.FonteFutGG, Ciclo: "26"}
	// Esta sugestão foi gravada para FUT.GG. FUTWIZ ainda não está disponível,
	// portanto a resposta atual precisa mostrar ausência explícita, não reciclar
	// a recomendação antiga numa escala incompatível.
	snap.Upgrades = []analyze.Upgrade{{Slot: domain.CM, Gain: 99}}
	srv, _ := newTestServerWithSnapshot(t, snap)
	srv.EvaluationContext = func() domain.ContextoAvaliacao {
		return domain.ContextoAvaliacao{Fonte: domain.FonteFUTWIZ, Ciclo: "26"}
	}
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/mercado", nil))
	if w.Code != http.StatusOK {
		t.Fatalf("consultando mercado: status %d: %s", w.Code, w.Body.String())
	}
	mercado := decodeJSON[mercadoCollectionResponse](t, w)
	if len(mercado.Upgrades) != 0 {
		t.Fatalf("trocou a fonte mas manteve upgrade de FUT.GG: %+v", mercado.Upgrades)
	}
}

func bytesReader(body []byte) *bytes.Reader { return bytes.NewReader(body) }
