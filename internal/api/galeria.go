package api

import (
	"encoding/json"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/gscarneiro/eafc-bot/internal/galeria"
	"github.com/gscarneiro/eafc-bot/internal/store"
)

type galleryPage struct {
	Value         []galeria.Record `json:"value"`
	Count         int              `json:"@odata.count"`
	Skip          int              `json:"@eafc.skip"`
	Top           int              `json:"@eafc.top"`
	Opportunities int              `json:"opportunities"`
	Facets        map[string]int   `json:"@eafc.facets,omitempty"`
}

func (s *Server) galleryStore(w http.ResponseWriter) (store.GaleriaStore, bool) {
	gs, ok := s.Store.(store.GaleriaStore)
	if !ok {
		http.Error(w, "FUT Gallery indisponível neste armazenamento", http.StatusNotImplemented)
		return nil, false
	}
	return gs, true
}

func (s *Server) galleryContext(r *http.Request) (string, string, string) {
	cycle, club, platform := s.Cycle, "default", "default"
	if snap, ok, err := s.Store.LatestSnapshot(r.Context(), s.Cycle); err == nil && ok {
		cycle = snap.Cycle
		club = snap.Club.GamerTag
		platform = snap.Club.Platform
	}
	return cycle, club, platform
}

func (s *Server) galleryRows(r *http.Request, gs store.GaleriaStore, cycle, club, platform string) ([]galeria.Record, error) {
	rows, err := gs.ListGallery(r.Context(), cycle, club, platform)
	if err != nil {
		return nil, err
	}
	completions, err := gs.ListGalleryCompletions(r.Context(), cycle, club, platform)
	if err != nil {
		return rows, nil
	}
	byID := make(map[string]galeria.Completion, len(completions))
	for _, c := range completions {
		byID[c.SetID] = c
	}
	for i := range rows {
		if c, ok := byID[rows[i].Set.ID]; ok {
			cc := c
			rows[i].Completion = &cc
		}
	}
	return rows, nil
}

func (s *Server) handleGaleria(w http.ResponseWriter, r *http.Request) {
	gs, ok := s.galleryStore(w)
	if !ok {
		return
	}
	cycle, club, platform := s.galleryContext(r)
	rows, err := s.galleryRows(r, gs, cycle, club, platform)
	if err != nil {
		http.Error(w, "lendo FUT Gallery: "+err.Error(), http.StatusInternalServerError)
		return
	}
	if rows == nil {
		rows = []galeria.Record{}
	}
	search := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("$search")))
	status := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("status")))
	filter := strings.TrimSpace(r.URL.Query().Get("$filter"))
	if status == "" {
		lower := strings.ToLower(filter)
		if i := strings.Index(lower, "status eq"); i >= 0 {
			part := strings.TrimSpace(filter[i+len("status eq"):])
			part = strings.Trim(part, " '\"")
			if part != "" {
				status = part
			}
		}
	}
	filtered := rows[:0]
	for _, row := range rows {
		if search != "" && !strings.Contains(strings.ToLower(row.Set.Name), search) {
			continue
		}
		if status != "" && string(row.Evaluation.Status) != status {
			continue
		}
		if strings.Contains(filter, "conclu") && row.Completion == nil {
			continue
		}
		if strings.Contains(filter, "dispon") && !row.Notify() {
			continue
		}
		if strings.Contains(filter, "pend") && len(row.Evaluation.Warnings) == 0 {
			continue
		}
		filtered = append(filtered, row)
	}
	sort.SliceStable(filtered, func(i, j int) bool {
		if filtered[i].Notify() != filtered[j].Notify() {
			return filtered[i].Notify()
		}
		return filtered[i].Set.Name < filtered[j].Set.Name
	})
	if order := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("$orderby"))); strings.Contains(order, "score") {
		sort.SliceStable(filtered, func(i, j int) bool {
			if strings.Contains(order, "desc") {
				return filtered[i].Evaluation.Score > filtered[j].Evaluation.Score
			}
			return filtered[i].Evaluation.Score < filtered[j].Evaluation.Score
		})
	}
	skip, _ := strconv.Atoi(r.URL.Query().Get("$skip"))
	if skip < 0 {
		skip = 0
	}
	top, _ := strconv.Atoi(r.URL.Query().Get("$top"))
	if top <= 0 || top > 500 {
		top = 500
	}
	count := len(filtered)
	if skip > count {
		skip = count
	}
	end := skip + top
	if end > count {
		end = count
	}
	page := filtered[skip:end]
	opp := 0
	for _, row := range filtered {
		if row.Notify() {
			opp++
		}
	}
	facets := map[string]int{}
	for _, row := range filtered {
		facets[string(row.Evaluation.Status)]++
	}
	writeJSON(w, galleryPage{Value: page, Count: count, Skip: skip, Top: len(page), Opportunities: opp, Facets: facets})
}

func (s *Server) handleGaleriaDetalhe(w http.ResponseWriter, r *http.Request) {
	gs, ok := s.galleryStore(w)
	if !ok {
		return
	}
	id := r.PathValue("id")
	cycle, club, platform := s.galleryContext(r)
	rows, err := s.galleryRows(r, gs, cycle, club, platform)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	for _, row := range rows {
		if row.Set.ID == id {
			writeJSON(w, row)
			return
		}
	}
	http.Error(w, "conjunto Gallery não encontrado", http.StatusNotFound)
}

type galleryCompletionRequest struct {
	Grade       galeria.Grade `json:"grade"`
	Score       int           `json:"score"`
	CompletedAt string        `json:"completed_at"`
	Notes       string        `json:"notes"`
}

func (s *Server) handleGaleriaConclusao(w http.ResponseWriter, r *http.Request) {
	gs, ok := s.galleryStore(w)
	if !ok {
		return
	}
	id := r.PathValue("id")
	var in galleryCompletionRequest
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		http.Error(w, "conclusão inválida", 400)
		return
	}
	if in.Grade != galeria.GradeD && in.Grade != galeria.GradeC && in.Grade != galeria.GradeB && in.Grade != galeria.GradeA && in.Grade != galeria.GradeS {
		http.Error(w, "letra deve ser D, C, B, A ou S", 422)
		return
	}
	if in.Score < 0 {
		http.Error(w, "pontuação não pode ser negativa", http.StatusUnprocessableEntity)
		return
	}
	if strings.TrimSpace(in.CompletedAt) == "" {
		http.Error(w, "data de conclusão é obrigatória", http.StatusUnprocessableEntity)
		return
	}
	completedAt := time.Now()
	if strings.TrimSpace(in.CompletedAt) != "" {
		parsed, parseErr := time.Parse(time.RFC3339, in.CompletedAt)
		if parseErr != nil {
			parsed, parseErr = time.Parse("2006-01-02", in.CompletedAt)
			if parseErr != nil {
				http.Error(w, "data de conclusão inválida", http.StatusUnprocessableEntity)
				return
			}
		}
		completedAt = parsed
	}
	cycle, club, platform := s.galleryContext(r)
	rowsBefore, rowsErr := s.galleryRows(r, gs, cycle, club, platform)
	if rowsErr != nil {
		http.Error(w, rowsErr.Error(), http.StatusInternalServerError)
		return
	}
	knownSet := false
	for _, row := range rowsBefore {
		if row.Set.ID == id {
			knownSet = true
			break
		}
	}
	if !knownSet {
		http.Error(w, "conjunto Gallery não encontrado", http.StatusNotFound)
		return
	}
	completion := galeria.Completion{SetID: id, Grade: in.Grade, Score: in.Score, CompletedAt: completedAt, Notes: in.Notes}
	if err := gs.SaveGalleryCompletion(r.Context(), cycle, club, platform, completion); err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	rows, err := s.galleryRows(r, gs, cycle, club, platform)
	if err == nil {
		for i := range rows {
			if rows[i].Set.ID == id {
				rows[i].Completion = &completion
			}
		}
		_ = gs.SaveGallery(r.Context(), cycle, club, platform, rows)
	}
	writeJSON(w, completion)
}
func (s *Server) handleGaleriaConclusaoDelete(w http.ResponseWriter, r *http.Request) {
	gs, ok := s.galleryStore(w)
	if !ok {
		return
	}
	cycle, club, platform := s.galleryContext(r)
	if err := gs.DeleteGalleryCompletion(r.Context(), cycle, club, platform, r.PathValue("id")); err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	rows, err := s.galleryRows(r, gs, cycle, club, platform)
	if err == nil {
		for i := range rows {
			if rows[i].Set.ID == r.PathValue("id") {
				rows[i].Completion = nil
			}
		}
		_ = gs.SaveGallery(r.Context(), cycle, club, platform, rows)
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleGaleriaColecao(w http.ResponseWriter, r *http.Request) {
	gs, ok := s.galleryStore(w)
	if !ok {
		return
	}
	cycle, club, platform := s.galleryContext(r)
	cards, err := gs.ListGalleryCards(r.Context(), cycle, club, platform)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	if cards == nil {
		cards = []galeria.Card{}
	}
	writeJSON(w, struct {
		Value []galeria.Card `json:"value"`
		Count int            `json:"@odata.count"`
	}{cards, len(cards)})
}
func (s *Server) handleGaleriaColecaoUpdate(w http.ResponseWriter, r *http.Request) {
	gs, ok := s.galleryStore(w)
	if !ok {
		return
	}
	var in galeria.CollectionOverride
	decodeErr := json.NewDecoder(r.Body).Decode(&in)
	if decodeErr != nil && r.PathValue("id") == "" {
		http.Error(w, "correção de carta inválida", http.StatusBadRequest)
		return
	}
	if decodeErr != nil || in.CardID == 0 {
		if in.CardID == 0 {
			if n, parseErr := strconv.ParseInt(r.PathValue("id"), 10, 64); parseErr == nil {
				in.CardID = n
			}
		}
	}
	if in.CardID == 0 {
		http.Error(w, "correção de carta inválida", 400)
		return
	}
	cycle, club, platform := s.galleryContext(r)
	if err := gs.SaveGalleryOverride(r.Context(), cycle, club, platform, in); err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	// A correção local deve refletir imediatamente nos avisos, sem esperar a
	// próxima coleta. Recalcular só com dados persistidos mantém esta rota sem
	// acesso à rede.
	if rows, e1 := s.galleryRows(r, gs, cycle, club, platform); e1 == nil {
		if cards, e2 := gs.ListGalleryCards(r.Context(), cycle, club, platform); e2 == nil {
			overrides, _ := gs.ListGalleryOverrides(r.Context(), cycle, club, platform)
			sets := make([]galeria.Set, 0, len(rows))
			completions := map[string]galeria.Completion{}
			for _, row := range rows {
				sets = append(sets, row.Set)
				if row.Completion != nil {
					completions[row.Set.ID] = *row.Completion
				}
			}
			recalculated := galeria.Evaluate(galeria.Input{Sets: sets, Cards: cards, Overrides: overrides, Completions: completions, Now: time.Now()})
			for i := range recalculated {
				if c, ok := completions[recalculated[i].Set.ID]; ok {
					recalculated[i].Completion = &c
				}
			}
			_ = gs.SaveGallery(r.Context(), cycle, club, platform, recalculated)
		}
	}
	writeJSON(w, in)
}

func (s *Server) handleGaleriaColecaoDelete(w http.ResponseWriter, r *http.Request) {
	gs, ok := s.galleryStore(w)
	if !ok {
		return
	}
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || id == 0 {
		http.Error(w, "id de carta inválido", http.StatusBadRequest)
		return
	}
	cycle, club, platform := s.galleryContext(r)
	if err := gs.DeleteGalleryOverride(r.Context(), cycle, club, platform, id); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
