// Package initempl embeds the default plugin scaffolding templates shipped
// with jughead. The `jughead init` subcommand renders these into a target
// directory to bootstrap a new site or auth-provider plugin.
//
// Templates live under templates/init/{site,provider} as files with a `.tmpl`
// suffix. The init command strips the suffix and executes the file as a
// text/template (see cmd/jughead/init for the rendering logic).
package initempl

import "embed"

// Default is the embedded default template tree. It contains two
// subdirectories, "site" and "provider", each holding the files for that
// plugin kind. A custom template directory passed via --template replaces the
// embedded tree; its layout must mirror these subdirectories.
//
//go:embed site provider
var Default embed.FS
