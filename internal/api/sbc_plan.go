package api

import (
	"encoding/json"
	"github.com/gscarneiro/eafc-bot/internal/analyze"
	"net/http"
	"time"
)

type planoSBCRequest struct {
	SBCID      string   `json:"sbc_id"`
	Challenge  int      `json:"challenge"`
	Bloqueados []string `json:"bloqueados"`
	MaxNos     int      `json:"max_nos,omitempty"`
	TimeoutMS  int      `json:"timeout_ms,omitempty"`
}

// handlePlanoSBC apenas calcula checklist local. Mesmo o POST nao persiste nem
// executa qualquer acao no jogo, e passa pelo guardLocalWrite comum.
func (s *Server) handlePlanoSBC(w http.ResponseWriter, r *http.Request) {
	var in planoSBCRequest
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		http.Error(w, "corpo de plano SBC invalido: "+err.Error(), http.StatusBadRequest)
		return
	}
	snap, ok := s.load(w, r)
	if !ok {
		return
	}
	var found bool
	for _, sbc := range snap.SBCs {
		if sbc.ID == in.SBCID && in.Challenge >= 0 && in.Challenge < len(sbc.Challenges) {
			found = true
			opt := analyze.DefaultPlanoSBCOpcoes()
			opt.MaxNos = in.MaxNos
			if in.TimeoutMS > 0 {
				opt.Timeout = time.Duration(in.TimeoutMS) * time.Millisecond
			}
			opt.Bloqueados = map[string]bool{}
			for _, id := range in.Bloqueados {
				opt.Bloqueados[id] = true
			}
			// O snapshot atual nao prove o estado de emprestimo por copia. Sem essa
			// prova o solver retorna dados_indisponiveis em vez de sugerir descarte.
			opt.DadosDeEmprestimoConfirmados = false
			writeJSON(w, analyze.MontarPlanoSBC(sbc.Challenges[in.Challenge], snap.Club, snap.Market, opt))
			return
		}
	}
	if !found {
		http.Error(w, "SBC ou desafio nao encontrado no snapshot", http.StatusNotFound)
	}
}
