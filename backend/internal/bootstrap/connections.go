package bootstrap

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"

	"github.com/portico/backend/internal/services/connection"
	"github.com/portico/backend/internal/services/syncjob"
)

// File is the on-disk shape for connections.json.
type File struct {
	Connections []connection.CreateInput `json:"connections"`
	SyncJobs    []SyncJobInput           `json:"sync_jobs"`
}

// SyncJobInput is a sync job in connections.json. Connections are referenced by name
// (must be unique). Nested relations nest under relations[]; relation field overrides
// nest under each relation's fields[]; root fields omit relation scope.
type SyncJobInput struct {
	Name                  string                  `json:"name"`
	SourceConnection      string                  `json:"source_connection"`
	DestinationConnection string                  `json:"destination_connection"`
	SourceTable           string                  `json:"source_table"`
	DestinationTable      string                  `json:"destination_table"`
	ChunkSize             int                     `json:"chunk_size"`
	Workers               int                     `json:"workers"`
	Config                json.RawMessage         `json:"config"`
	Relations             []syncjob.RelationInput `json:"relations"`
	Fields                []syncjob.FieldInput    `json:"fields"`
	Rules                 []syncjob.RuleInput     `json:"rules"`
}

// Result summarizes one upserted connection or sync job.
type Result struct {
	Name   string
	Type   string // connection type, or "sync_job"
	ID     uint
	Action string // "created" or "updated"
}

// ImportResult is the full outcome of ImportConnections.
type ImportResult struct {
	Connections []Result
	SyncJobs    []Result
}

// ImportConnections reads path, upserts connections by (name, type), then sync jobs by name.
func ImportConnections(connSvc *connection.Service, jobSvc *syncjob.Service, path string) (ImportResult, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return ImportResult{}, fmt.Errorf("read %s: %w", path, err)
	}
	var file File
	if err := json.Unmarshal(raw, &file); err != nil {
		return ImportResult{}, fmt.Errorf("decode %s: %w", path, err)
	}
	if len(file.Connections) == 0 && len(file.SyncJobs) == 0 {
		return ImportResult{}, fmt.Errorf("%s: no connections or sync_jobs to import", path)
	}

	out := ImportResult{
		Connections: make([]Result, 0, len(file.Connections)),
		SyncJobs:    make([]Result, 0, len(file.SyncJobs)),
	}
	for i, in := range file.Connections {
		if in.Name == "" || in.Type == "" || len(in.Config) == 0 {
			return out, fmt.Errorf("%s: connections[%d]: name, type, and config are required", path, i)
		}
		res, err := upsertConnection(connSvc, in)
		if err != nil {
			return out, fmt.Errorf("%s: connections[%d] %q (%s): %w", path, i, in.Name, in.Type, err)
		}
		out.Connections = append(out.Connections, res)
	}
	for i, in := range file.SyncJobs {
		res, err := upsertSyncJob(connSvc, jobSvc, in)
		if err != nil {
			return out, fmt.Errorf("%s: sync_jobs[%d] %q: %w", path, i, in.Name, err)
		}
		out.SyncJobs = append(out.SyncJobs, res)
	}
	return out, nil
}

func upsertConnection(svc *connection.Service, in connection.CreateInput) (Result, error) {
	existing, err := svc.FindByNameAndType(in.Name, in.Type)
	if errors.Is(err, connection.ErrNotFound) {
		created, err := svc.Create(in)
		if err != nil {
			return Result{}, err
		}
		return Result{Name: created.Name, Type: created.Type, ID: created.ID, Action: "created"}, nil
	}
	if errors.Is(err, connection.ErrAmbiguous) {
		return Result{}, err
	}
	if err != nil {
		return Result{}, err
	}

	name, typ := in.Name, in.Type
	updated, err := svc.Update(existing.ID, connection.UpdateInput{
		Name:   &name,
		Type:   &typ,
		Config: in.Config,
	})
	if err != nil {
		return Result{}, err
	}
	return Result{Name: updated.Name, Type: updated.Type, ID: updated.ID, Action: "updated"}, nil
}

func upsertSyncJob(connSvc *connection.Service, jobSvc *syncjob.Service, in SyncJobInput) (Result, error) {
	if in.Name == "" || in.SourceConnection == "" || in.DestinationConnection == "" ||
		in.SourceTable == "" || in.DestinationTable == "" {
		return Result{}, fmt.Errorf("name, source_connection, destination_connection, source_table, and destination_table are required")
	}

	src, err := connSvc.FindByName(in.SourceConnection)
	if err != nil {
		return Result{}, fmt.Errorf("source_connection %q: %w", in.SourceConnection, err)
	}
	dst, err := connSvc.FindByName(in.DestinationConnection)
	if err != nil {
		return Result{}, fmt.Errorf("destination_connection %q: %w", in.DestinationConnection, err)
	}

	chunk := in.ChunkSize
	if chunk <= 0 {
		chunk = 500
	}
	workers := in.Workers
	if workers <= 0 {
		workers = 2
	}

	existing, err := jobSvc.FindByName(in.Name)
	if errors.Is(err, syncjob.ErrNotFound) {
		created, err := jobSvc.Create(syncjob.CreateInput{
			Name:                    in.Name,
			SourceConnectionID:      src.ID,
			SourceTable:             in.SourceTable,
			DestinationConnectionID: dst.ID,
			DestinationTable:        in.DestinationTable,
			ChunkSize:               chunk,
			Workers:                 workers,
			Config:                  in.Config,
			Relations:               in.Relations,
			Fields:                  in.Fields,
			Rules:                   in.Rules,
		})
		if err != nil {
			return Result{}, err
		}
		return Result{Name: created.Name, Type: "sync_job", ID: created.ID, Action: "created"}, nil
	}
	if err != nil {
		return Result{}, err
	}

	name := in.Name
	srcTable := in.SourceTable
	dstTable := in.DestinationTable
	rels := in.Relations
	fields := in.Fields
	rules := in.Rules
	updated, err := jobSvc.Update(existing.ID, syncjob.UpdateInput{
		Name:                    &name,
		SourceConnectionID:      &src.ID,
		SourceTable:             &srcTable,
		DestinationConnectionID: &dst.ID,
		DestinationTable:        &dstTable,
		ChunkSize:               &chunk,
		Workers:                 &workers,
		Config:                  in.Config,
		Relations:               &rels,
		Fields:                  &fields,
		Rules:                   &rules,
	})
	if err != nil {
		return Result{}, err
	}
	return Result{Name: updated.Name, Type: "sync_job", ID: updated.ID, Action: "updated"}, nil
}
