package client

import (
	"net/http"
	"strings"
	"testing"
	"time"
)

// Fuzz targets for the code that parses untrusted responses. `go test` runs the
// seeds; `go test -fuzz=FuzzParseMSDate ./internal/client` explores further.

func FuzzParseMSDate(f *testing.F) {
	for _, s := range []string{"/Date(1315349995284-0700)/", "/Date(0)/", "/Date(-62135596800000+0000)/", "/Date(-62135596800000-0700)/",
		"/Date(253402300799999+0000)/", "/Date(9223372036854775807)/", "/Date(-1+9999)/", "Date(1)", ""} {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, s string) {
		got, ok := ParseMSDate(s)
		if !ok {
			return
		}
		text := got.Format(time.RFC3339Nano)
		if _, err := time.Parse(time.RFC3339Nano, text); err != nil {
			t.Fatalf("%q became %q, which is not RFC 3339: %v", s, text, err)
		}
		if back, ok := ParseMSDate(FormatMSDate(got)); !ok || !back.Equal(got) {
			t.Fatalf("%q does not round-trip: %v became %v (ok=%v)", s, got, back, ok)
		}
	})
}

func FuzzDecodeNeverLeaksTheKey(f *testing.F) {
	const key = "0123abcdKEY"
	f.Add(200, []byte(`{"d":[{"Url":"https://example.com/","Date":"/Date(0)/"}]}`))
	f.Add(400, []byte(`{"ErrorCode":3,"Message":"InvalidApiKey 0123abcdKEY"}`))
	f.Add(200, []byte(`{"ErrorCode":2,"Message":"0123abcdKEY"}`))
	f.Add(502, []byte("<html>apikey=0123abcdKEY</html>"))
	f.Add(200, []byte(`{"d":1} trailing`))
	f.Fuzz(func(t *testing.T, status int, body []byte) {
		status = 100 + (status%500+500)%500
		c := &Client{APIKey: key}
		data, err := c.decode("GetUserSites", &http.Response{StatusCode: status, Header: http.Header{}}, body)
		if err != nil {
			if strings.Contains(err.Error(), key) {
				t.Fatalf("error leaks the key: %v", err)
			}
			return
		}
		_ = Clean(data)
	})
}
