package repository

import (
	"strings"
	"unicode/utf8"

	"gorm.io/gorm"
	"payroll-backend/apperr"
)

type PageQuery struct {
	Page     int
	PageSize int
	Search   string
	Sort     string
	Order    string
}

type Pagination struct {
	Page       int   `json:"page"`
	PageSize   int   `json:"page_size"`
	Total      int64 `json:"total"`
	TotalPages int   `json:"total_pages"`
}

type Page[T any] struct {
	Items      []T        `json:"items"`
	Pagination Pagination `json:"pagination"`
}

func (q PageQuery) Validate() error {
	if q.Page < 1 || q.Page > 1000000 || q.PageSize < 1 || q.PageSize > 100 || utf8.RuneCountInString(q.Search) > 200 || (q.Order != "asc" && q.Order != "desc") {
		return apperr.Validation("Invalid pagination: page must be 1–1000000, page_size 1–100, search at most 200 characters, and order asc or desc")
	}
	return nil
}

// Only server-owned expressions can reach ORDER BY. The unique ID breaks ties
// so equal names/amounts cannot move unpredictably between adjacent pages.
func pageOrder(q PageQuery, sorts map[string]string, fallback, id string) (string, error) {
	if err := q.Validate(); err != nil {
		return "", err
	}
	column := fallback
	if q.Sort != "" {
		var ok bool
		column, ok = sorts[q.Sort]
		if !ok {
			return "", apperr.Validation("Unsupported sort field")
		}
	}
	return column + " " + q.Order + " NULLS LAST, " + id + " " + q.Order, nil
}

func searchPattern(search string) string {
	return "%" + strings.NewReplacer(`\`, `\\`, "%", `\%`, "_", `\_`).Replace(strings.TrimSpace(search)) + "%"
}

func readPage[T any](query *gorm.DB, q PageQuery, order string) (Page[T], error) {
	result := Page[T]{Items: make([]T, 0), Pagination: Pagination{Page: q.Page, PageSize: q.PageSize}}
	if err := query.Session(&gorm.Session{}).Count(&result.Pagination.Total).Error; err != nil {
		return result, err
	}
	result.Pagination.TotalPages = max(1, int((result.Pagination.Total+int64(q.PageSize)-1)/int64(q.PageSize)))
	result.Pagination.Page = min(q.Page, result.Pagination.TotalPages)
	err := query.Order(order).Limit(q.PageSize).Offset((result.Pagination.Page - 1) * q.PageSize).Find(&result.Items).Error
	return result, err
}
