package internal

import "github.com/su1uv/atom1c/internal/db"

type State struct {
	Db  *db.Queries
	Cfg *Config
}

type Config struct {
	DbURL           string
	CurrentUsername string
}
