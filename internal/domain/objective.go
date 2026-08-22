package domain

import "time"

// Reward é o que um objetivo ou SBC entrega ao ser concluído.
type Reward struct {
	Kind        string `json:"kind"` // "player", "pack", "coins", "evo_token"
	Description string `json:"description"`
	PlayerID    int64  `json:"player_id"`  // quando a recompensa é uma carta específica
	PackValue   int    `json:"pack_value"` // valor estimado do pacote em coins
	Coins       int    `json:"coins"`
	Untradeable bool   `json:"untradeable"`
}

// Objective é um objetivo do jogo (Rivals, Live, Sazonal).
type Objective struct {
	ID         string    `json:"id"`
	Name       string    `json:"name"`
	Group      string    `json:"group"` // "Objetivos Semanais", "Live", "Temporada"
	Tasks      []string  `json:"tasks"`
	Rewards    []Reward  `json:"rewards"`
	ExpiresAt  time.Time `json:"expires_at"`
	EstMinutes int       `json:"est_minutes"` // esforço estimado para concluir
	Cycle      string    `json:"cycle"`
	URL        string    `json:"url"`
}

// SBC é um Squad Building Challenge.
type SBC struct {
	ID           string    `json:"id"`
	Name         string    `json:"name"`
	Group        string    `json:"group"`
	Repeatable   bool      `json:"repeatable"`
	SolutionCost int       `json:"solution_cost"` // custo da solução mais barata, em coins
	Rewards      []Reward  `json:"rewards"`
	ExpiresAt    time.Time `json:"expires_at"`
	Cycle        string    `json:"cycle"`
	URL          string    `json:"url"`
}

// NetValue é o saldo do SBC: o que você recebe menos o que gasta.
// Positivo significa que o SBC paga mais do que custa.
func (s SBC) NetValue() int {
	var reward int
	for _, r := range s.Rewards {
		reward += r.Coins + r.PackValue
	}
	return reward - s.SolutionCost
}

// Expiring diz se acaba dentro da janela informada — o que decide se entra
// na seção "urgente" do relatório.
func (s SBC) Expiring(within time.Duration) bool {
	if s.ExpiresAt.IsZero() {
		return false
	}
	return time.Until(s.ExpiresAt) <= within
}

// Expiring é o equivalente para objetivos.
func (o Objective) Expiring(within time.Duration) bool {
	if o.ExpiresAt.IsZero() {
		return false
	}
	return time.Until(o.ExpiresAt) <= within
}

// NewsItem é uma notícia de carta nova / promo, a matéria-prima da
// vigilância diária.
type NewsItem struct {
	ID          string    `json:"id"`
	Title       string    `json:"title"`
	Summary     string    `json:"summary"`
	URL         string    `json:"url"`
	PublishedAt time.Time `json:"published_at"`
	Tags        []string  `json:"tags"`
	PlayerIDs   []int64   `json:"player_ids"` // cartas citadas, quando dá para extrair
}
