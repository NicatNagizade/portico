package models

import "time"

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
