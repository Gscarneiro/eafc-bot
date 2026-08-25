package futgg

import (
	"fmt"
	"time"

	"github.com/gscarneiro/eafc-bot/internal/domain"
)

// ObservationStatus resume, numa palavra, o quanto dá para confiar numa
// capability desta coleta — o vocabulário atravessa as fases do plano do
// copiloto (ver docs/planos/copiloto/README.md, "Contratos que atravessam
// as fases").
type ObservationStatus string

const (
	// StatusConfirmado: a fonte respondeu e não há ressalva conhecida.
	StatusConfirmado ObservationStatus = "confirmado"
	// StatusEstimado: a fonte respondeu, mas o dado passou por alguma
	// aproximação — hoje só usado por quem lê um snapshot ANTIGO, gravado
	// antes deste contrato existir (ver store.Snapshot.Capabilities).
	StatusEstimado ObservationStatus = "estimado"
	// StatusIncompleto: a fonte respondeu, mas faltou informação para
	// responder com certeza uma pergunta que o bot faz — por exemplo, duas
	// cópias da mesma carta no elenco e a escalação só trazendo o id do
	// jogador (playerEaId), não da cópia física em campo.
	StatusIncompleto ObservationStatus = "incompleto"
	// StatusIndisponivel: a fonte não trouxe dado nenhum, sem erro explícito
	// — clube sincronizado sem nenhuma carta é o caso central.
	StatusIndisponivel ObservationStatus = "indisponivel"
	// StatusErro: a busca falhou (rede, parsing, HTTP diferente de 2xx).
	StatusErro ObservationStatus = "erro"
)

// Observation é o metadado de procedência de UMA capability (clube, mercado,
// evoluções, objetivos, SBCs, notícias) nesta coleta — fonte, horário,
// cobertura, avisos, erro e estado, num resumo por capability em vez de
// envolver todo escalar do bot num wrapper genérico (ver o contrato da fase
// 01 em docs/planos/copiloto/01-fundacao-confiavel.md). É o que /api/saude
// expõe.
type Observation struct {
	Source     string            `json:"source"`
	ObservedAt time.Time         `json:"observed_at"`
	Coverage   int               `json:"coverage"` // itens trazidos por esta capability
	Status     ObservationStatus `json:"status"`
	Warnings   []string          `json:"warnings,omitempty"`
	Error      string            `json:"error,omitempty"`
}

// buildCapabilities monta o mapa de procedência a partir do que Collect já
// separou: um erro por capability (capErrs, populado por fail()) e o próprio
// snapshot já preenchido — coverage é sempre len(campo), não um contador à
// parte, para nunca divergir do dado de verdade devolvido.
func (c *Client) buildCapabilities(gamerTag string, capErrs map[string]error, snap *Snapshot) map[string]Observation {
	now := time.Now()
	obs := make(map[string]Observation, 6)

	simple := func(name string, coverage int) {
		o := Observation{Source: "futgg", ObservedAt: now, Coverage: coverage, Status: StatusConfirmado}
		if err, failed := capErrs[name]; failed {
			o.Status, o.Error, o.Coverage = StatusErro, err.Error(), 0
		}
		obs[name] = o
	}

	if gamerTag != "" {
		incomplete, estimated := clubObservationDetails(snap.Club)
		warnings := append(append([]string{}, incomplete...), estimated...)
		o := Observation{
			Source: "futgg", ObservedAt: now,
			Coverage: len(snap.Club.Players), Status: StatusConfirmado, Warnings: warnings,
		}
		switch {
		case capErrs["clube"] != nil:
			o.Status, o.Error, o.Coverage, o.Warnings = StatusErro, capErrs["clube"].Error(), 0, nil
		case len(snap.Club.Players) == 0:
			o.Status = StatusIndisponivel
		case len(incomplete) > 0:
			o.Status = StatusIncompleto
		case len(estimated) > 0:
			o.Status = StatusEstimado
		}
		obs["clube"] = o
	}
	simple("mercado", len(snap.Market))
	simple("evoluções", len(snap.Evolutions))
	simple("objetivos", len(snap.Objectives))
	simple("SBCs", len(snap.SBCs))
	simple("notícias", len(snap.News))
	return obs
}

// clubObservationDetails separa os dois jeitos de um clube não ser um
// "confirmado" simples: incomplete é o que falta pra responder uma pergunta
// com certeza (escalação não sincronizada, cópias ambíguas — StatusIncompleto);
// estimated é quando a fonte deu uma resposta usável, mas sem provar a
// identidade física de cada cópia — o ClubItemID continua ausente, e a
// identidade cai de volta no id da carta (StatusEstimado, ver o comentário
// de domain.ClubPlayer.ClubItemID e a "identidade derivada apenas como
// estimada" em docs/planos/copiloto/README.md). Incompleto pesa mais que
// estimado: faltar resposta é pior que ter uma aproximação.
func clubObservationDetails(club domain.Club) (incomplete, estimated []string) {
	if len(club.Squad.Starters) > 0 && !club.Squad.ChemistrySynced {
		incomplete = append(incomplete, "entrosamento não sincronizado com o jogo nesta coleta")
	}
	incomplete = append(incomplete, ambiguousStarterWarnings(club)...)
	if len(club.Players) > 0 && !anyClubItemID(club.Players) {
		estimated = append(estimated, "a fonte não forneceu identificador físico por cópia; "+
			"identidade das cartas é estimada (derivada do id da carta), sem linhagem por cópia")
	}
	return incomplete, estimated
}

func anyClubItemID(players []domain.ClubPlayer) bool {
	for _, p := range players {
		if p.ClubItemID != "" {
			return true
		}
	}
	return false
}

// ambiguousStarterWarnings sinaliza quando um titular da escalação tem mais
// de uma cópia no elenco: a rota /active-squad/ só informa o eaId
// (playerEaId) do jogador, não qual cópia física está em campo. Se a
// escalação só trouxer playerEaId e houver cópias ambíguas, o estado da
// capability precisa dizer "incompleto" em vez de escolher uma cópia em
// silêncio (CLAUDE.md: na dúvida, não afirma) — o resto do bot (Starter,
// PlayerByID) continua funcionando com a primeira cópia que casar, só que
// agora com o aviso explícito de que essa escolha pode estar errada.
func ambiguousStarterWarnings(club domain.Club) []string {
	counts := make(map[int64]int, len(club.Players))
	for _, p := range club.Players {
		counts[p.ID]++
	}
	seen := make(map[int64]bool, len(club.Squad.Starters))
	var warnings []string
	for _, slot := range club.Squad.Starters {
		if counts[slot.PlayerID] > 1 && !seen[slot.PlayerID] {
			seen[slot.PlayerID] = true
			warnings = append(warnings, fmt.Sprintf(
				"jogador %d tem %d cópias no elenco; a escalação só informa o id do jogador, não a cópia física em campo",
				slot.PlayerID, counts[slot.PlayerID]))
		}
	}
	return warnings
}
