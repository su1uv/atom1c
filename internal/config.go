package internal

import "github.com/su1uv/atom1c/internal/database"

type State struct {
	Db  *database.Queries
	Cfg *Config
}

type Config struct {
	DbURL           string
	CurrentUsername string
}
