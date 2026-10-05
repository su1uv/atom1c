package internal

import (
	"database/sql"

	"github.com/su1uv/atom1c/internal/database"
)

type State struct {
	Db    *database.Queries
	SQLDB *sql.DB
	Cfg   *Config
}

type Config struct {
	DbURL           string
	CurrentUsername string
}
