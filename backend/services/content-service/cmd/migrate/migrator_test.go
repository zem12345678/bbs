package migrate

import (
	"io/fs"
	"strings"
	"testing"

	servicemigrations "content-service/migrations"
)

func TestEmbeddedMigrationsAreCompatibleWithStatementSplitter(t *testing.T) {
	files, err := migrationFiles(servicemigrations.Files)
	if err != nil {
		t.Fatal(err)
	}
	for _, file := range files {
		contents, err := fs.ReadFile(servicemigrations.Files, file)
		if err != nil {
			t.Fatalf("read %s: %v", file, err)
		}
		for _, statement := range strings.Split(string(contents), ";") {
			if strings.Count(statement, "$$")%2 != 0 {
				t.Fatalf("%s contains a dollar-quoted block that the migration splitter truncates", file)
			}
		}
	}
}
