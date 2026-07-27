package migrations

import "embed"

// Files packages the migration source with the service binary so a release
// Job never depends on its working directory or a separately mounted folder.
//
//go:embed *.sql
var Files embed.FS
