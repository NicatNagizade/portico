package syncjob

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"

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

// RelationInput accepts positive ids for existing rows and negative ids as
// temporary client keys for same-request parent links (never written to the DB).
type RelationInput struct {
	ID         *int64          `json:"id"`
	ParentID   *int64          `json:"parent_id"`
	Name       string          `json:"name" binding:"required"`
	Type       string          `json:"type" binding:"required"`
	Table      string          `json:"table" binding:"required"`
	ForeignKey string          `json:"foreign_key"`
	RelatedKey string          `json:"related_key"`
	Config     json.RawMessage `json:"config" swaggertype:"object"`
	Active     *bool           `json:"active"`
}

type FieldInput struct {
	ID                *int64          `json:"id"`
	SyncJobRelationID *int64          `json:"sync_job_relation_id"`
	SourceName        string          `json:"source_name" binding:"required"`
	DestinationName   string          `json:"destination_name"`
	DestinationType   string          `json:"destination_type"`
	DestinationConfig json.RawMessage `json:"destination_config" swaggertype:"object"`
	Active            *bool           `json:"active"`
}

type RuleInput struct {
	ID       *int64 `json:"id"`
	Field    string `json:"field" binding:"required"`
	Operator string `json:"operator" binding:"required"`
	Value    string `json:"value"`
	Active   *bool  `json:"active"`
}

type CreateInput struct {
	Name                    string          `json:"name" binding:"required"`
	SourceConnectionID      uint            `json:"source_connection_id" binding:"required"`
	SourceTable             string          `json:"source_table" binding:"required"`
	DestinationConnectionID uint            `json:"destination_connection_id" binding:"required"`
	DestinationTable        string          `json:"destination_table" binding:"required"`
	ChunkSize               int             `json:"chunk_size"`
	Workers                 int             `json:"workers"`
	Config                  json.RawMessage `json:"config" swaggertype:"object"`
	Relations               []RelationInput `json:"relations"`
	Fields                  []FieldInput    `json:"fields"`
	Rules                   []RuleInput     `json:"rules"`
}

type UpdateInput struct {
	Name                    *string          `json:"name"`
	SourceConnectionID      *uint            `json:"source_connection_id"`
	SourceTable             *string          `json:"source_table"`
	DestinationConnectionID *uint            `json:"destination_connection_id"`
	DestinationTable        *string          `json:"destination_table"`
	ChunkSize               *int             `json:"chunk_size"`
	Workers                 *int             `json:"workers"`
	Config                  json.RawMessage  `json:"config" swaggertype:"object"`
	Relations               *[]RelationInput `json:"relations"`
	Fields                  *[]FieldInput    `json:"fields"`
	Rules                   *[]RuleInput     `json:"rules"`
}

func (s *Service) List(page, pageSize int) ([]models.SyncJob, int64, error) {
	var total int64
	if err := s.db.Model(&models.SyncJob{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var items []models.SyncJob
	q := s.db.Preload("SourceConnection").Preload("DestinationConnection").
		Order("id asc").
		Offset((page - 1) * pageSize).
		Limit(pageSize)
	if err := q.Find(&items).Error; err != nil {
		return nil, 0, err
	}
	return items, total, nil
}

func (s *Service) Get(id uint) (*models.SyncJob, error) {
	var item models.SyncJob
	err := s.db.
		Preload("SourceConnection").
		Preload("DestinationConnection").
		Preload("Relations").
		Preload("Relations.Fields").
		Preload("Fields", "sync_job_relation_id IS NULL").
		Preload("Rules").
		First(&item, id).Error
	if err != nil {
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
	workers := in.Workers
	if workers <= 0 {
		workers = 2
	}
	item := models.SyncJob{
		Name:                    in.Name,
		SourceConnectionID:      in.SourceConnectionID,
		SourceTable:             in.SourceTable,
		DestinationConnectionID: in.DestinationConnectionID,
		DestinationTable:        in.DestinationTable,
		ChunkSize:               chunk,
		Workers:                 workers,
		Config:                  normalizeConfig(in.Config),
	}
	if err := s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&item).Error; err != nil {
			return err
		}
		idMap, err := upsertRelations(tx, item.ID, in.Relations)
		if err != nil {
			return err
		}
		if err := upsertFields(tx, item.ID, in.Fields, idMap); err != nil {
			return err
		}
		return upsertRules(tx, item.ID, in.Rules)
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
	if in.SourceTable != nil {
		item.SourceTable = *in.SourceTable
	}
	if in.DestinationConnectionID != nil {
		item.DestinationConnectionID = *in.DestinationConnectionID
	}
	if in.DestinationTable != nil {
		item.DestinationTable = *in.DestinationTable
	}
	if in.ChunkSize != nil {
		item.ChunkSize = *in.ChunkSize
	}
	if in.Workers != nil {
		item.Workers = *in.Workers
	}
	if in.Config != nil {
		item.Config = normalizeConfig(in.Config)
	}

	err = s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Session(&gorm.Session{FullSaveAssociations: false}).Save(item).Error; err != nil {
			return err
		}
		var idMap map[int64]uint
		if in.Relations != nil {
			idMap, err = upsertRelations(tx, id, *in.Relations)
			if err != nil {
				return err
			}
		} else {
			idMap, err = existingRelationIDMap(tx, id)
			if err != nil {
				return err
			}
		}
		if in.Fields != nil {
			if err := upsertFields(tx, id, *in.Fields, idMap); err != nil {
				return err
			}
		}
		if in.Rules != nil {
			if err := upsertRules(tx, id, *in.Rules); err != nil {
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
	res := s.db.Select("Relations", "Fields", "Rules", "Logs").Delete(&models.SyncJob{ID: id})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return ErrNotFound
	}
	return nil
}

func normalizeConfig(raw json.RawMessage) datatypes.JSON {
	if len(raw) == 0 || string(raw) == "null" {
		return nil
	}
	return datatypes.JSON(raw)
}

func pivotTableFromConfig(cfg datatypes.JSON) string {
	if len(cfg) == 0 {
		return ""
	}
	var m map[string]any
	if err := json.Unmarshal(cfg, &m); err != nil {
		return ""
	}
	v, _ := m["pivot_table"].(string)
	return v
}

func existingRelationIDMap(tx *gorm.DB, jobID uint) (map[int64]uint, error) {
	var rels []models.SyncJobRelation
	if err := tx.Where("sync_job_id = ?", jobID).Find(&rels).Error; err != nil {
		return nil, err
	}
	out := make(map[int64]uint, len(rels))
	for _, r := range rels {
		out[int64(r.ID)] = r.ID
	}
	return out, nil
}

func upsertRelations(tx *gorm.DB, jobID uint, inputs []RelationInput) (map[int64]uint, error) {
	idMap := make(map[int64]uint)
	if err := validateRelationInputs(inputs); err != nil {
		return nil, err
	}

	var existing []models.SyncJobRelation
	if err := tx.Where("sync_job_id = ?", jobID).Find(&existing).Error; err != nil {
		return nil, err
	}
	existingByID := make(map[uint]models.SyncJobRelation, len(existing))
	for _, r := range existing {
		existingByID[r.ID] = r
		idMap[int64(r.ID)] = r.ID
	}

	keep := make(map[uint]struct{})
	type pendingParent struct {
		relationID uint
		parentRef  int64
	}
	var parents []pendingParent

	for _, in := range inputs {
		active := true
		if in.Active != nil {
			active = *in.Active
		}
		cfg := normalizeConfig(in.Config)
		rel := models.SyncJobRelation{
			SyncJobID:  jobID,
			Name:       in.Name,
			Type:       in.Type,
			Table:      in.Table,
			ForeignKey: in.ForeignKey,
			RelatedKey: in.RelatedKey,
			Config:     cfg,
			Active:     &active,
		}

		clientKey := int64(0)
		if in.ID != nil {
			clientKey = *in.ID
		}

		if clientKey > 0 {
			prev, ok := existingByID[uint(clientKey)]
			if !ok {
				return nil, fmt.Errorf("%w: relation id %d not found", ErrInvalid, clientKey)
			}
			rel.ID = prev.ID
			rel.ParentID = nil
			if err := tx.Select(
				"Name", "Type", "Table", "ForeignKey", "RelatedKey", "Config", "Active", "ParentID",
			).Save(&rel).Error; err != nil {
				return nil, err
			}
			keep[rel.ID] = struct{}{}
			idMap[clientKey] = rel.ID
		} else {
			if err := tx.Select(
				"SyncJobID", "Name", "Type", "Table", "ForeignKey", "RelatedKey", "Config", "Active",
			).Create(&rel).Error; err != nil {
				return nil, err
			}
			keep[rel.ID] = struct{}{}
			if clientKey < 0 {
				idMap[clientKey] = rel.ID
			}
			idMap[int64(rel.ID)] = rel.ID
		}

		if in.ParentID != nil {
			parents = append(parents, pendingParent{relationID: rel.ID, parentRef: *in.ParentID})
		}
	}

	for _, p := range parents {
		realParent, ok := idMap[p.parentRef]
		if !ok {
			return nil, fmt.Errorf("%w: parent_id %d not found", ErrInvalid, p.parentRef)
		}
		if realParent == p.relationID {
			return nil, fmt.Errorf("%w: relation cannot parent itself", ErrInvalid)
		}
		if err := tx.Model(&models.SyncJobRelation{}).
			Where("id = ? AND sync_job_id = ?", p.relationID, jobID).
			Update("parent_id", realParent).Error; err != nil {
			return nil, err
		}
	}

	if err := validateStoredRelationParents(tx, jobID); err != nil {
		return nil, err
	}

	for id := range existingByID {
		if _, ok := keep[id]; ok {
			continue
		}
		if err := tx.Delete(&models.SyncJobRelation{}, id).Error; err != nil {
			return nil, err
		}
	}
	return idMap, nil
}

func validateRelationInputs(inputs []RelationInput) error {
	if len(inputs) == 0 {
		return nil
	}
	names := make(map[string]struct{}, len(inputs))
	tempIDs := make(map[int64]struct{})
	for _, in := range inputs {
		if in.Name == "" {
			return fmt.Errorf("%w: relation name is required", ErrInvalid)
		}
		if _, exists := names[in.Name]; exists {
			return fmt.Errorf("%w: duplicate relation name %q", ErrInvalid, in.Name)
		}
		names[in.Name] = struct{}{}
		if in.ID != nil && *in.ID < 0 {
			if _, dup := tempIDs[*in.ID]; dup {
				return fmt.Errorf("%w: duplicate temporary relation id %d", ErrInvalid, *in.ID)
			}
			tempIDs[*in.ID] = struct{}{}
		}
		switch in.Type {
		case models.RelationTypeBelongsToMany:
			if pivotTableFromConfig(normalizeConfig(in.Config)) == "" {
				return fmt.Errorf("%w: config.pivot_table is required for relation %q", ErrInvalid, in.Name)
			}
		case models.RelationTypeHasMany, models.RelationTypeHasOne, models.RelationTypeBelongsTo:
			// ok
		default:
			return fmt.Errorf("%w: unsupported relation type %q", ErrInvalid, in.Type)
		}
	}
	return nil
}

func validateStoredRelationParents(tx *gorm.DB, jobID uint) error {
	var rels []models.SyncJobRelation
	if err := tx.Where("sync_job_id = ?", jobID).Find(&rels).Error; err != nil {
		return err
	}
	byID := make(map[uint]models.SyncJobRelation, len(rels))
	for _, r := range rels {
		byID[r.ID] = r
	}
	for _, r := range rels {
		seen := map[uint]struct{}{r.ID: {}}
		cur := r.ParentID
		for cur != nil {
			if _, loop := seen[*cur]; loop {
				return fmt.Errorf("%w: cyclic parent_id involving relation %q", ErrInvalid, r.Name)
			}
			parent, ok := byID[*cur]
			if !ok {
				return fmt.Errorf("%w: parent_id %d not found for relation %q", ErrInvalid, *cur, r.Name)
			}
			seen[*cur] = struct{}{}
			cur = parent.ParentID
		}
	}
	return nil
}

func upsertFields(tx *gorm.DB, jobID uint, inputs []FieldInput, relationIDMap map[int64]uint) error {
	var existing []models.SyncJobField
	if err := tx.Where("sync_job_id = ?", jobID).Find(&existing).Error; err != nil {
		return err
	}
	existingByID := make(map[uint]models.SyncJobField, len(existing))
	for _, f := range existing {
		existingByID[f.ID] = f
	}
	keep := make(map[uint]struct{})

	for _, in := range inputs {
		if in.SourceName == "" {
			return fmt.Errorf("%w: source_name is required", ErrInvalid)
		}
		if in.DestinationType != "" && !validFieldType(in.DestinationType) {
			return fmt.Errorf("%w: unsupported destination_type %q", ErrInvalid, in.DestinationType)
		}
		active := true
		if in.Active != nil {
			active = *in.Active
		}
		var relID *uint
		if in.SyncJobRelationID != nil {
			real, ok := relationIDMap[*in.SyncJobRelationID]
			if !ok {
				return fmt.Errorf("%w: sync_job_relation_id %d not found", ErrInvalid, *in.SyncJobRelationID)
			}
			relID = &real
		}
		field := models.SyncJobField{
			SyncJobID:         jobID,
			SyncJobRelationID: relID,
			SourceName:        in.SourceName,
			DestinationName:   in.DestinationName,
			DestinationType:   in.DestinationType,
			DestinationConfig: normalizeConfig(in.DestinationConfig),
			Active:            &active,
		}

		clientKey := int64(0)
		if in.ID != nil {
			clientKey = *in.ID
		}
		if clientKey > 0 {
			prev, ok := existingByID[uint(clientKey)]
			if !ok {
				return fmt.Errorf("%w: field id %d not found", ErrInvalid, clientKey)
			}
			field.ID = prev.ID
			if err := tx.Select(
				"SyncJobRelationID", "SourceName", "DestinationName", "DestinationType", "DestinationConfig", "Active",
			).Save(&field).Error; err != nil {
				return err
			}
			keep[field.ID] = struct{}{}
		} else {
			if err := tx.Select(
				"SyncJobID", "SyncJobRelationID", "SourceName", "DestinationName", "DestinationType", "DestinationConfig", "Active",
			).Create(&field).Error; err != nil {
				return err
			}
			keep[field.ID] = struct{}{}
		}
	}

	for id := range existingByID {
		if _, ok := keep[id]; ok {
			continue
		}
		if err := tx.Delete(&models.SyncJobField{}, id).Error; err != nil {
			return err
		}
	}
	return nil
}

func upsertRules(tx *gorm.DB, jobID uint, inputs []RuleInput) error {
	var existing []models.SyncJobRule
	if err := tx.Where("sync_job_id = ?", jobID).Find(&existing).Error; err != nil {
		return err
	}
	existingByID := make(map[uint]models.SyncJobRule, len(existing))
	for _, r := range existing {
		existingByID[r.ID] = r
	}
	keep := make(map[uint]struct{})

	for _, in := range inputs {
		rule, err := ruleFromInput(jobID, in)
		if err != nil {
			return err
		}

		clientKey := int64(0)
		if in.ID != nil {
			clientKey = *in.ID
		}
		if clientKey > 0 {
			prev, ok := existingByID[uint(clientKey)]
			if !ok {
				return fmt.Errorf("%w: rule id %d not found", ErrInvalid, clientKey)
			}
			rule.ID = prev.ID
			if err := tx.Select("Field", "Operator", "Value", "Active").Save(&rule).Error; err != nil {
				return err
			}
		} else {
			if err := tx.Select("SyncJobID", "Field", "Operator", "Value", "Active").Create(&rule).Error; err != nil {
				return err
			}
		}
		keep[rule.ID] = struct{}{}
	}

	for id := range existingByID {
		if _, ok := keep[id]; ok {
			continue
		}
		if err := tx.Delete(&models.SyncJobRule{}, id).Error; err != nil {
			return err
		}
	}
	return nil
}

func ruleFromInput(jobID uint, in RuleInput) (models.SyncJobRule, error) {
	field := strings.TrimSpace(in.Field)
	if field == "" {
		return models.SyncJobRule{}, fmt.Errorf("%w: rule field is required", ErrInvalid)
	}
	if !models.ValidRuleOperator(in.Operator) {
		return models.SyncJobRule{}, fmt.Errorf("%w: unsupported rule operator %q", ErrInvalid, in.Operator)
	}
	value := in.Value
	if models.RuleNeedsValue(in.Operator) {
		if strings.TrimSpace(value) == "" {
			return models.SyncJobRule{}, fmt.Errorf("%w: rule value is required for operator %q", ErrInvalid, in.Operator)
		}
	} else {
		value = ""
	}
	active := true
	if in.Active != nil {
		active = *in.Active
	}
	return models.SyncJobRule{
		SyncJobID: jobID,
		Field:     field,
		Operator:  in.Operator,
		Value:     value,
		Active:    &active,
	}, nil
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
