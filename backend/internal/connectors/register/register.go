package register

import (
	"github.com/portico/backend/internal/connectors"
	mongoconn "github.com/portico/backend/internal/connectors/mongodb"
	mysqlconn "github.com/portico/backend/internal/connectors/mysql"
	pgconn "github.com/portico/backend/internal/connectors/postgres"
	redisconn "github.com/portico/backend/internal/connectors/redis"
	sqliteconn "github.com/portico/backend/internal/connectors/sqlite"
	tsconn "github.com/portico/backend/internal/connectors/typesense"
	"github.com/portico/backend/internal/models"
)

// DefaultRegistry returns a registry with all connectors registered as source and destination.
func DefaultRegistry() *connectors.Registry {
	r := connectors.NewRegistry()

	r.RegisterSource(models.ConnectionTypeMySQL, mysqlconn.NewSource)
	r.RegisterDestination(models.ConnectionTypeMySQL, mysqlconn.NewDestination)

	r.RegisterSource(models.ConnectionTypePostgres, pgconn.NewSource)
	r.RegisterDestination(models.ConnectionTypePostgres, pgconn.NewDestination)

	r.RegisterSource(models.ConnectionTypeSQLite, sqliteconn.NewSource)
	r.RegisterDestination(models.ConnectionTypeSQLite, sqliteconn.NewDestination)

	r.RegisterSource(models.ConnectionTypeMongoDB, mongoconn.NewSource)
	r.RegisterDestination(models.ConnectionTypeMongoDB, mongoconn.NewDestination)

	r.RegisterSource(models.ConnectionTypeTypesense, tsconn.NewSource)
	r.RegisterDestination(models.ConnectionTypeTypesense, tsconn.NewDestination)

	r.RegisterSource(models.ConnectionTypeRedis, redisconn.NewSource)
	r.RegisterDestination(models.ConnectionTypeRedis, redisconn.NewDestination)

	return r
}
