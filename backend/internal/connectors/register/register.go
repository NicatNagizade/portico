package register

import (
	"github.com/portico/backend/internal/connectors"
	mongoconn "github.com/portico/backend/internal/connectors/mongodb"
	mysqlconn "github.com/portico/backend/internal/connectors/mysql"
	pgconn "github.com/portico/backend/internal/connectors/postgres"
	tsconn "github.com/portico/backend/internal/connectors/typesense"
	"github.com/portico/backend/internal/models"
)

// DefaultRegistry returns a registry with v1 connectors registered.
func DefaultRegistry() *connectors.Registry {
	r := connectors.NewRegistry()
	r.RegisterSource(models.ConnectionTypeMySQL, mysqlconn.NewSource)
	r.RegisterSource(models.ConnectionTypePostgres, pgconn.NewSource)
	r.RegisterDestination(models.ConnectionTypeTypesense, tsconn.NewDestination)
	r.RegisterDestination(models.ConnectionTypeMongoDB, mongoconn.NewDestination)
	r.RegisterDestination(models.ConnectionTypeMySQL, mysqlconn.NewDestination)
	r.RegisterDestination(models.ConnectionTypePostgres, pgconn.NewDestination)
	return r
}
