package config

import (
	"database/sql"
	"fmt"
	"strconv"
	"time"
)

type databasePool struct {
	maxOpen, maxIdle   int
	lifetime, idleTime time.Duration
}

func readDatabasePool() (databasePool, error) {
	var pool databasePool
	for _, setting := range []struct {
		key, fallback string
		minimum       int
		target        *int
	}{
		{"DB_MAX_OPEN_CONNS", "20", 2, &pool.maxOpen},
		{"DB_MAX_IDLE_CONNS", "5", 0, &pool.maxIdle},
	} {
		value, err := strconv.Atoi(getEnv(setting.key, setting.fallback))
		if err != nil || value < setting.minimum {
			return databasePool{}, fmt.Errorf("%s must be an integer >= %d", setting.key, setting.minimum)
		}
		*setting.target = value
	}
	if pool.maxIdle > pool.maxOpen {
		return databasePool{}, fmt.Errorf("DB_MAX_IDLE_CONNS must not exceed DB_MAX_OPEN_CONNS")
	}
	for _, setting := range []struct {
		key, fallback string
		target        *time.Duration
	}{
		{"DB_CONN_MAX_LIFETIME", "30m", &pool.lifetime},
		{"DB_CONN_MAX_IDLE_TIME", "5m", &pool.idleTime},
	} {
		value, err := time.ParseDuration(getEnv(setting.key, setting.fallback))
		if err != nil || value <= 0 {
			return databasePool{}, fmt.Errorf("%s must be a positive duration (for example 5m)", setting.key)
		}
		*setting.target = value
	}
	return pool, nil
}

func (p databasePool) apply(db *sql.DB) {
	db.SetMaxOpenConns(p.maxOpen)
	db.SetMaxIdleConns(p.maxIdle)
	db.SetConnMaxLifetime(p.lifetime)
	db.SetConnMaxIdleTime(p.idleTime)
}
