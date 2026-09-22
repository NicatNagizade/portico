package bootstrap

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"

	"github.com/portico/backend/internal/services/connection"
)

// File is the on-disk shape for connections.json.
type File struct {
	Connections []connection.CreateInput `json:"connections"`
}

// Result summarizes one upserted connection.
type Result struct {
	Name   string
	Type   string
	ID     uint
	Action string // "created" or "updated"
}

// ImportConnections reads path and upserts each entry by (name, type).
func ImportConnections(svc *connection.Service, path string) ([]Result, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	var file File
	if err := json.Unmarshal(raw, &file); err != nil {
		return nil, fmt.Errorf("decode %s: %w", path, err)
	}
	if len(file.Connections) == 0 {
		return nil, fmt.Errorf("%s: no connections to import", path)
	}

	results := make([]Result, 0, len(file.Connections))
	for i, in := range file.Connections {
		if in.Name == "" || in.Type == "" || len(in.Config) == 0 {
			return results, fmt.Errorf("%s: connections[%d]: name, type, and config are required", path, i)
		}
		res, err := upsert(svc, in)
		if err != nil {
			return results, fmt.Errorf("%s: connections[%d] %q (%s): %w", path, i, in.Name, in.Type, err)
		}
		results = append(results, res)
	}
	return results, nil
}

func upsert(svc *connection.Service, in connection.CreateInput) (Result, error) {
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
