package domain

import (
	"fmt"
	"sort"
	"time"
)

// WatchlistEntry é uma intenção local. Não representa uma ordem no mercado e
// nunca é enviada à EA; só preserva a decisão manual entre coletas.
type WatchlistEntry struct {
	ID          string    `json:"id"`
	EAID        int64     `json:"ea_id"`
	Name        string    `json:"name"`
	TargetCoins int       `json:"target_coins,omitempty"`
	Note        string    `json:"note,omitempty"`
	Protected   bool      `json:"protected,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

func (w WatchlistEntry) Validate() error {
	if w.ID == "" {
		return fmt.Errorf("watchlist: id é obrigatório")
	}
	if w.EAID <= 0 {
		return fmt.Errorf("watchlist: ea_id deve ser positivo")
	}
	if w.TargetCoins < 0 {
		return fmt.Errorf("watchlist: preço-alvo não pode ser negativo")
	}
	return nil
}

type LedgerKind string

const (
	LedgerCompra   LedgerKind = "compra"
	LedgerVenda    LedgerKind = "venda"
	LedgerSBC      LedgerKind = "sbc"
	LedgerEvolucao LedgerKind = "evolucao"
	LedgerAjuste   LedgerKind = "ajuste"
	LedgerReversao LedgerKind = "reversao"
)

type LedgerStatus string

const (
	LedgerPlanejado  LedgerStatus = "planejado"
	LedgerConfirmado LedgerStatus = "confirmado"
)

// LedgerEntry é append-only: uma correção entra como reversão referenciando
// o lançamento anterior, em vez de apagar a trilha de auditoria.
type LedgerEntry struct {
	ID         string       `json:"id"`
	Kind       LedgerKind   `json:"kind"`
	Status     LedgerStatus `json:"status"`
	EAID       int64        `json:"ea_id,omitempty"`
	GrossCoins int          `json:"gross_coins"`
	Note       string       `json:"note,omitempty"`
	ReversesID string       `json:"reverses_id,omitempty"`
	OccurredAt time.Time    `json:"occurred_at"`
	RecordedAt time.Time    `json:"recorded_at"`
}

func (e LedgerEntry) Validate() error {
	if e.ID == "" {
		return fmt.Errorf("ledger: id é obrigatório")
	}
	switch e.Kind {
	case LedgerCompra, LedgerVenda, LedgerSBC, LedgerEvolucao, LedgerAjuste, LedgerReversao:
	default:
		return fmt.Errorf("ledger: tipo %q não é suportado", e.Kind)
	}
	if e.Status != LedgerPlanejado && e.Status != LedgerConfirmado {
		return fmt.Errorf("ledger: status %q não é suportado", e.Status)
	}
	if e.Kind == LedgerReversao {
		if e.ReversesID == "" {
			return fmt.Errorf("ledger: reversão precisa informar reverses_id")
		}
		return nil
	}
	if e.GrossCoins < 0 && e.Kind != LedgerAjuste {
		return fmt.Errorf("ledger: valor só pode ser negativo em ajuste")
	}
	return nil
}

type LedgerSummary struct {
	Spent       int `json:"spent"`
	RaisedGross int `json:"raised_gross"`
	RaisedNet   int `json:"raised_net"`
	NetCash     int `json:"net_cash"`
	PnL         int `json:"pnl"`
	Committed   int `json:"committed"`
}

// BreakEvenGross é o menor preço de venda que recupera `cost` depois da taxa
// fixa de 5%. Não é previsão de liquidez: sem spread/volume observado, o bot
// só pode explicar a conta, não garantir que a carta venderá nesse valor.
func BreakEvenGross(cost int) int {
	if cost <= 0 {
		return 0
	}
	return (cost*100 + 94) / 95
}

// SummarizeLedger calcula os efeitos dos lançamentos ainda ativos. A taxa de
// 5% é aplicada uma única vez aqui nas vendas, para P&L, capital e break-even
// não divergirem por arredondamentos diferentes.
func SummarizeLedger(entries []LedgerEntry) LedgerSummary {
	reversed := make(map[string]bool, len(entries))
	for _, e := range entries {
		if e.Kind == LedgerReversao && e.ReversesID != "" {
			reversed[e.ReversesID] = true
		}
	}
	var out LedgerSummary
	for _, e := range entries {
		if reversed[e.ID] || e.Kind == LedgerReversao {
			continue
		}
		if e.Status == LedgerPlanejado {
			switch e.Kind {
			case LedgerCompra, LedgerSBC, LedgerEvolucao:
				out.Committed += e.GrossCoins
			}
			continue
		}
		switch e.Kind {
		case LedgerCompra, LedgerSBC, LedgerEvolucao:
			out.Spent += e.GrossCoins
			out.NetCash -= e.GrossCoins
			out.PnL -= e.GrossCoins
		case LedgerVenda:
			net := e.GrossCoins * 95 / 100
			out.RaisedGross += e.GrossCoins
			out.RaisedNet += net
			out.NetCash += net
			out.PnL += net
		case LedgerAjuste:
			out.NetCash += e.GrossCoins
			out.PnL += e.GrossCoins
		}
	}
	return out
}

func SortLedgerNewestFirst(entries []LedgerEntry) {
	sort.SliceStable(entries, func(i, j int) bool { return entries[i].RecordedAt.After(entries[j].RecordedAt) })
}
