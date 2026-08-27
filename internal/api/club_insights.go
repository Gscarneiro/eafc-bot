package api

import (
	"net/http"

	"github.com/gscarneiro/eafc-bot/internal/analyze"
)

// clubMemory monta a mesma base para as duas coleções da fase 05. Falha na
// memória persistida não vira coleção vazia: o snapshot atual continua sendo
// uma aproximação útil, só sem tempo de permanência confirmado.
func (s *Server) clubMemory(w http.ResponseWriter, r *http.Request) ([]analyze.CollectionCard, bool) {
	snap, ok := s.load(w, r)
	if !ok {
		return nil, false
	}
	rollups, err := s.Store.ClubRollups(r.Context(), s.Cycle, 0)
	if err != nil {
		rollups = nil
	}
	watchlist, err := s.Store.ListWatchlist(r.Context(), s.Cycle)
	if err != nil {
		watchlist = nil
	}
	protected := make(map[int64]bool, len(watchlist))
	for _, item := range watchlist {
		if item.Protected {
			protected[item.EAID] = true
		}
	}
	return analyze.BuildCollectionMemory(snap.Club, rollups, protected), true
}

func (s *Server) handleClubCollection(w http.ResponseWriter, r *http.Request) {
	collection, ok := s.clubMemory(w, r)
	if !ok {
		return
	}
	page, ok := serveList(w, r, clubCollectionSchema(), collection)
	if ok {
		writeJSON(w, collectionResponse[analyze.CollectionCard]{Page: page})
	}
}

func (s *Server) handleClubInsights(w http.ResponseWriter, r *http.Request) {
	snap, ok := s.load(w, r)
	if !ok {
		return
	}
	collection, ok := s.clubMemory(w, r)
	if !ok {
		return
	}
	page, ok := serveList(w, r, clubInsightsSchema(), analyze.BuildClubInsights(snap.Club, collection))
	if ok {
		writeJSON(w, collectionResponse[analyze.ClubInsight]{Page: page})
	}
}
