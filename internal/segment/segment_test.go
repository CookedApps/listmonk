package segment

import (
	"strings"
	"testing"
)

func compile(t *testing.T, doc string) string {
	t.Helper()
	s, err := Parse([]byte(doc))
	if err != nil {
		t.Fatalf("parse %s: %v", doc, err)
	}
	out, err := s.Compile()
	if err != nil {
		t.Fatalf("compile %s: %v", doc, err)
	}
	return out
}

func TestCompile(t *testing.T) {
	cases := map[string]string{
		`{"rules":[{"field":"attribs.country","op":"eq","value":"DE"}]}`:                                                      `(subscribers.attribs->>'country' = 'DE')`,
		`{"match":"any","rules":[{"field":"email","op":"contains","value":"gmail"},{"field":"name","op":"neq","value":"x"}]}`: `(subscribers.email ILIKE '%gmail%' OR subscribers.name != 'x')`,
		`{"rules":[{"field":"status","op":"one_of","value":"enabled, disabled"}]}`:                                            `(subscribers.status IN ('enabled', 'disabled'))`,
		`{"rules":[{"field":"created_at","op":"after","value":"2026-01-01"}]}`:                                                `(subscribers.created_at >= '2026-01-01')`,
		`{"rules":[{"field":"created_at","op":"in_last_days","value":30}]}`:                                                   `(subscribers.created_at >= NOW() - INTERVAL '30 days')`,
		`{"rules":[{"field":"attribs.premium","op":"eq","value":true}]}`:                                                      `(subscribers.attribs->>'premium' = 'true')`,
		`{"rules":[{"field":"attribs.games","op":"gt","value":10}]}`:                                                          `((CASE WHEN jsonb_typeof(subscribers.attribs->'games') = 'number' THEN (subscribers.attribs->>'games')::NUMERIC END) > 10)`,
		`{"rules":[{"field":"attribs.city","op":"is_set","value":null}]}`:                                                     `((subscribers.attribs->'city' IS NOT NULL AND subscribers.attribs->'city' != 'null'::JSONB))`,
		`{"rules":[{"field":"list","op":"not_in","value":7}]}`:                                                                `(subscribers.id NOT IN (SELECT subscriber_id FROM subscriber_lists WHERE list_id = 7 AND status != 'unsubscribed'))`,
		`{"rules":[{"field":"campaign","op":"received","value":42}]}`:                                                         `(subscribers.id IN (SELECT subscriber_id FROM campaign_sends WHERE campaign_id = 42))`,
		`{"rules":[{"field":"campaign","op":"not_opened","value":"42"}]}`:                                                     `(subscribers.id NOT IN (SELECT subscriber_id FROM campaign_views WHERE campaign_id = 42))`,
	}
	for doc, want := range cases {
		if got := compile(t, doc); got != want {
			t.Errorf("%s\n got: %s\nwant: %s", doc, got, want)
		}
	}
}

func TestEmptySegment(t *testing.T) {
	for _, doc := range []string{"", "null", `{"rules":[]}`, `{}`} {
		if got := compile(t, doc); got != "" {
			t.Errorf("%q compiled to %q, want empty", doc, got)
		}
	}
}

func TestEscaping(t *testing.T) {
	got := compile(t, `{"rules":[{"field":"attribs.city","op":"eq","value":"it's"}]}`)
	if got != `(subscribers.attribs->>'city' = 'it''s')` {
		t.Errorf("quote escaping wrong: %s", got)
	}

	got = compile(t, `{"rules":[{"field":"email","op":"contains","value":"50%_a\\b"}]}`)
	if got != `(subscribers.email ILIKE '%50\%\_a\\b%')` {
		t.Errorf("like escaping wrong: %s", got)
	}
}

func TestRejects(t *testing.T) {
	bad := []string{
		`{"match":"some","rules":[]}`,
		`{"rules":[{"field":"password","op":"eq","value":"x"}]}`,
		`{"rules":[{"field":"email","op":"gt","value":"x"}]}`,
		`{"rules":[{"field":"email","op":"eq","value":""}]}`,
		`{"rules":[{"field":"status","op":"eq","value":"deleted"}]}`,
		`{"rules":[{"field":"created_at","op":"after","value":"yesterday"}]}`,
		`{"rules":[{"field":"attribs.x;drop","op":"eq","value":"1"}]}`,
		`{"rules":[{"field":"attribs.n","op":"gt","value":"many"}]}`,
		`{"rules":[{"field":"list","op":"in","value":0}]}`,
		`{"rules":[{"field":"campaign","op":"received","value":1.5}]}`,
	}
	for _, doc := range bad {
		s, err := Parse([]byte(doc))
		if err == nil {
			_, err = s.Compile()
		}
		if err == nil {
			t.Errorf("%s should be rejected", doc)
		}
	}

	rules := strings.Repeat(`{"field":"email","op":"eq","value":"x"},`, maxRules+1)
	if _, err := Parse([]byte(`{"rules":[` + strings.TrimSuffix(rules, ",") + `]}`)); err == nil {
		t.Error("too many rules should be rejected")
	}
}

func TestTypedAttribs(t *testing.T) {
	const (
		num  = `(CASE WHEN jsonb_typeof(subscribers.attribs->'games') = 'number' THEN (subscribers.attribs->>'games')::NUMERIC WHEN jsonb_typeof(subscribers.attribs->'games') = 'string' AND subscribers.attribs->>'games' ~ '^\s*-?\d+(\.\d+)?\s*$' THEN (subscribers.attribs->>'games')::NUMERIC END)`
		date = `(CASE WHEN jsonb_typeof(subscribers.attribs->'seen') = 'string' AND subscribers.attribs->>'seen' ~ '^\d{4}-\d{2}-\d{2}([T ]\d{2}:\d{2}(:\d{2}(\.\d+)?)?(Z|[+-]\d{2}(:?\d{2})?)?)?$' AND pg_input_is_valid(subscribers.attribs->>'seen', 'timestamptz') THEN (subscribers.attribs->>'seen')::TIMESTAMPTZ END)`
	)

	cases := map[string]string{
		`{"rules":[{"field":"attribs.consent","type":"bool","op":"is_true","value":null}]}`:        `((subscribers.attribs->>'consent' = 'true'))`,
		`{"rules":[{"field":"attribs.restricted","type":"bool","op":"is_not_true","value":null}]}`: `((subscribers.attribs->>'restricted' IS DISTINCT FROM 'true'))`,
		`{"rules":[{"field":"attribs.games","type":"number","op":"gte","value":2}]}`:               `((` + num + ` >= 2))`,
		`{"rules":[{"field":"attribs.games","type":"number","op":"neq","value":"3"}]}`:             `((` + num + ` IS DISTINCT FROM 3))`,
		`{"rules":[{"field":"attribs.seen","type":"date","op":"in_last_days","value":200}]}`:       `((` + date + ` >= NOW() - INTERVAL '200 days'))`,
		`{"rules":[{"field":"attribs.seen","type":"date","op":"not_in_last_days","value":30}]}`:    `((` + date + ` < NOW() - INTERVAL '30 days'))`,
		`{"rules":[{"field":"attribs.seen","type":"date","op":"before","value":"2026-01-01"}]}`:    `((` + date + ` < '2026-01-01'))`,
		`{"rules":[{"field":"attribs.seen","type":"date","op":"is_set","value":null}]}`:            `((subscribers.attribs->'seen' IS NOT NULL AND subscribers.attribs->'seen' != 'null'::JSONB))`,
		`{"rules":[{"field":"attribs.country","type":"text","op":"neq","value":"DE"}]}`:            `((subscribers.attribs->>'country' IS DISTINCT FROM 'DE'))`,
		`{"rules":[{"field":"attribs.country","type":"text","op":"not_contains","value":"x"}]}`:    `((COALESCE(subscribers.attribs->>'country', '') NOT ILIKE '%x%'))`,
		`{"rules":[{"field":"attribs.country","type":"text","op":"eq","value":"DE"}]}`:             `(subscribers.attribs->>'country' = 'DE')`,
	}
	for doc, want := range cases {
		if got := compile(t, doc); got != want {
			t.Errorf("%s\n got: %s\nwant: %s", doc, got, want)
		}
	}
}

func TestTypedAttribsReject(t *testing.T) {
	for _, doc := range []string{
		`{"rules":[{"field":"attribs.a","type":"bool","op":"gt","value":1}]}`,
		`{"rules":[{"field":"attribs.a","type":"number","op":"contains","value":"x"}]}`,
		`{"rules":[{"field":"attribs.a","type":"number","op":"gte","value":"many"}]}`,
		`{"rules":[{"field":"attribs.a","type":"date","op":"in_last_days","value":-1}]}`,
		`{"rules":[{"field":"attribs.a","type":"date","op":"after","value":"yesterday"}]}`,
		`{"rules":[{"field":"attribs.a","type":"json","op":"eq","value":"x"}]}`,
		`{"rules":[{"field":"attribs.a';--","type":"text","op":"eq","value":"x"}]}`,
	} {
		s, err := Parse([]byte(doc))
		if err != nil {
			continue
		}
		if _, err := s.Compile(); err == nil {
			t.Errorf("%s compiled, want an error", doc)
		}
	}
}
