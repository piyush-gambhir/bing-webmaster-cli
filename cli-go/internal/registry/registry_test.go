package registry

import (
	"encoding/json"
	"os"
	"regexp"
	"strings"
	"testing"
)

type snapshot struct {
	Methods []struct {
		Method    string `json:"method"`
		HTTP      string `json:"http"`
		Obsolete  bool   `json:"obsolete"`
		Signature string `json:"signature"`
	} `json:"methods"`
}

func loadSnapshot(t *testing.T) snapshot {
	t.Helper()
	b, err := os.ReadFile("testdata/bing-webmaster-api.methods.json")
	if err != nil {
		t.Fatal(err)
	}
	var s snapshot
	if err := json.Unmarshal(b, &s); err != nil {
		t.Fatal(err)
	}
	if len(s.Methods) != 62 {
		t.Fatalf("snapshot has %d methods, want 62", len(s.Methods))
	}
	return s
}

var paramList = regexp.MustCompile(`\(([^()]*)\);\s*$`)

// The registry must describe exactly the methods in the vendored documentation
// snapshot, with the same HTTP verb, parameter names, and obsolete markers.
func TestRegistryMatchesSnapshot(t *testing.T) {
	s := loadSnapshot(t)
	if len(Ops) != len(s.Methods) {
		t.Fatalf("registry has %d methods, snapshot %d", len(Ops), len(s.Methods))
	}
	for _, m := range s.Methods {
		op, ok := Lookup(m.Method)
		if !ok {
			t.Errorf("%s missing from registry", m.Method)
			continue
		}
		if op.HTTP != m.HTTP {
			t.Errorf("%s: registry %s, documentation %s", m.Method, op.HTTP, m.HTTP)
		}
		if (op.Status == Obsolete) != m.Obsolete {
			t.Errorf("%s: obsolete mismatch (registry %s)", m.Method, op.Status)
		}
		match := paramList.FindStringSubmatch(m.Signature)
		if match == nil {
			t.Errorf("%s: cannot parse signature %q", m.Method, m.Signature)
			continue
		}
		var want []string
		for _, part := range strings.Split(match[1], ",") {
			fields := strings.Fields(part)
			if len(fields) > 0 {
				want = append(want, fields[len(fields)-1])
			}
		}
		var got []string
		for _, prm := range op.Params {
			got = append(got, prm.Name)
		}
		if strings.Join(got, ",") != strings.Join(want, ",") {
			t.Errorf("%s params: registry %v, documentation %v", m.Method, got, want)
		}
	}
}

// Effects follow behavior: GET methods only read, and every POST changes state
// except GetChildrenUrlInfo, which documents a read over POST.
func TestEffects(t *testing.T) {
	for _, op := range Ops {
		switch {
		case op.HTTP == "GET" && op.Effect != Read:
			t.Errorf("%s is GET but classified %s", op.Name, op.Effect)
		case op.HTTP == "POST" && op.Name == "GetChildrenUrlInfo" && op.Effect != Read:
			t.Errorf("GetChildrenUrlInfo must be a read")
		case op.HTTP == "POST" && op.Name != "GetChildrenUrlInfo" && op.Effect != Write:
			t.Errorf("%s is POST but classified %s", op.Name, op.Effect)
		}
		if op.Group == "" || op.Returns == "" {
			t.Errorf("%s lacks group or return type", op.Name)
		}
	}
}

func TestCounts(t *testing.T) {
	counts := map[Status]int{}
	for _, op := range Ops {
		counts[op.Status]++
	}
	if counts[Obsolete] != 3 || counts[Implemented]+counts[Experimental] != 59 {
		t.Fatalf("status counts %v", counts)
	}
}
