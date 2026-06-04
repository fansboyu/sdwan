package migrations

import "embed"

// FS contains the versioned database migrations.
//
//go:embed *.sql
var FS embed.FS
