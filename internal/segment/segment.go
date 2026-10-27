// Package segment compiles rule-builder JSON into a SQL expression over the subscribers table.
package segment

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"regexp"
	"strconv"
	"strings"
)

const (
	MatchAll = "all"
	MatchAny = "any"

	maxRules = 50
)

type Rule struct {
	Field string `json:"field"`
	Type  string `json:"type,omitempty"`
	Op    string `json:"op"`
	Value any    `json:"value"`
}

type Segment struct {
	Match string `json:"match"`
	Rules []Rule `json:"rules"`
}

var (
	reAttribKey = regexp.MustCompile(`^[\w.\- ]{1,100}$`)
	reDate      = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}$`)

	numericPattern = `^\s*-?\d+(\.\d+)?\s*$`
	isoDatePattern = `^\d{4}-\d{2}-\d{2}([T ]\d{2}:\d{2}(:\d{2}(\.\d+)?)?(Z|[+-]\d{2}(:?\d{2})?)?)?$`

	textOps   = ops("eq", "neq", "contains", "not_contains", "one_of")
	statusOps = ops("eq", "neq", "one_of")
	dateOps   = ops("before", "after", "in_last_days")
	attribOps = ops("eq", "neq", "contains", "not_contains", "one_of", "gt", "lt", "is_set", "not_set")
	listOps   = ops("in", "not_in")

	typedAttribOps = map[string]map[string]struct{}{
		"text":   ops("eq", "neq", "contains", "not_contains", "one_of", "is_set", "not_set"),
		"number": ops("eq", "neq", "gt", "gte", "lt", "lte", "is_set", "not_set"),
		"bool":   ops("is_true", "is_not_true"),
		"date":   ops("in_last_days", "not_in_last_days", "before", "after", "is_set", "not_set"),
	}
	campOps = ops("received", "not_received", "opened", "not_opened")

	subscriberStatuses = ops("enabled", "disabled", "blocklisted")
)

func ops(names ...string) map[string]struct{} {
	m := make(map[string]struct{}, len(names))
	for _, n := range names {
		m[n] = struct{}{}
	}
	return m
}

// Parse unmarshals and validates segment JSON. An empty or null document yields an empty segment.
func Parse(b []byte) (Segment, error) {
	var s Segment
	if len(b) == 0 || string(b) == "null" {
		return s, nil
	}
	if err := json.Unmarshal(b, &s); err != nil {
		return s, fmt.Errorf("invalid segment: %v", err)
	}
	if s.Match == "" {
		s.Match = MatchAll
	}
	if s.Match != MatchAll && s.Match != MatchAny {
		return s, fmt.Errorf("invalid match '%s'", s.Match)
	}
	if len(s.Rules) > maxRules {
		return s, fmt.Errorf("too many rules (max %d)", maxRules)
	}

	return s, nil
}

// IsEmpty reports whether the segment has no rules.
func (s Segment) IsEmpty() bool {
	return len(s.Rules) == 0
}

// Compile returns a SQL boolean expression for the segment, or "" for an empty segment.
func (s Segment) Compile() (string, error) {
	if s.IsEmpty() {
		return "", nil
	}

	parts := make([]string, 0, len(s.Rules))
	for i, r := range s.Rules {
		expr, err := compileRule(r)
		if err != nil {
			return "", fmt.Errorf("rule %d: %v", i+1, err)
		}
		parts = append(parts, expr)
	}

	joiner := " AND "
	if s.Match == MatchAny {
		joiner = " OR "
	}

	return "(" + strings.Join(parts, joiner) + ")", nil
}

func compileRule(r Rule) (string, error) {
	switch {
	case r.Field == "email" || r.Field == "name":
		return compileText("subscribers."+r.Field, r, textOps)
	case r.Field == "status":
		return compileStatus(r)
	case r.Field == "created_at" || r.Field == "updated_at":
		return compileDate("subscribers."+r.Field, r)
	case strings.HasPrefix(r.Field, "attribs.") && r.Type != "":
		return compileTypedAttrib(strings.TrimPrefix(r.Field, "attribs."), r)
	case strings.HasPrefix(r.Field, "attribs."):
		return compileAttrib(strings.TrimPrefix(r.Field, "attribs."), r)
	case r.Field == "list":
		return compileList(r)
	case r.Field == "campaign":
		return compileCampaign(r)
	}

	return "", fmt.Errorf("unknown field '%s'", r.Field)
}

func compileText(col string, r Rule, allowed map[string]struct{}) (string, error) {
	if _, ok := allowed[r.Op]; !ok {
		return "", fmt.Errorf("invalid operator '%s' for %s", r.Op, r.Field)
	}

	switch r.Op {
	case "eq", "neq":
		v, err := stringValue(r.Value)
		if err != nil {
			return "", err
		}
		return col + cmp(r.Op) + quote(v), nil
	case "contains", "not_contains":
		v, err := stringValue(r.Value)
		if err != nil {
			return "", err
		}
		not := ""
		if r.Op == "not_contains" {
			not = "NOT "
		}
		return col + " " + not + "ILIKE " + quote("%"+escapeLike(v)+"%"), nil
	case "one_of":
		vals, err := listValue(r.Value)
		if err != nil {
			return "", err
		}
		return col + " IN (" + strings.Join(vals, ", ") + ")", nil
	}

	return "", fmt.Errorf("invalid operator '%s'", r.Op)
}

func compileStatus(r Rule) (string, error) {
	if _, ok := statusOps[r.Op]; !ok {
		return "", fmt.Errorf("invalid operator '%s' for status", r.Op)
	}

	raw, err := stringValue(r.Value)
	if err != nil {
		return "", err
	}
	for _, st := range strings.Split(raw, ",") {
		if _, ok := subscriberStatuses[strings.TrimSpace(st)]; !ok {
			return "", fmt.Errorf("invalid status '%s'", st)
		}
	}

	return compileText("subscribers.status", r, statusOps)
}

func compileDate(col string, r Rule) (string, error) {
	if _, ok := dateOps[r.Op]; !ok {
		return "", fmt.Errorf("invalid operator '%s' for %s", r.Op, r.Field)
	}

	if r.Op == "in_last_days" {
		n, err := intValue(r.Value)
		if err != nil || n < 0 {
			return "", fmt.Errorf("days must be a positive number")
		}
		return fmt.Sprintf("%s >= NOW() - INTERVAL '%d days'", col, n), nil
	}

	v, err := stringValue(r.Value)
	if err != nil || !reDate.MatchString(v) {
		return "", errors.New("date must be YYYY-MM-DD")
	}
	if r.Op == "before" {
		return col + " < " + quote(v), nil
	}
	return col + " >= " + quote(v), nil
}

func compileAttrib(key string, r Rule) (string, error) {
	if !reAttribKey.MatchString(key) {
		return "", fmt.Errorf("invalid attribute key '%s'", key)
	}
	if _, ok := attribOps[r.Op]; !ok {
		return "", fmt.Errorf("invalid operator '%s' for attribute", r.Op)
	}

	text := "subscribers.attribs->>" + quote(key)
	node := "subscribers.attribs->" + quote(key)

	switch r.Op {
	case "is_set":
		return "(" + node + " IS NOT NULL AND " + node + " != 'null'::JSONB)", nil
	case "not_set":
		return "(" + node + " IS NULL OR " + node + " = 'null'::JSONB)", nil
	case "gt", "lt":
		n, err := floatValue(r.Value)
		if err != nil {
			return "", err
		}
		num := "(CASE WHEN jsonb_typeof(" + node + ") = 'number' THEN (" + text + ")::NUMERIC END)"
		return num + cmp(r.Op) + strconv.FormatFloat(n, 'f', -1, 64), nil
	}

	return compileText(text, Rule{Field: r.Field, Op: r.Op, Value: r.Value}, textOps)
}

func compileTypedAttrib(key string, r Rule) (string, error) {
	if !reAttribKey.MatchString(key) {
		return "", fmt.Errorf("invalid attribute key '%s'", key)
	}
	allowed, ok := typedAttribOps[r.Type]
	if !ok {
		return "", fmt.Errorf("invalid attribute type '%s'", r.Type)
	}
	if _, ok := allowed[r.Op]; !ok {
		return "", fmt.Errorf("invalid operator '%s' for %s attribute", r.Op, r.Type)
	}

	text := "subscribers.attribs->>" + quote(key)
	node := "subscribers.attribs->" + quote(key)

	switch r.Op {
	case "is_set":
		return "(" + node + " IS NOT NULL AND " + node + " != 'null'::JSONB)", nil
	case "not_set":
		return "(" + node + " IS NULL OR " + node + " = 'null'::JSONB)", nil
	case "is_true":
		return "(" + text + " = 'true')", nil
	case "is_not_true":
		return "(" + text + " IS DISTINCT FROM 'true')", nil
	}

	switch r.Type {
	case "number":
		return compileNumber(text, node, r)
	case "date":
		return compileAttribDate(text, node, r)
	}

	switch r.Op {
	case "neq":
		v, err := stringValue(r.Value)
		if err != nil {
			return "", err
		}
		return "(" + text + " IS DISTINCT FROM " + quote(v) + ")", nil
	case "not_contains":
		v, err := stringValue(r.Value)
		if err != nil {
			return "", err
		}
		return "(COALESCE(" + text + ", '') NOT ILIKE " + quote("%"+escapeLike(v)+"%") + ")", nil
	}

	return compileText(text, Rule{Field: r.Field, Op: r.Op, Value: r.Value}, textOps)
}

func compileNumber(text, node string, r Rule) (string, error) {
	n, err := floatValue(r.Value)
	if err != nil {
		return "", err
	}

	num := "(CASE WHEN jsonb_typeof(" + node + ") = 'number' THEN (" + text + ")::NUMERIC" +
		" WHEN jsonb_typeof(" + node + ") = 'string' AND " + text + " ~ " + quote(numericPattern) + " THEN (" + text + ")::NUMERIC END)"
	val := strconv.FormatFloat(n, 'f', -1, 64)
	if r.Op == "neq" {
		return "(" + num + " IS DISTINCT FROM " + val + ")", nil
	}

	return "(" + num + cmp(r.Op) + val + ")", nil
}

func compileAttribDate(text, node string, r Rule) (string, error) {
	date := "(CASE WHEN jsonb_typeof(" + node + ") = 'string' AND " + text + " ~ " + quote(isoDatePattern) +
		" AND pg_input_is_valid(" + text + ", 'timestamptz') THEN (" + text + ")::TIMESTAMPTZ END)"

	if r.Op == "in_last_days" || r.Op == "not_in_last_days" {
		n, err := intValue(r.Value)
		if err != nil || n < 0 {
			return "", errors.New("days must be a positive number")
		}
		op := " >= "
		if r.Op == "not_in_last_days" {
			op = " < "
		}
		return fmt.Sprintf("(%s%sNOW() - INTERVAL '%d days')", date, op, n), nil
	}

	v, err := stringValue(r.Value)
	if err != nil || !reDate.MatchString(v) {
		return "", errors.New("date must be YYYY-MM-DD")
	}
	if r.Op == "before" {
		return "(" + date + " < " + quote(v) + ")", nil
	}
	return "(" + date + " >= " + quote(v) + ")", nil
}

func compileList(r Rule) (string, error) {
	if _, ok := listOps[r.Op]; !ok {
		return "", fmt.Errorf("invalid operator '%s' for list", r.Op)
	}
	id, err := intValue(r.Value)
	if err != nil || id < 1 {
		return "", errors.New("list is required")
	}

	return membership(r.Op == "not_in",
		fmt.Sprintf("SELECT subscriber_id FROM subscriber_lists WHERE list_id = %d AND status != 'unsubscribed'", id)), nil
}

func compileCampaign(r Rule) (string, error) {
	if _, ok := campOps[r.Op]; !ok {
		return "", fmt.Errorf("invalid operator '%s' for campaign", r.Op)
	}
	id, err := intValue(r.Value)
	if err != nil || id < 1 {
		return "", errors.New("campaign is required")
	}

	table := "campaign_sends"
	if r.Op == "opened" || r.Op == "not_opened" {
		table = "campaign_views"
	}

	return membership(strings.HasPrefix(r.Op, "not_"),
		fmt.Sprintf("SELECT subscriber_id FROM %s WHERE campaign_id = %d", table, id)), nil
}

func membership(negate bool, sub string) string {
	if negate {
		return "subscribers.id NOT IN (" + sub + ")"
	}
	return "subscribers.id IN (" + sub + ")"
}

func cmp(op string) string {
	switch op {
	case "neq":
		return " != "
	case "gt":
		return " > "
	case "lt":
		return " < "
	case "gte":
		return " >= "
	case "lte":
		return " <= "
	}
	return " = "
}

func quote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "''") + "'"
}

func escapeLike(s string) string {
	r := strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`)
	return r.Replace(s)
}

func stringValue(v any) (string, error) {
	switch t := v.(type) {
	case string:
		if strings.TrimSpace(t) == "" {
			return "", errors.New("value is required")
		}
		return t, nil
	case float64:
		return strconv.FormatFloat(t, 'f', -1, 64), nil
	case bool:
		return strconv.FormatBool(t), nil
	}

	return "", errors.New("value is required")
}

func listValue(v any) ([]string, error) {
	raw, err := stringValue(v)
	if err != nil {
		return nil, err
	}

	var out []string
	for _, s := range strings.Split(raw, ",") {
		if s = strings.TrimSpace(s); s != "" {
			out = append(out, quote(s))
		}
	}
	if len(out) == 0 {
		return nil, errors.New("value is required")
	}

	return out, nil
}

func floatValue(v any) (float64, error) {
	switch t := v.(type) {
	case float64:
		if math.IsNaN(t) || math.IsInf(t, 0) {
			return 0, errors.New("value must be a number")
		}
		return t, nil
	case string:
		f, err := strconv.ParseFloat(strings.TrimSpace(t), 64)
		if err != nil {
			return 0, errors.New("value must be a number")
		}
		return f, nil
	}

	return 0, errors.New("value must be a number")
}

func intValue(v any) (int, error) {
	f, err := floatValue(v)
	if err != nil {
		return 0, err
	}
	if f != math.Trunc(f) {
		return 0, errors.New("value must be a whole number")
	}

	return int(f), nil
}
