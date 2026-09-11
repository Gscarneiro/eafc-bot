package api

import (
	"fmt"
	"time"

	"github.com/gscarneiro/eafc-bot/internal/analyze"
	"github.com/gscarneiro/eafc-bot/internal/chemistry"
	"github.com/gscarneiro/eafc-bot/internal/futgg"
	"github.com/gscarneiro/eafc-bot/internal/store"
)

// avisoSBCExpiringWindow é o horizonte pra um SBC ativo virar aviso — perto
// o bastante pra ser acionável hoje, longe o bastante pra não repetir o que
// a Agenda já lista ("esta_semana").
const avisoSBCExpiringWindow = 24 * time.Hour

// Aviso junta sinais de PROVENIÊNCIA e de decisão pendente que já existem
// espalhados pelo snapshot e pela Agenda — o selo "Hoje · N" do rail conta
// isto. É apresentação, não decisão nova: nada aqui decide comprar, vender
// ou confiar num número; só sinaliza o que internal/analyze ou a coleta já
// registraram, pra não ficar enterrado numa tela que ninguém abriu hoje.
type Aviso struct {
	Kind     string `json:"kind"` // "coleta" | "procedencia" | "quimica" | "conflito" | "sbc"
	Severity string `json:"severity"`
	Headline string `json:"headline"`
	Detail   string `json:"detail,omitempty"`
	Link     string `json:"link,omitempty"`
}

// buildAvisos NÃO toca rede nem recalcula nada — só lê o que já foi
// decidido/observado. agenda vem de buildAgenda para os conflitos baterem
// com os que a tela /agenda mostra.
func buildAvisos(snap store.Snapshot, chem *chemistry.Resultado, agenda analyze.Agenda) []Aviso {
	avisos := make([]Aviso, 0)
	for _, e := range snap.Errors {
		avisos = append(avisos, Aviso{Kind: "coleta", Severity: "erro", Headline: "Falha na coleta de hoje", Detail: e, Link: "/configuracoes"})
	}
	for fonte, obs := range snap.Capabilities {
		if obs.Status != futgg.StatusConfirmado {
			avisos = append(avisos, Aviso{
				Kind: "procedencia", Severity: "alerta",
				Headline: fmt.Sprintf("%s sem confirmação", fonte),
				Detail:   string(obs.Status), Link: "/configuracoes",
			})
		}
	}
	if chem != nil && chem.Verificacao.Status == chemistry.StatusDiverge {
		avisos = append(avisos, Aviso{
			Kind: "quimica", Severity: "alerta",
			Headline: "Modelo de química não confere com o jogo",
			Detail:   chem.Verificacao.Detalhe, Link: "/time",
		})
	}
	seen := map[string]bool{}
	for _, faixa := range [][]analyze.AcaoAgenda{agenda.Agora, agenda.EstaSemana, agenda.Observando} {
		for _, acao := range faixa {
			for _, conflito := range acao.Conflitos {
				key := acao.ID + "\x00" + conflito
				if seen[key] {
					continue
				}
				seen[key] = true
				avisos = append(avisos, Aviso{Kind: "conflito", Severity: "alerta", Headline: acao.Alvo, Detail: conflito, Link: acao.Link})
			}
		}
	}
	now := time.Now()
	for _, sbc := range snap.SBCs {
		if sbc.ExpiresAt.IsZero() || !sbc.ExpiresAt.After(now) || sbc.ExpiresAt.Sub(now) > avisoSBCExpiringWindow {
			continue
		}
		avisos = append(avisos, Aviso{
			Kind: "sbc", Severity: "alerta",
			Headline: sbc.Name + " expira em breve",
			Detail:   "até " + sbc.ExpiresAt.Format("02/01 15:04"), Link: "/hoje",
		})
	}
	return avisos
}
