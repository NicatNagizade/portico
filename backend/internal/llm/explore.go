package llm

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/portico/backend/internal/models"
)

// ExploreSuggestInput is what the explore UI sends for an AI fill.
type ExploreSuggestInput struct {
	Prompt       string   `json:"prompt"`
	FilterFields []string `json:"filter_fields"`
	Fields       []string `json:"fields"`
}

// Filter is an explore predicate (same shape as sync.FilterInput).
type Filter struct {
	Field    string `json:"field"`
	Operator string `json:"operator"`
	Value    string `json:"value"`
}

// ExploreSuggestResult is filters + column picks for the explore page.
// Fields=nil means show all columns; a non-nil slice (including empty) restricts columns.
type ExploreSuggestResult struct {
	Filters []Filter `json:"filters"`
	Fields  []string `json:"fields"`
}

type exploreLLMJSON struct {
	Filters []Filter   `json:"filters"`
	Fields  *[]string  `json:"fields"` // null = all columns
}

const exploreSystemPrompt = `You help fill explore UI filters and visible fields from a short user request.

Return ONLY a JSON object with this shape:
{"filters":[{"field":"...","operator":"...","value":"..."}],"fields":["col1","col2"] or null}

Rules:
- filters are AND'd. Use [] when the user does not ask for filters.
- operators must be one of: eq, neq, gt, gte, lt, lte, in, not_in, like, is_null, is_not_null
- for is_null / is_not_null, set value to ""
- for in / not_in, value is a comma-separated list
- for like, include % wildcards when useful (e.g. "%john%")
- field names MUST be chosen from the provided filter_fields list (exact spelling)
- fields: null means show all columns; otherwise pick a subset from the provided fields list (exact spelling)
- if the user only mentions filters, set fields to null
- if the user only mentions columns, set filters to []
- do not invent field names that are not in the lists`

// SuggestExplore asks the model to turn a natural-language request into filters + fields.
func SuggestExplore(ctx context.Context, c Completer, in ExploreSuggestInput) (*ExploreSuggestResult, error) {
	if c == nil {
		return nil, ErrNotConfigured
	}
	prompt := strings.TrimSpace(in.Prompt)
	if prompt == "" {
		return nil, fmt.Errorf("prompt is required")
	}

	filterOpts := uniqueTrimmed(in.FilterFields)
	fieldOpts := uniqueTrimmed(in.Fields)
	user := fmt.Sprintf(
		"User request:\n%s\n\nAvailable filter_fields:\n%s\n\nAvailable fields (columns):\n%s",
		prompt,
		joinOrNone(filterOpts),
		joinOrNone(fieldOpts),
	)

	raw, err := c.Complete(ctx, exploreSystemPrompt, user)
	if err != nil {
		return nil, err
	}
	return parseExploreSuggest(raw, filterOpts, fieldOpts)
}

func parseExploreSuggest(raw string, filterOpts, fieldOpts []string) (*ExploreSuggestResult, error) {
	raw = stripJSONFence(raw)
	var parsed exploreLLMJSON
	if err := json.Unmarshal([]byte(raw), &parsed); err != nil {
		return nil, fmt.Errorf("invalid ai json: %w", err)
	}

	filterIndex := indexFold(filterOpts)
	fieldIndex := indexFold(fieldOpts)

	out := &ExploreSuggestResult{Filters: make([]Filter, 0, len(parsed.Filters))}
	for _, f := range parsed.Filters {
		field, ok := filterIndex[strings.ToLower(strings.TrimSpace(f.Field))]
		if !ok {
			continue
		}
		op := strings.TrimSpace(f.Operator)
		if !models.ValidRuleOperator(op) {
			continue
		}
		value := strings.TrimSpace(f.Value)
		if !models.RuleNeedsValue(op) {
			value = ""
		} else if value == "" {
			continue
		}
		out.Filters = append(out.Filters, Filter{Field: field, Operator: op, Value: value})
	}

	if parsed.Fields == nil {
		out.Fields = nil
	} else {
		picked := make([]string, 0, len(*parsed.Fields))
		seen := map[string]struct{}{}
		for _, name := range *parsed.Fields {
			canon, ok := fieldIndex[strings.ToLower(strings.TrimSpace(name))]
			if !ok {
				continue
			}
			if _, dup := seen[canon]; dup {
				continue
			}
			seen[canon] = struct{}{}
			picked = append(picked, canon)
		}
		// All selected → treat as "show all" (null) for a quieter UI.
		if len(picked) == len(fieldOpts) && len(fieldOpts) > 0 {
			out.Fields = nil
		} else {
			out.Fields = picked
		}
	}
	return out, nil
}

func stripJSONFence(s string) string {
	s = strings.TrimSpace(s)
	if !strings.HasPrefix(s, "```") {
		return s
	}
	s = strings.TrimPrefix(s, "```")
	s = strings.TrimSpace(s)
	if strings.HasPrefix(strings.ToLower(s), "json") {
		s = strings.TrimSpace(s[4:])
	}
	if i := strings.LastIndex(s, "```"); i >= 0 {
		s = s[:i]
	}
	return strings.TrimSpace(s)
}

func uniqueTrimmed(in []string) []string {
	seen := map[string]struct{}{}
	out := make([]string, 0, len(in))
	for _, s := range in {
		s = strings.TrimSpace(s)
		if s == "" {
			continue
		}
		key := strings.ToLower(s)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, s)
	}
	return out
}

func indexFold(names []string) map[string]string {
	out := make(map[string]string, len(names))
	for _, n := range names {
		out[strings.ToLower(n)] = n
	}
	return out
}

func joinOrNone(names []string) string {
	if len(names) == 0 {
		return "(none)"
	}
	return strings.Join(names, "\n")
}
