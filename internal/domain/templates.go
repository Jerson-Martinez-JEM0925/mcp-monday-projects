package domain

import (
	"fmt"
	"sort"
	"strings"
)

// TemplateColumn declares one column of a board template.
type TemplateColumn struct {
	ID          string   `json:"id,omitempty"`
	Title       string   `json:"title"`
	Type        string   `json:"type"`
	Description string   `json:"description,omitempty"`
	Labels      []string `json:"labels,omitempty"`
	DoneLabel   string   `json:"done_label,omitempty"`
}

// TemplateItem is a seed item created by a template.
type TemplateItem struct {
	Group  string         `json:"group"`
	Name   string         `json:"name"`
	Values map[string]any `json:"values,omitempty"`
}

// BoardTemplate is a declarative board blueprint.
type BoardTemplate struct {
	Key         string           `json:"key"`
	Title       string           `json:"title"`
	Description string           `json:"description"`
	Groups      []string         `json:"groups"`
	Columns     []TemplateColumn `json:"columns"`
	SeedItems   []TemplateItem   `json:"seed_items,omitempty"`
}

var boardTemplates = map[string]BoardTemplate{
	"devops": {
		Key:         "devops",
		Title:       "DevOps delivery board",
		Description: "Kanban for platform/DevOps work: status, priority, owner, environment, due date, estimate, and PR link.",
		Groups:      []string{"Backlog", "In Progress", "Code Review", "Done"},
		Columns: []TemplateColumn{
			{ID: "status", Title: "Status", Type: "status", Labels: []string{"To Do", "Working on it", "Stuck", "Done"}, DoneLabel: "Done"},
			{ID: "priority", Title: "Priority", Type: "status", Labels: []string{"Critical", "High", "Medium", "Low"}},
			{ID: "owner", Title: "Owner", Type: "people"},
			{ID: "environment", Title: "Environment", Type: "dropdown", Labels: []string{"dev", "staging", "prod"}},
			{ID: "due_date", Title: "Due date", Type: "date"},
			{ID: "estimate", Title: "Estimate (pts)", Type: "numbers"},
			{ID: "pr_link", Title: "Pull request", Type: "link"},
			{ID: "notes", Title: "Notes", Type: "long_text"},
		},
	},
	"incident": {
		Key:         "incident",
		Title:       "Incident response board",
		Description: "Track incidents from detection to postmortem with severity, commander, timeline, and runbook link.",
		Groups:      []string{"Open", "Mitigated", "Resolved"},
		Columns: []TemplateColumn{
			{ID: "status", Title: "Status", Type: "status", Labels: []string{"Investigating", "Mitigating", "Monitoring", "Resolved"}, DoneLabel: "Resolved"},
			{ID: "severity", Title: "Severity", Type: "status", Labels: []string{"SEV1", "SEV2", "SEV3", "SEV4"}},
			{ID: "commander", Title: "Incident commander", Type: "people"},
			{ID: "detected", Title: "Detected", Type: "date"},
			{ID: "impact", Title: "Impact window", Type: "timeline"},
			{ID: "runbook", Title: "Runbook", Type: "link"},
			{ID: "summary", Title: "Summary", Type: "long_text"},
		},
	},
	"release": {
		Key:         "release",
		Title:       "Release train board",
		Description: "Plan releases with version, target date, readiness checkbox, and owner.",
		Groups:      []string{"Planned", "Release candidate", "Shipped"},
		Columns: []TemplateColumn{
			{ID: "status", Title: "Status", Type: "status", Labels: []string{"Planned", "In QA", "Blocked", "Shipped"}, DoneLabel: "Shipped"},
			{ID: "version", Title: "Version", Type: "text"},
			{ID: "owner", Title: "Release owner", Type: "people"},
			{ID: "target_date", Title: "Target date", Type: "date"},
			{ID: "ready", Title: "Go/No-go ready", Type: "checkbox"},
			{ID: "notes_link", Title: "Release notes", Type: "link"},
		},
	},
	"project": {
		Key:         "project",
		Title:       "Project tracker",
		Description: "General project tracker with status, owner, timeline, and priority.",
		Groups:      []string{"To Do", "Doing", "Done"},
		Columns: []TemplateColumn{
			{ID: "status", Title: "Status", Type: "status", Labels: []string{"Not started", "Working on it", "Stuck", "Done"}, DoneLabel: "Done"},
			{ID: "owner", Title: "Owner", Type: "people"},
			{ID: "timeline", Title: "Timeline", Type: "timeline"},
			{ID: "priority", Title: "Priority", Type: "status", Labels: []string{"High", "Medium", "Low"}},
		},
	},
}

// BoardTemplates returns every built-in template sorted by key.
func BoardTemplates() []BoardTemplate {
	list := make([]BoardTemplate, 0, len(boardTemplates))
	for _, template := range boardTemplates {
		list = append(list, template)
	}
	sort.Slice(list, func(i, j int) bool { return list[i].Key < list[j].Key })
	return list
}

// LookupBoardTemplate returns a template by key.
func LookupBoardTemplate(key string) (BoardTemplate, error) {
	template, ok := boardTemplates[key]
	if !ok {
		keys := make([]string, 0, len(boardTemplates))
		for k := range boardTemplates {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		return BoardTemplate{}, fmt.Errorf("unknown board template %q; available: %v", key, keys)
	}
	return template, nil
}

// ColumnDefaults returns monday create_column defaults for a labelled column,
// in the format monday accepts on write: status labels are keyed by color
// index; dropdown labels use {"settings":{"labels":[{id,name}]}}.
func ColumnDefaults(c TemplateColumn) map[string]any {
	if len(c.Labels) == 0 {
		return nil
	}
	if c.Type == "dropdown" {
		labels := make([]map[string]any, 0, len(c.Labels))
		for index, label := range c.Labels {
			labels = append(labels, map[string]any{"id": index + 1, "name": label})
		}
		return map[string]any{"settings": map[string]any{"labels": labels}}
	}
	used := map[int]bool{}
	byColor := map[string]any{}
	fallback := []int{6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19}
	for _, label := range c.Labels {
		color := semanticColor(label, label == c.DoneLabel)
		if color < 0 || used[color] {
			color = -1
			for _, candidate := range fallback {
				if !used[candidate] {
					color = candidate
					break
				}
			}
		}
		used[color] = true
		byColor[fmt.Sprint(color)] = label
	}
	return map[string]any{"labels": byColor}
}

// semanticColor maps a label to monday's classic color index.
func semanticColor(label string, done bool) int {
	l := strings.ToLower(label)
	has := func(words ...string) bool {
		for _, word := range words {
			if strings.Contains(l, word) {
				return true
			}
		}
		return false
	}
	switch {
	case done || has("done", "resolved", "shipped", "listo", "hecho", "completado"):
		return 1 // green
	case has("stuck", "blocked", "critical", "sev1", "bloque"):
		return 2 // red
	case has("working", "progress", "mitigat", "in qa", "doing", "curso"):
		return 0 // orange
	case has("high", "sev2"):
		return 4 // purple
	case has("medium", "sev3", "monitor"):
		return 3 // blue
	case has("low", "sev4", "not started", "to do", "planned", "backlog"):
		return 5 // grey
	}
	return -1
}
