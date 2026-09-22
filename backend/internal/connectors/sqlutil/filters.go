package sqlutil

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/portico/backend/internal/connectors"
	"github.com/portico/backend/internal/models"
	"gorm.io/gorm"
)

// ApplyFilters ANDs source filters onto a GORM query.
func ApplyFilters(db *gorm.DB, filters []connectors.Filter, quoteIdent func(string) string) (*gorm.DB, error) {
	for _, f := range filters {
		col := strings.TrimSpace(f.Column)
		if col == "" {
			return nil, fmt.Errorf("filter field is required")
		}
		qcol := quoteIdent(col)

		switch f.Operator {
		case models.RuleOperatorEq:
			db = db.Where(qcol+" = ?", coerceValue(f.Value))
		case models.RuleOperatorNeq:
			db = db.Where(qcol+" <> ?", coerceValue(f.Value))
		case models.RuleOperatorGt:
			db = db.Where(qcol+" > ?", coerceValue(f.Value))
		case models.RuleOperatorGte:
			db = db.Where(qcol+" >= ?", coerceValue(f.Value))
		case models.RuleOperatorLt:
			db = db.Where(qcol+" < ?", coerceValue(f.Value))
		case models.RuleOperatorLte:
			db = db.Where(qcol+" <= ?", coerceValue(f.Value))
		case models.RuleOperatorLike:
			db = db.Where(qcol+" LIKE ?", f.Value)
		case models.RuleOperatorIn:
			values, err := csvValues(f.Operator, f.Value)
			if err != nil {
				return nil, err
			}
			db = db.Where(qcol+" IN ?", values)
		case models.RuleOperatorNotIn:
			values, err := csvValues(f.Operator, f.Value)
			if err != nil {
				return nil, err
			}
			db = db.Where(qcol+" NOT IN ?", values)
		case models.RuleOperatorIsNull:
			db = db.Where(qcol + " IS NULL")
		case models.RuleOperatorIsNotNull:
			db = db.Where(qcol + " IS NOT NULL")
		default:
			return nil, fmt.Errorf("unsupported filter operator %q", f.Operator)
		}
	}
	return db, nil
}

func csvValues(operator, value string) ([]any, error) {
	parts := strings.Split(value, ",")
	out := make([]any, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		out = append(out, coerceValue(p))
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("operator %q requires at least one value", operator)
	}
	return out, nil
}

func coerceValue(v string) any {
	if i, err := strconv.ParseInt(v, 10, 64); err == nil {
		return i
	}
	if f, err := strconv.ParseFloat(v, 64); err == nil {
		return f
	}
	switch v {
	case "true":
		return true
	case "false":
		return false
	default:
		return v
	}
}
