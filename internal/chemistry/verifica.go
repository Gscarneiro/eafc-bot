package chemistry

import (
	"fmt"

	"github.com/gscarneiro/eafc-bot/internal/domain"
)

// Status possíveis de Verificacao.
const (
	StatusConfere    = "confere"
	StatusDiverge    = "diverge"
	StatusSemOraculo = "sem_oraculo"
)

// Verificacao é o confronto entre o modelo e o número que o próprio jogo
// reportou. É a aplicação literal de "na dúvida, não afirma": enquanto isto
// não confere, o peso da química no otimizador é ZERO — trocar GG Rating real
// por entrosamento de mentira piora o time de verdade.
//
// A comparação é POR JOGADOR, não só pelo total: o total é uma soma NOSSA dos
// 11 chemistryPoints (ver futgg.ActiveSquad), então um modelo errado pode
// acertar o total por cancelamento de erros e passar despercebido.
type Verificacao struct {
	Status    string `json:"status"`
	Modelo    string `json:"modelo"`
	Calculado int    `json:"calculado"`
	Observado int    `json:"observado"`
	Conferem  int    `json:"jogadores_conferem"`
	Total     int    `json:"jogadores_total"`
	PiorErro  int    `json:"pior_erro"`
	Detalhe   string `json:"detalhe,omitempty"`
}

// Confiavel diz se o entrosamento pode influenciar decisão. Status vazio
// (snapshot gravado antes deste campo existir) é falso — o padrão
// conservador certo.
func (v Verificacao) Confiavel() bool { return v.Status == StatusConfere }

// Verificar compara o modelo contra o que o fut.gg trouxe do jogo para o XI
// ativo. Roda no job diário: é uma passada sobre 11 cartas que já estão em
// memória, custo zero.
//
// sem_oraculo NUNCA é deduzido de "química observada == 0": esse zero pode
// ser a rota active-squad tendo falhado (ver domain.Squad.ChemistrySynced).
// Confundir os dois faria um modelo errado passar por "confere" justamente
// quando não há com o que comparar.
func Verificar(m Modelo, club domain.Club) Verificacao {
	v := Verificacao{Modelo: m.Nome}

	xi, ok := DoClube(club)
	if !ok {
		v.Status = StatusSemOraculo
		v.Detalhe = "escalação titular não sincronizada — sem XI não há o que comparar"
		return v
	}
	if !club.Squad.ChemistrySynced {
		v.Status = StatusSemOraculo
		v.Detalhe = "o fut.gg não trouxe o entrosamento de todos os titulares nesta coleta"
		return v
	}

	res := Calcular(m, xi)
	v.Calculado, v.Observado = res.Total, club.Squad.Chemistry
	v.Total = len(res.Jogadores)

	observadoPorID := make(map[int64]int, len(club.Squad.Starters))
	for _, s := range club.Squad.Starters {
		if p, ok := club.PlayerByID(s.PlayerID); ok {
			observadoPorID[p.ID] = p.Chemistry
		}
	}
	for _, j := range res.Jogadores {
		obs, temObs := observadoPorID[j.PlayerID]
		if !temObs {
			continue
		}
		erro := j.Pontos - obs
		if erro < 0 {
			erro = -erro
		}
		if erro == 0 {
			v.Conferem++
		} else if erro > v.PiorErro {
			v.PiorErro = erro
		}
	}

	if v.Conferem == v.Total && v.Calculado == v.Observado {
		v.Status = StatusConfere
		v.Detalhe = fmt.Sprintf("modelo %s reproduz o jogo (%d/%d, %d de %d jogadores)",
			m.Nome, v.Calculado, res.Maximo, v.Conferem, v.Total)
		return v
	}

	v.Status = StatusDiverge
	v.Detalhe = fmt.Sprintf(
		"modelo %s calcula %d, o jogo reporta %d (%d de %d jogadores conferem, pior erro %d) "+
			"— rode `eafcbot quimica -calibrar` e confira se a regra do jogo mudou",
		m.Nome, v.Calculado, v.Observado, v.Conferem, v.Total, v.PiorErro)
	return v
}
