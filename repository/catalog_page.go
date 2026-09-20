package repository

import (
	"strings"

	"gorm.io/gorm"
	"gorm.io/gorm/schema"
)

// Only JSON-visible scalar model fields and repository-owned expressions can be
// searched or sorted. Request values never become SQL identifiers.
func readCatalogPage[T any](query *gorm.DB, q PageQuery, extra map[string]string) (Page[T], error) {
	statement := &gorm.Statement{DB: query}
	if err := statement.Parse(new(T)); err != nil {
		return Page[T]{}, err
	}
	table := statement.Schema.Table
	sorts := make(map[string]string)
	expressions := make([]string, 0)
	for _, field := range statement.Schema.Fields {
		key := strings.Split(field.Tag.Get("json"), ",")[0]
		if field.DBName == "" || key == "" || key == "-" {
			continue
		}
		column := table + "." + field.DBName
		sorts[key] = column
		if field.DataType == schema.Time {
			expressions = append(expressions, "to_char("+column+" AT TIME ZONE 'Asia/Bangkok', 'YYYY-MM-DD HH24:MI')")
		} else {
			expressions = append(expressions, "CAST("+column+" AS TEXT)")
		}
	}
	for key, column := range extra {
		sorts[key] = column
		expressions = append(expressions, "CAST("+column+" AS TEXT)")
	}
	order, err := pageOrder(q, sorts, table+".id", table+".id")
	if err != nil {
		return Page[T]{}, err
	}
	if q.Search != "" {
		clauses := make([]string, 0, len(expressions))
		args := make([]any, 0, len(expressions))
		for _, expression := range expressions {
			clauses = append(clauses, expression+" ILIKE ?")
			args = append(args, searchPattern(q.Search))
		}
		query = query.Where("("+strings.Join(clauses, " OR ")+")", args...)
	}
	return readPage[T](query.Model(new(T)), q, order)
}
