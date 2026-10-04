package cmd

import (
	"fmt"
	"net/url"
	"sort"
	"strings"

	"github.com/piyush-gambhir/bing-webmaster-cli/cli-go/internal/client"
	"github.com/piyush-gambhir/bing-webmaster-cli/cli-go/internal/output"
	"github.com/piyush-gambhir/bing-webmaster-cli/cli-go/internal/registry"
	"github.com/spf13/cobra"
)

// opSpec turns one registry method into a command. args name the positional
// parameters; setup may register flags and returns a function that validates
// them and supplies the remaining parameters before any network call.
type opSpec struct {
	use, short, long, example string
	op                        string
	args                      []string
	cols                      []output.Col
	// confirm, when set, is the prompt for a destructive change; %s is the first argument.
	confirm string
	setup   func(*cobra.Command) func() (map[string]any, error)
	// validate checks positional arguments before any network call.
	validate func([]string) error
	// domainOK also accepts Bing's documented domain:example.com form for "url".
	domainOK bool
}

// annotate records the Bing methods a command calls; mutates and experimental
// follow from the registry so --read-only and the safety manifest cannot drift.
func annotate(c *cobra.Command, methods ...string) *cobra.Command {
	if c.Annotations == nil {
		c.Annotations = map[string]string{}
	}
	c.Annotations["ops"] = strings.Join(methods, ",")
	for _, m := range methods {
		op, ok := registry.Lookup(m)
		if !ok {
			panic("unknown Bing method " + m)
		}
		if op.Effect == registry.Write {
			c.Annotations["mutates"] = "true"
		}
		if op.Status == registry.Experimental {
			c.Annotations["experimental"] = "true"
		}
	}
	return c
}

func (a *app) opCmd(s opSpec) *cobra.Command {
	op, ok := registry.Lookup(s.op)
	if !ok {
		panic("unknown Bing method " + s.op)
	}
	c := &cobra.Command{Use: s.use, Short: s.short, Long: s.long, Example: s.example, Args: cobra.ExactArgs(len(s.args))}
	annotate(c, s.op)
	var extra func() (map[string]any, error)
	if s.setup != nil {
		extra = s.setup(c)
	}
	needsSite := false
	for _, prm := range op.Params {
		if prm.Name == "siteUrl" {
			needsSite = true
		}
	}
	c.RunE = func(cmd *cobra.Command, args []string) error {
		if s.validate != nil {
			if err := s.validate(args); err != nil {
				return err
			}
		}
		params := map[string]any{}
		for i, name := range s.args {
			if urlParams[name] {
				check := requireURL
				if s.domainOK && name == "url" {
					check = requireURLOrDomain
				}
				if err := check(strings.ToUpper(name), args[i]); err != nil {
					return err
				}
			}
			params[name] = args[i]
		}
		if extra != nil {
			more, err := extra()
			if err != nil {
				return err
			}
			for k, v := range more {
				params[k] = v
			}
		}
		cl, err := a.api()
		if err != nil {
			return err
		}
		if _, given := params["siteUrl"]; needsSite && !given {
			site, err := a.siteURL(cmd.Context(), cl)
			if err != nil {
				return err
			}
			params["siteUrl"] = site
		}
		if s.confirm != "" {
			subject := ""
			if len(args) > 0 {
				subject = args[0]
			} else if site, ok := params["siteUrl"].(string); ok {
				subject = site
			}
			if err := a.confirm(fmt.Sprintf(s.confirm, subject)); err != nil {
				return err
			}
		}
		result, err := cl.Call(cmd.Context(), s.op, params)
		if err != nil {
			return err
		}
		return a.emit(cl, s.op, params, result, s.cols)
	}
	return c
}

// emit prints a call's result: planned requests under --dry-run, a short
// acknowledgement for writes, and date-converted data for reads.
func (a *app) emit(cl *client.Client, method string, params map[string]any, result any, cols []output.Col) error {
	if len(cl.Planned) > 0 {
		return a.printPlanned(cl.Planned)
	}
	op, _ := registry.Lookup(method)
	if op.Effect == registry.Write {
		ack := map[string]any{"method": method, "status": "accepted by Bing"}
		if site, ok := params["siteUrl"]; ok {
			ack["site"] = site
		}
		if result != nil {
			ack["result"] = result
		}
		return a.print(ack)
	}
	if !a.raw {
		result = client.Clean(result)
	}
	// A single object with chosen columns is a one-row table, so tables never
	// dump every field (fetch get would otherwise print the whole document).
	if m, ok := result.(map[string]any); ok && len(cols) > 0 {
		return output.View(a.out, a.format, m, []map[string]any{m}, cols...)
	}
	return a.print(result, cols...)
}

// printPlanned shows --dry-run requests: one row per request in tables, the
// full requests (with bodies) in JSON and YAML.
func (a *app) printPlanned(planned []client.Planned) error {
	rows := make([]map[string]any, 0, len(planned))
	for _, p := range planned {
		rows = append(rows, map[string]any{"method": p.Method, "http": p.HTTP, "url": p.URL})
	}
	a.info("Dry run: nothing was sent. Use -o json to see the request bodies.")
	return output.View(a.out, a.format, map[string]any{"dry_run": true, "requests": planned}, rows, cols("method", "method", "http", "http", "url", "url")...)
}

// urlParams are positional parameters that must be absolute URLs. "page" is a
// URL wherever it is positional (stats page-queries and detail).
var urlParams = map[string]bool{"siteUrl": true, "url": true, "feedUrl": true, "masterUrl": true, "link": true, "page": true}

func col(header, key string) output.Col { return output.Col{Header: header, Key: key} }

// cols builds columns from header, key pairs.
func cols(pairs ...string) []output.Col {
	out := make([]output.Col, 0, len(pairs)/2)
	for i := 0; i+1 < len(pairs); i += 2 {
		out = append(out, col(pairs[i], pairs[i+1]))
	}
	return out
}

// requireURLOrDomain accepts an absolute URL or domain:example.com, the form
// Bing documents for its URL information methods.
func requireURLOrDomain(name, raw string) error {
	if host, ok := strings.CutPrefix(raw, "domain:"); ok {
		if host == "" || strings.ContainsAny(host, "/:?# ") || !strings.Contains(host, ".") {
			return fmt.Errorf("%s must be domain:example.com or an absolute http(s) URL, got %q", name, raw)
		}
		return nil
	}
	return requireURL(name, raw)
}

func requireURL(name, raw string) error {
	u, err := url.Parse(raw)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return fmt.Errorf("%s must be an absolute http(s) URL, got %q", name, raw)
	}
	return nil
}

// choice maps a flag value to Bing's numeric enum value.
func choice(flag, value string, options map[string]int) (int, error) {
	if v, ok := options[strings.ToLower(value)]; ok {
		return v, nil
	}
	names := make([]string, 0, len(options))
	for k := range options {
		names = append(names, k)
	}
	sort.Strings(names)
	return 0, fmt.Errorf("--%s must be one of %s", flag, strings.Join(names, ", "))
}

// enumName renders a numeric (or already named) enum value from a lookup table.
func enumName(v any, names map[string]string) any {
	if v == nil {
		return nil
	}
	key := fmt.Sprint(v)
	if name, ok := names[key]; ok {
		return name
	}
	return v
}
