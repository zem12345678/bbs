package migrations

import "embed"

// Files is compiled into the service binary so the migration command is
// self-contained in a release image.
//
//go:embed *.sql
var Files embed.FS
