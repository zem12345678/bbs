# BBS Local Infrastructure

This directory contains local infrastructure for backend development. It starts only infrastructure containers; Go services and the frontend still run from the host.

## First Run

```powershell
cd D:\projects\bbs\backend\deployments\local
Copy-Item .env.example .env
docker compose up -d
.\scripts\bootstrap.ps1
```

This starts the default profile:

- PostgreSQL
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
docker compose --profile search up -d
.\scripts\bootstrap.ps1 -Search
```

Full local P0 infrastructure:

```powershell
docker compose --profile comments --profile events --profile search --profile mail --profile files up -d
.\scripts\bootstrap.ps1 -Full
```

## Local Endpoints

| Component | URL / Address |
| --- | --- |
| PostgreSQL | `127.0.0.1:5432` |
| Redis | `127.0.0.1:6379` |
| etcd | `127.0.0.1:2379` |
| Nacos | `http://127.0.0.1:8848/nacos/` |
| Kafka | `127.0.0.1:9092` |
| Kafka UI | `http://127.0.0.1:8088` |
| MongoDB | `127.0.0.1:27017` |
| Elasticsearch | `http://127.0.0.1:9200` |
| MinIO Console | `http://127.0.0.1:9001` |
| Mailpit | `http://127.0.0.1:8025` |

## Health Checks

```powershell
docker compose ps
docker compose exec postgres pg_isready -U postgres -d bbs
docker compose exec redis redis-cli ping
docker compose exec etcd etcdctl endpoint health --endpoints=http://127.0.0.1:2379
Invoke-WebRequest http://127.0.0.1:8848/nacos/ -UseBasicParsing
```

Optional services:

```powershell
docker compose exec mongodb mongosh --eval "db.adminCommand('ping')"
docker compose exec kafka kafka-topics.sh --bootstrap-server 127.0.0.1:29092 --list
Invoke-WebRequest http://127.0.0.1:9200/_cluster/health -UseBasicParsing
Invoke-WebRequest http://127.0.0.1:9000/minio/health/live -UseBasicParsing
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
docker compose --profile comments --profile events --profile search --profile mail --profile files up -d
.\scripts\bootstrap.ps1 -Full
```

The reset script refuses to run unless the Compose project name is `bbs-local`.

## Bash Equivalents

```bash
cd backend/deployments/local
cp .env.example .env
docker compose up -d
./scripts/bootstrap.sh
```

Full:

```bash
docker compose --profile comments --profile events --profile search --profile mail --profile files up -d
./scripts/bootstrap.sh --full
```

Reset:

```bash
./scripts/reset.sh --confirm
```

