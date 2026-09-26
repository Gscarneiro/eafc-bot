package formations

import (
	"testing"

	"github.com/gscarneiro/eafc-bot/internal/domain"
)

func find(name string, t *testing.T) Formation {
	t.Helper()
	for _, formation := range Catalog() {
		if formation.Name == name {
			return formation
		}
	}
	t.Fatalf("formação %q ausente", name)
	return Formation{}
}

func TestCatalogoFC27TemVinteENoveFormacoesComOnzeVagasFisicas(t *testing.T) {
	catalog := Catalog()
	if len(catalog) != 29 {
		t.Fatalf("catálogo com %d formações, esperava 29", len(catalog))
	}
	names := map[string]bool{}
	valid := map[domain.Position]bool{}
	for _, position := range domain.AllPositions {
		valid[position] = true
	}
	for _, formation := range catalog {
		if names[formation.Name] {
			t.Errorf("formação duplicada: %s", formation.Name)
		}
		names[formation.Name] = true
		if len(formation.Slots) != 11 {
			t.Errorf("%s: %d vagas", formation.Name, len(formation.Slots))
			continue
		}
		seen := map[[2]float64]bool{}
		for index, slot := range formation.Slots {
			if slot.Index != index || !valid[slot.Position] || slot.X < 0 || slot.X > 100 || slot.Y < 0 || slot.Y > 100 {
				t.Errorf("%s: vaga inválida %+v", formation.Name, slot)
			}
			point := [2]float64{slot.X, slot.Y}
			if seen[point] {
				t.Errorf("%s: duas cartas na coordenada %v", formation.Name, point)
			}
			seen[point] = true
		}
		if formation.Slots[0].Position != domain.GK {
			t.Errorf("%s: índice 0 não é GK", formation.Name)
		}
	}
}

func TestQuatroDoisUmTresEVariantesTemPosicoesEGeometriasCorretas(t *testing.T) {
	formation := find("4-2-1-3", t)
	want := []domain.Position{domain.GK, domain.RB, domain.CB, domain.CB, domain.LB, domain.CDM, domain.CDM, domain.CAM, domain.RW, domain.ST, domain.LW}
	for index, position := range want {
		if formation.Slots[index].Position != position {
			t.Errorf("vaga %d: %s, esperava %s", index, formation.Slots[index].Position, position)
		}
	}
	narrow := find("4-2-3-1", t)
	wide := find("4-2-3-1 Wide", t)
	if narrow.Slots[7].Position != domain.CAM || wide.Slots[7].Position != domain.RM || narrow.Slots[7].X >= wide.Slots[7].X {
		t.Errorf("variantes estreita e larga perderam posição/largura: narrow=%+v wide=%+v", narrow.Slots[7], wide.Slots[7])
	}
	if wide.Slots[8].X >= wide.Slots[9].X {
		t.Errorf("LM precisa ficar à esquerda do CAM, embora venha antes no índice: %+v %+v", wide.Slots[8], wide.Slots[9])
	}
	if find("4-1-2-1-2 Narrow", t).Slots[6].X >= find("4-1-2-1-2 Wide", t).Slots[6].X {
		t.Error("a variante Wide deve abrir a linha lateral")
	}
}

func TestInferenciaAceitaSomenteSequenciaUnica(t *testing.T) {
	formation := find("4-2-1-3", t)
	slots := make([]domain.SquadSlot, 11)
	for i, slot := range formation.Slots {
		slots[i] = domain.SquadSlot{Index: slot.Index, Position: slot.Position}
	}
	if got := Infer(slots); got != "4-2-1-3" {
		t.Errorf("inferência = %q, esperava 4-2-1-3", got)
	}
	if got := inferFrom(slots, []Formation{formation, formation}); got != "" {
		t.Errorf("assinatura ambígua virou %q", got)
	}
	slots[7].Position = domain.ST
	if got := Infer(slots); got != "" {
		t.Errorf("sequência desconhecida virou %q", got)
	}
}
