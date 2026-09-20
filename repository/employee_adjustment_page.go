package repository

import "gorm.io/gorm"

// Table and relation names are repository constants, never request parameters.
// Both assignment lists share filtering, paging and minimal employee preloads.
func readEmployeeAdjustmentPage[T any](query *gorm.DB, q PageQuery, table, typeTable, relation string) (Page[T], error) {
	name := "concat_ws(' ', e.first_name, e.last_name)"
	order, err := pageOrder(q, map[string]string{
		"employee_name": name, typeTable + "_name": "t.name",
		"amount": table + ".amount", "employees.companies.name": "c.name",
	}, table+".id", table+".id")
	if err != nil {
		return Page[T]{}, err
	}
	query = query.Model(new(T)).
		Joins("LEFT JOIN employees e ON e.id = " + table + ".employees_id AND e.deleted_at = 0").
		Joins("LEFT JOIN companies c ON c.id = e.companies_id AND c.deleted_at = 0").
		Joins("LEFT JOIN " + typeTable + " t ON t.id = " + table + "." + typeTable + "_id AND t.deleted_at IS NULL")
	if q.Search != "" {
		pattern := searchPattern(q.Search)
		query = query.Where("("+name+" ILIKE ? OR e.employees_code ILIKE ? OR t.name ILIKE ? OR c.name ILIKE ? OR CAST("+table+".amount AS TEXT) ILIKE ?)", pattern, pattern, pattern, pattern, pattern)
	}
	return readPage[T](query.Preload(relation).Preload("Employees", func(db *gorm.DB) *gorm.DB {
		return db.Select("id, companies_id, first_name, last_name, employees_code")
	}).Preload("Employees.Companies"), q, order)
}
