package models

import "time"

const (
	SyncLogStatusRunning = "running"
	SyncLogStatusSuccess = "success"
	SyncLogStatusFailed  = "failed"
	SyncLogStatusStopped = "stopped"
)

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
