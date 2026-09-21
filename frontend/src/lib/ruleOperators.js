export const RULE_OPERATORS = [
  { value: 'eq', label: '=' },
  { value: 'neq', label: '≠' },
  { value: 'gt', label: '>' },
  { value: 'gte', label: '≥' },
  { value: 'lt', label: '<' },
  { value: 'lte', label: '≤' },
  { value: 'in', label: 'in' },
  { value: 'not_in', label: 'not in' },
  { value: 'like', label: 'like' },
  { value: 'is_null', label: 'is null' },
  { value: 'is_not_null', label: 'is not null' },
]

export function ruleNeedsValue(operator) {
  return operator !== 'is_null' && operator !== 'is_not_null'
}

export function ruleOperatorLabel(operator) {
  return RULE_OPERATORS.find((o) => o.value === operator)?.label || operator
}
