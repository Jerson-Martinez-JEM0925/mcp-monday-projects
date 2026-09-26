package domain

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strings"
	"time"
)

// BoardSnapshot is the input for every report: a schema plus a bounded item set.
type BoardSnapshot struct {
	Board     Board
	Columns   []Column
	Groups    []Group
	Items     []Item
	Truncated bool
}

// ReportColumns are the columns a report uses, auto-detected unless provided.
type ReportColumns struct {
	StatusColumnID string `json:"status_column_id,omitempty"`
	DateColumnID   string `json:"date_column_id,omitempty"`
	PeopleColumnID string `json:"people_column_id,omitempty"`
}

// Count is a labelled counter.
type Count struct {
	Key   string `json:"key"`
	Count int    `json:"count"`
}

// ItemBrief is a compact item row used in report listings.
type ItemBrief struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Group    string `json:"group,omitempty"`
	Status   string `json:"status,omitempty"`
	Owner    string `json:"owner,omitempty"`
	DueDate  string `json:"due_date,omitempty"`
	Updated  string `json:"updated_at,omitempty"`
	DaysLate int    `json:"days_late,omitempty"`
	URL      string `json:"url,omitempty"`
}

// BoardSummary aggregates a board.
type BoardSummary struct {
	BoardID        string        `json:"board_id"`
	BoardName      string        `json:"board_name"`
	TotalItems     int           `json:"total_items"`
	Truncated      bool          `json:"truncated"`
	Columns        ReportColumns `json:"columns"`
	ByGroup        []Count       `json:"by_group"`
	ByStatus       []Count       `json:"by_status"`
	DoneItems      int           `json:"done_items"`
	CompletionPct  float64       `json:"completion_pct"`
	OverdueItems   int           `json:"overdue_items"`
	UnassignedOpen int           `json:"unassigned_open_items"`
	BlockedItems   int           `json:"blocked_items"`
}

// DetectReportColumns picks sensible status/date/people columns.
func DetectReportColumns(columns []Column, override ReportColumns) ReportColumns {
	result := override
	pick := func(types []string, hints []string) string {
		first := ""
		for _, column := range columns {
			if column.Archived || !contains(types, column.Type) {
				continue
			}
			if first == "" {
				first = column.ID
			}
			title := strings.ToLower(column.Title)
			for _, hint := range hints {
				if strings.Contains(title, hint) {
					return column.ID
				}
			}
		}
		return first
	}
	if result.StatusColumnID == "" {
		result.StatusColumnID = pick([]string{"status"}, []string{"status", "estado", "state"})
	}
	if result.DateColumnID == "" {
		result.DateColumnID = pick([]string{"date"}, []string{"due", "deadline", "vence", "entrega", "fecha límite", "fecha limite"})
	}
	if result.PeopleColumnID == "" {
		result.PeopleColumnID = pick([]string{"people", "person"}, []string{"owner", "responsable", "assignee", "asignado", "person"})
	}
	return result
}

func contains(list []string, value string) bool {
	for _, entry := range list {
		if entry == value {
			return true
		}
	}
	return false
}

// CellText returns an item's display text for a column.
func (i Item) CellText(columnID string) string {
	for _, value := range i.ColumnValues {
		if value.ID == columnID {
			return value.Text
		}
	}
	return ""
}

// CellValue returns an item's raw JSON value for a column.
func (i Item) CellValue(columnID string) string {
	for _, value := range i.ColumnValues {
		if value.ID == columnID {
			return value.Value
		}
	}
	return ""
}

// DueDate returns the YYYY-MM-DD value of a date column, if any.
func (i Item) DueDate(columnID string) string {
	raw := i.CellValue(columnID)
	if raw != "" {
		var parsed struct {
			Date string `json:"date"`
		}
		if json.Unmarshal([]byte(raw), &parsed) == nil && validDate(parsed.Date) {
			return parsed.Date
		}
	}
	text := i.CellText(columnID)
	if len(text) >= 10 && validDate(text[:10]) {
		return text[:10]
	}
	return ""
}

// PersonIDs returns the person IDs assigned in a people column.
func (i Item) PersonIDs(columnID string) []string {
	raw := i.CellValue(columnID)
	var parsed struct {
		PersonsAndTeams []struct {
			ID   json.Number `json:"id"`
			Kind string      `json:"kind"`
		} `json:"personsAndTeams"`
	}
	if raw == "" || json.Unmarshal([]byte(raw), &parsed) != nil {
		return nil
	}
	ids := make([]string, 0, len(parsed.PersonsAndTeams))
	for _, entry := range parsed.PersonsAndTeams {
		ids = append(ids, entry.ID.String())
	}
	return ids
}

type classifier struct {
	cols      ReportColumns
	doneSet   map[string]bool
	today     string
	groupName map[string]string
}

func newClassifier(snapshot BoardSnapshot, cols ReportColumns, now time.Time) classifier {
	c := classifier{cols: cols, doneSet: map[string]bool{}, today: now.Format("2006-01-02"), groupName: map[string]string{}}
	for _, column := range snapshot.Columns {
		if column.ID == cols.StatusColumnID {
			c.doneSet = column.DoneLabels()
		}
	}
	if len(c.doneSet) == 0 {
		for _, fallback := range []string{"done", "listo", "hecho", "completado", "completed", "terminado", "closed", "cerrado"} {
			c.doneSet[fallback] = true
		}
	}
	for _, group := range snapshot.Groups {
		c.groupName[group.ID] = group.Title
	}
	return c
}

func (c classifier) status(item Item) string {
	if c.cols.StatusColumnID == "" {
		return ""
	}
	return item.CellText(c.cols.StatusColumnID)
}

func (c classifier) done(item Item) bool {
	return c.doneSet[strings.ToLower(strings.TrimSpace(c.status(item)))]
}

func (c classifier) blocked(item Item) bool {
	status := strings.ToLower(c.status(item))
	for _, hint := range []string{"stuck", "blocked", "bloque", "detenido", "atascado"} {
		if strings.Contains(status, hint) {
			return true
		}
	}
	return false
}

func (c classifier) overdueDays(item Item, now time.Time) int {
	if c.cols.DateColumnID == "" || c.done(item) {
		return 0
	}
	due := item.DueDate(c.cols.DateColumnID)
	if due == "" || due >= c.today {
		return 0
	}
	parsed, _ := time.Parse("2006-01-02", due)
	today, _ := time.Parse("2006-01-02", c.today)
	return int(today.Sub(parsed).Hours() / 24)
}

func (c classifier) brief(item Item, now time.Time) ItemBrief {
	group := ""
	if item.Group != nil {
		group = item.Group.Title
		if group == "" {
			group = c.groupName[item.Group.ID]
		}
	}
	brief := ItemBrief{ID: item.ID, Name: item.Name, Group: group, Status: c.status(item), Updated: item.UpdatedAt, URL: item.URL}
	if c.cols.PeopleColumnID != "" {
		brief.Owner = item.CellText(c.cols.PeopleColumnID)
	}
	if c.cols.DateColumnID != "" {
		brief.DueDate = item.DueDate(c.cols.DateColumnID)
	}
	brief.DaysLate = c.overdueDays(item, now)
	return brief
}

func sortedCounts(counts map[string]int) []Count {
	list := make([]Count, 0, len(counts))
	for key, count := range counts {
		list = append(list, Count{Key: key, Count: count})
	}
	sort.Slice(list, func(i, j int) bool {
		if list[i].Count != list[j].Count {
			return list[i].Count > list[j].Count
		}
		return list[i].Key < list[j].Key
	})
	return list
}

// Summarize computes a board summary.
func Summarize(snapshot BoardSnapshot, override ReportColumns, now time.Time) BoardSummary {
	cols := DetectReportColumns(snapshot.Columns, override)
	c := newClassifier(snapshot, cols, now)
	byGroup := map[string]int{}
	byStatus := map[string]int{}
	summary := BoardSummary{BoardID: snapshot.Board.ID, BoardName: snapshot.Board.Name, TotalItems: len(snapshot.Items), Truncated: snapshot.Truncated, Columns: cols}
	for _, item := range snapshot.Items {
		brief := c.brief(item, now)
		byGroup[orDash(brief.Group)]++
		if cols.StatusColumnID != "" {
			byStatus[orDash(brief.Status)]++
		}
		if c.done(item) {
			summary.DoneItems++
			continue
		}
		if brief.DaysLate > 0 {
			summary.OverdueItems++
		}
		if cols.PeopleColumnID != "" && brief.Owner == "" {
			summary.UnassignedOpen++
		}
		if c.blocked(item) {
			summary.BlockedItems++
		}
	}
	summary.ByGroup = sortedCounts(byGroup)
	summary.ByStatus = sortedCounts(byStatus)
	if summary.TotalItems > 0 {
		summary.CompletionPct = round1(float64(summary.DoneItems) * 100 / float64(summary.TotalItems))
	}
	return summary
}

func orDash(s string) string {
	if strings.TrimSpace(s) == "" {
		return "—"
	}
	return s
}

func round1(v float64) float64 { return math.Round(v*10) / 10 }

// ColumnDistribution counts display values of any column.
func ColumnDistribution(snapshot BoardSnapshot, columnID string) []Count {
	counts := map[string]int{}
	for _, item := range snapshot.Items {
		counts[orDash(item.CellText(columnID))]++
	}
	return sortedCounts(counts)
}

// WorkloadEntry is the load of one assignee.
type WorkloadEntry struct {
	PersonID string `json:"person_id,omitempty"`
	Name     string `json:"name"`
	Open     int    `json:"open"`
	Done     int    `json:"done"`
	Overdue  int    `json:"overdue"`
	Blocked  int    `json:"blocked"`
}

// Workload aggregates items per assignee of the people column.
func Workload(snapshot BoardSnapshot, override ReportColumns, now time.Time) (ReportColumns, []WorkloadEntry) {
	cols := DetectReportColumns(snapshot.Columns, override)
	c := newClassifier(snapshot, cols, now)
	entries := map[string]*WorkloadEntry{}
	for _, item := range snapshot.Items {
		names := []string{"Unassigned"}
		ids := []string{""}
		if cols.PeopleColumnID != "" {
			if text := item.CellText(cols.PeopleColumnID); text != "" {
				names = splitNames(text)
				ids = item.PersonIDs(cols.PeopleColumnID)
			}
		}
		for index, name := range names {
			entry, ok := entries[name]
			if !ok {
				entry = &WorkloadEntry{Name: name}
				if index < len(ids) {
					entry.PersonID = ids[index]
				}
				entries[name] = entry
			}
			switch {
			case c.done(item):
				entry.Done++
			default:
				entry.Open++
				if c.overdueDays(item, now) > 0 {
					entry.Overdue++
				}
				if c.blocked(item) {
					entry.Blocked++
				}
			}
		}
	}
	list := make([]WorkloadEntry, 0, len(entries))
	for _, entry := range entries {
		list = append(list, *entry)
	}
	sort.Slice(list, func(i, j int) bool {
		if list[i].Open != list[j].Open {
			return list[i].Open > list[j].Open
		}
		return list[i].Name < list[j].Name
	})
	return cols, list
}

func splitNames(text string) []string {
	parts := strings.Split(text, ",")
	names := make([]string, 0, len(parts))
	for _, part := range parts {
		if name := strings.TrimSpace(part); name != "" {
			names = append(names, name)
		}
	}
	return names
}

// Overdue lists open items whose due date is in the past, most late first.
func Overdue(snapshot BoardSnapshot, override ReportColumns, now time.Time) (ReportColumns, []ItemBrief) {
	cols := DetectReportColumns(snapshot.Columns, override)
	c := newClassifier(snapshot, cols, now)
	var list []ItemBrief
	for _, item := range snapshot.Items {
		if brief := c.brief(item, now); brief.DaysLate > 0 {
			list = append(list, brief)
		}
	}
	sort.Slice(list, func(i, j int) bool { return list[i].DaysLate > list[j].DaysLate })
	return cols, list
}

// Stale lists open items not updated for at least days.
func Stale(snapshot BoardSnapshot, override ReportColumns, days int, now time.Time) (ReportColumns, []ItemBrief) {
	cols := DetectReportColumns(snapshot.Columns, override)
	c := newClassifier(snapshot, cols, now)
	cutoff := now.AddDate(0, 0, -days)
	var list []ItemBrief
	for _, item := range snapshot.Items {
		if c.done(item) {
			continue
		}
		updated, err := time.Parse(time.RFC3339, item.UpdatedAt)
		if err != nil || updated.After(cutoff) {
			continue
		}
		list = append(list, c.brief(item, now))
	}
	sort.Slice(list, func(i, j int) bool { return list[i].Updated < list[j].Updated })
	return cols, list
}

// Standup is a daily digest of recent movement.
type Standup struct {
	WindowHours int         `json:"window_hours"`
	Updated     []ItemBrief `json:"recently_updated"`
	Completed   []ItemBrief `json:"recently_completed"`
	Blocked     []ItemBrief `json:"blocked"`
	Overdue     []ItemBrief `json:"overdue"`
	Markdown    string      `json:"markdown"`
}

// BuildStandup summarizes the last hours of board activity.
func BuildStandup(snapshot BoardSnapshot, override ReportColumns, hours int, now time.Time) Standup {
	cols := DetectReportColumns(snapshot.Columns, override)
	c := newClassifier(snapshot, cols, now)
	cutoff := now.Add(-time.Duration(hours) * time.Hour)
	result := Standup{WindowHours: hours}
	for _, item := range snapshot.Items {
		brief := c.brief(item, now)
		updated, err := time.Parse(time.RFC3339, item.UpdatedAt)
		recent := err == nil && updated.After(cutoff)
		switch {
		case recent && c.done(item):
			result.Completed = append(result.Completed, brief)
		case recent:
			result.Updated = append(result.Updated, brief)
		}
		if !c.done(item) && c.blocked(item) {
			result.Blocked = append(result.Blocked, brief)
		}
		if brief.DaysLate > 0 {
			result.Overdue = append(result.Overdue, brief)
		}
	}
	var md strings.Builder
	fmt.Fprintf(&md, "# Standup — %s\n\n_Window: last %dh · generated %s_\n", snapshot.Board.Name, hours, now.UTC().Format(time.RFC3339))
	section := func(title string, items []ItemBrief) {
		fmt.Fprintf(&md, "\n## %s (%d)\n\n", title, len(items))
		if len(items) == 0 {
			md.WriteString("- none\n")
		}
		for _, item := range items {
			fmt.Fprintf(&md, "- %s%s%s\n", item.Name, suffix(" · ", item.Status), suffix(" · @", item.Owner))
		}
	}
	section("✅ Completed", result.Completed)
	section("🔄 In motion", result.Updated)
	section("⛔ Blocked", result.Blocked)
	section("⏰ Overdue", result.Overdue)
	result.Markdown = md.String()
	return result
}

func suffix(prefix, value string) string {
	if value == "" {
		return ""
	}
	return prefix + value
}

// HealthReport scores a board and explains the score.
type HealthReport struct {
	Summary  BoardSummary `json:"summary"`
	Score    int          `json:"score"`
	Grade    string       `json:"grade"`
	Signals  []string     `json:"signals"`
	Markdown string       `json:"markdown"`
}

// BuildHealthReport computes a 0-100 health score with explicit penalties.
func BuildHealthReport(snapshot BoardSnapshot, override ReportColumns, staleDays int, now time.Time) HealthReport {
	summary := Summarize(snapshot, override, now)
	_, stale := Stale(snapshot, override, staleDays, now)
	open := summary.TotalItems - summary.DoneItems
	report := HealthReport{Summary: summary, Score: 100}
	penalty := func(count int, weight float64, message string) {
		if open == 0 || count == 0 {
			return
		}
		share := float64(count) / float64(open)
		report.Score -= int(math.Round(share * weight))
		report.Signals = append(report.Signals, fmt.Sprintf(message, count, round1(share*100)))
	}
	penalty(summary.OverdueItems, 40, "%d open items are overdue (%.1f%% of open work)")
	penalty(summary.BlockedItems, 25, "%d open items are blocked (%.1f%%)")
	penalty(len(stale), 20, "%d open items are stale (%.1f%%) — no update in "+fmt.Sprint(staleDays)+" days")
	penalty(summary.UnassignedOpen, 15, "%d open items have no owner (%.1f%%)")
	if report.Score < 0 {
		report.Score = 0
	}
	switch {
	case report.Score >= 85:
		report.Grade = "A"
	case report.Score >= 70:
		report.Grade = "B"
	case report.Score >= 50:
		report.Grade = "C"
	default:
		report.Grade = "D"
	}
	if len(report.Signals) == 0 {
		report.Signals = []string{"no risk signals detected"}
	}
	var md strings.Builder
	fmt.Fprintf(&md, "# Board health — %s\n\n**Score: %d/100 (grade %s)**\n\n", summary.BoardName, report.Score, report.Grade)
	fmt.Fprintf(&md, "| Metric | Value |\n|---|---:|\n| Items | %d |\n| Done | %d (%.1f%%) |\n| Overdue | %d |\n| Blocked | %d |\n| Stale (>%dd) | %d |\n| Unassigned open | %d |\n",
		summary.TotalItems, summary.DoneItems, summary.CompletionPct, summary.OverdueItems, summary.BlockedItems, staleDays, len(stale), summary.UnassignedOpen)
	if summary.Truncated {
		md.WriteString("\n> ⚠️ Item set was truncated by the configured limit; figures are a lower bound.\n")
	}
	md.WriteString("\n## Signals\n\n")
	for _, signal := range report.Signals {
		fmt.Fprintf(&md, "- %s\n", signal)
	}
	if len(summary.ByStatus) > 0 {
		md.WriteString("\n## Status distribution\n\n")
		for _, count := range summary.ByStatus {
			fmt.Fprintf(&md, "- %s: %d\n", count.Key, count.Count)
		}
	}
	report.Markdown = md.String()
	return report
}

// ExportMarkdown renders the board as a Markdown table grouped by group.
func ExportMarkdown(snapshot BoardSnapshot, columnIDs []string) string {
	columns := exportColumns(snapshot.Columns, columnIDs)
	var md strings.Builder
	fmt.Fprintf(&md, "# %s\n\n", snapshot.Board.Name)
	if snapshot.Board.Description != "" {
		fmt.Fprintf(&md, "%s\n\n", snapshot.Board.Description)
	}
	byGroup := map[string][]Item{}
	for _, item := range snapshot.Items {
		key := ""
		if item.Group != nil {
			key = item.Group.ID
		}
		byGroup[key] = append(byGroup[key], item)
	}
	header := "| Item |"
	divider := "|---|"
	for _, column := range columns {
		header += " " + escapeCell(column.Title) + " |"
		divider += "---|"
	}
	groups := snapshot.Groups
	if len(groups) == 0 {
		groups = []Group{{ID: "", Title: "Items"}}
	}
	for _, group := range groups {
		items := byGroup[group.ID]
		fmt.Fprintf(&md, "## %s (%d)\n\n", group.Title, len(items))
		if len(items) == 0 {
			md.WriteString("_empty_\n\n")
			continue
		}
		md.WriteString(header + "\n" + divider + "\n")
		for _, item := range items {
			row := "| " + escapeCell(item.Name) + " |"
			for _, column := range columns {
				row += " " + escapeCell(item.CellText(column.ID)) + " |"
			}
			md.WriteString(row + "\n")
		}
		md.WriteString("\n")
	}
	if snapshot.Truncated {
		md.WriteString("> ⚠️ Export truncated by the configured item limit.\n")
	}
	return md.String()
}

// ExportCSV renders the board as CSV with item metadata columns first.
func ExportCSV(snapshot BoardSnapshot, columnIDs []string) (string, error) {
	columns := exportColumns(snapshot.Columns, columnIDs)
	var buffer bytes.Buffer
	writer := csv.NewWriter(&buffer)
	header := []string{"item_id", "name", "group", "updated_at"}
	for _, column := range columns {
		header = append(header, column.Title)
	}
	if err := writer.Write(header); err != nil {
		return "", err
	}
	for _, item := range snapshot.Items {
		group := ""
		if item.Group != nil {
			group = item.Group.Title
		}
		row := []string{item.ID, item.Name, group, item.UpdatedAt}
		for _, column := range columns {
			row = append(row, item.CellText(column.ID))
		}
		if err := writer.Write(row); err != nil {
			return "", err
		}
	}
	writer.Flush()
	return buffer.String(), writer.Error()
}

func exportColumns(columns []Column, ids []string) []Column {
	wanted := map[string]bool{}
	for _, id := range ids {
		wanted[id] = true
	}
	var result []Column
	for _, column := range columns {
		if column.Type == "name" || column.Archived || column.Type == "subtasks" {
			continue
		}
		if len(wanted) > 0 && !wanted[column.ID] {
			continue
		}
		result = append(result, column)
	}
	return result
}

func escapeCell(s string) string {
	s = strings.ReplaceAll(s, "|", "\\|")
	return strings.ReplaceAll(strings.ReplaceAll(s, "\r", " "), "\n", " ")
}
