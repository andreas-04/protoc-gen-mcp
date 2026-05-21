package tmpl

// Package tmpl embeds all .go.tmpl files and exposes an Execute helper.
import (
	"bytes"
	"embed"
	"fmt"
	"text/template"
)

//go:embed *.go.tmpl
var files embed.FS

// funcMap contains helper functions available inside every template.
var funcMap = template.FuncMap{
	// q returns a Go double-quoted string literal of s.
	"q": func(s string) string { return fmt.Sprintf("%q", s) },
}

// Execute renders the named template (e.g. "register.go.tmpl") with data
// and returns the resulting source string.
func Execute(name string, data any) (string, error) {
	raw, err := files.ReadFile(name)
	if err != nil {
		return "", fmt.Errorf("tmpl: reading %q: %w", name, err)
	}

	t, err := template.New(name).Funcs(funcMap).Parse(string(raw))
	if err != nil {
		return "", fmt.Errorf("tmpl: parsing %q: %w", name, err)
	}

	var buf bytes.Buffer
	if err := t.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("tmpl: executing %q: %w", name, err)
	}
	return buf.String(), nil
}
