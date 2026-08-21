package main

import (
	"fmt"
	"log"
	"os"
	"strings"

	"gorm.io/gorm"
)

// isProduction mirrors the check in handlers/auth: any of the three markers is enough to treat this as a real deployment.
func isProduction() bool {
	return strings.EqualFold(os.Getenv("APP_ENV"), "production") ||
		strings.EqualFold(os.Getenv("GO_ENV"), "production") ||
		strings.EqualFold(os.Getenv("GIN_MODE"), "release")
}

// resetDatabase empties every table in the public schema, so a seed starts from nothing instead of merging onto whatever the database already held.
// It truncates rather than drops: the schema stays in place for AutoMigrate, and RESTART IDENTITY puts the id sequences back to 1 so seeded ids are reproducible across runs.
func resetDatabase(db *gorm.DB) error {
	var tables []string
	if err := db.Raw(`
		SELECT tablename FROM pg_tables
		WHERE schemaname = 'public'
		ORDER BY tablename
	`).Scan(&tables).Error; err != nil {
		return fmt.Errorf("list tables: %w", err)
	}
	if len(tables) == 0 {
		log.Println("Reset: database is empty, nothing to truncate")
		return nil
	}

	quoted := make([]string, 0, len(tables))
	for _, table := range tables {
		quoted = append(quoted, `"public"."`+table+`"`)
	}

	// CASCADE is required: every table is truncated in the same statement, so the foreign keys between them are satisfied at the end of it.
	statement := "TRUNCATE TABLE " + strings.Join(quoted, ", ") + " RESTART IDENTITY CASCADE"
	if err := db.Exec(statement).Error; err != nil {
		return fmt.Errorf("truncate %d table(s): %w", len(tables), err)
	}

	log.Printf("Reset: truncated %d table(s) — %s", len(tables), strings.Join(tables, ", "))
	return nil
}
