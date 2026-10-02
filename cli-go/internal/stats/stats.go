// Package stats filters, sorts, and summarizes the rows Bing's performance
// methods return. Bing's API has no server-side date range, limit, or sort, so
// everything here happens locally and callers must say so in their output.
package stats

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/piyush-gambhir/bing-webmaster-cli/cli-go/internal/client"
)

// Num reads a JSON number (or numeric string) as float64.
func Num(v any) (float64, bool) {
	switch x := v.(type) {
	case json.Number:
		f, err := x.Float64()
		return f, err == nil
	case float64:
		return x, true
	case int:
		return float64(x), true
	}
	return 0, false
}

// Day returns the calendar date of a row's Date field in the offset Bing sent,
// accepting RFC 3339 (converted) or Microsoft JSON dates (raw).
func Day(row map[string]any) (time.Time, bool) {
	s, ok := row["Date"].(string)
	if !ok {
		return time.Time{}, false
	}
	t, err := time.Parse(time.RFC3339Nano, s)
	if err != nil {
		var ok bool
		if t, ok = client.ParseMSDate(s); !ok {
			return time.Time{}, false
		}
	}
	y, m, d := t.Date()
	return time.Date(y, m, d, 0, 0, 0, 0, time.UTC), true
}

// CTR is clicks over impressions, or nil when impressions are zero or missing.
// Values are not clamped: Bing has been seen returning more clicks than impressions.
func CTR(row map[string]any) any {
	clicks, ok1 := Num(row["Clicks"])
	impressions, ok2 := Num(row["Impressions"])
	if !ok1 || !ok2 || impressions <= 0 {
		return nil
	}
	return clicks / impressions
}

// SortKeys maps --sort names to row fields.
var SortKeys = map[string]string{
	"date": "Date", "clicks": "Clicks", "impressions": "Impressions", "ctr": "", "query": "Query",
	"avg-click-position": "AvgClickPosition", "avg-impression-position": "AvgImpressionPosition", "position": "Position",
}

type Filter struct {
	Since, Until time.Time // inclusive calendar days; zero means unbounded
	Limit        int
	Sort         string
	Ascending    bool
}

func (f Filter) Active() bool {
	return !f.Since.IsZero() || !f.Until.IsZero() || f.Limit > 0 || f.Sort != ""
}

// Apply filters by day, sorts, and limits rows locally.
func Apply(rows []map[string]any, f Filter) ([]map[string]any, error) {
	out := make([]map[string]any, 0, len(rows))
	for _, r := range rows {
		if !f.Since.IsZero() || !f.Until.IsZero() {
			d, ok := Day(r)
			if !ok || (!f.Since.IsZero() && d.Before(f.Since)) || (!f.Until.IsZero() && d.After(f.Until)) {
				continue
			}
		}
		out = append(out, r)
	}
	if f.Sort != "" {
		field, ok := SortKeys[f.Sort]
		if !ok {
			return nil, fmt.Errorf("unknown --sort %q", f.Sort)
		}
		key := func(r map[string]any) (float64, string, bool) {
			if f.Sort == "ctr" {
				v, ok := CTR(r).(float64)
				return v, "", ok
			}
			if f.Sort == "date" {
				d, ok := Day(r)
				return float64(d.Unix()), "", ok
			}
			if n, ok := Num(r[field]); ok {
				return n, "", true
			}
			s, ok := r[field].(string)
			return 0, strings.ToLower(s), ok
		}
		sort.SliceStable(out, func(i, j int) bool {
			ni, si, oki := key(out[i])
			nj, sj, okj := key(out[j])
			if oki != okj {
				return oki // rows missing the field sort last
			}
			less := ni < nj || (ni == nj && si < sj)
			if f.Ascending {
				return less
			}
			return ni > nj || (ni == nj && si > sj)
		})
	}
	if f.Limit > 0 && len(out) > f.Limit {
		out = out[:f.Limit]
	}
	return out, nil
}

// Period is a window of daily totals.
type Period struct {
	Start        string  `json:"start" yaml:"start"`
	End          string  `json:"end" yaml:"end"`
	DaysWithData int     `json:"days_with_data" yaml:"days_with_data"`
	Clicks       float64 `json:"clicks" yaml:"clicks"`
	Impressions  float64 `json:"impressions" yaml:"impressions"`
	CTR          any     `json:"ctr" yaml:"ctr"`
}

// Change compares two periods. Percentages are nil when the earlier value is
// zero; CTR change is in percentage points.
type Change struct {
	ClicksPct      any `json:"clicks_pct" yaml:"clicks_pct"`
	ImpressionsPct any `json:"impressions_pct" yaml:"impressions_pct"`
	CTRPoints      any `json:"ctr_change_pp" yaml:"ctr_change_pp"`
}

// Window sums daily rows over the n calendar days ending at end.
func Window(rows []map[string]any, end time.Time, n int) Period {
	start := end.AddDate(0, 0, -(n - 1))
	p := Period{Start: start.Format("2006-01-02"), End: end.Format("2006-01-02")}
	days := map[string]bool{}
	for _, r := range rows {
		d, ok := Day(r)
		if !ok || d.Before(start) || d.After(end) {
			continue
		}
		c, _ := Num(r["Clicks"])
		i, _ := Num(r["Impressions"])
		p.Clicks += c
		p.Impressions += i
		days[d.Format("2006-01-02")] = true
	}
	p.DaysWithData = len(days)
	if p.Impressions > 0 {
		p.CTR = p.Clicks / p.Impressions
	}
	return p
}

// Latest is the most recent day present in rows.
func Latest(rows []map[string]any) (time.Time, bool) {
	var latest time.Time
	found := false
	for _, r := range rows {
		if d, ok := Day(r); ok && (!found || d.After(latest)) {
			latest, found = d, true
		}
	}
	return latest, found
}

func pct(now, before float64) any {
	if before == 0 {
		return nil
	}
	return (now - before) / before * 100
}

func Compare(now, before Period) Change {
	c := Change{ClicksPct: pct(now.Clicks, before.Clicks), ImpressionsPct: pct(now.Impressions, before.Impressions)}
	a, okA := now.CTR.(float64)
	b, okB := before.CTR.(float64)
	if okA && okB {
		c.CTRPoints = (a - b) * 100
	}
	return c
}
