package query

import (
	"sort"
	"strings"
)

type Field[T any] struct {
	Name   string
	Kind   Kind
	Get    func(T) Value
	Facet  bool
	Search bool
	Desc   bool
}

type Schema[T any] struct {
	Name    string
	Default string
	MaxTop  int
	Fields  []Field[T]
}

func NewSchema[T any](name, defaultOrder string, maxTop int, fields ...Field[T]) Schema[T] {
	if maxTop <= 0 {
		maxTop = 100
	}
	return Schema[T]{Name: name, Default: defaultOrder, MaxTop: maxTop, Fields: fields}
}

func (s Schema[T]) field(name string) (Field[T], bool) {
	for _, field := range s.Fields {
		if field.Name == name {
			return field, true
		}
	}
	return Field[T]{}, false
}

func (s Schema[T]) names() []string {
	names := make([]string, 0, len(s.Fields))
	for _, field := range s.Fields {
		names = append(names, field.Name)
	}
	sort.Strings(names)
	return names
}

func (s Schema[T]) defaultOrders() []Order {
	if strings.TrimSpace(s.Default) == "" {
		return nil
	}
	orders, err := parseOrder(s.Default)
	if err != nil {
		return nil
	}
	return orders
}
