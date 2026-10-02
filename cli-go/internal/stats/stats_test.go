package stats

import (
	"encoding/json"
	"strconv"
	"testing"
	"time"
)

func row(date string, clicks, impressions int) map[string]any {
	return map[string]any{"Date": date, "Clicks": json.Number(strconv.Itoa(clicks)), "Impressions": json.Number(strconv.Itoa(impressions))}
}

func day(s string) time.Time { t, _ := time.Parse("2006-01-02", s); return t }

func TestDayKeepsBingOffset(t *testing.T) {
	// 23:30 at -07:00 is still September 1 for Bing, even though it is September 2 in UTC.
	d, ok := Day(map[string]any{"Date": "2026-09-01T23:30:00-07:00"})
	if !ok || d.Format("2006-01-02") != "2026-09-01" {
		t.Fatal(d)
	}
	if d, ok := Day(map[string]any{"Date": "/Date(0)/"}); !ok || d.Year() != 1970 {
		t.Fatal("raw MS date")
	}
}

func TestApplyFiltersSortsLimits(t *testing.T) {
	rows := []map[string]any{row("2026-09-01T00:00:00-07:00", 5, 100), row("2026-09-02T00:00:00-07:00", 9, 10), row("2026-09-03T00:00:00-07:00", 1, 0)}
	got, err := Apply(rows, Filter{Since: day("2026-09-02"), Sort: "clicks"})
	if err != nil || len(got) != 2 || got[0]["Clicks"].(json.Number) != "9" {
		t.Fatalf("%v %v", got, err)
	}
	got, _ = Apply(rows, Filter{Sort: "ctr", Limit: 2})
	if len(got) != 2 || got[0]["Clicks"].(json.Number) != "9" {
		t.Fatalf("ctr sort: %v", got)
	}
	if _, err := Apply(rows, Filter{Sort: "nope"}); err == nil {
		t.Fatal("bad sort accepted")
	}
	if CTR(rows[2]) != nil || CTR(row("x", 12, 10)).(float64) != 1.2 {
		t.Fatal("ctr must be nil for zero impressions and unclamped otherwise")
	}
}

func TestWindowAndCompare(t *testing.T) {
	var rows []map[string]any
	for d := 1; d <= 14; d++ {
		rows = append(rows, row(time.Date(2026, 9, d, 0, 0, 0, 0, time.UTC).Format(time.RFC3339), d, 100))
	}
	end, _ := Latest(rows)
	now := Window(rows, end, 7)
	before := Window(rows, end.AddDate(0, 0, -7), 7)
	if now.Start != "2026-09-08" || now.End != "2026-09-14" || now.DaysWithData != 7 || now.Clicks != 77 || now.Impressions != 700 {
		t.Fatalf("window: %+v", now)
	}
	if before.Clicks != 28 {
		t.Fatalf("previous: %+v", before)
	}
	c := Compare(now, before)
	if c.ClicksPct.(float64) != 175 || c.ImpressionsPct.(float64) != 0 {
		t.Fatalf("change: %+v", c)
	}
	if pp := c.CTRPoints.(float64); pp < 6.99 || pp > 7.01 {
		t.Fatalf("ctr pp: %v", pp)
	}
	empty := Window(nil, end, 7)
	if Compare(now, empty).ClicksPct != nil || empty.CTR != nil {
		t.Fatal("zero base must give null percentages")
	}
}
