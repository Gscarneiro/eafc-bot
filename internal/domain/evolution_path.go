package domain

// EvolutionPath é uma sequência completa de evolução: a carta em cada passo,
// do estado atual ao final — inclusive o GG Rating final, que não existe em
// nenhum outro lugar da API do fut.gg. Uma Evolution isolada (evolution.go)
// descreve UM upgrade; um EvolutionPath é a cadeia inteira já resolvida pelo
// fut.gg contra uma carta específica seguindo uma sequência de Evolutions.
type EvolutionPath struct {
	Steps        []Player `json:"steps"` // Steps[0] = carta hoje, Steps[len-1] = carta final
	Chain        []string `json:"chain"` // nomes das evoluções encadeadas, em ordem
	CoinsCost    int      `json:"coin_cost"`
	PointsCost   int      `json:"point_cost"`
	IsExpired    bool     `json:"is_expired"`
	TrainingTime string   `json:"training_time"` // já formatado pelo fut.gg ("3 days")
}

// Initial é a carta como está hoje — MAS SEM GGRating: confirmado ao vivo
// contra a API que path[0].ggRating vem sempre nulo, em todo caminho de
// todo jogador testado. O GG Rating "atual" de verdade vem do elenco (a
// mesma carta que já está em ClubPlayer.Player), não daqui. Os outros
// campos (overall, PlayStyles, atributos) vêm corretos e batem com o elenco.
func (p EvolutionPath) Initial() Player {
	if len(p.Steps) == 0 {
		return Player{}
	}
	return p.Steps[0]
}

// Final é a carta depois de completar a cadeia inteira.
func (p EvolutionPath) Final() Player {
	if len(p.Steps) == 0 {
		return Player{}
	}
	return p.Steps[len(p.Steps)-1]
}

// GainedPlayStyles são os PlayStyles (normais e +) que a carta NÃO tem hoje
// e passaria a ter no final — "sem os PlayStyles preenchidos" contra "o
// potencial dela", ambos lidos direto do fut.gg.
func (p EvolutionPath) GainedPlayStyles() []PlayStyle {
	have := map[string]bool{}
	for _, ps := range p.Initial().PlayStyles {
		have[ps.Name] = true
	}
	var out []PlayStyle
	for _, ps := range p.Final().PlayStyles {
		// PlayStyle normal virando + no mesmo nome também conta como ganho:
		// "Rapid" -> "Rapid+" é uma evolução real, não uma repetição.
		if !have[ps.Name] || (ps.Plus && !hasPlus(p.Initial().PlayStyles, ps.Name)) {
			out = append(out, ps)
		}
	}
	return out
}

func hasPlus(list []PlayStyle, name string) bool {
	for _, ps := range list {
		if ps.Name == name && ps.Plus {
			return true
		}
	}
	return false
}
