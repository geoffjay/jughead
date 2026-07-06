// Package initcmd implements the `jughead init` subcommand. It bootstraps a
// minimal plugin scaffold (a site plugin or an auth-provider plugin) from a
// template directory into a target path.
//
// Default templates are embedded from ../../templates/init/{site,provider};
// a custom template directory can be supplied via --template to override
// them. The custom directory must mirror the default layout (subdirectories
// named "site" and "provider" holding the kind's files).
//
// The subcommand is interactive when required flags are omitted: it prompts
// for the plugin name, the plugin kind (site or provider), and the target
// directory. All three can instead be supplied via flags and a positional
// argument for non-interactive use:
//
//	jughead init --plugin site --name foo path/to/foo
//	jughead init --plugin site --template path/to/custom/template/ path/to/foo
//	jughead init   # prompts for everything
package initcmd

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"text/template"
	"unicode"

	initempl "github.com/geoffjay/jughead/templates/init"

	"github.com/spf13/cobra"
)

// defaultJugheadVersion is the jughead module version written into the
// generated go.mod's require directive when the host doesn't override it. It
// is bumped alongside each tagged release so freshly scaffolded plugins pin a
// real version instead of "dev". The binary's own version (printed by
// `jughead version`) is still set via -ldflags at build time.
const defaultJugheadVersion = "v0.1.1"

// pluginKind is the set of plugin templates supported by `jughead init`.
type pluginKind string

const (
	kindSite     pluginKind = "site"
	kindProvider pluginKind = "provider"

	// dirMode is the permission bits used for created directories. Templates
	// and rendered files keep the umask-defaulted mode from os.Create.
	dirMode os.FileMode = 0o755
)

func (k pluginKind) valid() bool {
	return k == kindSite || k == kindProvider
}

// templateData is the value passed to text/template when rendering each
// template file. Field names are exported so they are reachable from the
// templates regardless of template origin (default or custom).
type templateData struct {
	// Name is the plugin name (e.g. "foo"). Used in logs, the Plugin struct,
	// and the rendered module path.
	Name string

	// SitePath is the URL path the site is served under, computed as
	// "/sites/" + Name. Only meaningful for site plugins but rendered into
	// every template (provider templates ignore it).
	SitePath string

	// TitleName is the PascalCase form of Name, used to derive exported
	// identifier names in provider templates (e.g. "Foo" -> "FooProvider").
	TitleName string

	// GoVersion is the Go toolchain version written into the rendered
	// go.mod (e.g. "1.25.0"). Sourced from runtime.Version().
	GoVersion string

	// JugheadVersion is the jughead module version written into the rendered
	// go.mod's require directive (e.g. "v0.1.0"). Plugins must be built
	// against the same jughead version as the host.
	JugheadVersion string
}

// flags
var (
	flagPlugin   string
	flagName     string
	flagTemplate string
)

// Cmd is the cobra command for `jughead init`.
var Cmd = &cobra.Command{
	Use:   "init [path]",
	Short: "Bootstrap a minimal plugin from a template",
	Long: `Bootstrap a minimal jughead plugin (site or auth-provider) from a template.

Without flags, 'jughead init' is interactive: it prompts for the plugin name,
the plugin kind (site or provider), and the target directory.

Non-interactive usage:

  jughead init --plugin site --name foo path/to/foo
  jughead init --plugin provider --name bar path/to/bar

A custom template directory can be supplied via --template. It must contain
subdirectories named "site" and "provider" mirroring the default templates;
the selected kind's subdirectory is rendered into the target path.

  jughead init --plugin site --template path/to/custom/template/ path/to/foo`,
	Args: cobra.MaximumNArgs(1),
	RunE: run,
}

// SetupFlags registers the init command's flags. Called by main before
// Execute so the flags are available for parsing. This avoids an init()
// function (the project's linter forbids them).
func SetupFlags() {
	Cmd.Flags().StringVar(&flagPlugin, "plugin", "", "plugin kind: site | provider")
	Cmd.Flags().StringVar(&flagName, "name", "", "plugin name (used in module path and logs)")
	Cmd.Flags().StringVar(&flagTemplate, "template", "", "path to a custom template directory")
}

func run(_ *cobra.Command, args []string) error {
	kind, name, dest, err := resolveInputs(args)
	if err != nil {
		return err
	}

	tmplRoot, err := resolveTemplateRoot(flagTemplate, kind)
	if err != nil {
		return err
	}

	if err := os.MkdirAll(dest, dirMode); err != nil {
		return fmt.Errorf("create target directory: %w", err)
	}

	data := templateData{
		Name:           name,
		SitePath:       "/sites/" + name,
		TitleName:      pascalize(name),
		GoVersion:      goVersion(),
		JugheadVersion: defaultJugheadVersion,
	}

	if err := renderTree(tmplRoot, kind, dest, &data); err != nil {
		return err
	}

	slog.Info("Plugin scaffold created",
		"kind", kind, "name", name, "path", dest)
	return nil
}

// resolveInputs determines the plugin kind, name, and target path from flags,
// positional args, and (when needed) interactive prompts. It returns an error
// when an invalid kind or empty name is supplied non-interactively.
func resolveInputs(args []string) (kind pluginKind, name, dest string, err error) {
	stdin := bufio.NewReader(os.Stdin)

	kind = pluginKind(flagPlugin)
	if kind == "" {
		kind = pluginKind(prompt(stdin, os.Stdout, "Plugin kind (site|provider)", string(kindSite)))
	}
	if !kind.valid() {
		return "", "", "", fmt.Errorf("invalid plugin kind %q (want site or provider)", flagPlugin)
	}

	name = flagName
	if name == "" {
		name = prompt(stdin, os.Stdout, "Plugin name", "")
	}
	if name == "" {
		return "", "", "", errors.New("plugin name is required")
	}
	if !isValidName(name) {
		return "", "", "", fmt.Errorf("invalid plugin name %q (lowercase letters, digits, hyphens; must start with a letter)", name)
	}

	if len(args) == 1 {
		dest = args[0]
	} else {
		cwd, _ := os.Getwd()
		dest = prompt(stdin, os.Stdout, "Target directory", filepath.Join(cwd, name))
	}
	if dest == "" {
		return "", "", "", errors.New("target directory is required")
	}

	return kind, name, dest, nil
}

// resolveTemplateRoot returns the FS root containing the per-kind template
// subdirectory. For the default templates it is the embedded tree rooted at
// the templates/init directory (so the kind's files live under "site" or
// "provider"); for a custom template it is the supplied directory on disk.
func resolveTemplateRoot(customDir string, kind pluginKind) (fs.FS, error) {
	if customDir == "" {
		root := initempl.Default
		if _, err := fs.Stat(root, string(kind)); err != nil {
			return nil, fmt.Errorf("default template missing for kind %q: %w", kind, err)
		}
		return root, nil
	}

	abs, err := filepath.Abs(customDir)
	if err != nil {
		return nil, fmt.Errorf("resolve template path: %w", err)
	}
	if _, err := os.Stat(filepath.Join(abs, string(kind))); err != nil {
		return nil, fmt.Errorf("custom template missing %q subdir for kind %q: %w", string(kind), kind, err)
	}
	return os.DirFS(abs), nil
}

// renderTree walks the kind's template directory and renders each file into
// the destination, stripping the ".tmpl" suffix from output filenames.
func renderTree(root fs.FS, kind pluginKind, dest string, data *templateData) error {
	kindDir := string(kind)
	return fs.WalkDir(root, kindDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}

		rel, err := filepath.Rel(kindDir, path)
		if err != nil {
			return fmt.Errorf("rel path: %w", err)
		}
		rel = strings.TrimSuffix(rel, ".tmpl")

		outPath := filepath.Join(dest, rel)
		if err := os.MkdirAll(filepath.Dir(outPath), dirMode); err != nil {
			return fmt.Errorf("mkdir for %s: %w", outPath, err)
		}

		raw, err := fs.ReadFile(root, path)
		if err != nil {
			return fmt.Errorf("read template %s: %w", path, err)
		}
		tmpl, err := template.New(path).Parse(string(raw))
		if err != nil {
			return fmt.Errorf("parse template %s: %w", path, err)
		}

		f, err := os.Create(outPath)
		if err != nil {
			return fmt.Errorf("create %s: %w", outPath, err)
		}
		defer func() {
			if cerr := f.Close(); cerr != nil {
				slog.Error("Failed to close file", "path", outPath, "details", cerr.Error())
			}
		}()

		if err := tmpl.Execute(f, data); err != nil {
			return fmt.Errorf("execute template %s: %w", path, err)
		}
		slog.Info("Rendered", "file", rel)
		return nil
	})
}

// prompt reads a line from stdin, returning def when the user enters an empty
// line. The prompt is written to out (separate from the reader so it does not
// interfere with stdin echoing).
func prompt(r *bufio.Reader, out io.Writer, label, def string) string {
	if def != "" {
		fmt.Fprintf(out, "%s [%s]: ", label, def)
	} else {
		fmt.Fprintf(out, "%s: ", label)
	}
	line, err := r.ReadString('\n')
	if err != nil && line == "" {
		return def
	}
	line = strings.TrimSpace(line)
	if line == "" {
		return def
	}
	return line
}

// isValidName enforces a conservative plugin name: lowercase letters, digits,
// and hyphens, starting with a letter. This keeps the module path and the
// generated identifiers sane.
func isValidName(s string) bool {
	if s == "" {
		return false
	}
	for i, r := range s {
		if i == 0 && !unicode.IsLetter(r) {
			return false
		}
		if !unicode.IsLower(r) && !unicode.IsDigit(r) && r != '-' {
			return false
		}
	}
	return true
}

// pascalize converts a kebab/lowercase name into a PascalCase identifier
// suitable for exported Go type names (e.g. "foo-bar" -> "FooBar").
func pascalize(s string) string {
	capitalize := true
	var b strings.Builder
	for _, r := range s {
		if r == '-' || r == '_' {
			capitalize = true
			continue
		}
		if capitalize {
			b.WriteRune(unicode.ToUpper(r))
			capitalize = false
		} else {
			b.WriteRune(r)
		}
	}
	return b.String()
}

// goVersion returns the current Go toolchain version in go.mod format
// (e.g. "1.25.0" from "go1.25.0").
func goVersion() string {
	v := runtime.Version()
	return strings.TrimPrefix(v, "go")
}
