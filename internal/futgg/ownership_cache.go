package futgg

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"
)

// ClubItemState é a procedência que o GG Club publicou para uma cópia física
// da carta. Os ponteiros mantêm falso separado de campo ausente.
type ClubItemState struct {
	FirstOwner *bool
	Loan       *bool
	ObservedAt time.Time
}

// RecoverClubItemStates recupera apenas campos explicitamente publicados em
// respostas já guardadas. A chave é ClubItemID, que inclui a cópia física do
// clube: uma página de outra coleta nunca vira um elenco atual inteiro, nem a
// ausência de uma carta é usada como evidência sobre ela.
func RecoverClubItemStates(cacheDir string, wanted map[string]bool) map[string]ClubItemState {
	result := make(map[string]ClubItemState)
	if cacheDir == "" || len(wanted) == 0 {
		return result
	}
	entries, err := os.ReadDir(cacheDir)
	if err != nil {
		return result
	}
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".cache" {
			continue
		}
		path := filepath.Join(cacheDir, entry.Name())
		info, err := entry.Info()
		if err != nil {
			continue
		}
		body, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		var raw any
		if json.Unmarshal(body, &raw) != nil {
			continue
		}
		findClubItemStates(raw, wanted, info.ModTime(), result)
	}
	return result
}

func findClubItemStates(value any, wanted map[string]bool, observed time.Time, result map[string]ClubItemState) {
	switch current := value.(type) {
	case []any:
		for _, item := range current {
			findClubItemStates(item, wanted, observed, result)
		}
	case map[string]any:
		itemID, _ := current["id"].(string)
		if wanted[itemID] {
			if definition, ok := current["playerDef"].(map[string]any); ok {
				state := ClubItemState{ObservedAt: observed}
				if firstOwner, ok := definition["isFirstOwner"].(bool); ok {
					state.FirstOwner = &firstOwner
				}
				if duration, ok := number(definition["loanDuration"]); ok {
					loan := duration > 0
					state.Loan = &loan
				}
				if state.FirstOwner != nil || state.Loan != nil {
					if previous, exists := result[itemID]; !exists || previous.ObservedAt.Before(observed) {
						result[itemID] = state
					}
				}
			}
		}
		for _, item := range current {
			findClubItemStates(item, wanted, observed, result)
		}
	}
}

func number(value any) (int, bool) {
	switch n := value.(type) {
	case float64:
		return int(n), true
	case json.Number:
		i, err := n.Int64()
		return int(i), err == nil
	default:
		return 0, false
	}
}
