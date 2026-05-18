package httpx

import (
	"math"
	"net/http"
	"strconv"
)

// Pagination holds parsed pagination params.
type Pagination struct {
	Page     int
	PageSize int
	Offset   int
}

// DefaultPageSize is the default number of items per page.
const DefaultPageSize = 20

// MaxPageSize is the maximum allowed page size.
const MaxPageSize = 100

// ParsePagination extracts page and pageSize from query params.
func ParsePagination(r *http.Request) Pagination {
	page := queryInt(r, "page", 1)
	pageSize := queryInt(r, "pageSize", DefaultPageSize)

	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = DefaultPageSize
	}
	if pageSize > MaxPageSize {
		pageSize = MaxPageSize
	}

	return Pagination{
		Page:     page,
		PageSize: pageSize,
		Offset:   (page - 1) * pageSize,
	}
}

// PaginatedResponse is the standard paginated response envelope.
type PaginatedResponse[T any] struct {
	Data       []T            `json:"data"`
	Pagination PaginationMeta `json:"pagination"`
}

// PaginationMeta holds pagination metadata for responses.
type PaginationMeta struct {
	Page       int `json:"page"`
	PageSize   int `json:"pageSize"`
	Total      int `json:"total"`
	TotalPages int `json:"totalPages"`
}

// NewPaginatedResponse creates a paginated response.
func NewPaginatedResponse[T any](data []T, p Pagination, total int) PaginatedResponse[T] {
	totalPages := int(math.Ceil(float64(total) / float64(p.PageSize)))
	if data == nil {
		data = []T{}
	}
	return PaginatedResponse[T]{
		Data: data,
		Pagination: PaginationMeta{
			Page:       p.Page,
			PageSize:   p.PageSize,
			Total:      total,
			TotalPages: totalPages,
		},
	}
}

func queryInt(r *http.Request, key string, fallback int) int {
	v := r.URL.Query().Get(key)
	if v == "" {
		return fallback
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return fallback
	}
	return n
}

// QueryString extracts a string query param with fallback.
func QueryString(r *http.Request, key, fallback string) string {
	v := r.URL.Query().Get(key)
	if v == "" {
		return fallback
	}
	return v
}
