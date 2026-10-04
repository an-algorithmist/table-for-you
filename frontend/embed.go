package web

import (
	"embed"
	"io/fs"
)

//go:embed assets/*
var assets embed.FS

// Files exposes the embedded asset directory to the backend file server.
func Files() fs.FS { f, _ := fs.Sub(assets, "assets"); return f }
