// Package skills embeds the shipped Agent Skills library.
package skills

import "embed"

//go:embed perch/SKILL.md
var FS embed.FS
