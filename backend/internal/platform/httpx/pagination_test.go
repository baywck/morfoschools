package httpx

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestParsePagination_Defaults(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/items", nil)
	p := ParsePagination(r)

	if p.Page != 1 {
		t.Errorf("expected page 1, got %d", p.Page)
	}
	if p.PageSize != DefaultPageSize {
		t.Errorf("expected pageSize %d, got %d", DefaultPageSize, p.PageSize)
	}
	if p.Offset != 0 {
		t.Errorf("expected offset 0, got %d", p.Offset)
	}
}

func TestParsePagination_Custom(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/items?page=3&pageSize=10", nil)
	p := ParsePagination(r)

	if p.Page != 3 {
		t.Errorf("expected page 3, got %d", p.Page)
	}
	if p.PageSize != 10 {
		t.Errorf("expected pageSize 10, got %d", p.PageSize)
	}
	if p.Offset != 20 {
		t.Errorf("expected offset 20, got %d", p.Offset)
	}
}

func TestParsePagination_ClampsMax(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/items?pageSize=500", nil)
	p := ParsePagination(r)

	if p.PageSize != MaxPageSize {
		t.Errorf("expected pageSize clamped to %d, got %d", MaxPageSize, p.PageSize)
	}
}

func TestParsePagination_InvalidValues(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/items?page=-1&pageSize=abc", nil)
	p := ParsePagination(r)

	if p.Page != 1 {
		t.Errorf("expected page 1 for negative, got %d", p.Page)
	}
	if p.PageSize != DefaultPageSize {
		t.Errorf("expected default pageSize for invalid, got %d", p.PageSize)
	}
}

func TestNewPaginatedResponse(t *testing.T) {
	data := []string{"a", "b", "c"}
	p := Pagination{Page: 2, PageSize: 10, Offset: 10}
	resp := NewPaginatedResponse(data, p, 23)

	if len(resp.Data) != 3 {
		t.Errorf("expected 3 items, got %d", len(resp.Data))
	}
	if resp.Pagination.Total != 23 {
		t.Errorf("expected total 23, got %d", resp.Pagination.Total)
	}
	if resp.Pagination.TotalPages != 3 {
		t.Errorf("expected 3 total pages, got %d", resp.Pagination.TotalPages)
	}
}

func TestNewPaginatedResponse_NilData(t *testing.T) {
	resp := NewPaginatedResponse[string](nil, Pagination{Page: 1, PageSize: 20}, 0)

	if resp.Data == nil {
		t.Error("expected empty slice, got nil")
	}
	if len(resp.Data) != 0 {
		t.Errorf("expected 0 items, got %d", len(resp.Data))
	}
}
