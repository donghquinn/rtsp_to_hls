package configs

import (
	"os"
	"strconv"
)

type DatabaseConf struct {
	Host     string
	Port     int
	User     string
	Passwd   string
	Database string
}

var DatabaseConfig DatabaseConf

func SetDatabaseConfig() {
	DatabaseConfig.Host = os.Getenv("POSTGRES_HOST")

	dbPort := 5432

	port, convErr := strconv.Atoi(os.Getenv("POSTGRES_PORT"))

	if convErr == nil {
		dbPort = port
	}

	DatabaseConfig.Port = dbPort

	DatabaseConfig.Database = os.Getenv("POSTGRES_NAME")
	DatabaseConfig.User = os.Getenv("POSTGRES_USER")
	DatabaseConfig.Passwd = os.Getenv("POSTGRES_PASSWD")
}
