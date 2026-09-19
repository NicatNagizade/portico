package register

import (
	"github.com/portico/backend/internal/connectors"
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
	// Future: RegisterDestination(models.ConnectionTypeMySQL, ...),
	// Future: RegisterDestination(models.ConnectionTypeMongoDB, ...),
	return r
}
