/*
Copyright 2026-present RAYNET s.r.o.
SPDX-License-Identifier: MIT
*/

package cmd

import (
	"errors"
	"fmt"
	"io"
	"strings"

	"charm.land/huh/v2"
	"github.com/raynetcrm/raynet-cli/internal/config"
)

// otherHost is the host menu's "Other…" choice.
const otherHost = "\x00other"

// wizard asks the `auth login` questions with huh, on stderr. On a real
// terminal it shows huh's screens and hides the API key and bearer token
// while typed. Otherwise (tests, or stdin piped into an interactive session)
// and when accessible mode is requested, huh runs in its accessible mode:
// plain line prompts for screen readers.
type wizard struct {
	in         io.Reader
	out        io.Writer
	accessible bool
	// hidden reports whether the API key and bearer token can be read
	// without echo, which needs stdin to be a real terminal.
	hidden bool
}

func (a *app) newWizard() *wizard {
	w := &wizard{in: a.stdin, out: a.stderr}
	if a.stdinTerminal() {
		w.hidden = true
		w.accessible = a.accessible()
		return w
	}
	// Accessible mode starts a new line scanner per question; feeding it one
	// byte at a time keeps each scanner from swallowing the later answers.
	w.in = oneByteReader{a.stdin}
	w.accessible = true
	return w
}

// ask runs a one-question form. Accessible mode ignores hidden groups, so
// the wizard only ever asks the questions it needs, one form at a time.
func (w *wizard) ask(field huh.Field) error {
	f := huh.NewForm(huh.NewGroup(field)).
		WithInput(w.in).
		WithOutput(w.out).
		WithAccessible(w.accessible)
	if w.accessible {
		f = f.WithTheme(huh.ThemeFunc(huh.ThemeBase))
	}
	if err := f.Run(); err != nil {
		if errors.Is(err, huh.ErrUserAborted) {
			return errors.New("login cancelled, nothing saved")
		}
		return err
	}
	return nil
}

// field asks for a login field. The API key and bearer token are hidden
// while typed whenever stdin is a terminal; without one huh can't hide the
// input and would return an empty value, so the answer is read as a plain
// line.
func (w *wizard) field(f loginField, value *string) error {
	mode := huh.EchoModeNormal
	if f.hidden && w.hidden {
		mode = huh.EchoModePassword
	}
	return w.ask(huh.NewInput().Title(f.title).EchoMode(mode).Validate(required).Value(value))
}

func (w *wizard) confirm(title string) (bool, error) {
	var yes bool
	err := w.ask(huh.NewConfirm().Title(title).Value(&yes))
	return yes, err
}

func required(s string) error {
	if strings.TrimSpace(s) == "" {
		return errors.New("required")
	}
	return nil
}

// credentials asks for every login value o lacks: the host, the login
// method and that method's fields. A flag answers its question.
func (w *wizard) credentials(o *loginOptions) error {
	if err := w.host(o); err != nil {
		return err
	}
	m, err := w.method(o)
	if err != nil || m == "" {
		return err
	}
	o.authType = string(m.authType())
	for _, f := range o.missing(m) {
		if err := w.field(f, f.value(o)); err != nil {
			return err
		}
	}
	return nil
}

func (w *wizard) host(o *loginOptions) error {
	if o.host != "" {
		return nil
	}
	choice := config.KnownHosts[0]
	opts := huh.NewOptions(config.KnownHosts...)
	opts = append(opts, huh.NewOption("Other…", otherHost))
	if err := w.ask(huh.NewSelect[string]().Title("Host").Options(opts...).Value(&choice)); err != nil {
		return err
	}
	if choice != otherHost {
		o.host = choice
		return nil
	}
	return w.ask(huh.NewInput().Title("Host").
		Validate(validateHostInput).Value(&o.host))
}

// validateHostInput accepts what loginProfile accepts as --host.
func validateHostInput(s string) error {
	_, _, err := parseHost(s)
	if errors.Is(err, errNoHost) {
		return errors.New("required")
	}
	return err
}

// method returns the login method, asking only when the flags leave more
// than one open; "" for an unknown --auth-type, left to loginProfile to
// report.
func (w *wizard) method(o *loginOptions) (loginMethod, error) {
	methods := o.methods()
	switch len(methods) {
	case 0:
		return "", nil
	case 1:
		return methods[0], nil
	}
	opts := make([]huh.Option[loginMethod], len(methods))
	for i, m := range methods {
		opts[i] = huh.NewOption(m.label(), m)
	}
	m := methods[0]
	err := w.ask(huh.NewSelect[loginMethod]().Title("Login method").Options(opts...).Value(&m))
	return m, err
}

// profileName asks for the profile name, suggesting suggested, and before
// replacing an existing profile. Declining the replacement asks for another
// name.
func (w *wizard) profileName(cfg *config.Config, suggested string) (string, error) {
	for {
		name := suggested
		err := w.ask(huh.NewInput().Title("Profile name").Value(&name).Validate(func(s string) error {
			// Accessible mode validates the typed line before applying the
			// suggestion, so an empty line means "take the suggestion".
			if strings.TrimSpace(s) == "" && suggested == "" {
				return errors.New("required")
			}
			return nil
		}))
		if err != nil {
			return "", err
		}
		name = strings.TrimSpace(name)
		if name == "" {
			name = suggested
		}
		if name == "" {
			return "", errors.New("missing profile name, nothing saved")
		}
		if _, exists := cfg.Profiles[name]; !exists {
			return name, nil
		}
		replace, err := w.confirm(fmt.Sprintf("Profile %q exists. Replace it?", name))
		if err != nil || replace {
			return name, err
		}
		suggested = ""
	}
}

// oneByteReader reads at most one byte per Read, so a line scanner never
// reads past the line it returns.
type oneByteReader struct{ r io.Reader }

func (o oneByteReader) Read(p []byte) (int, error) {
	if len(p) == 0 {
		return 0, nil
	}
	return o.r.Read(p[:1])
}
