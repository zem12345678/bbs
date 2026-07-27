# BBS Local Infrastructure

This directory manages only the BBS Mailpit container. PostgreSQL, Redis, etcd, Nacos, Kafka, MongoDB, Elasticsearch, and MinIO are shared external dependencies. BBS Compose and its reset scripts never create, stop, delete, or reset them.

## First Run

```powershell
cd D:\projects\bbs\backend\deployments\local
Copy-Item .env.example .env
# Set MINIO_ACCESS_KEY and MINIO_SECRET_KEY for the existing MinIO instance,
# plus the BBS_LOCAL_* JWT/internal-token values used when publishing Nacos
# templates. The bootstrap scripts reject missing placeholder values.
# Set MINIO_CONTAINER only when attachment-smoke should inspect objects through that existing container.
docker compose up -d # Starts only bbs-local-mailpit.
.\scripts\bootstrap.ps1
```

`bootstrap` verifies the external dependencies, applies the BBS PostgreSQL schemas and local app users, and publishes BBS-only entries in the `bbs-local` Nacos namespace. It creates Elasticsearch indices only when missing. It never changes external Kafka topics, MongoDB indexes, or application tables.

Before starting any BBS service, provision every topic in
[`kafka/topics.txt`](kafka/topics.txt) in the external Kafka cluster. That
file is the canonical BBS topic inventory, including `chat.events` and the
dead-letter topics. Bootstrap deliberately does not create or alter topics;
producers disable automatic topic creation, and chat-service additionally
verifies that `chat.events` has readable partitions during startup.

## Application migrations

Run application migrations explicitly after bootstrap and before starting backend
services. This mirrors the production release flow and keeps regular service
startup free of schema changes.

```powershell
.\scripts\migrate.ps1
```

To migrate only selected services:

```powershell
.\scripts\migrate.ps1 -Services credit-service, mall-service
```

## Optional Checks

Events:

```powershell
.\scripts\bootstrap.ps1 -Events
```

Comments:

```powershell
.\scripts\bootstrap.ps1 -Comments
```

Search:

```powershell
.\scripts\bootstrap.ps1 -Search
```

Files:

```powershell
.\scripts\bootstrap.ps1 -Files
```

Full local verification:

```powershell
docker compose up -d # Starts only bbs-local-mailpit if it is not already running.
.\scripts\bootstrap.ps1 -Full
.\scripts\migrate.ps1
```

## Local Endpoints

| Component | URL / Address |
| --- | --- |
| PostgreSQL (external) | `127.0.0.1:5432` |
| Redis (external) | `127.0.0.1:6379` |
| etcd (external) | `127.0.0.1:2379` |
| Nacos (external) | `NACOS_URL` (`http://127.0.0.1:8848` by default) |
| Kafka (external) | `127.0.0.1:9092` |
| MongoDB (external) | `127.0.0.1:27017` |
| Elasticsearch (external) | `ELASTICSEARCH_URL` (`http://127.0.0.1:9200` by default) |
| MinIO Console (external) | `MINIO_CONSOLE_URL` (`http://127.0.0.1:19001` by default) |
| BBS Mailpit | `http://127.0.0.1:8025` |

## Health Checks

```powershell
docker compose ps
psql --host 127.0.0.1 --port 5432 --username postgres --dbname bbs --command "select 1"
Test-NetConnection 127.0.0.1 -Port 6379
Test-NetConnection 127.0.0.1 -Port 2379
Test-NetConnection 127.0.0.1 -Port 9092
Test-NetConnection 127.0.0.1 -Port 27017
Invoke-WebRequest http://127.0.0.1:8848/nacos/ -UseBasicParsing
Invoke-WebRequest http://127.0.0.1:9200/_cluster/health -UseBasicParsing
Invoke-WebRequest http://127.0.0.1:19000/minio/health/live -UseBasicParsing
Invoke-WebRequest http://127.0.0.1:8025 -UseBasicParsing
```

## Reset

Soft reset, without deleting data:

```powershell
.\scripts\bootstrap.ps1 -Full
```

Hard reset only stops and removes the BBS Mailpit container. It does not call project-wide `docker compose down` or remove any volumes:

```powershell
.\scripts\reset.ps1 -Confirm
docker compose up -d
.\scripts\bootstrap.ps1 -Full
```

The reset script refuses to run unless the Compose project name is `bbs-local`. It does not delete PostgreSQL, Redis, etcd, Nacos, Kafka, MongoDB, Elasticsearch, MinIO, or containers belonging to another project.

## Bash Equivalents

```bash
cd backend/deployments/local
cp .env.example .env
# Set MINIO_ACCESS_KEY and MINIO_SECRET_KEY for the existing MinIO instance.
docker compose up -d # Starts only bbs-local-mailpit.
./scripts/bootstrap.sh
./scripts/migrate.sh
```

Override local PostgreSQL connection with `POSTGRES_HOST`, `POSTGRES_PORT`, `POSTGRES_USER`, and `POSTGRES_DATABASE` if needed.

Full:

```bash
docker compose up -d
./scripts/bootstrap.sh --full
./scripts/migrate.sh
```

Reset:

```bash
./scripts/reset.sh --confirm
```
