package models

import (
	"time"

	"gorm.io/datatypes"
	"gorm.io/gorm"
)

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

// PreloadSyncJob loads the job graph used by the API and the sync runner.
func PreloadSyncJob(db *gorm.DB) *gorm.DB {
	return db.
		Preload("SourceConnection").
		Preload("DestinationConnection").
		Preload("Relations").
		Preload("Relations.Fields").
		Preload("Relations.Fields.Values").
		Preload("Fields", "sync_job_relation_id IS NULL").
		Preload("Fields.Values").
		Preload("Rules")
}
