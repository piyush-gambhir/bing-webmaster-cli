package cmd

import (
	"context"
	"testing"
)

func TestPagerStopsAtTheTypeLimitAndComparesWholePages(t *testing.T) {
	var asked []int
	p := pager{page: 65534, all: true, maxPages: 100}
	if err := p.validate(65535); err != nil {
		t.Fatal(err)
	}
	rows, fetched, complete, err := p.walk(context.Background(), func(_ context.Context, page int) ([]map[string]any, int, error) {
		asked = append(asked, page)
		return []map[string]any{{"Url": "same-first"}, {"Url": page}}, 0, nil
	})
	if err != nil || complete || fetched != 2 || len(rows) != 4 || asked[len(asked)-1] != 65535 {
		t.Fatalf("rows=%d fetched=%d complete=%v err=%v asked=%v", len(rows), fetched, complete, err, asked)
	}
	q := pager{all: true, maxPages: 100}
	_ = q.validate(32767)
	calls := 0
	_, _, _, err = q.walk(context.Background(), func(context.Context, int) ([]map[string]any, int, error) {
		calls++
		return []map[string]any{{"Url": "a"}, {"Url": "b"}}, 0, nil
	})
	if err == nil || calls != 2 {
		t.Fatalf("identical pages must stop with an error after the repeat: calls=%d err=%v", calls, err)
	}
}
