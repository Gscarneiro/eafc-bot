package analyze

import (
	"testing"
	"time"

	"github.com/gscarneiro/eafc-bot/internal/domain"
)

func TestBuildCollectionMemoryPreservaDuplicatasEOrigemDesconhecida(t *testing.T) {
	now := time.Date(2026, 8, 27, 12, 0, 0, 0, time.UTC)
	club := domain.Club{Cycle: "26", Source: "futgg", SyncedAt: now, Players: []domain.ClubPlayer{
		{Player: domain.Player{ID: 8, Name: "Duplicada", Rating: 84}},
		{Player: domain.Player{ID: 8, Name: "Duplicada", Rating: 84}},
	}}
	rollup := domain.ClubRollup{Cycle: "26", Source: "futgg", ObservedAt: now.AddDate(0, 0, -3), Entries: []domain.ClubRollupEntry{{EAID: 8, Count: 2}}}
	items := BuildCollectionMemory(club, []domain.ClubRollup{rollup}, nil)
	if len(items) != 1 || items[0].Count != 2 {
		t.Fatalf("coleção perdeu duplicatas: %+v", items)
	}
	if items[0].Identity != "incompleta" || items[0].Origin != "desconhecida" {
		t.Fatalf("identidade/origem inventada: %+v", items[0])
	}
	if items[0].PermanenceDays != 3 {
		t.Fatalf("permanência = %d, esperava 3", items[0].PermanenceDays)
	}
}

func TestBuildFodderValueNaoTransformaPrecoAusenteEmZeroConfirmado(t *testing.T) {
	items := []CollectionCard{{Player: domain.ClubPlayer{Player: domain.Player{ID: 1}}, Count: 2, Fodder: true}}
	value := BuildFodderValue(items)
	if value.MissingPrices != 2 || value.Confidence != "incompleta" {
		t.Fatalf("fodder sem preço = %+v", value)
	}
}
