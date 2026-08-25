package futgg

import (
	"errors"
	"testing"

	"github.com/gscarneiro/eafc-bot/internal/domain"
)

// Duas cópias da mesma carta são normais no FUT. A rota /active-squad/ só
// informa playerEaId — sem clubItemId nenhum —, então quando as duas cópias
// aparecem no elenco e uma delas está escalada, não dá pra saber QUAL das
// duas está em campo. Silenciar isso seria "consertar" a resposta com um
// palpite (CLAUDE.md: na dúvida, não afirma).
func TestAmbiguousStarterWarningsSinalizaDuasCopiasNaEscalacao(t *testing.T) {
	club := domain.Club{
		Players: []domain.ClubPlayer{
			{Player: domain.Player{ID: 42}},
			{Player: domain.Player{ID: 42}},
			{Player: domain.Player{ID: 7}},
		},
		Squad: domain.Squad{Starters: []domain.SquadSlot{
			{Index: 0, Position: domain.CM, PlayerID: 42},
			{Index: 1, Position: domain.CB, PlayerID: 7},
		}},
	}
	warnings := ambiguousStarterWarnings(club)
	if len(warnings) != 1 {
		t.Fatalf("esperava 1 aviso (o jogador 42 ambíguo), veio %d: %v", len(warnings), warnings)
	}
}

// Sem cópia duplicada nenhuma, não há aviso — a escalação não é ambígua.
func TestAmbiguousStarterWarningsSemDuplicataNaoAvisa(t *testing.T) {
	club := domain.Club{
		Players: []domain.ClubPlayer{{Player: domain.Player{ID: 42}}, {Player: domain.Player{ID: 7}}},
		Squad:   domain.Squad{Starters: []domain.SquadSlot{{Index: 0, Position: domain.CM, PlayerID: 42}}},
	}
	if warnings := ambiguousStarterWarnings(club); len(warnings) != 0 {
		t.Fatalf("esperava zero avisos, veio %v", warnings)
	}
}

// buildCapabilities é a peça que /api/saude expõe: erro vira StatusErro com
// cobertura zerada, sucesso vira StatusConfirmado com a contagem de itens.
func TestBuildCapabilitiesRefleteErroECobertura(t *testing.T) {
	c := &Client{}
	snap := &Snapshot{
		Club:   domain.Club{Players: []domain.ClubPlayer{{Player: domain.Player{ID: 1}, ClubItemID: "item-1"}}},
		Market: []domain.Player{{ID: 1}, {ID: 2}},
	}
	capErrs := map[string]error{"mercado": errors.New("HTTP 500")}

	obs := c.buildCapabilities("BilingualBee", capErrs, snap)

	clube := obs["clube"]
	if clube.Status != StatusConfirmado || clube.Coverage != 1 {
		t.Fatalf("clube = %+v, esperava confirmado com cobertura 1", clube)
	}
	mercado := obs["mercado"]
	if mercado.Status != StatusErro || mercado.Coverage != 0 || mercado.Error == "" {
		t.Fatalf("mercado = %+v, esperava erro com cobertura zerada", mercado)
	}
}

// Clube sincronizado sem nenhuma carta é indisponível, não confirmado —
// coleta que respondeu 200 mas sem elenco nenhum precisa continuar visível
// como problema em /api/saude, mesmo sem erro de rede explícito.
func TestBuildCapabilitiesClubeVazioViraIndisponivel(t *testing.T) {
	c := &Client{}
	snap := &Snapshot{Club: domain.Club{}}
	obs := c.buildCapabilities("BilingualBee", map[string]error{}, snap)
	if got := obs["clube"].Status; got != StatusIndisponivel {
		t.Fatalf("status do clube vazio = %q, esperava %q", got, StatusIndisponivel)
	}
}

// A ambiguidade de cópias detectada por ambiguousStarterWarnings precisa
// chegar como StatusIncompleto no mapa de capabilities, não só como aviso
// solto — é o que faz /api/saude sinalizar o problema sem o usuário ter que
// ler cada warning para perceber que o estado não é "confirmado".
func TestBuildCapabilitiesClubeComEscalacaoAmbiguaViraIncompleto(t *testing.T) {
	c := &Client{}
	snap := &Snapshot{Club: domain.Club{
		Players: []domain.ClubPlayer{{Player: domain.Player{ID: 42}}, {Player: domain.Player{ID: 42}}},
		Squad:   domain.Squad{Starters: []domain.SquadSlot{{Index: 0, Position: domain.CM, PlayerID: 42}}},
	}}
	obs := c.buildCapabilities("BilingualBee", map[string]error{}, snap)
	got := obs["clube"]
	if got.Status != StatusIncompleto || len(got.Warnings) == 0 {
		t.Fatalf("clube = %+v, esperava incompleto com aviso de ambiguidade", got)
	}
}

// Sem ClubItemID em NENHUMA carta, a fonte não provou identidade física
// nenhuma — o clube ainda é usável (não é erro, não é incompleto: nenhuma
// pergunta ficou sem resposta), mas a identidade das cartas é uma
// aproximação, não confirmada. Ver domain.ClubPlayer.ClubItemID.
func TestBuildCapabilitiesSemClubItemIDViraEstimado(t *testing.T) {
	c := &Client{}
	snap := &Snapshot{Club: domain.Club{Players: []domain.ClubPlayer{{Player: domain.Player{ID: 1}}}}}
	obs := c.buildCapabilities("BilingualBee", map[string]error{}, snap)
	got := obs["clube"]
	if got.Status != StatusEstimado || len(got.Warnings) == 0 {
		t.Fatalf("clube = %+v, esperava estimado com aviso de identidade", got)
	}
}

// Sem gamer tag configurado, Collect nem tenta buscar o clube — não faz
// sentido reportar uma capability "clube" que nunca foi perguntada.
func TestBuildCapabilitiesSemGamerTagNaoReportaClube(t *testing.T) {
	c := &Client{}
	obs := c.buildCapabilities("", map[string]error{}, &Snapshot{})
	if _, ok := obs["clube"]; ok {
		t.Fatalf("esperava nenhuma entrada \"clube\" sem gamer tag, veio %+v", obs["clube"])
	}
}
