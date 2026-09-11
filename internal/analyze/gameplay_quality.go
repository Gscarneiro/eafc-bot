package analyze

import (
	"sort"

	"github.com/gscarneiro/eafc-bot/internal/domain"
)

// QualidadeRankingGameplay mede o ranking contra comparações independentes.
// Testes técnicos só mostram que a fórmula roda; este resultado deixa claro
// quando a ordem publicada acompanha, diverge ou ainda não cobre a vivência.
type QualidadeRankingGameplay struct {
	Amostra       string               `json:"amostra"`
	Comparacoes   int                  `json:"comparacoes"`
	Cobertas      int                  `json:"cobertas"`
	Concordancias int                  `json:"concordancias"`
	Divergencias  int                  `json:"divergencias"`
	Empates       int                  `json:"empates"`
	Insuficientes int                  `json:"insuficientes"`
	Concordancia  float64              `json:"concordancia,omitempty"`
	PorFuncao     []QualidadePorFuncao `json:"por_funcao,omitempty"`
}

type QualidadePorFuncao struct {
	Funcao       string  `json:"funcao"`
	Comparacoes  int     `json:"comparacoes"`
	Cobertas     int     `json:"cobertas"`
	Concordancia float64 `json:"concordancia,omitempty"`
}

// AvaliarQualidadeGameplay recebe apenas a ordem prevista para A e B. Isso
// separa a medição do ranking do motor que a produziu e permite comparar um
// perfil candidato, o vigente ou uma fonte externa sem misturar escalas.
// prever devolve -1 para B, 0 para empate e 1 para A; false indica cobertura
// insuficiente para aquela comparação.
func AvaliarQualidadeGameplay(entries []domain.FeedbackGameplay, amostra string, prever func(domain.FeedbackGameplay) (int, bool)) QualidadeRankingGameplay {
	out := QualidadeRankingGameplay{Amostra: amostra}
	type partial struct{ comparacoes, cobertas, concordancias int }
	byRole := map[string]*partial{}
	for _, entry := range entries {
		if entry.Amostra != amostra {
			continue
		}
		out.Comparacoes++
		if entry.Preferencia == "insuficiente" {
			out.Insuficientes++
			continue
		}
		role := entry.Funcao
		if role == "" {
			role = string(entry.Posicao)
		}
		if role == "" {
			role = "sem função"
		}
		stats := byRole[role]
		if stats == nil {
			stats = &partial{}
			byRole[role] = stats
		}
		stats.comparacoes++
		predicted, ok := prever(entry)
		if !ok {
			continue
		}
		out.Cobertas++
		stats.cobertas++
		want := preferenciaComoOrdem(entry.Preferencia)
		if want == 0 {
			out.Empates++
		}
		if predicted == want {
			out.Concordancias++
			stats.concordancias++
		} else {
			out.Divergencias++
		}
	}
	if out.Cobertas > 0 {
		out.Concordancia = float64(out.Concordancias) / float64(out.Cobertas)
	}
	for role, stats := range byRole {
		row := QualidadePorFuncao{Funcao: role, Comparacoes: stats.comparacoes, Cobertas: stats.cobertas}
		if stats.cobertas > 0 {
			row.Concordancia = float64(stats.concordancias) / float64(stats.cobertas)
		}
		out.PorFuncao = append(out.PorFuncao, row)
	}
	sort.Slice(out.PorFuncao, func(i, j int) bool { return out.PorFuncao[i].Funcao < out.PorFuncao[j].Funcao })
	return out
}

func preferenciaComoOrdem(preferencia string) int {
	switch preferencia {
	case "a":
		return 1
	case "b":
		return -1
	default:
		return 0
	}
}
