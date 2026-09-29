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
// quoteTable qualifies table names for relation subqueries (defaults to quoteIdent).
func ApplyFilters(db *gorm.DB, filters []connectors.Filter, quoteIdent, quoteTable func(string) string) (*gorm.DB, error) {
	if quoteTable == nil {
		quoteTable = quoteIdent
	}
	for _, f := range filters {
		col := strings.TrimSpace(f.Column)
		if col == "" {
			return nil, fmt.Errorf("filter field is required")
		}
		qcol := quoteIdent(col)

		if f.Rel != nil {
			sub, err := relationSubquery(db, f.Rel, quoteIdent, quoteTable)
			if err != nil {
				return nil, err
			}
			db = db.Where(qcol+" IN (?)", sub)
			continue
		}

		var err error
		db, err = applyPredicate(db, qcol, f)
		if err != nil {
			return nil, err
		}
	}
	return db, nil
}

func relationSubquery(
	db *gorm.DB,
	rel *connectors.RelationSubquery,
	quoteIdent, quoteTable func(string) string,
) (*gorm.DB, error) {
	if rel.Table == "" || rel.Select == "" {
		return nil, fmt.Errorf("relation subquery requires table and select")
	}
	sub := db.Session(&gorm.Session{NewDB: true}).Table(quoteTable(rel.Table)).Select(quoteIdent(rel.Select))
	return ApplyFilters(sub, rel.Where, quoteIdent, quoteTable)
}

func applyPredicate(db *gorm.DB, qcol string, f connectors.Filter) (*gorm.DB, error) {
	switch f.Operator {
	case models.RuleOperatorEq:
		return db.Where(qcol+" = ?", coerceValue(f.Value)), nil
	case models.RuleOperatorNeq:
		return db.Where(qcol+" <> ?", coerceValue(f.Value)), nil
	case models.RuleOperatorGt:
		return db.Where(qcol+" > ?", coerceValue(f.Value)), nil
	case models.RuleOperatorGte:
		return db.Where(qcol+" >= ?", coerceValue(f.Value)), nil
	case models.RuleOperatorLt:
		return db.Where(qcol+" < ?", coerceValue(f.Value)), nil
	case models.RuleOperatorLte:
		return db.Where(qcol+" <= ?", coerceValue(f.Value)), nil
	case models.RuleOperatorLike:
		return db.Where(qcol+" LIKE ?", f.Value), nil
	case models.RuleOperatorIn:
		values, err := csvValues(f.Operator, f.Value)
		if err != nil {
			return nil, err
		}
		return db.Where(qcol+" IN ?", values), nil
	case models.RuleOperatorNotIn:
		values, err := csvValues(f.Operator, f.Value)
		if err != nil {
			return nil, err
		}
		return db.Where(qcol+" NOT IN ?", values), nil
	case models.RuleOperatorIsNull:
		return db.Where(qcol + " IS NULL"), nil
	case models.RuleOperatorIsNotNull:
		return db.Where(qcol + " IS NOT NULL"), nil
	default:
		return nil, fmt.Errorf("unsupported filter operator %q", f.Operator)
	}
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
