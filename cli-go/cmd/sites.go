package cmd

import (
	"fmt"

	"github.com/piyush-gambhir/bing-webmaster-cli/cli-go/internal/config"
	"github.com/spf13/cobra"
)

func (a *app) sitesCmd() *cobra.Command {
	c := &cobra.Command{Use: "sites", Short: "List, add, verify, remove, and select sites"}
	c.AddCommand(
		a.opCmd(opSpec{use: "list", short: "List the sites in your Bing Webmaster account", op: "GetUserSites",
			long: "Lists your sites. Use these URLs exactly (scheme and trailing slash) in other commands.\nJSON also includes AuthenticationCode and DnsVerificationCode for verification.",
			cols: cols("url", "Url", "verified", "IsVerified")}),
		a.siteUseCmd(),
		a.opCmd(opSpec{use: "add SITE", short: "Add a site to your account (it still needs verification)", op: "AddSite", args: []string{"siteUrl"},
			long: "Adds the site to your account. Adding is harmless if the site is already there.\nVerify ownership afterwards with an XML file, meta tag, or DNS record, then run bwt sites verify."}),
		a.siteVerifyCmd(),
		a.opCmd(opSpec{use: "remove SITE", short: "Remove a site from your account", op: "RemoveSite", args: []string{"siteUrl"},
			confirm: "This removes %s from your Bing Webmaster account"}),
	)
	return c
}

func (a *app) siteUseCmd() *cobra.Command {
	return &cobra.Command{Use: "use SITE", Short: "Set the profile's default site (local)", Args: cobra.ExactArgs(1),
		Annotations: map[string]string{"writes-local": "true"},
		Long:        "Saves SITE as the default for the selected profile. A bare host is matched against your sites.",
		RunE: func(cmd *cobra.Command, args []string) error {
			site := args[0]
			cfg, path, err := a.load()
			if err != nil {
				return err
			}
			name := cfg.ProfileName(a.profile)
			if name == "" {
				name = "default"
			}
			if err := requireURL("SITE", site); err != nil {
				cl, cerr := a.api()
				if cerr != nil {
					return fmt.Errorf("%v (a bare host needs credentials to match it against your sites)", err)
				}
				if site, err = a.matchSite(cmd.Context(), cl, site); err != nil {
					return err
				}
			}
			if err := config.Update(cmd.Context(), path, func(c *config.Config) error {
				p := c.Profiles[name]
				p.Site = site
				c.Profiles[name] = p
				if c.CurrentProfile == "" {
					c.CurrentProfile = name
				}
				return nil
			}); err != nil {
				return err
			}
			return a.print(map[string]any{"profile": name, "site": site})
		}}
}

func (a *app) siteVerifyCmd() *cobra.Command {
	c := &cobra.Command{Use: "verify SITE", Short: "Ask Bing to verify ownership of a site", Args: cobra.ExactArgs(1),
		Long: "Asks Bing to check the site's verification file, meta tag, or DNS record. Prints verified: false when Bing could not confirm ownership."}
	annotate(c, "VerifySite")
	c.RunE = func(cmd *cobra.Command, args []string) error {
		cl, err := a.api()
		if err != nil {
			return err
		}
		result, err := cl.Call(cmd.Context(), "VerifySite", map[string]any{"siteUrl": args[0]})
		if err != nil {
			return err
		}
		if len(cl.Planned) > 0 {
			return a.printPlanned(cl.Planned)
		}
		return a.print(map[string]any{"site": args[0], "verified": result == true})
	}
	return c
}
