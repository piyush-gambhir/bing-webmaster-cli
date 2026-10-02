package cmd

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/piyush-gambhir/bing-webmaster-cli/cli-go/internal/output"
	"github.com/spf13/cobra"
)

var (
	roleNames    = map[string]string{"0": "admin", "1": "read-only", "2": "read-write", "Administrator": "admin", "ReadOnly": "read-only", "ReadWrite": "read-write"}
	entityNames  = map[string]string{"0": "page", "1": "directory", "Page": "page", "Directory": "directory"}
	requestNames = map[string]string{"0": "cache-only", "1": "full-removal", "CacheOnly": "cache-only", "FullRemoval": "full-removal"}
)

func enumCol(header, key string, names map[string]string) output.Col {
	return output.Col{Header: header, Value: func(r map[string]any) any { return enumName(r[key], names) }}
}

func (a *app) usersCmd() *cobra.Command {
	c := &cobra.Command{Use: "users", Short: "Users and roles delegated on a site"}
	var allSubdomains bool
	list := a.opCmd(opSpec{use: "list", short: "List users with access to the site", op: "GetSiteRoles",
		cols: []output.Col{col("email", "Email"), enumCol("role", "Role", roleNames), col("site", "Site"), col("verification_site", "VerificationSite"),
			col("delegated_by", "DelegatorEmail"), col("date", "Date"), col("expired", "Expired")},
		setup: func(c *cobra.Command) func() (map[string]any, error) {
			c.Flags().BoolVar(&allSubdomains, "all-subdomains", false, "Include users of all subdomains")
			return func() (map[string]any, error) { return map[string]any{"includeAllSubdomains": allSubdomains}, nil }
		}})
	c.AddCommand(list, a.userAddCmd(), a.userRemoveCmd())
	return c
}

func (a *app) userAddCmd() *cobra.Command {
	var role, delegatedURL, authCode string
	c := &cobra.Command{Use: "add EMAIL", Short: "Delegate access to a user", Args: cobra.ExactArgs(1),
		Long: "Grants EMAIL a role on the site (or on --delegated-url, a host under it).\n" +
			"--auth-code defaults to the delegated site's AuthenticationCode from your site list."}
	annotate(c, "AddSiteRoles", "GetUserSites")
	c.Flags().StringVar(&role, "role", "", "Role: admin, read-only, or read-write (required)")
	c.Flags().StringVar(&delegatedURL, "delegated-url", "", "Site or host to delegate (default: the selected site)")
	c.Flags().StringVar(&authCode, "auth-code", "", "Authentication code of the delegated site (default: looked up)")
	c.RunE = func(cmd *cobra.Command, args []string) error {
		admin, readOnly := false, false
		switch role {
		case "admin":
			admin = true
		case "read-only":
			readOnly = true
		case "read-write":
		default:
			return fmt.Errorf("--role must be admin, read-only, or read-write")
		}
		if !strings.Contains(args[0], "@") {
			return fmt.Errorf("EMAIL must be an email address")
		}
		if delegatedURL != "" {
			if err := requireURL("--delegated-url", delegatedURL); err != nil {
				return err
			}
		}
		cl, err := a.api()
		if err != nil {
			return err
		}
		site, err := a.siteURL(cmd.Context(), cl)
		if err != nil {
			return err
		}
		if delegatedURL == "" {
			delegatedURL = site
		}
		if authCode == "" {
			sites, err := cl.Call(cmd.Context(), "GetUserSites", map[string]any{})
			if err != nil {
				return err
			}
			for _, s := range asList(sites) {
				if s["Url"] == delegatedURL {
					authCode, _ = s["AuthenticationCode"].(string)
				}
			}
			if authCode == "" {
				return fmt.Errorf("no authentication code found for %s in your site list; pass --auth-code", delegatedURL)
			}
		}
		params := map[string]any{"siteUrl": site, "delegatedUrl": delegatedURL, "userEmail": args[0], "authenticationCode": authCode, "isAdministrator": admin, "isReadOnly": readOnly}
		result, err := cl.Call(cmd.Context(), "AddSiteRoles", params)
		if err != nil {
			return err
		}
		return a.emit(cl, "AddSiteRoles", params, result, nil)
	}
	return c
}

func (a *app) userRemoveCmd() *cobra.Command {
	var delegatedURL string
	c := &cobra.Command{Use: "remove EMAIL", Short: "Remove a user's delegated access", Args: cobra.ExactArgs(1),
		Long: "Looks up the user's role with GetSiteRoles and removes exactly that role. Several matches are an error;\nnarrow them with --delegated-url."}
	annotate(c, "GetSiteRoles", "RemoveSiteRole")
	c.Flags().StringVar(&delegatedURL, "delegated-url", "", "Only the role on this site or host")
	c.RunE = func(cmd *cobra.Command, args []string) error {
		cl, err := a.api()
		if err != nil {
			return err
		}
		site, err := a.siteURL(cmd.Context(), cl)
		if err != nil {
			return err
		}
		roles, err := cl.Call(cmd.Context(), "GetSiteRoles", map[string]any{"siteUrl": site, "includeAllSubdomains": true})
		if err != nil {
			return err
		}
		var matches []map[string]any
		for _, r := range asList(roles) {
			email, _ := r["Email"].(string)
			if strings.EqualFold(email, args[0]) && (delegatedURL == "" || r["Site"] == delegatedURL) {
				matches = append(matches, r)
			}
		}
		switch len(matches) {
		case 0:
			return fmt.Errorf("no role for %s on %s; see bwt users list --all-subdomains", args[0], site)
		case 1:
		default:
			return fmt.Errorf("%s has %d roles; pass --delegated-url to choose one", args[0], len(matches))
		}
		if err := a.confirm(fmt.Sprintf("This removes %s's access to %v", args[0], matches[0]["Site"])); err != nil {
			return err
		}
		params := map[string]any{"siteUrl": site, "siteRole": matches[0]}
		result, err := cl.Call(cmd.Context(), "RemoveSiteRole", params)
		if err != nil {
			return err
		}
		return a.emit(cl, "RemoveSiteRole", params, result, nil)
	}
	return c
}

// Bing documents that query parameters "may contain only unreserved letters and colon".
var paramName = regexp.MustCompile(`^[A-Za-z0-9._~:-]+$`)

func (a *app) paramsCmd() *cobra.Command {
	c := &cobra.Command{Use: "params", Short: "URL query parameters Bing should ignore when normalizing URLs"}
	validName := func(args []string) error {
		if !paramName.MatchString(args[0]) {
			return fmt.Errorf("NAME may contain only letters, digits, and . _ ~ : -")
		}
		return nil
	}
	mk := func(use, short, op string, extra map[string]any) *cobra.Command {
		return a.opCmd(opSpec{use: use, short: short, op: op, args: []string{"queryParameter"}, validate: validName,
			setup: func(*cobra.Command) func() (map[string]any, error) {
				return func() (map[string]any, error) { return extra, nil }
			}})
	}
	c.AddCommand(
		a.opCmd(opSpec{use: "list", short: "List normalization parameters", op: "GetQueryParameters",
			cols: cols("parameter", "Parameter", "enabled", "IsEnabled", "source", "Source", "date", "Date")}),
		mk("add NAME", "Add a parameter to ignore", "AddQueryParameter", nil),
		mk("remove NAME", "Remove a parameter", "RemoveQueryParameter", nil),
		mk("enable NAME", "Enable a parameter", "EnableDisableQueryParameter", map[string]any{"isEnabled": true}),
		mk("disable NAME", "Disable a parameter", "EnableDisableQueryParameter", map[string]any{"isEnabled": false}),
	)
	return c
}

func (a *app) blockCmd() *cobra.Command {
	c := &cobra.Command{Use: "block", Short: "Block URLs from Bing results (Block URLs tool)"}
	c.AddCommand(
		a.opCmd(opSpec{use: "list", short: "List blocked URLs", op: "GetBlockedUrls",
			cols: []output.Col{col("url", "Url"), enumCol("type", "EntityType", entityNames), enumCol("request", "RequestType", requestNames),
				col("date", "Date"), col("days_to_expire", "DaysToExpire")}}),
		a.blockAddCmd(),
		a.blockRemoveCmd(),
	)
	return c
}

func blockEnums(typ, request string) (int, int, error) {
	entity, err := choice("type", typ, map[string]int{"page": 0, "directory": 1})
	if err != nil {
		return 0, 0, err
	}
	req, err := choice("request", request, map[string]int{"cache-only": 0, "full-removal": 1})
	return entity, req, err
}

func (a *app) blockAddCmd() *cobra.Command {
	var typ, request string
	c := &cobra.Command{Use: "add URL", Short: "Block a page or directory from Bing results", Args: cobra.ExactArgs(1),
		Long: "--request cache-only removes the cached copy; full-removal removes the URL from Bing results until it expires.\n" +
			"There is no default for --request, and full removal asks for confirmation."}
	annotate(c, "AddBlockedUrl")
	c.Flags().StringVar(&typ, "type", "page", "page or directory")
	c.Flags().StringVar(&request, "request", "", "cache-only or full-removal (required)")
	c.RunE = func(cmd *cobra.Command, args []string) error {
		if err := requireURL("URL", args[0]); err != nil {
			return err
		}
		entity, req, err := blockEnums(typ, request)
		if err != nil {
			return err
		}
		cl, err := a.api()
		if err != nil {
			return err
		}
		site, err := a.siteURL(cmd.Context(), cl)
		if err != nil {
			return err
		}
		if req == 1 {
			if err := a.confirm(fmt.Sprintf("This removes %s from Bing results", args[0])); err != nil {
				return err
			}
		}
		// Date and DaysToExpire are left for Bing to set, as in its own sample request.
		params := map[string]any{"siteUrl": site, "blockedUrl": map[string]any{"Url": args[0], "EntityType": entity, "RequestType": req}}
		result, err := cl.Call(cmd.Context(), "AddBlockedUrl", params)
		if err != nil {
			return err
		}
		return a.emit(cl, "AddBlockedUrl", params, result, nil)
	}
	return c
}

func (a *app) blockRemoveCmd() *cobra.Command {
	c := &cobra.Command{Use: "remove URL", Short: "Unblock a URL", Args: cobra.ExactArgs(1),
		Long: "Looks up the block with GetBlockedUrls and removes exactly that entry."}
	annotate(c, "GetBlockedUrls", "RemoveBlockedUrl")
	c.RunE = func(cmd *cobra.Command, args []string) error {
		cl, err := a.api()
		if err != nil {
			return err
		}
		site, err := a.siteURL(cmd.Context(), cl)
		if err != nil {
			return err
		}
		blocked, err := cl.Call(cmd.Context(), "GetBlockedUrls", map[string]any{"siteUrl": site})
		if err != nil {
			return err
		}
		var match map[string]any
		for _, b := range asList(blocked) {
			if b["Url"] == args[0] {
				if match != nil {
					return fmt.Errorf("%s has several block entries; remove them in the dashboard", args[0])
				}
				match = b
			}
		}
		if match == nil {
			return fmt.Errorf("%s is not in bwt block list", args[0])
		}
		params := map[string]any{"siteUrl": site, "blockedUrl": match}
		result, err := cl.Call(cmd.Context(), "RemoveBlockedUrl", params)
		if err != nil {
			return err
		}
		return a.emit(cl, "RemoveBlockedUrl", params, result, nil)
	}
	return c
}

func (a *app) previewBlocksCmd() *cobra.Command {
	var reason string
	c := &cobra.Command{Use: "preview-blocks", Short: "Block or allow page previews (snippets) in Bing results"}
	c.AddCommand(
		a.opCmd(opSpec{use: "list", short: "List active page preview blocks", op: "GetActivePagePreviewBlocks"}),
		a.opCmd(opSpec{use: "add URL", short: "Block the preview for a URL", op: "AddPagePreviewBlock", args: []string{"url"},
			long: "--reason is Bing's BlockReason number. Microsoft does not publish the enum values; existing blocks in\npreview-blocks list show the values in use.",
			setup: func(c *cobra.Command) func() (map[string]any, error) {
				c.Flags().StringVar(&reason, "reason", "", "BlockReason number (required)")
				return func() (map[string]any, error) {
					n, err := strconv.Atoi(reason)
					if err != nil || n < 0 {
						return nil, fmt.Errorf("--reason must be a non-negative BlockReason number")
					}
					return map[string]any{"reason": n}, nil
				}
			}}),
		a.opCmd(opSpec{use: "remove URL", short: "Remove a preview block", op: "RemovePagePreviewBlock", args: []string{"url"}}),
	)
	return c
}
