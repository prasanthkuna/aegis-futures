package aegis

import "encore.dev/storage/sqldb"

var db = sqldb.NewDatabase("aegis", sqldb.DatabaseConfig{
	Migrations: "./migrations",
})
