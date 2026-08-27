package futgg

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"sync"
	"testing"
)

func TestEvolutionsPercorrePaginasEDeduplicaItens(t *testing.T) {
	var mu sync.Mutex
	var pages []int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		page, _ := strconv.Atoi(r.URL.Query().Get("page"))
		if page == 0 {
			page = 1
		}
		mu.Lock()
		pages = append(pages, page)
		mu.Unlock()
		payload := map[string]any{"currentPage": page, "totalPages": 2, "next": page < 2}
		if page == 1 {
			payload["data"] = []map[string]any{{"id": "a", "slug": "uma", "name": "Uma"}, {"id": "b", "slug": "duas", "name": "Duas"}}
		} else {
			payload["data"] = []map[string]any{{"id": "b", "slug": "duas", "name": "Duas"}, {"id": "c", "slug": "tres", "name": "Três"}}
		}
		_ = json.NewEncoder(w).Encode(payload)
	}))
	defer srv.Close()

	client := New(Config{BaseURL: srv.URL, Cycle: "26", Endpoints: map[string]string{"evolutions": "/evolutions"}})
	got, err := client.Evolutions(context.Background())
	if err != nil {
		t.Fatalf("Evolutions: %v", err)
	}
	if len(got) != 3 {
		t.Fatalf("quantidade = %d, want 3", len(got))
	}
	mu.Lock()
	defer mu.Unlock()
	if len(pages) != 2 || pages[0] != 1 || pages[1] != 2 {
		t.Fatalf("páginas requisitadas = %v, want [1 2]", pages)
	}
}

func TestEvolutionsNaoRepeteQuandoNextFalse(t *testing.T) {
	var requests int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		_ = json.NewEncoder(w).Encode(map[string]any{
			"currentPage": 1,
			"totalPages":  0,
			"next":        false,
			"data":        []map[string]any{{"id": "a", "name": "Uma"}},
		})
	}))
	defer srv.Close()

	client := New(Config{BaseURL: srv.URL, Cycle: "26", Endpoints: map[string]string{"evolutions": "/evolutions"}})
	if _, err := client.Evolutions(context.Background()); err != nil {
		t.Fatalf("Evolutions: %v", err)
	}
	if requests != 1 {
		t.Fatalf("requisições = %d, want 1", requests)
	}
}
