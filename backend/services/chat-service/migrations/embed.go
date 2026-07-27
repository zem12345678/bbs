package migrations

import "embed"

// Files packages every chat migration into the service binary so release Jobs
// do not depend on the container working directory.
//
//go:embed *.sql
var Files embed.FS
