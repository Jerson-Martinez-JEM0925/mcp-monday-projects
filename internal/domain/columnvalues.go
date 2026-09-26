package domain

import (
	"encoding/json"
	"fmt"
	"net/mail"
	"net/url"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

// ValidationIssue explains why one column value was rejected.
type ValidationIssue struct {
	ColumnID string `json:"column_id"`
	Type     string `json:"type,omitempty"`
	Message  string `json:"message"`
}

// ColumnChange describes the normalization of one input column value.
type ColumnChange struct {
	ColumnID   string `json:"column_id"`
	Title      string `json:"title"`
	Type       string `json:"type"`
	Input      any    `json:"input"`
	Normalized any    `json:"normalized"`
}

// ColumnValuePlan is the validated, monday-ready representation of a write.
type ColumnValuePlan struct {
	Valid      bool              `json:"valid"`
	Normalized map[string]any    `json:"normalized"`
	Changes    []ColumnChange    `json:"changes"`
	Issues     []ValidationIssue `json:"issues,omitempty"`
}

// ValidationError is returned when a write is rejected before reaching monday.
type ValidationError struct {
	Issues []ValidationIssue
}

func (e *ValidationError) Error() string {
	parts := make([]string, 0, len(e.Issues))
	for _, issue := range e.Issues {
		parts = append(parts, issue.ColumnID+": "+issue.Message)
	}
	return "column values rejected: " + strings.Join(parts, "; ")
}

// ReadOnlyColumnTypes cannot be written through column values.
var ReadOnlyColumnTypes = map[string]bool{
	"auto_number": true, "button": true, "creation_log": true, "formula": true,
	"item_id": true, "last_updated": true, "mirror": true, "progress": true,
	"subtasks": true, "time_tracking": true, "vote": true, "file": true,
	"doc": true, "direct_doc": true, "integration": true, "unsupported": true,
	"item_assignees": true, "group": true,
}

// StatusLabel is one entry of a status or dropdown column's settings.
type StatusLabel struct {
	ID            int    `json:"id"`
	Label         string `json:"label"`
	Index         int    `json:"index"`
	IsDone        bool   `json:"is_done"`
	IsDeactivated bool   `json:"is_deactivated"`
}

// Labels decodes status/dropdown labels from column settings.
func (c Column) Labels() []StatusLabel {
	if len(c.Settings) == 0 {
		return nil
	}
	raw, err := json.Marshal(map[string]any(c.Settings))
	if err != nil {
		return nil
	}
	var settings struct {
		Labels json.RawMessage `json:"labels"`
	}
	if json.Unmarshal(raw, &settings) != nil || len(settings.Labels) == 0 {
		return nil
	}
	var list []StatusLabel
	if json.Unmarshal(settings.Labels, &list) == nil {
		return list
	}
	// Legacy shape: {"0":"Working on it","1":"Done"}.
	var legacy map[string]string
	if json.Unmarshal(settings.Labels, &legacy) == nil {
		for key, label := range legacy {
			id, err := strconv.Atoi(key)
			if err == nil {
				list = append(list, StatusLabel{ID: id, Label: label, Index: id})
			}
		}
		sort.Slice(list, func(i, j int) bool { return list[i].ID < list[j].ID })
	}
	return list
}

// DoneLabels returns the labels monday flags as "done" for a status column.
func (c Column) DoneLabels() map[string]bool {
	done := map[string]bool{}
	for _, label := range c.Labels() {
		if label.IsDone {
			done[strings.ToLower(label.Label)] = true
		}
	}
	return done
}

var (
	dateRe = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}$`)
	timeRe = regexp.MustCompile(`^\d{2}:\d{2}(:\d{2})?$`)
	hourRe = regexp.MustCompile(`^(\d{1,2}):(\d{2})$`)
	iso2Re = regexp.MustCompile(`^[A-Za-z]{2}$`)
)

// PlanColumnValues validates friendly input against a board schema and
// returns monday-ready JSON values. It never contacts monday.
func PlanColumnValues(columns []Column, input map[string]any) ColumnValuePlan {
	byID := make(map[string]Column, len(columns))
	for _, column := range columns {
		byID[column.ID] = column
	}
	plan := ColumnValuePlan{Normalized: map[string]any{}}
	keys := make([]string, 0, len(input))
	for key := range input {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, id := range keys {
		value := input[id]
		column, ok := byID[id]
		if !ok {
			plan.Issues = append(plan.Issues, ValidationIssue{ColumnID: id, Message: "unknown column ID for this board"})
			continue
		}
		if column.Archived {
			plan.Issues = append(plan.Issues, ValidationIssue{ColumnID: id, Type: column.Type, Message: "column is archived"})
			continue
		}
		if ReadOnlyColumnTypes[column.Type] {
			plan.Issues = append(plan.Issues, ValidationIssue{ColumnID: id, Type: column.Type, Message: "column type is computed or read-only and cannot be written"})
			continue
		}
		normalized, err := NormalizeColumnValue(column, value)
		if err != nil {
			plan.Issues = append(plan.Issues, ValidationIssue{ColumnID: id, Type: column.Type, Message: err.Error()})
			continue
		}
		plan.Normalized[id] = normalized
		plan.Changes = append(plan.Changes, ColumnChange{ColumnID: id, Title: column.Title, Type: column.Type, Input: value, Normalized: normalized})
	}
	plan.Valid = len(plan.Issues) == 0
	return plan
}

// Err returns a ValidationError when the plan is invalid.
func (p ColumnValuePlan) Err() error {
	if p.Valid {
		return nil
	}
	return &ValidationError{Issues: p.Issues}
}

// NormalizeColumnValue converts one friendly value into monday's JSON shape.
// A nil value clears the column.
func NormalizeColumnValue(column Column, value any) (any, error) {
	if value == nil {
		switch column.Type {
		case "text", "numbers", "name":
			return "", nil
		default:
			return map[string]any{}, nil
		}
	}
	switch column.Type {
	case "name", "text":
		s, ok := value.(string)
		if !ok {
			return nil, fmt.Errorf("expected a string")
		}
		if column.Type == "name" && strings.TrimSpace(s) == "" {
			return nil, fmt.Errorf("item name cannot be empty")
		}
		return s, nil
	case "long_text":
		if m, ok := value.(map[string]any); ok {
			if _, ok := m["text"].(string); !ok {
				return nil, fmt.Errorf("expected {\"text\": string}")
			}
			return m, nil
		}
		s, ok := value.(string)
		if !ok {
			return nil, fmt.Errorf("expected a string")
		}
		return map[string]any{"text": s}, nil
	case "numbers":
		return normalizeNumber(value)
	case "status", "color":
		return normalizeStatus(column, value)
	case "dropdown":
		return normalizeDropdown(column, value)
	case "date":
		return normalizeDate(value)
	case "timeline":
		return normalizeTimeline(value)
	case "people", "person", "team":
		return normalizePeople(value)
	case "checkbox":
		b, ok := asBool(value)
		if !ok {
			return nil, fmt.Errorf("expected a boolean")
		}
		if !b {
			return nil, nil
		}
		return map[string]any{"checked": "true"}, nil
	case "email":
		return normalizeEmail(value)
	case "link":
		return normalizeLink(value)
	case "phone":
		return normalizePhone(value)
	case "rating":
		n, ok := asInt(value)
		if !ok || n < 0 || n > 5 {
			return nil, fmt.Errorf("expected an integer rating between 0 and 5")
		}
		return map[string]any{"rating": n}, nil
	case "hour":
		return normalizeHour(value)
	case "week":
		return normalizeWeek(value)
	case "country":
		return normalizeCountry(value)
	case "location":
		return normalizeLocation(value)
	case "world_clock":
		s, ok := value.(string)
		if !ok {
			return nil, fmt.Errorf("expected an IANA time zone string")
		}
		if _, err := time.LoadLocation(s); err != nil {
			return nil, fmt.Errorf("unknown IANA time zone %q", s)
		}
		return map[string]any{"timezone": s}, nil
	case "tags":
		ids, err := intList(value)
		if err != nil {
			return nil, fmt.Errorf("expected a list of tag IDs: %w", err)
		}
		return map[string]any{"tag_ids": ids}, nil
	case "board_relation", "dependency":
		ids, err := intList(value)
		if err != nil {
			return nil, fmt.Errorf("expected a list of item IDs: %w", err)
		}
		return map[string]any{"item_ids": ids}, nil
	case "color_picker":
		s, ok := value.(string)
		if !ok || !regexp.MustCompile(`^#[0-9A-Fa-f]{6}$`).MatchString(s) {
			return nil, fmt.Errorf("expected a hex color like #1F76C2")
		}
		return map[string]any{"color": map[string]any{"hex": s}}, nil
	default:
		if m, ok := value.(map[string]any); ok {
			return m, nil
		}
		return nil, fmt.Errorf("column type %q requires a raw JSON object value", column.Type)
	}
}

func normalizeNumber(value any) (any, error) {
	switch v := value.(type) {
	case float64:
		return strconv.FormatFloat(v, 'f', -1, 64), nil
	case int:
		return strconv.Itoa(v), nil
	case int64:
		return strconv.FormatInt(v, 10), nil
	case json.Number:
		return v.String(), nil
	case string:
		if _, err := strconv.ParseFloat(strings.TrimSpace(v), 64); err != nil {
			return nil, fmt.Errorf("expected a number, got %q", v)
		}
		return strings.TrimSpace(v), nil
	}
	return nil, fmt.Errorf("expected a number")
}

func normalizeStatus(column Column, value any) (any, error) {
	labels := column.Labels()
	findLabel := func(text string) (StatusLabel, bool) {
		for _, label := range labels {
			if strings.EqualFold(label.Label, strings.TrimSpace(text)) {
				return label, true
			}
		}
		return StatusLabel{}, false
	}
	findID := func(id int) (StatusLabel, bool) {
		for _, label := range labels {
			if label.ID == id {
				return label, true
			}
		}
		return StatusLabel{}, false
	}
	var chosen StatusLabel
	var found bool
	switch v := value.(type) {
	case string:
		if n, err := strconv.Atoi(v); err == nil {
			chosen, found = findID(n)
		} else {
			chosen, found = findLabel(v)
		}
	case map[string]any:
		if s, ok := v["label"].(string); ok {
			chosen, found = findLabel(s)
		} else if n, ok := asInt(v["index"]); ok {
			chosen, found = findID(n)
		} else {
			return nil, fmt.Errorf("expected {\"label\": string} or {\"index\": int}")
		}
	default:
		n, ok := asInt(value)
		if !ok {
			return nil, fmt.Errorf("expected a label string or label index")
		}
		chosen, found = findID(n)
	}
	if !found {
		return nil, fmt.Errorf("unknown status label; valid labels: %s", joinLabels(labels))
	}
	if chosen.IsDeactivated {
		return nil, fmt.Errorf("status label %q is deactivated", chosen.Label)
	}
	return map[string]any{"label": chosen.Label}, nil
}

func normalizeDropdown(column Column, value any) (any, error) {
	labels := column.Labels()
	valid := map[string]string{}
	validIDs := map[int]bool{}
	for _, label := range labels {
		if !label.IsDeactivated {
			valid[strings.ToLower(label.Label)] = label.Label
			validIDs[label.ID] = true
		}
	}
	var entries []any
	switch v := value.(type) {
	case string:
		entries = []any{v}
	case []any:
		entries = v
	case []string:
		for _, s := range v {
			entries = append(entries, s)
		}
	case map[string]any:
		return v, nil
	default:
		entries = []any{v}
	}
	if len(entries) == 0 {
		return map[string]any{}, nil
	}
	var names []string
	var ids []int
	for _, entry := range entries {
		if s, ok := entry.(string); ok {
			canonical, ok := valid[strings.ToLower(strings.TrimSpace(s))]
			if !ok {
				return nil, fmt.Errorf("unknown dropdown label %q; valid labels: %s", s, joinLabels(labels))
			}
			names = append(names, canonical)
			continue
		}
		n, ok := asInt(entry)
		if !ok || !validIDs[n] {
			return nil, fmt.Errorf("unknown dropdown label ID %v", entry)
		}
		ids = append(ids, n)
	}
	if len(names) > 0 && len(ids) > 0 {
		return nil, fmt.Errorf("mix of dropdown labels and IDs is not allowed")
	}
	if len(ids) > 0 {
		return map[string]any{"ids": ids}, nil
	}
	return map[string]any{"labels": names}, nil
}

func normalizeDate(value any) (any, error) {
	if m, ok := value.(map[string]any); ok {
		date, _ := m["date"].(string)
		if !validDate(date) {
			return nil, fmt.Errorf("expected {\"date\": \"YYYY-MM-DD\"}")
		}
		if t, ok := m["time"].(string); ok && !timeRe.MatchString(t) {
			return nil, fmt.Errorf("time must be HH:MM or HH:MM:SS")
		}
		return m, nil
	}
	s, ok := value.(string)
	if !ok {
		return nil, fmt.Errorf("expected a YYYY-MM-DD date string")
	}
	s = strings.TrimSpace(strings.Replace(s, "T", " ", 1))
	parts := strings.Fields(s)
	if len(parts) == 0 || !validDate(parts[0]) {
		return nil, fmt.Errorf("expected a YYYY-MM-DD date, got %q", s)
	}
	result := map[string]any{"date": parts[0]}
	if len(parts) > 1 {
		t := strings.TrimSuffix(parts[1], "Z")
		if !timeRe.MatchString(t) {
			return nil, fmt.Errorf("time must be HH:MM or HH:MM:SS")
		}
		if len(t) == 5 {
			t += ":00"
		}
		result["time"] = t
	}
	return result, nil
}

func validDate(s string) bool {
	if !dateRe.MatchString(s) {
		return false
	}
	_, err := time.Parse("2006-01-02", s)
	return err == nil
}

func normalizeTimeline(value any) (any, error) {
	m, ok := value.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("expected {\"from\": \"YYYY-MM-DD\", \"to\": \"YYYY-MM-DD\"}")
	}
	from, _ := m["from"].(string)
	to, _ := m["to"].(string)
	if !validDate(from) || !validDate(to) {
		return nil, fmt.Errorf("from and to must be YYYY-MM-DD dates")
	}
	if to < from {
		return nil, fmt.Errorf("timeline end %s is before start %s", to, from)
	}
	return map[string]any{"from": from, "to": to}, nil
}

func normalizePeople(value any) (any, error) {
	if m, ok := value.(map[string]any); ok {
		if _, ok := m["personsAndTeams"]; ok {
			return m, nil
		}
		return nil, fmt.Errorf("expected a list of user IDs or {\"personsAndTeams\": [...]}")
	}
	var entries []any
	switch v := value.(type) {
	case []any:
		entries = v
	default:
		entries = []any{v}
	}
	persons := make([]map[string]any, 0, len(entries))
	for _, entry := range entries {
		kind := "person"
		raw := entry
		if s, ok := entry.(string); ok && strings.HasPrefix(s, "team:") {
			kind = "team"
			raw = strings.TrimPrefix(s, "team:")
		}
		id, ok := asInt(raw)
		if !ok || id <= 0 {
			return nil, fmt.Errorf("invalid user ID %v (use numeric IDs, or \"team:<id>\" for teams)", entry)
		}
		persons = append(persons, map[string]any{"id": id, "kind": kind})
	}
	return map[string]any{"personsAndTeams": persons}, nil
}

func normalizeEmail(value any) (any, error) {
	var email, text string
	switch v := value.(type) {
	case string:
		email, text = v, v
	case map[string]any:
		email, _ = v["email"].(string)
		text, _ = v["text"].(string)
		if text == "" {
			text = email
		}
	default:
		return nil, fmt.Errorf("expected an email string")
	}
	if _, err := mail.ParseAddress(email); err != nil {
		return nil, fmt.Errorf("invalid email address %q", email)
	}
	return map[string]any{"email": email, "text": text}, nil
}

func normalizeLink(value any) (any, error) {
	var link, text string
	switch v := value.(type) {
	case string:
		link, text = v, v
	case map[string]any:
		link, _ = v["url"].(string)
		text, _ = v["text"].(string)
		if text == "" {
			text = link
		}
	default:
		return nil, fmt.Errorf("expected a URL string")
	}
	parsed, err := url.Parse(link)
	if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" {
		return nil, fmt.Errorf("invalid http(s) URL %q", link)
	}
	return map[string]any{"url": link, "text": text}, nil
}

func normalizePhone(value any) (any, error) {
	var phone, country string
	switch v := value.(type) {
	case string:
		if code, number, ok := strings.Cut(v, ":"); ok && iso2Re.MatchString(code) {
			country, phone = code, number
		} else {
			return nil, fmt.Errorf("expected \"CC:number\" (e.g. \"NI:+50588887777\") or {\"phone\", \"countryShortName\"}")
		}
	case map[string]any:
		phone, _ = v["phone"].(string)
		country, _ = v["countryShortName"].(string)
	default:
		return nil, fmt.Errorf("expected a phone object")
	}
	digits := strings.TrimPrefix(strings.ReplaceAll(phone, " ", ""), "+")
	if _, err := strconv.ParseUint(digits, 10, 64); err != nil || len(digits) < 6 {
		return nil, fmt.Errorf("invalid phone number %q", phone)
	}
	if !iso2Re.MatchString(country) {
		return nil, fmt.Errorf("countryShortName must be an ISO-3166 alpha-2 code")
	}
	return map[string]any{"phone": phone, "countryShortName": strings.ToUpper(country)}, nil
}

func normalizeHour(value any) (any, error) {
	if m, ok := value.(map[string]any); ok {
		h, okH := asInt(m["hour"])
		mi, okM := asInt(m["minute"])
		if !okH || !okM || h < 0 || h > 23 || mi < 0 || mi > 59 {
			return nil, fmt.Errorf("expected {\"hour\": 0-23, \"minute\": 0-59}")
		}
		return map[string]any{"hour": h, "minute": mi}, nil
	}
	s, ok := value.(string)
	match := hourRe.FindStringSubmatch(strings.TrimSpace(s))
	if !ok || match == nil {
		return nil, fmt.Errorf("expected an HH:MM string")
	}
	h, _ := strconv.Atoi(match[1])
	mi, _ := strconv.Atoi(match[2])
	if h > 23 || mi > 59 {
		return nil, fmt.Errorf("hour out of range")
	}
	return map[string]any{"hour": h, "minute": mi}, nil
}

func normalizeWeek(value any) (any, error) {
	if s, ok := value.(string); ok && validDate(s) {
		day, _ := time.Parse("2006-01-02", s)
		offset := (int(day.Weekday()) + 6) % 7 // Monday-based week
		start := day.AddDate(0, 0, -offset)
		return map[string]any{"week": map[string]any{"startDate": start.Format("2006-01-02"), "endDate": start.AddDate(0, 0, 6).Format("2006-01-02")}}, nil
	}
	m, ok := value.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("expected a YYYY-MM-DD date inside the week or {\"startDate\",\"endDate\"}")
	}
	if inner, ok := m["week"].(map[string]any); ok {
		m = inner
	}
	start, _ := m["startDate"].(string)
	end, _ := m["endDate"].(string)
	if !validDate(start) || !validDate(end) || end < start {
		return nil, fmt.Errorf("startDate and endDate must be ordered YYYY-MM-DD dates")
	}
	return map[string]any{"week": map[string]any{"startDate": start, "endDate": end}}, nil
}

func normalizeCountry(value any) (any, error) {
	m, ok := value.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("expected {\"countryCode\": \"NI\", \"countryName\": \"Nicaragua\"}")
	}
	code, _ := m["countryCode"].(string)
	name, _ := m["countryName"].(string)
	if !iso2Re.MatchString(code) || strings.TrimSpace(name) == "" {
		return nil, fmt.Errorf("countryCode must be ISO-3166 alpha-2 and countryName is required")
	}
	return map[string]any{"countryCode": strings.ToUpper(code), "countryName": name}, nil
}

func normalizeLocation(value any) (any, error) {
	m, ok := value.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("expected {\"lat\", \"lng\", \"address\"}")
	}
	lat, okLat := asFloat(m["lat"])
	lng, okLng := asFloat(m["lng"])
	if !okLat || !okLng || lat < -90 || lat > 90 || lng < -180 || lng > 180 {
		return nil, fmt.Errorf("lat must be -90..90 and lng -180..180")
	}
	address, _ := m["address"].(string)
	return map[string]any{"lat": strconv.FormatFloat(lat, 'f', -1, 64), "lng": strconv.FormatFloat(lng, 'f', -1, 64), "address": address}, nil
}

func joinLabels(labels []StatusLabel) string {
	names := make([]string, 0, len(labels))
	for _, label := range labels {
		if !label.IsDeactivated && label.Label != "" {
			names = append(names, fmt.Sprintf("%q", label.Label))
		}
	}
	if len(names) == 0 {
		return "(none)"
	}
	return strings.Join(names, ", ")
}

func asInt(value any) (int, bool) {
	switch v := value.(type) {
	case int:
		return v, true
	case int64:
		return int(v), true
	case float64:
		if v == float64(int(v)) {
			return int(v), true
		}
	case json.Number:
		n, err := v.Int64()
		return int(n), err == nil
	case string:
		n, err := strconv.Atoi(strings.TrimSpace(v))
		return n, err == nil
	}
	return 0, false
}

func asFloat(value any) (float64, bool) {
	switch v := value.(type) {
	case float64:
		return v, true
	case int:
		return float64(v), true
	case string:
		f, err := strconv.ParseFloat(v, 64)
		return f, err == nil
	}
	return 0, false
}

func asBool(value any) (bool, bool) {
	switch v := value.(type) {
	case bool:
		return v, true
	case string:
		b, err := strconv.ParseBool(v)
		return b, err == nil
	}
	return false, false
}

func intList(value any) ([]int, error) {
	var entries []any
	switch v := value.(type) {
	case []any:
		entries = v
	default:
		entries = []any{v}
	}
	ids := make([]int, 0, len(entries))
	for _, entry := range entries {
		n, ok := asInt(entry)
		if !ok || n <= 0 {
			return nil, fmt.Errorf("invalid ID %v", entry)
		}
		ids = append(ids, n)
	}
	return ids, nil
}

// ColumnFormat documents the friendly input accepted for one column type.
type ColumnFormat struct {
	Type     string `json:"type"`
	Writable bool   `json:"writable"`
	Accepts  string `json:"accepts"`
	Example  string `json:"example,omitempty"`
}

// ColumnFormats is the machine-readable catalog of supported inputs.
func ColumnFormats() []ColumnFormat {
	formats := []ColumnFormat{
		{"name", true, "non-empty string", `"Deploy v2"`},
		{"text", true, "string", `"release notes"`},
		{"long_text", true, "string or {text}", `"multi-line text"`},
		{"numbers", true, "number or numeric string", `42.5`},
		{"status", true, "label text, label ID, {label} or {index}; validated against board labels", `"Done"`},
		{"dropdown", true, "label, list of labels, or list of label IDs; validated", `["npm","pip"]`},
		{"date", true, "YYYY-MM-DD, 'YYYY-MM-DD HH:MM', or {date,time}", `"2026-10-01"`},
		{"timeline", true, "{from,to} ordered dates", `{"from":"2026-10-01","to":"2026-10-15"}`},
		{"people", true, "user ID, list of user IDs, 'team:<id>', or {personsAndTeams}", `[28744356]`},
		{"checkbox", true, "boolean", `true`},
		{"email", true, "address or {email,text}", `"ops@example.com"`},
		{"link", true, "http(s) URL or {url,text}", `{"url":"https://example.com","text":"Runbook"}`},
		{"phone", true, "'CC:number' or {phone,countryShortName}", `"NI:+50588887777"`},
		{"rating", true, "integer 0-5", `4`},
		{"hour", true, "HH:MM or {hour,minute}", `"09:30"`},
		{"week", true, "any date inside the week, or {startDate,endDate}", `"2026-10-01"`},
		{"country", true, "{countryCode,countryName}", `{"countryCode":"NI","countryName":"Nicaragua"}`},
		{"location", true, "{lat,lng,address}", `{"lat":12.13,"lng":-86.25,"address":"Managua"}`},
		{"world_clock", true, "IANA time zone", `"America/Managua"`},
		{"tags", true, "list of tag IDs", `[123]`},
		{"board_relation", true, "list of item IDs", `[1234567890]`},
		{"dependency", true, "list of item IDs", `[1234567890]`},
		{"color_picker", true, "hex color", `"#1F76C2"`},
	}
	readOnly := make([]string, 0, len(ReadOnlyColumnTypes))
	for kind := range ReadOnlyColumnTypes {
		readOnly = append(readOnly, kind)
	}
	sort.Strings(readOnly)
	for _, kind := range readOnly {
		formats = append(formats, ColumnFormat{Type: kind, Writable: false, Accepts: "computed or read-only; not writable through column values"})
	}
	return formats
}
