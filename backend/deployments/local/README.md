# BBS Local Infrastructure

This directory contains local infrastructure for backend development. It starts only infrastructure containers; Go services and the frontend still run from the host.

## First Run

```powershell
cd D:\projects\bbs\backend\deployments\local
Copy-Item .env.example .env
# Set MINIO_ACCESS_KEY and MINIO_SECRET_KEY in .env for the existing MinIO instance.
docker compose up -d
.\scripts\bootstrap.ps1
```

PostgreSQL, Nacos, Elasticsearch, and MinIO are external dependencies. They are never started, stopped, or reset by this Compose project. By default the bootstrap script connects to PostgreSQL at `127.0.0.1:5432` as user `postgres`, creates the `bbs` database if needed, then applies schemas and local app users. If your local PostgreSQL requires a password, set `PGPASSWORD` in the shell before running bootstrap.

`bootstrap` creates the `bbs-local` namespace and BBS-only config entries in the external Nacos instance. It creates Elasticsearch indices only when missing. The `bbs-local` MinIO bucket is created lazily by the API gateway on its first upload, so bootstrap does not run a MinIO client container.

This starts the default profile:

- Redis
- etcd
- Nacos

## Optional Profiles

Comments:

```powershell
docker compose --profile comments up -d
.\scripts\bootstrap.ps1 -Comments
```

Events:

```powershell
docker compose --profile events up -d
.\scripts\bootstrap.ps1 -Events
```

Search:

```powershell
.\scripts\bootstrap.ps1 -Search
```

Files:

```powershell
.\scripts\bootstrap.ps1 -Files
```

Full local P0 infrastructure:

```powershell
docker compose --profile comments --profile events --profile mail up -d
.\scripts\bootstrap.ps1 -Full
```

## Local Endpoints

| Component | URL / Address |
| --- | --- |
| PostgreSQL | `127.0.0.1:5432` |
| Redis | `127.0.0.1:6379` |
| etcd | `127.0.0.1:2379` |
| Nacos (external) | `NACOS_URL` (`http://127.0.0.1:8848` by default) |
| Kafka | `127.0.0.1:9092` |
| Kafka UI | `http://127.0.0.1:8088` |
| MongoDB | `127.0.0.1:27017` |
| Elasticsearch (external) | `ELASTICSEARCH_URL` (`http://127.0.0.1:9200` by default) |
| MinIO Console (external) | `MINIO_CONSOLE_URL` (`http://127.0.0.1:19001` by default) |
| Mailpit | `http://127.0.0.1:8025` |

## Health Checks

```powershell
docker compose ps
psql --host 127.0.0.1 --port 5432 --username postgres --dbname bbs --command "select 1"
docker compose exec redis redis-cli ping
docker compose exec etcd etcdctl endpoint health --endpoints=http://127.0.0.1:2379
Invoke-WebRequest http://127.0.0.1:8848/nacos/ -UseBasicParsing
```

Optional services:

```powershell
docker compose exec mongodb mongosh --eval "db.adminCommand('ping')"
docker compose exec kafka kafka-topics.sh --bootstrap-server 127.0.0.1:29092 --list
Invoke-WebRequest http://127.0.0.1:9200/_cluster/health -UseBasicParsing
Invoke-WebRequest http://127.0.0.1:19000/minio/health/live -UseBasicParsing
Invoke-WebRequest http://127.0.0.1:8025 -UseBasicParsing
```

## Reset

Soft reset, without deleting data:

```powershell
.\scripts\bootstrap.ps1 -Full
```

Hard reset, deleting local Docker volumes for this Compose project:

```powershell
.\scripts\reset.ps1 -Confirm
docker compose --profile comments --profile events --profile mail up -d
.\scripts\bootstrap.ps1 -Full
```

The reset script refuses to run unless the Compose project name is `bbs-local`.
It does not delete PostgreSQL, Nacos, Elasticsearch, MinIO, or orphaned containers from an earlier Compose definition.

## Bash Equivalents

```bash
cd backend/deployments/local
cp .env.example .env
# Set MINIO_ACCESS_KEY and MINIO_SECRET_KEY in .env for the existing MinIO instance.
docker compose up -d
./scripts/bootstrap.sh
```

Override local PostgreSQL connection with `POSTGRES_HOST`, `POSTGRES_PORT`, `POSTGRES_USER`, and `POSTGRES_DATABASE` if needed.

Full:

```bash
docker compose --profile comments --profile events --profile mail up -d
./scripts/bootstrap.sh --full
```

Reset:

```bash
./scripts/reset.sh --confirm
```
