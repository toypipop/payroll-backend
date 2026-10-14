package models

import (
	"fmt"
	"testing"
	"time"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func TestPermissionDeletesInTheSameSecondHaveDistinctTombstones(t *testing.T) {
	var values []string
	for _, nanos := range []int64{200, 201} {
		db, err := gorm.Open(postgres.Open("host=127.0.0.1 port=1 user=test dbname=test sslmode=disable"), &gorm.Config{
			DisableAutomaticPing: true, DryRun: true, SkipDefaultTransaction: true, Logger: logger.Discard,
			NowFunc: func() time.Time { return time.Unix(100, nanos) },
		})
		if err != nil {
			t.Fatal(err)
		}
		statement := db.Where("roles_id = ?", 1).Delete(&RolesModuleAccess{}).Statement
		if len(statement.Vars) == 0 {
			t.Fatal("delete has no tombstone")
		}
		values = append(values, fmt.Sprint(statement.Vars[0]))
	}
	if values[0] == values[1] {
		t.Fatal("same-second permission saves would collide")
	}
}
