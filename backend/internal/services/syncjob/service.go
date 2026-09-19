package syncjob

import (
	"encoding/json"
	"errors"
	"fmt"

	"github.com/portico/backend/internal/connectors"
	"github.com/portico/backend/internal/models"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

var (
	ErrNotFound = errors.New("sync job not found")
	ErrInvalid  = errors.New("invalid sync job")
)

type Service struct {
	db *gorm.DB
}

func NewService(db *gorm.DB) *Service {
	return &Service{db: db}
}

type RelationInput struct {
	Name       string `json:"name" binding:"required"`
	Type       string `json:"type" binding:"required"`
	Table      string `json:"table" binding:"required"`
	PivotTable string `json:"pivot_table"`
	ForeignKey string `json:"foreign_key"`
	RelatedKey string `json:"related_key"`
	Active     *bool  `json:"active"`
}

type FieldInput struct {
	SourceName        string          `json:"source_name" binding:"required"`
	DestinationName   string          `json:"destination_name"`
	DestinationType   string          `json:"destination_type"`
	DestinationConfig json.RawMessage `json:"destination_config" swaggertype:"object"`
	Active            *bool           `json:"active"`
}

type CreateInput struct {
	Name                    string          `json:"name" binding:"required"`
	SourceConnectionID      uint            `json:"source_connection_id" binding:"required"`
	DestinationConnectionID uint            `json:"destination_connection_id" binding:"required"`
	SourceTable             string          `json:"source_table" binding:"required"`
	DestinationTable        string          `json:"destination_table" binding:"required"`
	ChunkSize               int             `json:"chunk_size"`
	ParallelCount           int             `json:"parallel_count"`
	Config                  json.RawMessage `json:"config" swaggertype:"object"`
	Relations               []RelationInput `json:"relations"`
	Fields                  []FieldInput    `json:"fields"`
}

type UpdateInput struct {
	Name                    *string          `json:"name"`
	SourceConnectionID      *uint            `json:"source_connection_id"`
	DestinationConnectionID *uint            `json:"destination_connection_id"`
	SourceTable             *string          `json:"source_table"`
	DestinationTable        *string          `json:"destination_table"`
	ChunkSize               *int             `json:"chunk_size"`
	ParallelCount           *int             `json:"parallel_count"`
	Config                  json.RawMessage  `json:"config" swaggertype:"object"`
	Relations               *[]RelationInput `json:"relations"`
	Fields                  *[]FieldInput    `json:"fields"`
}

func (s *Service) List() ([]models.SyncJob, error) {
	var items []models.SyncJob
	if err := s.db.Preload("SourceConnection").Preload("DestinationConnection").Preload("Relations").Preload("Fields").Order("id asc").Find(&items).Error; err != nil {
		return nil, err
	}
	return items, nil
}

func (s *Service) Get(id uint) (*models.SyncJob, error) {
	var item models.SyncJob
	if err := s.db.Preload("SourceConnection").Preload("DestinationConnection").Preload("Relations").Preload("Fields").First(&item, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return &item, nil
}

func (s *Service) Create(in CreateInput) (*models.SyncJob, error) {
	chunk := in.ChunkSize
	if chunk <= 0 {
		chunk = 500
	}
	parallel := in.ParallelCount
	if parallel <= 0 {
		parallel = 2
	}
	relations, err := buildRelations(in.Relations)
	if err != nil {
		return nil, err
	}
	fields, err := buildFields(in.Fields)
	if err != nil {
		return nil, err
	}
	item := models.SyncJob{
		Name:                    in.Name,
		SourceConnectionID:      in.SourceConnectionID,
		DestinationConnectionID: in.DestinationConnectionID,
		SourceTable:             in.SourceTable,
		DestinationTable:        in.DestinationTable,
		ChunkSize:               chunk,
		ParallelCount:           parallel,
		Config:                  normalizeConfig(in.Config),
	}
	if err := s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&item).Error; err != nil {
			return err
		}
		if err := createRelations(tx, item.ID, relations); err != nil {
			return err
		}
		return createFields(tx, item.ID, fields)
	}); err != nil {
		return nil, err
	}
	return s.Get(item.ID)
}

func (s *Service) Update(id uint, in UpdateInput) (*models.SyncJob, error) {
	item, err := s.Get(id)
	if err != nil {
		return nil, err
	}
	if in.Name != nil {
		item.Name = *in.Name
	}
	if in.SourceConnectionID != nil {
		item.SourceConnectionID = *in.SourceConnectionID
	}
	if in.DestinationConnectionID != nil {
		item.DestinationConnectionID = *in.DestinationConnectionID
	}
	if in.SourceTable != nil {
		item.SourceTable = *in.SourceTable
	}
	if in.DestinationTable != nil {
		item.DestinationTable = *in.DestinationTable
	}
	if in.ChunkSize != nil {
		item.ChunkSize = *in.ChunkSize
	}
	if in.ParallelCount != nil {
		item.ParallelCount = *in.ParallelCount
	}
	if in.Config != nil {
		item.Config = normalizeConfig(in.Config)
	}

	err = s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Session(&gorm.Session{FullSaveAssociations: false}).Save(item).Error; err != nil {
			return err
		}
		if in.Relations != nil {
			relations, err := buildRelations(*in.Relations)
			if err != nil {
				return err
			}
			if err := tx.Where("sync_job_id = ?", id).Delete(&models.SyncJobRelation{}).Error; err != nil {
				return err
			}
			if err := createRelations(tx, id, relations); err != nil {
				return err
			}
		}
		if in.Fields != nil {
			fields, err := buildFields(*in.Fields)
			if err != nil {
				return err
			}
			if err := tx.Where("sync_job_id = ?", id).Delete(&models.SyncJobField{}).Error; err != nil {
				return err
			}
			if err := createFields(tx, id, fields); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return s.Get(id)
}

func (s *Service) Delete(id uint) error {
	res := s.db.Select("Relations", "Fields").Delete(&models.SyncJob{}, id)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return ErrNotFound
	}
	return nil
}

// createRelations / createFields insert with Select so active=false is persisted
// (GORM otherwise omits zero-value bools on Create).
func createRelations(tx *gorm.DB, syncJobID uint, relations []models.SyncJobRelation) error {
	if len(relations) == 0 {
		return nil
	}
	for i := range relations {
		relations[i].SyncJobID = syncJobID
	}
	return tx.Select(
		"SyncJobID",
		"Name",
		"Type",
		"Table",
		"PivotTable",
		"ForeignKey",
		"RelatedKey",
		"Active",
	).Create(&relations).Error
}

func createFields(tx *gorm.DB, syncJobID uint, fields []models.SyncJobField) error {
	if len(fields) == 0 {
		return nil
	}
	for i := range fields {
		fields[i].SyncJobID = syncJobID
	}
	return tx.Select(
		"SyncJobID",
		"SourceName",
		"DestinationName",
		"DestinationType",
		"DestinationConfig",
		"Active",
	).Create(&fields).Error
}

func normalizeConfig(raw json.RawMessage) datatypes.JSON {
	if len(raw) == 0 || string(raw) == "null" {
		return nil
	}
	return datatypes.JSON(raw)
}

func buildRelations(inputs []RelationInput) ([]models.SyncJobRelation, error) {
	if len(inputs) == 0 {
		return nil, nil
	}
	out := make([]models.SyncJobRelation, 0, len(inputs))
	for _, in := range inputs {
		if in.Type != models.RelationTypeBelongsToMany {
			return nil, fmt.Errorf("%w: unsupported relation type %q", ErrInvalid, in.Type)
		}
		if in.PivotTable == "" {
			return nil, fmt.Errorf("%w: pivot_table is required for relation %q", ErrInvalid, in.Name)
		}
		active := true
		if in.Active != nil {
			active = *in.Active
		}
		out = append(out, models.SyncJobRelation{
			Name:       in.Name,
			Type:       in.Type,
			Table:      in.Table,
			PivotTable: in.PivotTable,
			ForeignKey: in.ForeignKey,
			RelatedKey: in.RelatedKey,
			Active:     &active,
		})
	}
	return out, nil
}

func buildFields(inputs []FieldInput) ([]models.SyncJobField, error) {
	if len(inputs) == 0 {
		return nil, nil
	}
	out := make([]models.SyncJobField, 0, len(inputs))
	for _, in := range inputs {
		if in.SourceName == "" {
			return nil, fmt.Errorf("%w: source_name is required", ErrInvalid)
		}
		if in.DestinationType != "" && !validFieldType(in.DestinationType) {
			return nil, fmt.Errorf("%w: unsupported destination_type %q", ErrInvalid, in.DestinationType)
		}
		active := true
		if in.Active != nil {
			active = *in.Active
		}
		out = append(out, models.SyncJobField{
			SourceName:        in.SourceName,
			DestinationName:   in.DestinationName,
			DestinationType:   in.DestinationType,
			DestinationConfig: normalizeConfig(in.DestinationConfig),
			Active:            &active,
		})
	}
	return out, nil
}

func validFieldType(t string) bool {
	switch connectors.FieldType(t) {
	case connectors.FieldTypeString,
		connectors.FieldTypeInt64,
		connectors.FieldTypeFloat64,
		connectors.FieldTypeBool,
		connectors.FieldTypeObject,
		connectors.FieldTypeObjectArray:
		return true
	default:
		return false
	}
}
