package platform

import (
	"net/http"
	"strconv"
)

const listPageSize = 20

type pageResult[T any] struct {
	Items   []T  `json:"items"`
	Page    int  `json:"page"`
	HasNext bool `json:"has_next"`
	HasPrev bool `json:"has_prev"`
}

func requestPage(w http.ResponseWriter, r *http.Request) (int, bool) {
	raw := r.URL.Query().Get("page")
	if raw == "" {
		return 1, true
	}
	page, err := strconv.Atoi(raw)
	if err != nil || page < 1 || page > 10000 {
		fail(w, 400, "Invalid page")
		return 0, false
	}
	return page, true
}

func paged[T any](items []T, page int) pageResult[T] {
	hasNext := len(items) > listPageSize
	if hasNext {
		items = items[:listPageSize]
	}
	return pageResult[T]{Items: items, Page: page, HasNext: hasNext, HasPrev: page > 1}
}
