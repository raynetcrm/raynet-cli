/*
Copyright 2026-present RAYNET s.r.o.
SPDX-License-Identifier: MIT
*/

package cmd

import (
	"fmt"
	"net/url"
	"strings"
)

// webFlagUsage is the help of the --web flag on list and view.
const webFlagUsage = "open in the CRM web app instead of printing"

// openRecordWeb opens the record ref names in the CRM web app:
// {web base}/{instance}/redir?en={webName}&ei={id}. An "ext:" ref is
// resolved to its id first.
func (a *app) openRecordWeb(s *entitySpec, ref recordRef) error {
	id := ref.value
	if ref.ext {
		c, err := a.resolveClient()
		if err != nil {
			return err
		}
		if id, err = a.recordID(c, s, ref); err != nil {
			return err
		}
	}
	return a.openWeb(fmt.Sprintf("/redir?en=%s&ei=%s", url.QueryEscape(s.webName), url.QueryEscape(id)))
}

// openListWeb opens the entity's list view in the CRM web app.
func (a *app) openListWeb(s *entitySpec) error {
	return a.openWeb("/?view=ListView&en=" + url.QueryEscape(s.webName))
}

// openWeb opens {web base}/{instance}{suffix} in the browser. The web base
// is the profile's web URL, else the API's scheme + host; the instance name
// is the profile's (its auth instance name, else the one `auth login`
// learned), else learned from GET /security/info.
func (a *app) openWeb(suffix string) error {
	p, err := a.resolveProfile()
	if err != nil {
		return err
	}
	base := strings.TrimRight(p.WebURL, "/")
	if base == "" {
		base = p.GetScheme() + "://" + p.Host
	}
	instance := p.InstanceName
	if instance == "" {
		instance = p.WebInstanceName
	}
	if instance == "" {
		c, err := a.resolveClient()
		if err != nil {
			return err
		}
		info, err := fetchSecurityInfo(c)
		if err != nil {
			return fmt.Errorf("looking up the instance name for the web link: %w", err)
		}
		if info.InstanceName == "" {
			return fmt.Errorf("the API did not report an instance name for the web link")
		}
		instance = info.InstanceName
	}
	link := base + "/" + url.PathEscape(instance) + suffix
	if a.isTerminal() {
		fmt.Fprintf(a.stderr, "Opening %s in your browser.\n", link)
	}
	return a.openBrowser(link)
}
