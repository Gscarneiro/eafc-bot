package futgg

import (
	"context"
	"github.com/gscarneiro/eafc-bot/internal/galeria"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestGalleryCatalogNormalizaTagsETiers(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/fut/gallery/fc27/" {
			t.Fatalf("rota = %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":{"schemaVersion":1,"tags":[{"id":1,"name":"First Owner","bonusType":"ITEM_SCORE_PERCENTAGE","thresholdType":"COUNT","rules":[{"type":"COUNT","target":"ATTRIBUTE","attribute":"FIRST_OWNED","values":["1"]}],"tiers":[{"minItems":5,"bonus":150}]}],"categories":[{"id":7,"name":"Rarities","sets":[{"id":116,"name":"Starter Set","requiredCards":5,"grades":[{"name":"D","threshold":10,"rewards":[{"id":"badge","label":"Starter Badge","type":"BADGE","count":2,"value":500,"imagePath":"2027/gg-club-badge/116.webp","teamEaId":116}]},{"name":"S","threshold":2000}]}]}]}}`))
	}))
	defer server.Close()
	client := New(Config{BaseURL: server.URL, Cycle: "27", Endpoints: map[string]string{"gallery_catalog": "/api/fut/gallery/fc{cycle}/"}, CacheDir: t.TempDir(), RequestsPerSec: 100})
	sets, err := client.GalleryCatalog(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(sets) != 1 || sets[0].RequiredCards != 5 || sets[0].Thresholds[galeria.GradeS] != 2000 || sets[0].CategoryID != 7 {
		t.Fatalf("sets = %#v", sets)
	}
	if sets[0].TeamID != 116 || sets[0].BadgeURL != "https://game-assets.fut.gg/cdn-cgi/image/quality=85,format=auto,width=300/2027/gg-club-badge/116.webp" {
		t.Fatalf("emblema = %#v", sets[0])
	}
	if len(sets[0].Rules) != 1 || sets[0].Rules[0].Operator != "COUNT" || sets[0].Rules[0].Tiers[0].BonusPercent != 150 {
		t.Fatalf("rules = %#v", sets[0].Rules)
	}
	reward := sets[0].Rewards[galeria.GradeD][0]
	if reward.ID != "badge" || reward.Type != "BADGE" || reward.Count != 2 || reward.Value != 500 || reward.ImageURL == "" {
		t.Fatalf("recompensa = %#v", reward)
	}
}

func TestGalleryPoolPreservaAtributosDoItem(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/fut/gallery/fc27/sets/116/pool/" {
			t.Fatalf("rota = %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":{"setId":116,"requiredCards":5,"poolSize":1,"isTruncated":true,"items":[{"eaId":10,"playerEaId":20,"score":90000,"overall":91,"nationEaId":1,"clubEaId":2,"leagueEaId":3,"rarityEaId":12,"positions":["ST","CF"],"weakFoot":5,"skillMoves":4,"holographic":true}]}}`))
	}))
	defer server.Close()
	client := New(Config{BaseURL: server.URL, Cycle: "27", Endpoints: map[string]string{"gallery_pool": "/api/fut/gallery/fc{cycle}/sets/{setId}/pool/"}, CacheDir: t.TempDir(), RequestsPerSec: 100})
	set, cards, err := client.GalleryPool(context.Background(), "116")
	if err != nil || len(cards) != 1 {
		t.Fatalf("pool = %#v, %v", cards, err)
	}
	if !set.PoolTruncated || cards[0].ItemScore != 90000 || cards[0].ClubID != 2 || len(cards[0].Positions) != 2 {
		t.Fatalf("card = %#v", cards[0])
	}
}
