package api

import (
	"sort"

	"github.com/gscarneiro/eafc-bot/internal/domain"
	"github.com/gscarneiro/eafc-bot/internal/store"
)

// FaixaOVR é uma barra do histograma "Distribuição do clube" da tela Meu
// time: quantas cópias físicas caem na faixa de overall e quantas delas
// estão no XI. Mora no servidor porque o banco que a tela recebe é
// paginado — somar só a página desenharia um clube que não existe.
type FaixaOVR struct {
	Faixa string `json:"faixa"`
	Min   int    `json:"min"`
	Max   int    `json:"max,omitempty"` // 0 = sem teto
	Total int    `json:"total"`
	NoXI  int    `json:"no_xi"`
}

var faixasOVR = []FaixaOVR{
	{Faixa: "≤77", Min: 0, Max: 77},
	{Faixa: "78-79", Min: 78, Max: 79},
	{Faixa: "80-81", Min: 80, Max: 81},
	{Faixa: "82-83", Min: 82, Max: 83},
	{Faixa: "84-85", Min: 84, Max: 85},
	{Faixa: "86-87", Min: 86, Max: 87},
	{Faixa: "88-89", Min: 88, Max: 89},
	{Faixa: "90+", Min: 90},
}

// distribuicaoOVR conta cada cópia física (duas cópias da mesma carta são
// duas cartas no clube). noXI usa domain.ClubPlayer.IdentityKey, a mesma
// chave com que handleTime separa titular de reserva.
func distribuicaoOVR(players []domain.ClubPlayer, noXI map[string]bool) []FaixaOVR {
	faixas := make([]FaixaOVR, len(faixasOVR))
	copy(faixas, faixasOVR)
	for _, p := range players {
		for i := range faixas {
			if p.Rating < faixas[i].Min || (faixas[i].Max > 0 && p.Rating > faixas[i].Max) {
				continue
			}
			faixas[i].Total++
			if noXI[p.IdentityKey()] {
				faixas[i].NoXI++
			}
			break
		}
	}
	return faixas
}

// ValorXI é a série "Valor do XI / 30 dias": a soma, dia a dia, do preço
// dos titulares negociáveis com cotação. Cotadas diz quantos titulares
// entram na soma; SemCotacao, quantos ficam fora (untradeable, alvo ou
// sem histórico nenhum) — a tela diz isso em vez de fingir que o XI
// inteiro foi somado.
type ValorXI struct {
	Pontos     []PontoValorXI `json:"pontos"`
	Cotadas    int            `json:"cotadas"`
	SemCotacao int            `json:"sem_cotacao"`
}

type PontoValorXI struct {
	Dia    string `json:"dia"` // "2006-01-02"
	Moedas int    `json:"moedas"`
}

// valorDoXI só publica um dia quando TODAS as cartas cotadas têm preço
// nele. Somar um dia em que a coleta de uma carta falhou faria o valor do
// XI "cair" sem ninguém ter vendido nada — na dúvida, o dia fica de fora.
func valorDoXI(starters []StarterCard, series map[int64][]store.PricePoint) *ValorXI {
	valor := &ValorXI{Pontos: []PontoValorXI{}}
	porCarta := make([]map[string]int, 0, len(starters))
	for _, s := range starters {
		if s.Player.Untradeable || s.Player.AlvoPlano {
			valor.SemCotacao++
			continue
		}
		dias := make(map[string]int)
		pts := append([]store.PricePoint(nil), series[s.Player.ID]...)
		sort.Slice(pts, func(i, j int) bool { return pts[i].ObservedAt.Before(pts[j].ObservedAt) })
		for _, pt := range pts {
			if pt.Extinct || pt.Coins <= 0 {
				continue
			}
			dias[pt.ObservedAt.UTC().Format("2006-01-02")] = pt.Coins
		}
		if len(dias) == 0 {
			valor.SemCotacao++
			continue
		}
		porCarta = append(porCarta, dias)
	}
	valor.Cotadas = len(porCarta)
	if len(porCarta) == 0 {
		return valor
	}
	var dias []string
	for dia := range porCarta[0] {
		dias = append(dias, dia)
	}
	sort.Strings(dias)
	for _, dia := range dias {
		total, completo := 0, true
		for _, carta := range porCarta {
			coins, ok := carta[dia]
			if !ok {
				completo = false
				break
			}
			total += coins
		}
		if completo {
			valor.Pontos = append(valor.Pontos, PontoValorXI{Dia: dia, Moedas: total})
		}
	}
	return valor
}
