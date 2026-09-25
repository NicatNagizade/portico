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

// RelationInput accepts positive ids for existing rows. Nest children under
// Relations and field overrides under Fields (same shape as root fields).
type RelationInput struct {
	ID         *int64          `json:"id"`
	Name       string          `json:"name" binding:"required"`
	Type       string          `json:"type" binding:"required"`
	Table      string          `json:"table" binding:"required"`
	ForeignKey string          `json:"foreign_key"`
	RelatedKey string          `json:"related_key"`
	Config     json.RawMessage `json:"config" swaggertype:"object"`
	Active     *bool           `json:"active"`
	Fields     []FieldInput    `json:"fields"`
	Relations  []RelationInput `json:"relations"`
}

type FieldValueInput struct {
	ID               *int64 `json:"id"`
	SourceValue      string `json:"source_value" binding:"required"`
	DestinationValue string `json:"destination_value" binding:"required"`
}

type FieldInput struct {
	ID                *int64            `json:"id"`
	SyncJobRelationID *int64            `json:"sync_job_relation_id"`
	Relation          string            `json:"relation"`
	SourceName        string            `json:"source_name" binding:"required"`
	DestinationName   string            `json:"destination_name"`
	DestinationType   string            `json:"destination_type"`
	DestinationConfig json.RawMessage   `json:"destination_config" swaggertype:"object"`
	Active            *bool             `json:"active"`
	Values            []FieldValueInput `json:"values"`
}

// relationMaps resolves parent/field links by DB id or relation name.
type relationMaps struct {
	byID   map[int64]uint
	byName map[string]uint
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
		Preload("Relations.Fields.Values").
		Preload("Fields", "sync_job_relation_id IS NULL").
		Preload("Fields.Values").
		Preload("Rules").
		First(&item, id).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	item.Relations = NestRelations(item.Relations)
	return &item, nil
}

// NestRelations turns a flat parent_id list into a tree under Relations.
func NestRelations(flat []models.SyncJobRelation) []models.SyncJobRelation {
	if len(flat) == 0 {
		return flat
	}
	children := make(map[uint][]models.SyncJobRelation, len(flat))
	roots := make([]models.SyncJobRelation, 0, len(flat))
	for _, r := range flat {
		r.Relations = nil
		if r.ParentID == nil {
			roots = append(roots, r)
			continue
		}
		children[*r.ParentID] = append(children[*r.ParentID], r)
	}
	var attach func(models.SyncJobRelation) models.SyncJobRelation
	attach = func(rel models.SyncJobRelation) models.SyncJobRelation {
		kids := children[rel.ID]
		if len(kids) == 0 {
			return rel
		}
		rel.Relations = make([]models.SyncJobRelation, 0, len(kids))
		for _, k := range kids {
			rel.Relations = append(rel.Relations, attach(k))
		}
		return rel
	}
	out := make([]models.SyncJobRelation, 0, len(roots))
	for _, r := range roots {
		out = append(out, attach(r))
	}
	return out
}

// FindByName returns the single sync job with the given name.
// ErrNotFound if none; ErrInvalid if more than one.
func (s *Service) FindByName(name string) (*models.SyncJob, error) {
	var items []models.SyncJob
	if err := s.db.Where("name = ?", name).Find(&items).Error; err != nil {
		return nil, err
	}
	switch len(items) {
	case 0:
		return nil, ErrNotFound
	case 1:
		return s.Get(items[0].ID)
	default:
		return nil, fmt.Errorf("%w: multiple sync jobs named %q", ErrInvalid, name)
	}
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
		rels, err := upsertRelations(tx, item.ID, in.Relations)
		if err != nil {
			return err
		}
		fields := append(append([]FieldInput{}, in.Fields...), fieldsFromRelations(in.Relations)...)
		if err := upsertFields(tx, item.ID, fields, rels); err != nil {
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
		var rels relationMaps
		var err error
		if in.Relations != nil {
			rels, err = upsertRelations(tx, id, *in.Relations)
			if err != nil {
				return err
			}
		} else {
			rels, err = existingRelationMaps(tx, id)
			if err != nil {
				return err
			}
		}
		if in.Fields != nil || in.Relations != nil {
			fields, err := mergeFieldInputs(tx, id, in.Fields, in.Relations, rels)
			if err != nil {
				return err
			}
			if err := upsertFields(tx, id, fields, rels); err != nil {
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

// fieldsFromRelations flattens nested relation.fields into FieldInputs scoped by relation name.
func fieldsFromRelations(inputs []RelationInput) []FieldInput {
	var out []FieldInput
	var walk func([]RelationInput)
	walk = func(nodes []RelationInput) {
		for _, n := range nodes {
			for _, f := range n.Fields {
				out = append(out, FieldInput{
					ID:                f.ID,
					Relation:          n.Name,
					SourceName:        f.SourceName,
					DestinationName:   f.DestinationName,
					DestinationType:   f.DestinationType,
					DestinationConfig: f.DestinationConfig,
					Active:            f.Active,
					Values:            f.Values,
				})
			}
			walk(n.Relations)
		}
	}
	walk(inputs)
	return out
}

// mergeFieldInputs builds the full field list for an update.
// Root fields come from inFields (or existing root rows when nil).
// Relation-scoped fields come from nested relation.fields (or existing when Relations is nil).
func mergeFieldInputs(
	tx *gorm.DB,
	jobID uint,
	inFields *[]FieldInput,
	inRelations *[]RelationInput,
	rels relationMaps,
) ([]FieldInput, error) {
	var out []FieldInput
	if inFields != nil {
		out = append(out, *inFields...)
	} else {
		root, err := loadFieldInputs(tx, jobID, true)
		if err != nil {
			return nil, err
		}
		out = append(out, root...)
	}
	if inRelations != nil {
		out = append(out, fieldsFromRelations(*inRelations)...)
	} else {
		scoped, err := loadFieldInputs(tx, jobID, false)
		if err != nil {
			return nil, err
		}
		// Re-attach relation names so upsertFields can resolve them.
		nameByID := make(map[uint]string, len(rels.byName))
		for name, id := range rels.byName {
			nameByID[id] = name
		}
		for i := range scoped {
			if scoped[i].SyncJobRelationID == nil {
				continue
			}
			if name, ok := nameByID[uint(*scoped[i].SyncJobRelationID)]; ok {
				scoped[i].Relation = name
				scoped[i].SyncJobRelationID = nil
			}
		}
		out = append(out, scoped...)
	}
	return out, nil
}

func loadFieldInputs(tx *gorm.DB, jobID uint, rootOnly bool) ([]FieldInput, error) {
	q := tx.Preload("Values").Where("sync_job_id = ?", jobID)
	if rootOnly {
		q = q.Where("sync_job_relation_id IS NULL")
	} else {
		q = q.Where("sync_job_relation_id IS NOT NULL")
	}
	var rows []models.SyncJobField
	if err := q.Find(&rows).Error; err != nil {
		return nil, err
	}
	out := make([]FieldInput, 0, len(rows))
	for _, f := range rows {
		id := int64(f.ID)
		in := FieldInput{
			ID:                &id,
			SourceName:        f.SourceName,
			DestinationName:   f.DestinationName,
			DestinationType:   f.DestinationType,
			DestinationConfig: json.RawMessage(f.DestinationConfig),
			Active:            f.Active,
		}
		if f.SyncJobRelationID != nil {
			relID := int64(*f.SyncJobRelationID)
			in.SyncJobRelationID = &relID
		}
		if len(f.Values) > 0 {
			in.Values = make([]FieldValueInput, 0, len(f.Values))
			for _, v := range f.Values {
				vid := int64(v.ID)
				in.Values = append(in.Values, FieldValueInput{
					ID:               &vid,
					SourceValue:      v.SourceValue,
					DestinationValue: v.DestinationValue,
				})
			}
		}
		out = append(out, in)
	}
	return out, nil
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

func existingRelationMaps(tx *gorm.DB, jobID uint) (relationMaps, error) {
	var rels []models.SyncJobRelation
	if err := tx.Where("sync_job_id = ?", jobID).Find(&rels).Error; err != nil {
		return relationMaps{}, err
	}
	out := relationMaps{
		byID:   make(map[int64]uint, len(rels)),
		byName: make(map[string]uint, len(rels)),
	}
	for _, r := range rels {
		out.byID[int64(r.ID)] = r.ID
		out.byName[r.Name] = r.ID
	}
	return out, nil
}

func upsertRelations(tx *gorm.DB, jobID uint, inputs []RelationInput) (relationMaps, error) {
	maps := relationMaps{
		byID:   make(map[int64]uint),
		byName: make(map[string]uint),
	}
	if err := validateRelationInputs(inputs); err != nil {
		return relationMaps{}, err
	}

	var existing []models.SyncJobRelation
	if err := tx.Where("sync_job_id = ?", jobID).Find(&existing).Error; err != nil {
		return relationMaps{}, err
	}
	existingByID := make(map[uint]models.SyncJobRelation, len(existing))
	for _, r := range existing {
		existingByID[r.ID] = r
		maps.byID[int64(r.ID)] = r.ID
	}

	keep := make(map[uint]struct{})
	if err := upsertRelationNodes(tx, jobID, nil, inputs, existingByID, maps, keep); err != nil {
		return relationMaps{}, err
	}

	if err := validateStoredRelationParents(tx, jobID); err != nil {
		return relationMaps{}, err
	}

	for id := range existingByID {
		if _, ok := keep[id]; ok {
			continue
		}
		if err := tx.Delete(&models.SyncJobRelation{}, id).Error; err != nil {
			return relationMaps{}, err
		}
	}
	return maps, nil
}

func upsertRelationNodes(
	tx *gorm.DB,
	jobID uint,
	parentID *uint,
	inputs []RelationInput,
	existingByID map[uint]models.SyncJobRelation,
	maps relationMaps,
	keep map[uint]struct{},
) error {
	for _, in := range inputs {
		active := true
		if in.Active != nil {
			active = *in.Active
		}
		cfg := normalizeConfig(in.Config)
		rel := models.SyncJobRelation{
			SyncJobID:  jobID,
			ParentID:   parentID,
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
		if clientKey < 0 {
			return fmt.Errorf("%w: relation id must be a positive existing id", ErrInvalid)
		}

		if clientKey > 0 {
			prev, ok := existingByID[uint(clientKey)]
			if !ok {
				return fmt.Errorf("%w: relation id %d not found", ErrInvalid, clientKey)
			}
			rel.ID = prev.ID
			if err := tx.Select(
				"Name", "Type", "Table", "ForeignKey", "RelatedKey", "Config", "Active", "ParentID",
			).Save(&rel).Error; err != nil {
				return err
			}
			keep[rel.ID] = struct{}{}
			maps.byID[clientKey] = rel.ID
		} else {
			if err := tx.Select(
				"SyncJobID", "ParentID", "Name", "Type", "Table", "ForeignKey", "RelatedKey", "Config", "Active",
			).Create(&rel).Error; err != nil {
				return err
			}
			keep[rel.ID] = struct{}{}
			maps.byID[int64(rel.ID)] = rel.ID
		}
		maps.byName[rel.Name] = rel.ID

		if err := upsertRelationNodes(tx, jobID, &rel.ID, in.Relations, existingByID, maps, keep); err != nil {
			return err
		}
	}
	return nil
}

func validateRelationInputs(inputs []RelationInput) error {
	names := make(map[string]struct{})
	var walk func([]RelationInput) error
	walk = func(nodes []RelationInput) error {
		for _, in := range nodes {
			if in.Name == "" {
				return fmt.Errorf("%w: relation name is required", ErrInvalid)
			}
			if _, exists := names[in.Name]; exists {
				return fmt.Errorf("%w: duplicate relation name %q", ErrInvalid, in.Name)
			}
			names[in.Name] = struct{}{}
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
			if err := walk(in.Relations); err != nil {
				return err
			}
		}
		return nil
	}
	return walk(inputs)
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

func upsertFields(tx *gorm.DB, jobID uint, inputs []FieldInput, rels relationMaps) error {
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
		if in.Relation != "" && in.SyncJobRelationID != nil {
			return fmt.Errorf("%w: set relation or sync_job_relation_id, not both", ErrInvalid)
		}
		active := true
		if in.Active != nil {
			active = *in.Active
		}
		var relID *uint
		if in.Relation != "" {
			real, ok := rels.byName[in.Relation]
			if !ok {
				return fmt.Errorf("%w: relation %q not found", ErrInvalid, in.Relation)
			}
			relID = &real
		} else if in.SyncJobRelationID != nil {
			if *in.SyncJobRelationID <= 0 {
				return fmt.Errorf("%w: sync_job_relation_id must be a positive existing id (use relation by name)", ErrInvalid)
			}
			real, ok := rels.byID[*in.SyncJobRelationID]
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
		if err := upsertFieldValues(tx, field.ID, in.Values); err != nil {
			return err
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

func upsertFieldValues(tx *gorm.DB, fieldID uint, inputs []FieldValueInput) error {
	var existing []models.SyncJobFieldValue
	if err := tx.Where("sync_job_field_id = ?", fieldID).Find(&existing).Error; err != nil {
		return err
	}
	existingByID := make(map[uint]models.SyncJobFieldValue, len(existing))
	for _, v := range existing {
		existingByID[v.ID] = v
	}
	keep := make(map[uint]struct{})
	seenSource := make(map[string]struct{}, len(inputs))

	for _, in := range inputs {
		src := strings.TrimSpace(in.SourceValue)
		if src == "" {
			return fmt.Errorf("%w: field value source_value is required", ErrInvalid)
		}
		if _, dup := seenSource[src]; dup {
			return fmt.Errorf("%w: duplicate field value source_value %q", ErrInvalid, src)
		}
		seenSource[src] = struct{}{}

		row := models.SyncJobFieldValue{
			SyncJobFieldID:   fieldID,
			SourceValue:      src,
			DestinationValue: in.DestinationValue,
		}

		clientKey := int64(0)
		if in.ID != nil {
			clientKey = *in.ID
		}
		if clientKey > 0 {
			prev, ok := existingByID[uint(clientKey)]
			if !ok {
				return fmt.Errorf("%w: field value id %d not found", ErrInvalid, clientKey)
			}
			row.ID = prev.ID
			if err := tx.Select("SourceValue", "DestinationValue").Save(&row).Error; err != nil {
				return err
			}
		} else {
			if err := tx.Select("SyncJobFieldID", "SourceValue", "DestinationValue").Create(&row).Error; err != nil {
				return err
			}
		}
		keep[row.ID] = struct{}{}
	}

	for id := range existingByID {
		if _, ok := keep[id]; ok {
			continue
		}
		if err := tx.Delete(&models.SyncJobFieldValue{}, id).Error; err != nil {
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
