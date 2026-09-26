// Package formations mantém os desenhos confirmados das formações do FC 27.
// A UI, a API e a inferência de snapshots antigos leem o mesmo catálogo.
package formations

import (
	"math"
	"sort"
	"strings"

	"github.com/gscarneiro/eafc-bot/internal/domain"
)

type Slot struct {
	Index    int             `json:"index"`
	Position domain.Position `json:"posicao"`
	X        float64         `json:"x"`
	Y        float64         `json:"y"`
}

type Formation struct {
	Name  string `json:"nome"`
	Slots []Slot `json:"vagas"`
}

type line struct {
	positions string
	x         []float64
}

func row(positions string) line              { return line{positions: positions} }
func at(positions string, x ...float64) line { return line{positions: positions, x: x} }

type definition struct {
	name  string
	lines []line // defesa até ataque; posições da direita para a esquerda do elenco
}

func form(name string, lines ...line) definition { return definition{name: name, lines: lines} }

// Fonte: https://www.fut.gg/tactics/ e a seção Positions de cada uma das
// 29 páginas ligadas ali (FC 27, consultadas em setembro de 2026). RCB/LCB,
// RDM/LDM, RAM/LAM e RS/LS são lados físicos da mesma Position do domínio.
// A ordem abaixo é a dos positionIdx: por exemplo, 4-2-1-3 traz GK, defesa,
// dois CDM, CAM, RW, ST e LW. Algumas linhas não vêm em ordem espacial no
// site; at() fixa o lado correto sem trocar o índice físico da carta.
var definitions = []definition{
	form("4-4-1-1", row("RB CB CB LB"), row("RM CM CM LM"), row("CAM"), row("ST")),
	form("4-2-1-3", row("RB CB CB LB"), row("CDM CDM"), row("CAM"), row("RW ST LW")),
	form("4-2-3-1 Wide", row("RB CB CB LB"), row("CDM CDM"), at("RM LM CAM", 84, 16, 50), row("ST")),
	form("4-4-2", row("RB CB CB LB"), row("RM CM CM LM"), row("ST ST")),
	form("4-3-3 Attack", row("RB CB CB LB"), row("CM CM"), row("CAM"), row("RW ST LW")),
	form("4-1-2-1-2 Narrow", row("RB CB CB LB"), row("CDM"), at("CM CM", 64, 36), row("CAM"), row("ST ST")),
	form("4-3-3 Holding", row("RB CB CB LB"), row("CDM"), row("CM CM"), row("RW ST LW")),
	form("4-3-1-2", row("RB CB CB LB"), row("CM CM CM"), row("CAM"), row("ST ST")),
	form("4-4-2 Holding", row("RB CB CB LB"), row("CDM CDM"), at("RM LM", 84, 16), row("ST ST")),
	form("4-1-2-1-2 Wide", row("RB CB CB LB"), row("CDM"), at("RM LM", 84, 16), row("CAM"), row("ST ST")),
	form("4-2-3-1", row("RB CB CB LB"), row("CDM CDM"), at("CAM CAM CAM", 72, 50, 28), row("ST")),
	form("4-1-4-1", row("RB CB CB LB"), row("CDM"), row("RM CM CM LM"), row("ST")),
	form("4-5-1 Flat", row("RB CB CB LB"), row("RM CM CM CM LM"), row("ST")),
	form("3-4-2-1", row("CB CB CB"), row("RM CM CM LM"), row("CAM CAM"), row("ST")),
	form("4-3-2-1", row("RB CB CB LB"), row("CM CM CM"), row("CAM CAM"), row("ST")),
	form("4-3-3 Defend", row("RB CB CB LB"), row("CDM CDM"), row("CM"), row("RW ST LW")),
	form("4-2-2-2", row("RB CB CB LB"), row("CDM CDM"), row("CAM CAM"), row("ST ST")),
	form("4-3-3", row("RB CB CB LB"), row("CM CM CM"), row("RW ST LW")),
	form("4-2-4", row("RB CB CB LB"), row("CM CM"), row("RW ST ST LW")),
	form("5-2-1-2", row("RB CB CB CB LB"), row("CM CM"), row("CAM"), row("ST ST")),
	form("4-1-3-2", row("RB CB CB LB"), row("CDM"), row("RM CM LM"), row("ST ST")),
	form("4-5-1 Attack", row("RB CB CB LB"), row("RM CM LM"), row("CAM CAM"), row("ST")),
	form("3-5-2", row("CB CB CB"), row("CDM CDM"), at("RM LM CAM", 84, 16, 50), row("ST ST")),
	form("3-4-1-2", row("CB CB CB"), row("RM CM CM LM"), row("CAM"), row("ST ST")),
	form("5-3-2", row("RB CB CB CB LB"), row("CDM"), row("CM CM"), row("ST ST")),
	form("5-2-3", row("RB CB CB CB LB"), row("CM CM"), row("RW ST LW")),
	form("5-4-1", row("RB CB CB CB LB"), row("RM CM CM LM"), row("ST")),
	form("3-4-3", row("CB CB CB"), row("RM CM CM LM"), row("RW ST LW")),
	form("3-1-4-2", row("CB CB CB"), row("CDM"), row("RM CM CM LM"), row("ST ST")),
}

func build(d definition) Formation {
	result := Formation{Name: d.name, Slots: []Slot{{Index: 0, Position: domain.GK, X: 50, Y: 90}}}
	index := 1
	for level, line := range d.lines {
		positions := strings.Fields(line.positions)
		gap := 0.0
		if len(positions) > 1 {
			gap = math.Min(34, 76/float64(len(positions)-1))
		}
		for column, pos := range positions {
			x := 50 + (float64(len(positions)-1)/2-float64(column))*gap
			if len(line.x) == len(positions) {
				x = line.x[column]
			}
			result.Slots = append(result.Slots, Slot{
				Index: index, Position: domain.Position(pos), X: x,
				Y: 90 - float64(level+1)*81/float64(len(d.lines)),
			})
			index++
		}
	}
	return result
}

func Catalog() []Formation {
	result := make([]Formation, len(definitions))
	for i, d := range definitions {
		result[i] = build(d)
	}
	return result
}

// Infer só nomeia um XI antigo quando cada posição física corresponde a um
// único desenho. O palpite vazio deixa a tabela disponível na interface.
func Infer(slots []domain.SquadSlot) string {
	return inferFrom(slots, Catalog())
}

func inferFrom(slots []domain.SquadSlot, catalog []Formation) string {
	if len(slots) != 11 {
		return ""
	}
	ordered := append([]domain.SquadSlot(nil), slots...)
	sort.Slice(ordered, func(i, j int) bool { return ordered[i].Index < ordered[j].Index })
	for i, slot := range ordered {
		if slot.Index != i || slot.Position == "" {
			return ""
		}
	}
	match := ""
	for _, formation := range catalog {
		valid := true
		for i, slot := range ordered {
			if slot.Position != formation.Slots[i].Position {
				valid = false
				break
			}
		}
		if valid {
			if match != "" {
				return ""
			}
			match = formation.Name
		}
	}
	return match
}
