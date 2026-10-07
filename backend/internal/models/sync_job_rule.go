package models

import "time"

const (
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
