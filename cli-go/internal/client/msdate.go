package client

import (
	"fmt"
	"regexp"
	"strconv"
	"time"
)

var msDate = regexp.MustCompile(`^/Date\((-?\d+)([+-]\d{4})?\)/$`)

func ParseMSDate(s string) (time.Time, bool) {
	m := msDate.FindStringSubmatch(s)
	if m == nil {
		return time.Time{}, false
	}
	ms, err := strconv.ParseInt(m[1], 10, 64)
	if err != nil {
		return time.Time{}, false
	}
	t := time.UnixMilli(ms).UTC()
	if !rfc3339Year(t) {
		return time.Time{}, false
	}
	if m[2] != "" {
		hours, _ := strconv.Atoi(m[2][1:3])
		minutes, _ := strconv.Atoi(m[2][3:5])
		if hours > 23 || minutes > 59 {
			return time.Time{}, false
		}
		offset := hours*3600 + minutes*60
		if m[2][0] == '-' {
			offset = -offset
		}
		t = t.In(time.FixedZone("", offset))
	}
	return t, rfc3339Year(t)
}

func rfc3339Year(t time.Time) bool { return t.Year() >= 0 && t.Year() <= 9999 }

func FormatMSDate(t time.Time) string {
	return fmt.Sprintf("/Date(%d+0000)/", t.UnixMilli())
}

func ConvertDates(v any) any {
	switch x := v.(type) {
	case map[string]any:
		out := make(map[string]any, len(x))
		for k, val := range x {
			out[k] = ConvertDates(val)
		}
		return out
	case []any:
		out := make([]any, len(x))
		for i, val := range x {
			out[i] = ConvertDates(val)
		}
		return out
	case string:
		if t, ok := ParseMSDate(x); ok {
			return t.Format(time.RFC3339Nano)
		}
		return x
	default:
		return v
	}
}

func Clean(v any) any { return stripType(ConvertDates(v)) }

func stripType(v any) any {
	switch x := v.(type) {
	case map[string]any:
		out := make(map[string]any, len(x))
		for k, val := range x {
			if k != "__type" {
				out[k] = stripType(val)
			}
		}
		return out
	case []any:
		out := make([]any, len(x))
		for i, val := range x {
			out[i] = stripType(val)
		}
		return out
	default:
		return v
	}
}
