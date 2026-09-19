package models

import (
	"time"

	"gorm.io/datatypes"
)

const (
	ConnectionTypeMySQL     = "mysql"
	ConnectionTypePostgres  = "postgres"
	ConnectionTypeTypesense = "typesense"
	ConnectionTypeMongoDB   = "mongodb"

	SyncLogStatusRunning = "running"
	SyncLogStatusSuccess = "success"
	SyncLogStatusFailed  = "failed"

	RelationTypeBelongsToMany = "belongs_to_many"
)

type Connection struct {
	ID        uint           `json:"id" gorm:"primaryKey"`
	Name      string         `json:"name" gorm:"size:255;not null"`
	Type      string         `json:"type" gorm:"size:50;not null;index"`
	Config    datatypes.JSON `json:"config" gorm:"type:json;not null" swaggertype:"object"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
}

type SyncJob struct {
	ID                      uint              `json:"id" gorm:"primaryKey"`
	Name                    string            `json:"name" gorm:"size:255;not null"`
	SourceConnectionID      uint              `json:"source_connection_id" gorm:"not null;index"`
	DestinationConnectionID uint              `json:"destination_connection_id" gorm:"not null;index"`
	SourceTable             string            `json:"source_table" gorm:"size:255;not null"`
	DestinationTable        string            `json:"destination_table" gorm:"size:255;not null"`
	ChunkSize               int               `json:"chunk_size" gorm:"not null;default:500"`
	ParallelCount           int               `json:"parallel_count" gorm:"not null;default:2"`
	Config                  datatypes.JSON    `json:"config,omitempty" gorm:"type:json" swaggertype:"object"`
	SourceConnection        *Connection       `json:"source_connection,omitempty" gorm:"foreignKey:SourceConnectionID"`
	DestinationConnection   *Connection       `json:"destination_connection,omitempty" gorm:"foreignKey:DestinationConnectionID"`
	Relations               []SyncJobRelation `json:"relations,omitempty" gorm:"foreignKey:SyncJobID;constraint:OnDelete:CASCADE"`
	Fields                  []SyncJobField    `json:"fields,omitempty" gorm:"foreignKey:SyncJobID;constraint:OnDelete:CASCADE"`
	CreatedAt               time.Time         `json:"created_at"`
	UpdatedAt               time.Time         `json:"updated_at"`
}

type SyncJobRelation struct {
	ID         uint      `json:"id" gorm:"primaryKey"`
	SyncJobID  uint      `json:"sync_job_id" gorm:"not null;index"`
	Name       string    `json:"name" gorm:"size:255;not null"`
	Type       string    `json:"type" gorm:"size:50;not null"`
	Table      string    `json:"table" gorm:"size:255;not null"`
	PivotTable string    `json:"pivot_table" gorm:"size:255"`
	ForeignKey string    `json:"foreign_key" gorm:"size:255"`
	RelatedKey string    `json:"related_key" gorm:"size:255"`
	Active     *bool     `json:"active" gorm:"not null;default:true"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

func (r SyncJobRelation) IsActive() bool {
	return r.Active == nil || *r.Active
}

type SyncJobField struct {
	ID                uint           `json:"id" gorm:"primaryKey"`
	SyncJobID         uint           `json:"sync_job_id" gorm:"not null;index"`
	SourceName        string         `json:"source_name" gorm:"size:255;not null"`
	DestinationName   string         `json:"destination_name" gorm:"size:255"`
	DestinationType   string         `json:"destination_type" gorm:"size:50"`
	DestinationConfig datatypes.JSON `json:"destination_config,omitempty" gorm:"type:json" swaggertype:"object"`
	Active            *bool          `json:"active" gorm:"not null;default:true"`
	CreatedAt         time.Time      `json:"created_at"`
	UpdatedAt         time.Time      `json:"updated_at"`
}

func (f SyncJobField) IsActive() bool {
	return f.Active == nil || *f.Active
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
