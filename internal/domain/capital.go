package domain

// Capital é a visão auditável do dinheiro disponível para decisões. Os
// valores de venda são separados em bruto e líquido porque a taxa de 5% da
// EA não é uma opinião de estratégia: é uma transformação de domínio.
// Reserve e Committed ficam explícitos para o ledger futuro, em vez de
// esconder moedas já prometidas dentro de um número de orçamento.
type Capital struct {
	Cash          int `json:"cash"`
	ExtraBudget   int `json:"extra_budget"`
	Reserve       int `json:"reserve"`
	GrossRaisable int `json:"gross_raisable"`
	NetRaisable   int `json:"net_raisable"`
	Committed     int `json:"committed"`
	Available     int `json:"available"`
}

// Capital calcula a posição de caixa a partir do retrato do clube. Reserve e
// committed nunca podem virar negativos por configuração acidental; Available
// pode ser negativo para que uma tela de risco mostre um déficit real em vez
// de mascará-lo como zero.
func (c Club) Capital(extraBudget, reserve, committed int) Capital {
	if extraBudget < 0 {
		extraBudget = 0
	}
	if reserve < 0 {
		reserve = 0
	}
	if committed < 0 {
		committed = 0
	}

	capital := Capital{
		Cash:        c.Coins,
		ExtraBudget: extraBudget,
		Reserve:     reserve,
		Committed:   committed,
	}
	for _, p := range c.Players {
		if p.InSquad || p.Untradeable {
			continue
		}
		capital.GrossRaisable += p.SellValue()
		capital.NetRaisable += p.NetSellValue()
	}
	capital.Available = capital.Cash + capital.ExtraBudget + capital.NetRaisable - capital.Reserve - capital.Committed
	return capital
}
