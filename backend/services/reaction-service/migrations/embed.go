package migrations

import "embed"

// Files packages migrations with the service binary so deployment jobs do not
// depend on a writable filesystem or a separately mounted migrations folder.
//
//go:embed *.sql
var Files embed.FS
