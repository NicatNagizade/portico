package models

import (
	"time"

	"gorm.io/datatypes"
)

const (
	RelationTypeBelongsToMany = "belongs_to_many"
	RelationTypeHasMany       = "has_many"
	RelationTypeHasOne        = "has_one"
	RelationTypeBelongsTo     = "belongs_to"
)

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
