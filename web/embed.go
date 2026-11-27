// Package web embeds the built web app (web/dist) in the binary, so one file serves the API and the UI.
package web

import (
	"embed"
	"io/fs"
)

//go:embed all:dist
var _dist embed.FS

// Dist returns the frontend build with dist/ as the root.
func Dist() fs.FS {
	sub, err := fs.Sub(_dist, "dist")
	if err != nil {
		panic(err)
	}
	return sub
}
