package api

import (
	"encoding/json"
	"hash/fnv"
	"net/http"
	"strings"
	"time"

	"github.com/gscarneiro/eafc-bot/internal/analyze"
	"github.com/gscarneiro/eafc-bot/internal/domain"
	"github.com/gscarneiro/eafc-bot/internal/store"
)

func (s *Server) handleGameplayFeedback(w http.ResponseWriter, r *http.Request) {
	backend, ok := s.Store.(store.GameplayFeedbackStore)
	if !ok {
		http.Error(w, "armazenamento não suporta feedback de gameplay", http.StatusNotImplemented)
		return
	}
	entries, err := backend.ListGameplayFeedback(r.Context(), s.Cycle)
	if err != nil {
		http.Error(w, "lendo feedback de gameplay: "+err.Error(), http.StatusInternalServerError)
		return
	}
	for i := range entries {
		if entries[i].Amostra == "" {
			entries[i].Amostra = amostraFeedback(entries[i].ComparacaoID)
		}
	}
	writeJSON(w, struct {
		Value []domain.FeedbackGameplay `json:"value"`
		Count int                       `json:"@odata.count"`
	}{Value: entries, Count: len(entries)})
}

func (s *Server) handleGameplayFeedbackUpsert(w http.ResponseWriter, r *http.Request) {
	backend, ok := s.Store.(store.GameplayFeedbackStore)
	if !ok {
		http.Error(w, "armazenamento não suporta feedback de gameplay", http.StatusNotImplemented)
		return
	}
	var entry domain.FeedbackGameplay
	if err := json.NewDecoder(r.Body).Decode(&entry); err != nil {
		http.Error(w, "lendo feedback de gameplay: "+err.Error(), http.StatusBadRequest)
		return
	}
	entry.ComparacaoID, entry.CartaA, entry.CartaB = strings.TrimSpace(entry.ComparacaoID), strings.TrimSpace(entry.CartaA), strings.TrimSpace(entry.CartaB)
	if entry.ComparacaoID == "" || entry.CartaA == "" || entry.CartaB == "" {
		http.Error(w, "comparacao_id, carta_a e carta_b são obrigatórios", http.StatusBadRequest)
		return
	}
	switch entry.Preferencia {
	case "a", "b", "empate", "insuficiente":
	default:
		http.Error(w, "preferencia deve ser a, b, empate ou insuficiente", http.StatusBadRequest)
		return
	}
	entry.Ciclo = s.Cycle
	entry.Amostra = amostraFeedback(entry.ComparacaoID)
	if entry.ID == "" {
		entry.ID = localID("campo")
	}
	entry.RegistradoEm = time.Now().UTC().Format(time.RFC3339)
	if err := backend.UpsertGameplayFeedback(r.Context(), entry); err != nil {
		http.Error(w, "gravando feedback de gameplay: "+err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, entry)
}

// qualidadeGameplayResponse compara a ordem do perfil com a externa somente
// onde ambas cobrem a mesma comparação. Ajuste e avaliação ficam separados.
type qualidadeGameplayResponse struct {
	Perfil    string                           `json:"perfil"`
	Candidata analyze.QualidadeRankingGameplay `json:"candidata"`
	ExternaGG analyze.QualidadeRankingGameplay `json:"externa_gg"`
}

func (s *Server) handleGameplayFeedbackQuality(w http.ResponseWriter, r *http.Request) {
	backend, ok := s.Store.(store.GameplayFeedbackStore)
	if !ok {
		http.Error(w, "armazenamento não suporta feedback de gameplay", http.StatusNotImplemented)
		return
	}
	snap, loaded := s.load(w, r)
	if !loaded {
		return
	}
	entries, err := backend.ListGameplayFeedback(r.Context(), s.Cycle)
	if err != nil {
		http.Error(w, "lendo feedback de gameplay: "+err.Error(), http.StatusInternalServerError)
		return
	}
	for i := range entries {
		if entries[i].Amostra == "" {
			entries[i].Amostra = amostraFeedback(entries[i].ComparacaoID)
		}
	}
	perfil := strings.TrimSpace(r.URL.Query().Get("perfil"))
	if perfil == "" {
		perfil = snap.Avaliacao.Perfil
	}
	if perfil == "" {
		perfil = "meta_competitivo"
	}
	candidate := func(entry domain.FeedbackGameplay) (int, bool) {
		return s.ordemFeedback(snap, entry, domain.FonteBot, perfil)
	}
	externa := func(entry domain.FeedbackGameplay) (int, bool) {
		return s.ordemFeedback(snap, entry, domain.FonteFutGG, "")
	}
	writeJSON(w, qualidadeGameplayResponse{
		Perfil: perfil,
		// A tela publica somente a amostra reservada; a de ajuste continua
		// armazenada para calibrar sem contaminar a medição independente.
		Candidata: analyze.AvaliarQualidadeGameplay(entries, "avaliacao", candidate),
		ExternaGG: analyze.AvaliarQualidadeGameplay(entries, "avaliacao", externa),
	})
}

func (s *Server) ordemFeedback(snap store.Snapshot, entry domain.FeedbackGameplay, fonte domain.FonteAvaliacao, perfil string) (int, bool) {
	if entry.Posicao == "" {
		return 0, false
	}
	a, okA := cartaDoFeedback(snap.Club, entry.CartaAID, entry.CartaAClubItemID)
	b, okB := cartaDoFeedback(snap.Club, entry.CartaBID, entry.CartaBClubItemID)
	if !okA || !okB {
		return 0, false
	}
	ctx := snap.Avaliacao
	ctx.Fonte, ctx.Perfil, ctx.VersaoPerfil, ctx.VersaoMotor = fonte, perfil, "", ""
	ctx.Posicao, ctx.Funcao, ctx.EstiloJogo = entry.Posicao, entry.Funcao, entry.EstiloJogo
	if entry.Patch != "" {
		ctx.Patch = entry.Patch
	}
	if entry.Plataforma != "" {
		ctx.Plataforma = entry.Plataforma
	}
	ctx.Quimica = entry.Quimica
	evaluator := s.resolveEvaluator()
	if evaluator == nil {
		return 0, false
	}
	left, right := evaluator.Avaliar(a.Player, entry.Posicao, ctx), evaluator.Avaliar(b.Player, entry.Posicao, ctx)
	if !left.Disponivel || !right.Disponivel {
		return 0, false
	}
	delta := left.Nota - right.Nota
	if delta > 0.05 {
		return 1, true
	}
	if delta < -0.05 {
		return -1, true
	}
	return 0, true
}

func cartaDoFeedback(club domain.Club, id int64, itemID string) (domain.ClubPlayer, bool) {
	if itemID != "" {
		return club.PlayerForSlot(domain.SquadSlot{ClubItemID: itemID})
	}
	if id == 0 {
		return domain.ClubPlayer{}, false
	}
	var found []domain.ClubPlayer
	for _, player := range club.Players {
		if player.ID == id {
			found = append(found, player)
		}
	}
	if len(found) != 1 {
		return domain.ClubPlayer{}, false
	}
	return found[0], true
}

func amostraFeedback(comparacaoID string) string {
	// Uma em cinco comparações fica reservada para medir qualidade. O hash é
	// estável por contexto, então retentativas não mudam o grupo da evidência.
	h := fnv.New32a()
	_, _ = h.Write([]byte(comparacaoID))
	if h.Sum32()%5 == 0 {
		return "avaliacao"
	}
	return "ajuste"
}
