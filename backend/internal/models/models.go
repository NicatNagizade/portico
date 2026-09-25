package models

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/portico/backend/internal/secretbox"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

const (
	ConnectionTypeMySQL     = "mysql"
	ConnectionTypePostgres  = "postgres"
	ConnectionTypeTypesense = "typesense"
	ConnectionTypeMongoDB   = "mongodb"
	ConnectionTypeSQLite    = "sqlite"
	ConnectionTypeRedis     = "redis"

	SyncLogStatusRunning = "running"
	SyncLogStatusSuccess = "success"
	SyncLogStatusFailed  = "failed"
	SyncLogStatusStopped = "stopped"

	RelationTypeBelongsToMany = "belongs_to_many"
	RelationTypeHasMany       = "has_many"
	RelationTypeHasOne        = "has_one"
	RelationTypeBelongsTo     = "belongs_to"

	RuleOperatorEq        = "eq"
	RuleOperatorNeq       = "neq"
	RuleOperatorGt        = "gt"
	RuleOperatorGte       = "gte"
	RuleOperatorLt        = "lt"
	RuleOperatorLte       = "lte"
	RuleOperatorIn        = "in"
	RuleOperatorNotIn     = "not_in"
	RuleOperatorLike      = "like"
	RuleOperatorIsNull    = "is_null"
	RuleOperatorIsNotNull = "is_not_null"
)

// ValidRuleOperator reports whether op is a supported SyncJobRule.operator.
func ValidRuleOperator(op string) bool {
	switch op {
	case RuleOperatorEq, RuleOperatorNeq,
		RuleOperatorGt, RuleOperatorGte, RuleOperatorLt, RuleOperatorLte,
		RuleOperatorIn, RuleOperatorNotIn, RuleOperatorLike,
		RuleOperatorIsNull, RuleOperatorIsNotNull:
		return true
	default:
		return false
	}
}

// RuleNeedsValue is false for null-check operators (value is ignored).
func RuleNeedsValue(op string) bool {
	return op != RuleOperatorIsNull && op != RuleOperatorIsNotNull
}

type Connection struct {
	ID        uint           `json:"id" gorm:"primaryKey"`
	Name      string         `json:"name" gorm:"size:255;not null"`
	Type      string         `json:"type" gorm:"size:50;not null;index"`
	Config    datatypes.JSON `json:"config" gorm:"type:json;not null" swaggertype:"object"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
}

// BeforeSave encrypts secret config fields so the connections table does not store them in plaintext.
func (c *Connection) BeforeSave(*gorm.DB) error {
	sealed, err := secretbox.Seal(c.Config)
	if err != nil {
		return fmt.Errorf("seal connection config: %w", err)
	}
	c.Config = sealed
	return nil
}

// AfterFind restores secret config fields for connectors. API responses redact them in MarshalJSON.
func (c *Connection) AfterFind(*gorm.DB) error {
	return c.openConfig()
}

// AfterSave restores the in-memory config after the sealed copy is written.
func (c *Connection) AfterSave(*gorm.DB) error {
	return c.openConfig()
}

func (c *Connection) openConfig() error {
	opened, err := secretbox.Open(c.Config)
	if err != nil {
		return fmt.Errorf("open connection config: %w", err)
	}
	c.Config = opened
	return nil
}

// MarshalJSON omits secret config values from API responses.
func (c Connection) MarshalJSON() ([]byte, error) {
	redacted, err := secretbox.Redact(c.Config)
	if err != nil {
		return nil, err
	}
	type alias Connection
	out := alias(c)
	out.Config = redacted
	return json.Marshal(out)
}

type SyncJob struct {
	ID                      uint              `json:"id" gorm:"primaryKey"`
	Name                    string            `json:"name" gorm:"size:255;not null"`
	SourceConnectionID      uint              `json:"source_connection_id" gorm:"not null;index"`
	SourceTable             string            `json:"source_table" gorm:"size:255;not null"`
	DestinationConnectionID uint              `json:"destination_connection_id" gorm:"not null;index"`
	DestinationTable        string            `json:"destination_table" gorm:"size:255;not null"`
	ChunkSize               int               `json:"chunk_size" gorm:"not null;default:500"`
	Workers                 int               `json:"workers" gorm:"not null;default:2"`
	Config                  datatypes.JSON    `json:"config,omitempty" gorm:"type:json" swaggertype:"object"`
	SourceConnection        *Connection       `json:"source_connection,omitempty" gorm:"foreignKey:SourceConnectionID"`
	DestinationConnection   *Connection       `json:"destination_connection,omitempty" gorm:"foreignKey:DestinationConnectionID"`
	Relations               []SyncJobRelation `json:"relations,omitempty" gorm:"foreignKey:SyncJobID;constraint:OnDelete:CASCADE"`
	Fields                  []SyncJobField    `json:"fields,omitempty" gorm:"foreignKey:SyncJobID;constraint:OnDelete:CASCADE"`
	Rules                   []SyncJobRule     `json:"rules,omitempty" gorm:"foreignKey:SyncJobID;constraint:OnDelete:CASCADE"`
	Logs                    []SyncLog         `json:"-" gorm:"foreignKey:SyncJobID;constraint:OnDelete:CASCADE"`
	CreatedAt               time.Time         `json:"created_at"`
	UpdatedAt               time.Time         `json:"updated_at"`
}

type SyncJobRelation struct {
	ID         uint              `json:"id" gorm:"primaryKey"`
	SyncJobID  uint              `json:"sync_job_id" gorm:"not null;index"`
	ParentID   *uint             `json:"-" gorm:"index"` // nesting stored flat; API nests under Relations
	Name       string            `json:"name" gorm:"size:255;not null"`
	Type       string            `json:"type" gorm:"size:50;not null"`
	Table      string            `json:"table" gorm:"size:255;not null"`
	ForeignKey string            `json:"foreign_key" gorm:"size:255"`
	RelatedKey string            `json:"related_key" gorm:"size:255"`
	Config     datatypes.JSON    `json:"config,omitempty" gorm:"type:json" swaggertype:"object"`
	Active     *bool             `json:"active" gorm:"not null;default:true"`
	Fields     []SyncJobField    `json:"fields,omitempty" gorm:"foreignKey:SyncJobRelationID;constraint:OnDelete:CASCADE"`
	Relations  []SyncJobRelation `json:"relations,omitempty" gorm:"-"`
	CreatedAt  time.Time         `json:"created_at"`
	UpdatedAt  time.Time         `json:"updated_at"`
}

func (r SyncJobRelation) IsActive() bool {
	return r.Active == nil || *r.Active
}

type SyncJobField struct {
	ID                uint                `json:"id" gorm:"primaryKey"`
	SyncJobID         uint                `json:"sync_job_id" gorm:"not null;index"`
	SyncJobRelationID *uint               `json:"sync_job_relation_id,omitempty" gorm:"index"`
	SourceName        string              `json:"source_name" gorm:"size:255;not null"`
	DestinationName   string              `json:"destination_name" gorm:"size:255"`
	DestinationType   string              `json:"destination_type" gorm:"size:50"`
	DestinationConfig datatypes.JSON      `json:"destination_config,omitempty" gorm:"type:json" swaggertype:"object"`
	Active            *bool               `json:"active" gorm:"not null;default:true"`
	Values            []SyncJobFieldValue `json:"values,omitempty" gorm:"foreignKey:SyncJobFieldID;constraint:OnDelete:CASCADE"`
	CreatedAt         time.Time           `json:"created_at"`
	UpdatedAt         time.Time           `json:"updated_at"`
}

func (f SyncJobField) IsActive() bool {
	return f.Active == nil || *f.Active
}

// SyncJobFieldValue maps a source field value to a destination value (e.g. 1 → "success").
type SyncJobFieldValue struct {
	ID               uint      `json:"id" gorm:"primaryKey"`
	SyncJobFieldID   uint      `json:"sync_job_field_id" gorm:"not null;index"`
	SourceValue      string    `json:"source_value" gorm:"size:255;not null"`
	DestinationValue string    `json:"destination_value" gorm:"size:255;not null"`
	CreatedAt        time.Time `json:"created_at"`
	UpdatedAt        time.Time `json:"updated_at"`
}

func (SyncJobFieldValue) TableName() string {
	return "sync_job_fields_values"
}

// SyncJobRule filters source rows before import (AND'd together).
type SyncJobRule struct {
	ID        uint      `json:"id" gorm:"primaryKey"`
	SyncJobID uint      `json:"sync_job_id" gorm:"not null;index"`
	Field     string    `json:"field" gorm:"size:255;not null"`
	Operator  string    `json:"operator" gorm:"size:20;not null"`
	Value     string    `json:"value" gorm:"type:text"`
	Active    *bool     `json:"active" gorm:"not null;default:true"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

func (r SyncJobRule) IsActive() bool {
	return r.Active == nil || *r.Active
}

type SyncLog struct {
	ID         uint       `json:"id" gorm:"primaryKey"`
	SyncJobID  uint       `json:"sync_job_id" gorm:"not null;index"`
	Status     string     `json:"status" gorm:"size:50;not null;index"`
	Message    string     `json:"message" gorm:"type:text"`
	RowsTotal  *int64     `json:"rows_total"`
	RowsSynced *int64     `json:"rows_synced"`
	DurationMs *int64     `json:"duration_ms"`
	StartedAt  time.Time  `json:"started_at"`
	FinishedAt *time.Time `json:"finished_at"`
	SyncJob    *SyncJob   `json:"sync_job,omitempty" gorm:"foreignKey:SyncJobID"`
}
