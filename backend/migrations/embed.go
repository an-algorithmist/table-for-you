package migrations

import "embed"

// Files contains ordered SQL migrations bundled with the backend binary.
//
//go:embed *.sql
var Files embed.FS
