package api

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gscarneiro/eafc-bot/internal/domain"
	"github.com/gscarneiro/eafc-bot/internal/store"
)

// handleClubImport recebe somente dado que o usuario escolheu localmente. O
// contrato nunca aceita cookies, tokens, senha ou sessao; a API continua sem
// capacidade de falar com EA, mesmo quando a UI envia um arquivo.
func (s *Server) handleClubImport(w http.ResponseWriter, r *http.Request) {
	body := json.RawMessage{}
	var club domain.Club
	ct := r.Header.Get("Content-Type")
	if strings.HasPrefix(ct, "text/csv") {
		b, err := io.ReadAll(r.Body)
		if err != nil || len(b) > maxLocalWriteBody {
			http.Error(w, "arquivo CSV invalido", http.StatusBadRequest)
			return
		}
		club, err = decodeClubCSVAPI(b)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
	} else {
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			http.Error(w, "JSON de clube invalido", http.StatusBadRequest)
			return
		}
		if contemSegredoAPI(body) {
			http.Error(w, "arquivo recusado: remova segredo ou sessao", http.StatusBadRequest)
			return
		}
		var envelope struct {
			Club domain.Club `json:"club"`
		}
		if err := json.Unmarshal(body, &envelope); err != nil {
			http.Error(w, "JSON de clube invalido", http.StatusBadRequest)
			return
		}
		club = envelope.Club
		if len(club.Players) == 0 {
			_ = json.Unmarshal(body, &club)
		}
	}
	if err := validarClubAPI(club); err != nil {
		http.Error(w, err.Error(), http.StatusUnprocessableEntity)
		return
	}
	club.Cycle = s.Cycle
	club.Source = "importacao_local"
	club.SyncedAt = time.Now()
	if err := s.Store.SaveClub(r.Context(), club); err != nil {
		http.Error(w, "gravando clube importado: "+err.Error(), http.StatusInternalServerError)
		return
	}
	if err := s.Store.SaveSnapshot(r.Context(), store.Snapshot{GeneratedAt: club.SyncedAt, Cycle: s.Cycle, Club: club, Errors: []string{"importacao local: sem acesso a conta EA"}}); err != nil {
		http.Error(w, "gravando snapshot importado: "+err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, club)
}
func contemSegredoAPI(b []byte) bool {
	var v any
	if json.Unmarshal(b, &v) != nil {
		return false
	}
	var scan func(any) bool
	scan = func(x any) bool {
		switch x := x.(type) {
		case map[string]any:
			for k, v := range x {
				n := strings.ToLower(k)
				if strings.Contains(n, "password") || strings.Contains(n, "cookie") || strings.Contains(n, "token") || strings.Contains(n, "session") || strings.Contains(n, "authorization") {
					return true
				}
				if scan(v) {
					return true
				}
			}
		case []any:
			for _, v := range x {
				if scan(v) {
					return true
				}
			}
		}
		return false
	}
	return scan(v)
}
func validarClubAPI(c domain.Club) error {
	if len(c.Players) == 0 {
		return fmt.Errorf("arquivo sem cartas; o snapshot atual foi preservado")
	}
	for i, p := range c.Players {
		if p.ID <= 0 || strings.TrimSpace(p.Name) == "" || p.Rating <= 0 || p.Position == "" {
			return fmt.Errorf("carta %d incompleta", i+1)
		}
	}
	return nil
}
func decodeClubCSVAPI(b []byte) (domain.Club, error) {
	rows, err := csv.NewReader(strings.NewReader(string(b))).ReadAll()
	if err != nil || len(rows) < 2 {
		return domain.Club{}, fmt.Errorf("CSV sem cartas valido")
	}
	idx := map[string]int{}
	for i, h := range rows[0] {
		idx[strings.ToLower(strings.TrimSpace(h))] = i
	}
	getcol := func(k string) (int, error) {
		i, ok := idx[k]
		if !ok {
			return 0, fmt.Errorf("CSV precisa da coluna %q", k)
		}
		return i, nil
	}
	idc, e := getcol("id")
	if e != nil {
		return domain.Club{}, e
	}
	nc, e := getcol("name")
	if e != nil {
		return domain.Club{}, e
	}
	rc, e := getcol("rating")
	if e != nil {
		return domain.Club{}, e
	}
	pc, e := getcol("position")
	if e != nil {
		return domain.Club{}, e
	}
	out := domain.Club{}
	for line, row := range rows[1:] {
		at := func(i int) string {
			if i >= len(row) {
				return ""
			}
			return strings.TrimSpace(row[i])
		}
		id, er := strconv.ParseInt(at(idc), 10, 64)
		if er != nil {
			return out, fmt.Errorf("linha %d: id invalido", line+2)
		}
		rating, er := strconv.Atoi(at(rc))
		if er != nil {
			return out, fmt.Errorf("linha %d: rating invalido", line+2)
		}
		pos, er := domain.ParsePosition(at(pc))
		if er != nil {
			return out, fmt.Errorf("linha %d: posicao invalida", line+2)
		}
		out.Players = append(out.Players, domain.ClubPlayer{Player: domain.Player{ID: id, Name: at(nc), Rating: rating, Position: pos}})
	}
	return out, nil
}
