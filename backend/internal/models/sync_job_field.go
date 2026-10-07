package models

import (
	"time"

	"gorm.io/datatypes"
)

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
