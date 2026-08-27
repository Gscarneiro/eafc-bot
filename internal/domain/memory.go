package domain

import "time"

// ClubRollup é uma fotografia compacta da coleção. Ele guarda somente
// identidade e multiplicidade necessárias para memória; o snapshot completo
// continua sendo a fonte de atributos e relatórios detalhados.
type ClubRollup struct {
	Cycle      string            `json:"cycle"`
	ObservedAt time.Time         `json:"observed_at"`
	Source     string            `json:"source"`
	Entries    []ClubRollupEntry `json:"entries"`
}

// ClubRollupEntry preserva cópias físicas quando ClubItemID existe. Sem ele,
// Count é apenas um multiconjunto de versões de carta: nunca inventa qual
// cópia entrou ou saiu.
type ClubRollupEntry struct {
	EAID       int64  `json:"ea_id"`
	ClubItemID string `json:"club_item_id,omitempty"`
	Count      int    `json:"count"`
}

// RollupFromClub cria a memória compacta de uma coleta. A ordem de jogadores
// nunca é usada como identidade física; só agrega por EAID quando a fonte não
// informou ClubItemID.
func RollupFromClub(club Club, observedAt time.Time) ClubRollup {
	if observedAt.IsZero() {
		observedAt = club.SyncedAt
	}
	out := ClubRollup{Cycle: club.Cycle, ObservedAt: observedAt, Source: club.Source}
	unknown := make(map[int64]int)
	for _, player := range club.Players {
		if player.ClubItemID != "" {
			out.Entries = append(out.Entries, ClubRollupEntry{EAID: player.ID, ClubItemID: player.ClubItemID, Count: 1})
			continue
		}
		unknown[player.ID]++
	}
	for id, count := range unknown {
		out.Entries = append(out.Entries, ClubRollupEntry{EAID: id, Count: count})
	}
	return out
}
