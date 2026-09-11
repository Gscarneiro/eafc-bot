package domain

import "strings"

// NotaExterna guarda uma escala publicada por um fornecedor diferente do
// FUT.GG. Ela conserva métrica, escala e contexto porque "99" só pode ser
// comparado com outro 99 da mesma régua; nunca há média entre fontes.
type NotaExterna struct {
	Fonte        FonteAvaliacao       `json:"fonte"`
	Metrica      string               `json:"metrica"`
	EscalaMinima float64              `json:"escala_minima"`
	EscalaMaxima float64              `json:"escala_maxima"`
	Ciclo        string               `json:"ciclo"`
	Patch        string               `json:"patch,omitempty"`
	Plataforma   string               `json:"plataforma,omitempty"`
	Evidencia    string               `json:"evidencia"`
	CapturadaEm  string               `json:"capturada_em"`
	PorPosicao   map[Position]float64 `json:"por_posicao"`
}

// RatingAt só aceita uma nota posicional dentro do contexto que a fonte
// declarou. Uma importação de outro ciclo, patch ou plataforma fica
// indisponível em vez de parecer uma nota atual.
func (n NotaExterna) RatingAt(pos Position, ctx ContextoAvaliacao) (float64, bool) {
	if n.EscalaMaxima <= n.EscalaMinima || n.Ciclo == "" || n.Evidencia == "" || n.Metrica == "" {
		return 0, false
	}
	if ctx.Ciclo != "" && n.Ciclo != ctx.Ciclo {
		return 0, false
	}
	if ctx.Patch != "" && n.Patch != "" && n.Patch != ctx.Patch {
		return 0, false
	}
	if ctx.Plataforma != "" && n.Plataforma != "" && !strings.EqualFold(n.Plataforma, ctx.Plataforma) {
		return 0, false
	}
	v, ok := n.PorPosicao[pos]
	if !ok || v < n.EscalaMinima || v > n.EscalaMaxima {
		return 0, false
	}
	return v, true
}
