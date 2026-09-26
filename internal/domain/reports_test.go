package domain

import (
	"strings"
	"testing"
	"time"
)

var reportNow = time.Date(2026, 9, 26, 12, 0, 0, 0, time.UTC)

func cell(id, text, value string) ColumnValue { return ColumnValue{ID: id, Text: text, Value: value} }

func sampleSnapshot() BoardSnapshot {
	columns := []Column{
		{ID: "name", Type: "name", Title: "Name"},
		statusColumn(),
		{ID: "due", Type: "date", Title: "Due date"},
		{ID: "owner", Type: "people", Title: "Owner"},
		{ID: "prio", Type: "status", Title: "Priority"},
	}
	groups := []Group{{ID: "todo", Title: "To Do"}, {ID: "done", Title: "Done"}}
	items := []Item{
		{ID: "1", Name: "Late task", UpdatedAt: "2026-09-01T00:00:00Z", Group: &Group{ID: "todo", Title: "To Do"}, ColumnValues: []ColumnValue{
			cell("status", "Working on it", `{"index":0}`), cell("due", "2026-09-20", `{"date":"2026-09-20"}`),
			cell("owner", "Ana, Luis", `{"personsAndTeams":[{"id":1,"kind":"person"},{"id":2,"kind":"person"}]}`), cell("prio", "High", ""),
		}},
		{ID: "2", Name: "Blocked task", UpdatedAt: "2026-09-26T08:00:00Z", Group: &Group{ID: "todo", Title: "To Do"}, ColumnValues: []ColumnValue{
			cell("status", "Stuck", ""), cell("due", "2026-10-10", `{"date":"2026-10-10"}`), cell("owner", "", ""), cell("prio", "High", ""),
		}},
		{ID: "3", Name: "Finished", UpdatedAt: "2026-09-26T09:00:00Z", Group: &Group{ID: "done", Title: "Done"}, ColumnValues: []ColumnValue{
			cell("status", "Done", ""), cell("due", "2026-09-01", `{"date":"2026-09-01"}`), cell("owner", "Ana", `{"personsAndTeams":[{"id":1,"kind":"person"}]}`), cell("prio", "Low", ""),
		}},
	}
	return BoardSnapshot{Board: Board{ID: "10", Name: "Ops"}, Columns: columns, Groups: groups, Items: items}
}

func TestDetectReportColumnsPrefersHintsAndHonorsOverrides(t *testing.T) {
	cols := DetectReportColumns(sampleSnapshot().Columns, ReportColumns{})
	if cols.StatusColumnID != "status" || cols.DateColumnID != "due" || cols.PeopleColumnID != "owner" {
		t.Fatalf("detected = %+v", cols)
	}
	if got := DetectReportColumns(sampleSnapshot().Columns, ReportColumns{StatusColumnID: "prio"}); got.StatusColumnID != "prio" {
		t.Fatalf("override ignored: %+v", got)
	}
}

func TestSummarize(t *testing.T) {
	summary := Summarize(sampleSnapshot(), ReportColumns{}, reportNow)
	if summary.TotalItems != 3 || summary.DoneItems != 1 || summary.CompletionPct != 33.3 {
		t.Fatalf("summary = %+v", summary)
	}
	if summary.OverdueItems != 1 || summary.BlockedItems != 1 || summary.UnassignedOpen != 1 {
		t.Fatalf("risk counters = %+v", summary)
	}
	if summary.ByGroup[0].Key != "To Do" || summary.ByGroup[0].Count != 2 {
		t.Fatalf("by group = %+v", summary.ByGroup)
	}
}

func TestOverdueStaleAndWorkload(t *testing.T) {
	snap := sampleSnapshot()
	_, overdue := Overdue(snap, ReportColumns{}, reportNow)
	if len(overdue) != 1 || overdue[0].ID != "1" || overdue[0].DaysLate != 6 {
		t.Fatalf("overdue = %+v", overdue)
	}
	_, stale := Stale(snap, ReportColumns{}, 14, reportNow)
	if len(stale) != 1 || stale[0].ID != "1" {
		t.Fatalf("stale = %+v", stale)
	}
	_, people := Workload(snap, ReportColumns{}, reportNow)
	byName := map[string]WorkloadEntry{}
	for _, entry := range people {
		byName[entry.Name] = entry
	}
	if byName["Ana"].Open != 1 || byName["Ana"].Done != 1 || byName["Ana"].Overdue != 1 || byName["Ana"].PersonID != "1" {
		t.Fatalf("Ana = %+v", byName["Ana"])
	}
	if byName["Unassigned"].Blocked != 1 || byName["Luis"].PersonID != "2" {
		t.Fatalf("workload = %+v", people)
	}
	if dist := ColumnDistribution(snap, "prio"); dist[0].Key != "High" || dist[0].Count != 2 {
		t.Fatalf("distribution = %+v", dist)
	}
}

func TestStandupAndHealth(t *testing.T) {
	standup := BuildStandup(sampleSnapshot(), ReportColumns{}, 24, reportNow)
	if len(standup.Completed) != 1 || len(standup.Updated) != 1 || len(standup.Blocked) != 1 || len(standup.Overdue) != 1 {
		t.Fatalf("standup = %+v", standup)
	}
	if !strings.Contains(standup.Markdown, "## ⛔ Blocked (1)") {
		t.Fatalf("markdown = %s", standup.Markdown)
	}
	health := BuildHealthReport(sampleSnapshot(), ReportColumns{}, 14, reportNow)
	// open=2: overdue 1/2*40=20, blocked 1/2*25=13, stale 1/2*20=10, unassigned 1/2*15=8
	if health.Score != 49 || health.Grade != "D" || len(health.Signals) != 4 {
		t.Fatalf("health = %d %s %v", health.Score, health.Grade, health.Signals)
	}
	empty := BuildHealthReport(BoardSnapshot{Board: Board{Name: "Empty"}}, ReportColumns{}, 14, reportNow)
	if empty.Score != 100 || empty.Grade != "A" {
		t.Fatalf("empty board health = %+v", empty)
	}
}

func TestExports(t *testing.T) {
	snap := sampleSnapshot()
	snap.Items[0].Name = "Pipe | name"
	md := ExportMarkdown(snap, []string{"status", "owner"})
	if !strings.Contains(md, "## To Do (2)") || !strings.Contains(md, "Pipe \\| name") || strings.Contains(md, "Priority") {
		t.Fatalf("markdown = %s", md)
	}
	csv, err := ExportCSV(snap, nil)
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(csv), "\n")
	if len(lines) != 4 || lines[0] != "item_id,name,group,updated_at,Status,Due date,Owner,Priority" {
		t.Fatalf("csv = %s", csv)
	}
	if !strings.Contains(lines[1], `"Ana, Luis"`) {
		t.Fatalf("csv quoting = %s", lines[1])
	}
}

func TestTemplatesAndDefaults(t *testing.T) {
	if len(BoardTemplates()) != 4 {
		t.Fatalf("templates = %d", len(BoardTemplates()))
	}
	if _, err := LookupBoardTemplate("nope"); err == nil || !strings.Contains(err.Error(), "devops") {
		t.Fatalf("lookup error = %v", err)
	}
	for _, template := range BoardTemplates() {
		for _, column := range template.Columns {
			defaults := ColumnDefaults(column)
			if len(column.Labels) == 0 {
				if defaults != nil {
					t.Fatalf("%s/%s unexpected defaults", template.Key, column.ID)
				}
				continue
			}
			if column.Type == "status" {
				labels := defaults["labels"].(map[string]any)
				if len(labels) != len(column.Labels) {
					t.Fatalf("%s/%s colors collide: %v", template.Key, column.ID, labels)
				}
			}
		}
	}
	done := ColumnDefaults(TemplateColumn{Type: "status", Labels: []string{"Working on it", "Done", "Stuck", "Custom"}, DoneLabel: "Done"})
	labels := done["labels"].(map[string]any)
	if labels["1"] != "Done" || labels["2"] != "Stuck" || labels["0"] != "Working on it" || labels["6"] != "Custom" {
		t.Fatalf("semantic colors = %v", labels)
	}
	dropdown := ColumnDefaults(TemplateColumn{Type: "dropdown", Labels: []string{"dev"}})
	if dropdown["settings"] == nil {
		t.Fatalf("dropdown defaults = %v", dropdown)
	}
}
