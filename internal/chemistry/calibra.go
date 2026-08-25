package chemistry

import (
	"sort"

	"github.com/gscarneiro/eafc-bot/internal/domain"
)

// Divergencia é um dia em que o modelo não reproduziu o jogo.
type Divergencia struct {
	GamerTag  string `json:"gamer_tag"`
	Calculado int    `json:"calculado"`
	Observado int    `json:"observado"`
	Conferem  int    `json:"jogadores_conferem"`
	Total     int    `json:"jogadores_total"`
}

// Relatorio é o placar da calibração: o modelo replayado contra todos os
// retratos de clube guardados.
type Relatorio struct {
	Modelo     string `json:"modelo"`
	Dias       int    `json:"dias"`
	Conferem   int    `json:"conferem"`
	Divergem   int    `json:"divergem"`
	SemOraculo int    `json:"sem_oraculo"`

	// ValoresDistintos é quantos valores DIFERENTES o jogo reportou na
	// amostra. Um só valor não prova modelo nenhum: um modelo que devolvesse
	// 33 fixo passaria igual. É o número que impede este relatório de virar
	// autoengano — Confirma() exige pelo menos dois.
	ValoresDistintos int `json:"valores_distintos"`

	Piores []Divergencia `json:"piores,omitempty"`
}

// Confirma diz se a amostra de fato CONFIRMA o modelo, e não apenas deixa de
// contrariá-lo. Exige que nenhum dia divirja, que haja dia com oráculo, e que
// o jogo tenha reportado ao menos dois valores diferentes — sem variação, o
// teste não tem poder discriminante nenhum.
func (r Relatorio) Confirma() bool {
	return r.Divergem == 0 && r.Conferem > 0 && r.ValoresDistintos >= 2
}

// Calibrar replaya o modelo contra um histórico de retratos do clube.
//
// NÃO lê disco nem rede: quem chama decide de onde vêm os retratos (o comando
// `eafcbot quimica -calibrar` lê os snapshots; o teste lê testdata). Isso
// mantém o pacote testável sem fixture de 30 MB.
func Calibrar(m Modelo, historico []domain.Club) Relatorio {
	r := Relatorio{Modelo: m.Nome, Dias: len(historico)}
	valores := map[int]bool{}

	for _, club := range historico {
		v := Verificar(m, club)
		switch v.Status {
		case StatusSemOraculo:
			r.SemOraculo++
			continue
		case StatusConfere:
			r.Conferem++
		default:
			r.Divergem++
			r.Piores = append(r.Piores, Divergencia{
				GamerTag: club.GamerTag, Calculado: v.Calculado,
				Observado: v.Observado, Conferem: v.Conferem, Total: v.Total,
			})
		}
		valores[v.Observado] = true
	}
	r.ValoresDistintos = len(valores)

	sort.Slice(r.Piores, func(i, j int) bool {
		di := r.Piores[i].Calculado - r.Piores[i].Observado
		dj := r.Piores[j].Calculado - r.Piores[j].Observado
		if di < 0 {
			di = -di
		}
		if dj < 0 {
			dj = -dj
		}
		return di > dj
	})
	if len(r.Piores) > 5 {
		r.Piores = r.Piores[:5]
	}
	return r
}
