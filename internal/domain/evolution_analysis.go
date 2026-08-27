package domain

import "time"

// AnalysisSource é uma referência que o agente usou para sustentar a
// recomendação. O servidor só aceita URLs HTTPS (ou hosts locais em testes)
// e a UI sempre exibe as referências ao lado do texto do agente.
type AnalysisSource struct {
	Title string `json:"title"`
	URL   string `json:"url"`
}

// EvolutionAnalysis é o resultado estruturado e persistível de uma consulta
// ao agente especialista. InputHash permite reutilizar uma análise idêntica;
// o payload da carta não é armazenado aqui, apenas o resultado auditável.
type EvolutionAnalysis struct {
	ID              string           `json:"id"`
	Cycle           string           `json:"cycle"`
	EvolutionID     string           `json:"evolution_id"`
	EvolutionSlug   string           `json:"evolution_slug"`
	PlayerKey       string           `json:"player_key"`
	InputHash       string           `json:"input_hash"`
	ContractVersion string           `json:"contract_version"`
	Status          string           `json:"status"` // queued, running, completed, failed
	CreatedAt       time.Time        `json:"created_at"`
	UpdatedAt       time.Time        `json:"updated_at"`
	Verdict         string           `json:"verdict,omitempty"`
	Summary         string           `json:"summary,omitempty"`
	Strengths       []string         `json:"strengths,omitempty"`
	Risks           []string         `json:"risks,omitempty"`
	BestPositions   []string         `json:"best_positions,omitempty"`
	Sources         []AnalysisSource `json:"sources,omitempty"`
	Error           string           `json:"error,omitempty"`
}
