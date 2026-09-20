package futgg

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/gscarneiro/eafc-bot/internal/galeria"
)

type GalleryPoolResult struct {
	Set   galeria.Set
	Cards []galeria.Card
}

// GalleryCatalog lÃª o contrato pÃºblico do FUT.GG sem depender do nome das
// categorias. O site pode embrulhar os conjuntos em categories, sets ou data;
// a caminhada recursiva mantÃ©m a descoberta tolerante Ã  evoluÃ§Ã£o do JSON.
func (c *Client) GalleryCatalog(ctx context.Context) ([]galeria.Set, error) {
	u, err := c.URL("gallery_catalog", nil)
	if err != nil {
		return nil, err
	}
	var raw map[string]any
	if err := c.GetJSON(ctx, u, &raw); err != nil {
		return nil, err
	}
	data, ok := raw["data"].(map[string]any)
	if !ok {
		return nil, fmt.Errorf("catálogo Gallery sem envelope data")
	}
	if v := galleryInt(data["schemaVersion"]); v != 0 && v != 1 {
		return nil, fmt.Errorf("schema Gallery não suportado: %d", v)
	}
	globalRules := make([]galeria.TagRule, 0)
	if tags, ok := data["tags"].([]any); ok {
		for _, tag := range tags {
			if m, ok := tag.(map[string]any); ok {
				globalRules = append(globalRules, mapGalleryTag(m))
			}
		}
	}
	var out []galeria.Set
	if categories, ok := data["categories"].([]any); ok {
		for _, category := range categories {
			cm, ok := category.(map[string]any)
			if !ok {
				continue
			}
			categoryName := stringValue(cm, "name", "slug")
			sets, _ := cm["sets"].([]any)
			for _, item := range sets {
				if sm, ok := item.(map[string]any); ok {
					s := mapGallerySet(sm, stringValue(sm, "id", "setId"), categoryName)
					s.Rules = append([]galeria.TagRule(nil), globalRules...)
					out = append(out, s)
				}
			}
		}
	}
	return out, nil
}

func stringValue(x map[string]any, keys ...string) string {
	for _, k := range keys {
		if v, ok := x[k]; ok {
			switch y := v.(type) {
			case string:
				return y
			case float64:
				return strconv.FormatInt(int64(y), 10)
			}
		}
	}
	return ""
}
func intValue(x map[string]any, keys ...string) int {
	for _, k := range keys {
		if v, ok := x[k]; ok {
			switch y := v.(type) {
			case float64:
				return int(y)
			case int:
				return y
			case json.Number:
				n, _ := y.Int64()
				return int(n)
			}
		}
	}
	return 0
}
func mapGallerySet(x map[string]any, id, category string) galeria.Set {
	s := galeria.Set{ID: id, Name: stringValue(x, "name", "title", "label"), Category: category, RequiredCards: intValue(x, "requiredCards", "required_cards", "slots", "itemCount"), PoolSize: intValue(x, "poolSize", "pool_size"), PoolTruncated: boolValue(x, "isTruncated", "pool_truncated"), UpdatedAt: time.Now(), Thresholds: map[galeria.Grade]int{}, Rewards: map[galeria.Grade][]string{}}
	if u := stringValue(x, "url", "path", "slug"); u != "" {
		s.URL = u
	}
	if th, ok := x["thresholds"].(map[string]any); ok {
		for k, v := range th {
			s.Thresholds[galeria.Grade(strings.ToUpper(k))] = galleryInt(v)
		}
	}
	if gs, ok := x["grades"].([]any); ok {
		for _, g := range gs {
			if m, ok := g.(map[string]any); ok {
				grade := galeria.Grade(strings.ToUpper(stringValue(m, "grade", "letter", "name")))
				if grade != "" {
					s.Thresholds[grade] = intValue(m, "score", "threshold", "required")
					if rewards, ok := m["rewards"].([]any); ok {
						for _, rv := range rewards {
							if rm, ok := rv.(map[string]any); ok {
								label := stringValue(rm, "label", "type")
								if label != "" {
									s.Rewards[grade] = append(s.Rewards[grade], label)
								}
							}
						}
					}
				}
			}
		}
	}
	return s
}
func mapGalleryTag(x map[string]any) galeria.TagRule {
	r := galeria.TagRule{Name: stringValue(x, "name", "id"), BonusType: stringValue(x, "bonusType", "bonus_type")}
	if tiers, ok := x["tiers"].([]any); ok {
		for _, v := range tiers {
			if m, ok := v.(map[string]any); ok {
				r.Tiers = append(r.Tiers, galeria.Tier{MinItems: intValue(m, "minItems", "min_items"), BonusPercent: intValue(m, "bonus", "bonusPercent", "bonus_percent")})
			}
		}
	}
	if rules, ok := x["rules"].([]any); ok && len(rules) > 0 {
		if m, ok := rules[0].(map[string]any); ok {
			r.Operator = stringValue(m, "type", "operator")
			r.Attribute = stringValue(m, "attribute")
			if vs, ok := m["values"].([]any); ok {
				for _, v := range vs {
					r.Values = append(r.Values, fmt.Sprint(v))
				}
			}
		}
	}
	return r
}
func boolValue(x map[string]any, keys ...string) bool {
	for _, k := range keys {
		if v, ok := x[k]; ok {
			if b, ok := v.(bool); ok {
				return b
			}
		}
	}
	return false
}
func galleryInt(v any) int {
	switch x := v.(type) {
	case float64:
		return int(x)
	case int:
		return x
	case json.Number:
		n, _ := x.Int64()
		return int(n)
	}
	return 0
}

// GalleryPool busca apenas o conjunto solicitado. O envelope preserva
// isTruncated: cartas ausentes de um pool truncado nÃ£o sÃ£o tratadas como
// inelegÃ­veis pelo motor.
func (c *Client) GalleryPool(ctx context.Context, setID string) (galeria.Set, []galeria.Card, error) {
	u, err := c.URL("gallery_pool", map[string]string{"setId": setID})
	if err != nil {
		return galeria.Set{}, nil, err
	}
	var raw map[string]any
	if err := c.GetJSON(ctx, u, &raw); err != nil {
		return galeria.Set{}, nil, err
	}
	data := raw
	if d, ok := raw["data"].(map[string]any); ok {
		data = d
	}
	set := galeria.Set{ID: setID, RequiredCards: intValue(data, "requiredCards", "required_cards"), PoolSize: intValue(data, "poolSize", "pool_size"), PoolTruncated: boolValue(data, "isTruncated", "pool_truncated"), UpdatedAt: time.Now()}
	var cards []galeria.Card
	if items, ok := data["items"].([]any); ok {
		for _, item := range items {
			if m, ok := item.(map[string]any); ok {
				card := mapGalleryCard(m)
				cards = append(cards, card)
				if card.ID != 0 {
					set.EligibleIDs = append(set.EligibleIDs, card.ID)
				}
			}
		}
	}
	return set, cards, nil
}
func mapGalleryCard(m map[string]any) galeria.Card {
	c := galeria.Card{ID: int64(intValue(m, "eaId", "ea_id", "id")), ClubItemID: stringValue(m, "clubItemId", "club_item_id", "itemId", "item_id"), PlayerID: int64(intValue(m, "playerEaId", "player_ea_id")), Name: stringValue(m, "name", "commonName", "playerName"), Version: stringValue(m, "version", "rarityName"), Rating: intValue(m, "overall", "rating"), Club: stringValue(m, "club", "clubName"), League: stringValue(m, "league", "leagueName"), Nation: stringValue(m, "nation", "nationName"), Rarity: stringValue(m, "rarity", "rarityName"), NationID: int64(intValue(m, "nationEaId", "nationId", "nation_id")), ClubID: int64(intValue(m, "clubEaId", "clubId", "club_id")), LeagueID: int64(intValue(m, "leagueEaId", "leagueId", "league_id")), RarityID: int64(intValue(m, "rarityEaId", "rarityId", "rarity_id")), ItemScore: intValue(m, "score", "itemScore", "item_score"), WeakFoot: intValue(m, "weakFoot", "weak_foot"), SkillMoves: intValue(m, "skillMoves", "skill_moves"), Holographic: boolValue(m, "holographic", "isHolographic"), ObservedAt: time.Now(), Source: "futgg"}
	if positions, ok := m["positions"].([]any); ok {
		for _, p := range positions {
			c.Positions = append(c.Positions, fmt.Sprint(p))
		}
	}
	return c
}
