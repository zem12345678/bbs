# admin-service

## Test

Run the default unit test suite:

```powershell
go test ./...
```

Run the optional Postgres-backed repository test against the local development schema:

```powershell
$env:BBS_ADMIN_TEST_DSN='postgres://bbs_admin_app:local_admin_pass@127.0.0.1:5432/bbs?sslmode=disable&search_path=bbs_admin'
$env:BBS_ADMIN_TEST_USE_EXISTING_SCHEMA='1'
go test ./internal/infrastructure/persistence -run TestRepositoryProtectsBuiltInSystemRoles -count=1 -v
```

Without `BBS_ADMIN_TEST_USE_EXISTING_SCHEMA=1`, the test creates and drops a temporary schema. The configured database user must have `CREATE SCHEMA` permission.
