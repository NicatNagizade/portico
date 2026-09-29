package mongodb

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/portico/backend/internal/connectors"
	"github.com/portico/backend/internal/connectors/docutil"
	"github.com/portico/backend/internal/models"
	"go.mongodb.org/mongo-driver/v2/bson"
)

// BuildFilter maps Portico filters to a MongoDB filter document (AND'd).
func BuildFilter(filters []connectors.Filter) (bson.D, error) {
	if len(filters) == 0 {
		return bson.D{}, nil
	}
	clauses := make(bson.A, 0, len(filters))
	for _, f := range filters {
		clause, err := filterClause(f)
		if err != nil {
			return nil, err
		}
		clauses = append(clauses, clause)
	}
	if len(clauses) == 1 {
		return clauses[0].(bson.D), nil
	}
	return bson.D{{Key: "$and", Value: clauses}}, nil
}

func filterClause(f connectors.Filter) (bson.D, error) {
	col := docutil.IDFilterColumn(f.Column, f.Operator, "_id", SortableIDField)
	if col == "" {
		return nil, fmt.Errorf("filter field is required")
	}

	switch f.Operator {
	case models.RuleOperatorEq:
		return bson.D{{Key: col, Value: filterValue(col, f.Value)}}, nil
	case models.RuleOperatorNeq:
		return bson.D{{Key: col, Value: bson.D{{Key: "$ne", Value: filterValue(col, f.Value)}}}}, nil
	case models.RuleOperatorGt:
		return bson.D{{Key: col, Value: bson.D{{Key: "$gt", Value: filterValue(col, f.Value)}}}}, nil
	case models.RuleOperatorGte:
		return bson.D{{Key: col, Value: bson.D{{Key: "$gte", Value: filterValue(col, f.Value)}}}}, nil
	case models.RuleOperatorLt:
		return bson.D{{Key: col, Value: bson.D{{Key: "$lt", Value: filterValue(col, f.Value)}}}}, nil
	case models.RuleOperatorLte:
		return bson.D{{Key: col, Value: bson.D{{Key: "$lte", Value: filterValue(col, f.Value)}}}}, nil
	case models.RuleOperatorIn:
		values, err := csvValues(col, f.Operator, f.Value)
		if err != nil {
			return nil, err
		}
		return bson.D{{Key: col, Value: bson.D{{Key: "$in", Value: values}}}}, nil
	case models.RuleOperatorNotIn:
		values, err := csvValues(col, f.Operator, f.Value)
		if err != nil {
			return nil, err
		}
		return bson.D{{Key: col, Value: bson.D{{Key: "$nin", Value: values}}}}, nil
	case models.RuleOperatorLike:
		return bson.D{{Key: col, Value: bson.D{
			{Key: "$regex", Value: likeToRegex(f.Value)},
		}}}, nil
	case models.RuleOperatorIsNull:
		// Matches missing field or explicit null (same as MongoDB equality to null).
		return bson.D{{Key: col, Value: nil}}, nil
	case models.RuleOperatorIsNotNull:
		return bson.D{{Key: col, Value: bson.D{
			{Key: "$exists", Value: true},
			{Key: "$ne", Value: nil},
		}}}, nil
	default:
		return nil, fmt.Errorf("unsupported filter operator %q", f.Operator)
	}
}

func csvValues(col, operator, value string) ([]any, error) {
	parts := strings.Split(value, ",")
	out := make([]any, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		out = append(out, filterValue(col, p))
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("operator %q requires at least one value", operator)
	}
	return out, nil
}

// filterValue keeps "_id" as string (document keys are always strings).
// Other fields coerce numbers/bools when the filter value looks like one.
func filterValue(col, v string) any {
	if col == "_id" {
		return v
	}
	return coerceValue(v)
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

// likeToRegex converts a SQL LIKE pattern (% / _) into an anchored regex.
func likeToRegex(pattern string) string {
	var b strings.Builder
	b.WriteString("^")
	for i := 0; i < len(pattern); i++ {
		switch pattern[i] {
		case '%':
			b.WriteString(".*")
		case '_':
			b.WriteString(".")
		case '\\':
			if i+1 < len(pattern) {
				i++
				b.WriteString(regexp.QuoteMeta(string(pattern[i])))
			}
		default:
			b.WriteString(regexp.QuoteMeta(string(pattern[i])))
		}
	}
	b.WriteString("$")
	return b.String()
}
