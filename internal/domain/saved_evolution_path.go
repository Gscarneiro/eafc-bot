package domain

import (
	"fmt"
	"time"
)

// SavedEvolutionImpact conserva a decisão que a pessoa viu ao salvar um
// caminho. Ã‰ uma fotografia local: nunca representa uma alteraÃ§Ã£o na conta
// EA nem tenta escalar a carta automaticamente.
type SavedEvolutionImpact struct {
	Kind            string     `json:"kind"`
	SlotIndex       int        `json:"slot_index,omitempty"`
	Position        Position   `json:"position,omitempty"`
	Starter         ClubPlayer `json:"starter,omitempty"`
	StarterGGRating float64    `json:"starter_gg_rating,omitempty"`
	FinalGGRating   float64    `json:"final_gg_rating,omitempty"`
	Gain            float64    `json:"gain,omitempty"`
}

// SavedEvolutionPath Ã© um marcador auditÃ¡vel de um path confirmado. A
// fotografia fica guardada mesmo quando a prÃ³xima coleta nÃ£o encontra mais a
// carta ou a evoluÃ§Ã£o; a API compara VersionHash com o snapshot atual para
// explicar se o caminho mudou, expirou ou desapareceu.
type SavedEvolutionPath struct {
	ID               string               `json:"id"`
	PathID           string               `json:"path_id"`
	Cycle            string               `json:"cycle"`
	CardKey          string               `json:"card_key"`
	CardSlug         string               `json:"card_slug,omitempty"`
	IdentityComplete bool                 `json:"identity_complete"`
	Player           ClubPlayer           `json:"player"`
	Path             EvolutionPath        `json:"path"`
	CurrentGGRating  float64              `json:"current_gg_rating,omitempty"`
	FinalOverall     int                  `json:"final_overall"`
	FinalGGRating    float64              `json:"final_gg_rating"`
	GGRatingGain     float64              `json:"gg_rating_gain"`
	Impact           SavedEvolutionImpact `json:"impact"`
	VersionHash      string               `json:"version_hash"`
	SavedAt          time.Time            `json:"saved_at"`
}

func (p SavedEvolutionPath) Validate() error {
	if p.ID == "" || p.PathID == "" || p.Cycle == "" {
		return fmt.Errorf("path salvo precisa de id, path_id e ciclo")
	}
	if p.CardKey == "" {
		return fmt.Errorf("path salvo precisa identificar a carta")
	}
	if len(p.Path.Steps) < 2 {
		return fmt.Errorf("path salvo precisa de carta inicial e final")
	}
	return nil
}
