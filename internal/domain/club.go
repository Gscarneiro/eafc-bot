package domain

import "time"

// ClubPlayer é uma carta que já está no seu clube. Carrega o estado que só
// existe para você: se dá para vender, se está escalado, e o que já foi
// gasto de evolução nela.
type ClubPlayer struct {
	Player
	// ClubItemID é o identificador do registro físico no clube, quando a
	// fonte o fornece. Ele é diferente de Player.ID: duas cópias da mesma
	// carta compartilham o segundo, mas não deveriam ser colapsadas no diff
	// do elenco. Vazio significa que a fonte ainda não provou essa identidade.
	ClubItemID   string     `json:"club_item_id,omitempty"`
	Untradeable  bool       `json:"untradeable"`
	InSquad      bool       `json:"in_squad"`
	SquadSlot    Position   `json:"squad_slot"` // onde ele joga HOJE no seu time
	OutOfPos     bool       `json:"out_of_pos"` // escalado fora da posição natural
	Chemistry    int        `json:"chemistry"`  // pontos de química da carta (0-3)
	EvosApplied  []string   `json:"evos_applied"`
	EvoExhausted bool       `json:"evo_exhausted"` // já usou evolução, não aceita outra
	Contracts    int        `json:"contracts"`
	AcquiredAt   time.Time  `json:"acquired_at"`
	Stats        *ClubStats `json:"club_stats,omitempty"`
}

type ClubStats struct {
	Games        *int `json:"games,omitempty"`
	Goals        *int `json:"goals,omitempty"`
	Assists      *int `json:"assists,omitempty"`
	YellowCards  *int `json:"yellow_cards,omitempty"`
	RedCards     *int `json:"red_cards,omitempty"`
	PurchasedFor *int `json:"purchased_for,omitempty"`
	KitNumber    *int `json:"kit_number,omitempty"`
}

// Evolvable diz se a carta ainda pode entrar numa evolução.
// Cartas já evoluídas não podem ser evoluídas de novo no mesmo slot.
func (c ClubPlayer) Evolvable() bool { return !c.EvoExhausted }

// SellValue é o valor bruto da carta no mercado. Untradeable não vira coin.
func (c ClubPlayer) SellValue() int {
	if c.Untradeable {
		return 0
	}
	return c.Price.Coins
}

// NetSellValue é o que realmente entra no saldo depois da taxa de 5% da EA.
// A conta inteira evita arredondar com float e mantém mercado, upgrades e
// orçamento usando exatamente o mesmo valor líquido.
func (c ClubPlayer) NetSellValue() int {
	return c.SellValue() * 95 / 100
}

// SquadSlot é UM lugar físico na escalação titular — não uma posição
// lógica. A distinção importa porque uma formação real repete posição: um
// 4-4-1-1 tem dois CB e dois CM, e um mapa "Position -> jogador" só teria
// espaço para um de cada, descartando o resto em silêncio.
type SquadSlot struct {
	Index    int      `json:"index"`    // 0..10, a ordem "FIELD" que o fut.gg usa
	Position Position `json:"position"` // carta de posição quando há, senão a natural
	PlayerID int64    `json:"player_id"`
}

// Squad é a escalação atual: formação + quem joga onde.
type Squad struct {
	Name      string      `json:"name"`
	Formation string      `json:"formation"` // "4-2-3-1", "4-3-3(4)"...
	Starters  []SquadSlot `json:"starters"`
	Chemistry int         `json:"chemistry"`

	// ChemistrySynced diz se Chemistry de fato veio do jogo nesta coleta.
	// Existe porque zero é ambíguo: pode ser um XI sem vínculo nenhum, ou a
	// rota active-squad tendo falhado (nesse caso o Squad inteiro é
	// substituído e Chemistry vira 0 sem ninguém perceber). Química só serve
	// de oráculo quando isto é verdadeiro — "na dúvida, não afirma" aplicado
	// a um número que parece exato. Falso é o valor de snapshot antigo, e é
	// o padrão conservador certo.
	//
	// É bandeira de PROVENIÊNCIA do dado, não regra de jogo: virar o ciclo
	// não mexe aqui.
	ChemistrySynced bool `json:"chemistry_synced"`

	SyncedAt time.Time `json:"synced_at"`
}

// Club é o retrato do seu Ultimate Team num instante.
type Club struct {
	GamerTag string       `json:"gamer_tag"`
	Platform string       `json:"platform"`
	Coins    int          `json:"coins"`
	Players  []ClubPlayer `json:"players"`
	Squad    Squad        `json:"squad"`
	Cycle    string       `json:"cycle"`
	SyncedAt time.Time    `json:"synced_at"`
	Source   string       `json:"source"` // "futgg", "csv", "chrome"
}

// Starter devolve quem está escalado numa posição. Existe para responder
// "a evolução supera o titular atual?" (ver analyze.FindEvolutions) — para
// varrer o XI titular completo, incluindo as posições duplicadas de uma
// formação como 4-4-1-1, use Squad.Starters diretamente.
//
// Quando a posição se repete (dois CB, dois CM), devolve o de MENOR overall.
// É a escolha certa para essa pergunta: uma evolução que supera o pior dos
// dois já vira titular de verdade, substituindo-o — exigir que supere o
// melhor sub-relataria os casos em que a evolução realmente compensa.
func (c Club) Starter(pos Position) (ClubPlayer, bool) {
	var best ClubPlayer
	found := false
	for _, slot := range c.Squad.Starters {
		if slot.Position != pos {
			continue
		}
		p, ok := c.PlayerByID(slot.PlayerID)
		if !ok {
			continue
		}
		if !found || p.Rating < best.Rating {
			best, found = p, true
		}
	}
	return best, found
}

// PlayerByID acha uma carta do clube pelo id do recurso.
func (c Club) PlayerByID(id int64) (ClubPlayer, bool) {
	for _, p := range c.Players {
		if p.ID == id {
			return p, true
		}
	}
	return ClubPlayer{}, false
}

// Budget é quanto você pode gastar: coins em caixa mais o que dá para
// levantar vendendo cartas que não são titulares.
func (c Club) Budget() (cash int, raisable int) {
	capital := c.Capital(0, 0, 0)
	return capital.Cash, capital.NetRaisable
}
