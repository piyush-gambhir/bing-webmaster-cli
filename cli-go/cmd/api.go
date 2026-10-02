package cmd

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/piyush-gambhir/bing-webmaster-cli/cli-go/internal/registry"
	"github.com/spf13/cobra"
)

func (a *app) apiCmd() *cobra.Command {
	var params []string
	var data, verb string
	c := &cobra.Command{Use: "api METHOD", Short: "Call any Bing Webmaster API method directly", Args: cobra.ExactArgs(1),
		Annotations: map[string]string{"effect": "from registry; unknown methods count as writes"},
		Long: "Sends an authenticated request to https://ssl.bing.com/webmaster/api.svc/json/METHOD with the same\n" +
			"redaction, fault handling, and date conversion as other commands. Known methods use their documented\n" +
			"HTTP verb and effect; unknown methods need --http and count as writes, so --read-only refuses them.\n" +
			"Writes ask for confirmation (or --yes): a raw call can remove sites, sitemaps, or users.\n" +
			"--param values are sent as strings; use --data for typed JSON values (POST only).",
		Example: "  bwt api GetUserSites\n  bwt api GetQueryStats --param siteUrl=https://example.com/\n" +
			"  bwt api SubmitUrlBatch --data '{\"siteUrl\":\"https://example.com/\",\"urlList\":[\"https://example.com/a\"]}'"}
	c.Flags().StringArrayVar(&params, "param", nil, "Parameter as name=value (repeatable)")
	c.Flags().StringVar(&data, "data", "", "JSON object of parameters (POST)")
	c.Flags().StringVar(&verb, "http", "", "HTTP method for methods not in the registry: GET or POST")
	c.RunE = func(cmd *cobra.Command, args []string) error {
		method := args[0]
		op, known := registry.Lookup(method)
		effect := registry.Write
		switch {
		case known && op.Status == registry.Obsolete:
			return fmt.Errorf("%s is marked obsolete by Microsoft and is not supported", method)
		case known:
			if verb != "" && !strings.EqualFold(verb, op.HTTP) {
				return fmt.Errorf("%s is documented as %s", method, op.HTTP)
			}
			verb, effect = op.HTTP, op.Effect
		case verb == "":
			return fmt.Errorf("%s is not in the registry; pass --http GET or --http POST (it will count as a write)", method)
		default:
			verb = strings.ToUpper(verb)
		}
		if effect == registry.Write && a.readOnly {
			return fmt.Errorf("%s changes Bing state (or is unknown) and is blocked by --read-only", method)
		}
		if effect == registry.Write {
			if err := a.confirm(fmt.Sprintf("bwt api %s changes Bing state", method)); err != nil {
				return err
			}
		}
		values := map[string]any{}
		for _, p := range params {
			name, value, ok := strings.Cut(p, "=")
			if !ok || name == "" {
				return fmt.Errorf("--param must be name=value, got %q", p)
			}
			values[name] = value
		}
		if data != "" {
			if verb == "GET" {
				return fmt.Errorf("--data is for POST methods; use --param for GET")
			}
			var body map[string]any
			dec := json.NewDecoder(strings.NewReader(data))
			dec.UseNumber()
			if err := dec.Decode(&body); err != nil {
				return fmt.Errorf("--data must be a JSON object")
			}
			for k, v := range body {
				values[k] = v
			}
		}
		cl, err := a.api()
		if err != nil {
			return err
		}
		result, err := cl.Do(cmd.Context(), method, verb, effect, values)
		if err != nil {
			return err
		}
		if len(cl.Planned) > 0 {
			return a.print(map[string]any{"dry_run": true, "requests": cl.Planned})
		}
		return a.print(a.convert(result))
	}
	return c
}
