package client

import (
	"fmt"
	"regexp"
	"strconv"
	"time"
)

// Microsoft (WCF) JSON dates look like /Date(1315349995284-0700)/: milliseconds
// since the Unix epoch, optionally followed by the sender's UTC offset. The
// offset describes the local time of that instant; it is not added again.
var msDate = regexp.MustCompile(`^/Date\((-?\d+)([+-]\d{4})?\)/$`)

// ParseMSDate converts a Microsoft JSON date. The result keeps the offset as its
// zone so RFC 3339 output shows the same wall clock Bing used.
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
	if m[2] != "" {
		hours, _ := strconv.Atoi(m[2][1:3])
		minutes, _ := strconv.Atoi(m[2][3:5])
		offset := hours*3600 + minutes*60
		if m[2][0] == '-' {
			offset = -offset
		}
		t = t.In(time.FixedZone("", offset))
	}
	return t, true
}

// FormatMSDate renders t for request bodies, using UTC.
func FormatMSDate(t time.Time) string {
	return fmt.Sprintf("/Date(%d+0000)/", t.UnixMilli())
}

// ConvertDates returns a copy of v with every Microsoft JSON date string replaced
// by RFC 3339. Other values are unchanged.
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
