package domain

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"
)

func statusColumn() Column {
	return Column{ID: "status", Title: "Status", Type: "status", Settings: JSONObject{"labels": []any{
		map[string]any{"id": 0, "label": "Working on it", "index": 0},
		map[string]any{"id": 1, "label": "Done", "index": 1, "is_done": true},
		map[string]any{"id": 2, "label": "Stuck", "index": 2},
		map[string]any{"id": 5, "label": "Legacy", "index": 3, "is_deactivated": true},
	}}}
}

func dropdownColumn() Column {
	return Column{ID: "env", Title: "Environment", Type: "dropdown", Settings: JSONObject{"labels": []any{
		map[string]any{"id": 1, "label": "dev"}, map[string]any{"id": 2, "label": "prod"},
	}}}
}

func TestNormalizeColumnValueAcceptsFriendlyInput(t *testing.T) {
	tests := []struct {
		name   string
		column Column
		input  any
		want   any
	}{
		{"text", Column{Type: "text"}, "hello", "hello"},
		{"name", Column{Type: "name"}, "Deploy", "Deploy"},
		{"long text string", Column{Type: "long_text"}, "body", map[string]any{"text": "body"}},
		{"numbers float", Column{Type: "numbers"}, 4.5, "4.5"},
		{"numbers string", Column{Type: "numbers"}, " 7 ", "7"},
		{"status label case-insensitive", statusColumn(), "working ON it", map[string]any{"label": "Working on it"}},
		{"status by id", statusColumn(), float64(2), map[string]any{"label": "Stuck"}},
		{"status object index", statusColumn(), map[string]any{"index": float64(1)}, map[string]any{"label": "Done"}},
		{"dropdown labels", dropdownColumn(), []any{"DEV", "prod"}, map[string]any{"labels": []string{"dev", "prod"}}},
		{"dropdown ids", dropdownColumn(), []any{float64(2)}, map[string]any{"ids": []int{2}}},
		{"date", Column{Type: "date"}, "2026-10-01", map[string]any{"date": "2026-10-01"}},
		{"date time", Column{Type: "date"}, "2026-10-01 09:30", map[string]any{"date": "2026-10-01", "time": "09:30:00"}},
		{"timeline", Column{Type: "timeline"}, map[string]any{"from": "2026-10-01", "to": "2026-10-03"}, map[string]any{"from": "2026-10-01", "to": "2026-10-03"}},
		{"people", Column{Type: "people"}, []any{"123", "team:9"}, map[string]any{"personsAndTeams": []map[string]any{{"id": 123, "kind": "person"}, {"id": 9, "kind": "team"}}}},
		{"checkbox true", Column{Type: "checkbox"}, true, map[string]any{"checked": "true"}},
		{"checkbox false clears", Column{Type: "checkbox"}, false, nil},
		{"email", Column{Type: "email"}, "ops@example.com", map[string]any{"email": "ops@example.com", "text": "ops@example.com"}},
		{"link", Column{Type: "link"}, "https://example.com", map[string]any{"url": "https://example.com", "text": "https://example.com"}},
		{"phone", Column{Type: "phone"}, "ni:+50588887777", map[string]any{"phone": "+50588887777", "countryShortName": "NI"}},
		{"rating", Column{Type: "rating"}, float64(4), map[string]any{"rating": 4}},
		{"hour", Column{Type: "hour"}, "9:05", map[string]any{"hour": 9, "minute": 5}},
		{"week from date", Column{Type: "week"}, "2026-10-01", map[string]any{"week": map[string]any{"startDate": "2026-09-28", "endDate": "2026-10-04"}}},
		{"country", Column{Type: "country"}, map[string]any{"countryCode": "ni", "countryName": "Nicaragua"}, map[string]any{"countryCode": "NI", "countryName": "Nicaragua"}},
		{"location", Column{Type: "location"}, map[string]any{"lat": 12.1, "lng": -86.2, "address": "Managua"}, map[string]any{"lat": "12.1", "lng": "-86.2", "address": "Managua"}},
		{"world clock", Column{Type: "world_clock"}, "America/Managua", map[string]any{"timezone": "America/Managua"}},
		{"tags", Column{Type: "tags"}, []any{float64(10)}, map[string]any{"tag_ids": []int{10}}},
		{"board relation", Column{Type: "board_relation"}, []any{"55"}, map[string]any{"item_ids": []int{55}}},
		{"color picker", Column{Type: "color_picker"}, "#1F76C2", map[string]any{"color": map[string]any{"hex": "#1F76C2"}}},
		{"nil clears text", Column{Type: "text"}, nil, ""},
		{"nil clears status", statusColumn(), nil, map[string]any{}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := NormalizeColumnValue(tt.column, tt.input)
			if err != nil {
				t.Fatalf("NormalizeColumnValue() error = %v", err)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("got %#v, want %#v", got, tt.want)
			}
		})
	}
}

func TestNormalizeColumnValueRejectsInvalidInput(t *testing.T) {
	tests := []struct {
		name   string
		column Column
		input  any
		hint   string
	}{
		{"empty name", Column{Type: "name"}, "  ", "empty"},
		{"text not string", Column{Type: "text"}, 5.0, "string"},
		{"not a number", Column{Type: "numbers"}, "abc", "number"},
		{"unknown status", statusColumn(), "Nope", "valid labels"},
		{"deactivated status", statusColumn(), "Legacy", "deactivated"},
		{"unknown dropdown", dropdownColumn(), "qa", "unknown dropdown"},
		{"mixed dropdown", dropdownColumn(), []any{"dev", float64(2)}, "mix"},
		{"bad date", Column{Type: "date"}, "2026-02-30", "YYYY-MM-DD"},
		{"bad time", Column{Type: "date"}, "2026-02-01 25", "time"},
		{"inverted timeline", Column{Type: "timeline"}, map[string]any{"from": "2026-10-05", "to": "2026-10-01"}, "before"},
		{"bad person", Column{Type: "people"}, []any{"abc"}, "invalid user ID"},
		{"bad email", Column{Type: "email"}, "nope", "invalid email"},
		{"bad link scheme", Column{Type: "link"}, "ftp://x", "http"},
		{"phone without country", Column{Type: "phone"}, "+50588887777", "CC:number"},
		{"rating range", Column{Type: "rating"}, float64(9), "between 0 and 5"},
		{"hour range", Column{Type: "hour"}, "24:00", "range"},
		{"bad zone", Column{Type: "world_clock"}, "Mars/Base", "unknown IANA"},
		{"bad location", Column{Type: "location"}, map[string]any{"lat": 100.0, "lng": 0.0}, "lat"},
		{"bad color", Column{Type: "color_picker"}, "blue", "hex"},
		{"unknown type scalar", Column{Type: "future_type"}, "x", "raw JSON"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NormalizeColumnValue(tt.column, tt.input)
			if err == nil || !strings.Contains(err.Error(), tt.hint) {
				t.Fatalf("error = %v, want mention of %q", err, tt.hint)
			}
		})
	}
}

func TestPlanColumnValuesReportsEveryIssue(t *testing.T) {
	columns := []Column{statusColumn(), {ID: "est", Type: "numbers"}, {ID: "formula", Type: "formula"}, {ID: "old", Type: "text", Archived: true}}
	plan := PlanColumnValues(columns, map[string]any{"status": "Done", "est": "x", "formula": 1, "old": "t", "ghost": 1})
	if plan.Valid {
		t.Fatal("plan.Valid = true, want false")
	}
	if len(plan.Issues) != 4 {
		t.Fatalf("issues = %+v, want 4", plan.Issues)
	}
	if plan.Normalized["status"] == nil || len(plan.Changes) != 1 {
		t.Fatalf("valid column not normalized: %+v", plan)
	}
	var verr *ValidationError
	if err := plan.Err(); err == nil || !strings.Contains(err.Error(), "ghost") {
		t.Fatalf("Err() = %v", err)
	} else if ok := asValidationError(err, &verr); !ok || len(verr.Issues) != 4 {
		t.Fatalf("Err() is not a ValidationError: %T", err)
	}
}

func asValidationError(err error, target **ValidationError) bool {
	v, ok := err.(*ValidationError)
	*target = v
	return ok
}

func TestPlanColumnValuesProducesMondayJSON(t *testing.T) {
	plan := PlanColumnValues([]Column{statusColumn(), {ID: "due", Type: "date"}}, map[string]any{"status": "Stuck", "due": "2026-10-01"})
	if err := plan.Err(); err != nil {
		t.Fatal(err)
	}
	encoded, _ := json.Marshal(plan.Normalized)
	if string(encoded) != `{"due":{"date":"2026-10-01"},"status":{"label":"Stuck"}}` {
		t.Fatalf("encoded = %s", encoded)
	}
}

func TestColumnLabelsSupportsLegacyAndEncodedSettings(t *testing.T) {
	var column Column
	if err := json.Unmarshal([]byte(`{"id":"s","type":"status","settings":"{\"labels\":{\"1\":\"Done\",\"0\":\"Working\"}}"}`), &column); err != nil {
		t.Fatal(err)
	}
	labels := column.Labels()
	if len(labels) != 2 || labels[0].Label != "Working" || labels[1].ID != 1 {
		t.Fatalf("labels = %+v", labels)
	}
	if done := statusColumn().DoneLabels(); !done["done"] || len(done) != 1 {
		t.Fatalf("DoneLabels = %v", done)
	}
}

func TestColumnFormatsCoverReadOnlyTypes(t *testing.T) {
	formats := ColumnFormats()
	seen := map[string]bool{}
	for _, format := range formats {
		seen[format.Type] = true
		if ReadOnlyColumnTypes[format.Type] == format.Writable {
			t.Fatalf("format %s writable=%v contradicts read-only set", format.Type, format.Writable)
		}
	}
	for kind := range ReadOnlyColumnTypes {
		if !seen[kind] {
			t.Fatalf("read-only type %s missing from formats", kind)
		}
	}
}
