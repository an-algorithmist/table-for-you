package web

import (
	"embed"
	"io/fs"
)

//go:embed assets/*
var assets embed.FS

func Files() fs.FS { f, _ := fs.Sub(assets, "assets"); return f }
