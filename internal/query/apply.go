package query

import (
	"sort"
	"strings"
)

type Facet struct {
	Value string `json:"value"`
	Count int    `json:"count"`
}

type Page[T any] struct {
	Value  []T                `json:"value"`
	Count  int                `json:"@odata.count"`
	Skip   int                `json:"@eafc.skip"`
	Top    int                `json:"@eafc.top"`
	Facets map[string][]Facet `json:"@eafc.facets,omitempty"`
}

// Apply executa a ordem fixa do contrato: validação, facetas, busca, filtro,
// contagem, ordenação e paginação. Assim os contadores continuam úteis mesmo
// quando o filtro atual zera a lista visível.
func Apply[T any](schema Schema[T], rows []T, options Options) (Page[T], error) {
	if err := validate(schema, options); err != nil {
		return Page[T]{}, err
	}
	page := Page[T]{Skip: options.Skip}
	if options.Top > 0 && options.Top < schema.MaxTop {
		page.Top = options.Top
	} else {
		page.Top = schema.MaxTop
	}
	page.Facets = buildFacets(schema, rows)

	filtered := make([]T, 0, len(rows))
	for _, row := range rows {
		if options.Search != "" && !matchesSearch(schema, row, options.Search) {
			continue
		}
		if options.Filter != nil && !options.Filter.Eval(func(name string) (Value, bool) {
			field, ok := schema.field(name)
			if !ok || field.Get == nil {
				return Value{}, false
			}
			return field.Get(row), true
		}) {
			continue
		}
		filtered = append(filtered, row)
	}
	page.Count = len(filtered)

	orders := options.Orders
	if len(orders) == 0 {
		orders = schema.defaultOrders()
	}
	if len(orders) > 0 {
		sort.SliceStable(filtered, func(i, j int) bool {
			for _, order := range orders {
				field, ok := schema.field(order.Field)
				if !ok || field.Get == nil {
					continue
				}
				cmp := compareValues(field.Get(filtered[i]), field.Get(filtered[j]))
				if cmp == 0 {
					continue
				}
				if order.Desc {
					return cmp > 0
				}
				return cmp < 0
			}
			return false
		})
	}

	start := options.Skip
	if start > len(filtered) {
		start = len(filtered)
	}
	end := start + page.Top
	if end > len(filtered) {
		end = len(filtered)
	}
	page.Value = filtered[start:end]
	return page, nil
}

func validate[T any](schema Schema[T], options Options) error {
	valid := func(name string) error {
		if _, ok := schema.field(name); ok {
			return nil
		}
		return &Error{Parameter: "$filter/$orderby", Message: "campo desconhecido " + strconvQuote(name), ValidFields: schema.names()}
	}
	fields := map[string]bool{}
	options.Filter.fields(fields)
	for name := range fields {
		if err := valid(name); err != nil {
			return err
		}
	}
	for _, order := range options.Orders {
		if err := valid(order.Field); err != nil {
			return err
		}
	}
	return nil
}

func buildFacets[T any](schema Schema[T], rows []T) map[string][]Facet {
	result := map[string][]Facet{}
	for _, field := range schema.Fields {
		if !field.Facet || field.Get == nil {
			continue
		}
		counts := map[string]int{}
		for _, row := range rows {
			value := field.Get(row)
			if value.Kind == Null || value.Kind == Invalid {
				continue
			}
			counts[value.String()]++
		}
		facets := make([]Facet, 0, len(counts))
		for value, count := range counts {
			facets = append(facets, Facet{Value: value, Count: count})
		}
		sort.Slice(facets, func(i, j int) bool {
			if facets[i].Count != facets[j].Count {
				return facets[i].Count > facets[j].Count
			}
			return strings.Compare(Fold(facets[i].Value), Fold(facets[j].Value)) < 0
		})
		result[field.Name] = facets
	}
	if len(result) == 0 {
		return nil
	}
	return result
}

func matchesSearch[T any](schema Schema[T], row T, search string) bool {
	needle := Fold(search)
	for _, field := range schema.Fields {
		if !field.Search || field.Get == nil {
			continue
		}
		value := field.Get(row)
		if value.Kind == String && strings.Contains(Fold(value.S), needle) {
			return true
		}
	}
	return false
}

func strconvQuote(value string) string {
	return "\"" + strings.ReplaceAll(value, "\"", "\\\"") + "\""
}
