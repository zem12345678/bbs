# Local Infra Compose Draft

This document translates `10-local-dev-topology.md` into a concrete Docker Compose design for local infrastructure. It is still a draft, not the final `docker-compose.yaml`.

The goal is to make the next implementation step mechanical: create `backend/deployments/local/docker-compose.yaml`, `.env.example`, bootstrap scripts, and initialization assets from this design.

Implementation status: the first local Compose implementation now lives under `backend/deployments/local`. Keep this design document as the rationale and checklist for future changes.

## Scope

Included local infrastructure:

- PostgreSQL
- Redis
- MongoDB
- Kafka
- Kafka UI
- etcd
- Nacos
- Elasticsearch
- MinIO
- Mailpit

Not included in the infra Compose file:

- Go application services.
- Frontend Vite dev server.
- Production-grade HA clustering.
- Production secrets or TLS.

Go services should run on the host during development and connect to local infrastructure through `127.0.0.1` ports.

## Compose Strategy

Use one Compose project:

```text
project name: bbs-local
network: bbs-local-net
```

Use named volumes for persistent local data:

```text
postgres-data
redis-data
mongo-data
kafka-data
etcd-data
nacos-data
elasticsearch-data
minio-data
```

Use Compose profiles so developers can start only what a slice needs:

| Profile | Services | Use Case |
| --- | --- | --- |
| default | PostgreSQL, Redis, etcd, Nacos | S2 auth/profile |
| `comments` | MongoDB | S4 comments |
| `events` | Kafka, Kafka UI | S3/S4/S5 events |
| `search` | Elasticsearch | S5 search |
| `files` | MinIO | Later file-service work |
| `mail` | Mailpit | Password reset/email verification |
| `full` | All optional infrastructure | Full P0 integration |

Compose detail: services without a profile start by default. Optional services should have one or more profiles.

## Planned File Layout

When implementation starts, create this structure:

```text
backend/
  deployments/
    local/
      docker-compose.yaml
      .env.example
      README.md
      postgres/
        init/
          001-create-database-and-schemas.sql
      kafka/
        topics.txt
      mongo/
        init/
          001-comments-indexes.js
      elasticsearch/
        bbs_topics.mapping.json
        bbs_articles.mapping.json
        bbs_users.mapping.json
        bbs_tags.mapping.json
      nacos/
        configs/
          bbs-common.yaml
          api-gateway.yaml
          auth-service.yaml
          user-service.yaml
          content-service.yaml
          comment-service.yaml
          reaction-service.yaml
          search-service.yaml
          notification-service.yaml
          admin-service.yaml
      scripts/
        bootstrap.ps1
        bootstrap.sh
        reset.ps1
        reset.sh
```

Keep scripts small and idempotent. Windows is the primary local environment in this workspace, so PowerShell scripts should be first-class.

## Environment Variables

The final `.env.example` should pin concrete image versions. This draft uses placeholders so the team can verify and pin versions during implementation.

```dotenv
COMPOSE_PROJECT_NAME=bbs-local

POSTGRES_IMAGE=postgres:<pin-version>
REDIS_IMAGE=redis:<pin-version>
MONGO_IMAGE=mongo:<pin-version>
KAFKA_IMAGE=bitnami/kafka:<pin-version>
KAFKA_UI_IMAGE=provectuslabs/kafka-ui:<pin-version>
ETCD_IMAGE=bitnami/etcd:<pin-version>
NACOS_IMAGE=nacos/nacos-server:<pin-version>
ELASTICSEARCH_IMAGE=docker.elastic.co/elasticsearch/elasticsearch:<pin-version>
MINIO_IMAGE=minio/minio:<pin-version>
MAILPIT_IMAGE=axllent/mailpit:<pin-version>

POSTGRES_PORT=5432
POSTGRES_DB=bbs
POSTGRES_SUPERUSER=postgres
POSTGRES_SUPERPASS=postgres

REDIS_PORT=6379
MONGO_PORT=27017
KAFKA_PORT=9092
KAFKA_INTERNAL_PORT=29092
KAFKA_UI_PORT=8088
ETCD_CLIENT_PORT=2379
ETCD_PEER_PORT=2380
NACOS_HTTP_PORT=8848
NACOS_GRPC_PORT=9848
NACOS_RAFT_PORT=9849
ELASTICSEARCH_HTTP_PORT=9200
ELASTICSEARCH_TRANSPORT_PORT=9300
MINIO_API_PORT=9000
MINIO_CONSOLE_PORT=9001
MAILPIT_SMTP_PORT=1025
MAILPIT_HTTP_PORT=8025

MINIO_ROOT_USER=minioadmin
MINIO_ROOT_PASSWORD=minioadmin

BBS_ENV=local
NACOS_NAMESPACE=bbs-local
NACOS_GROUP=BBS_LOCAL
ETCD_ENDPOINTS=127.0.0.1:2379
```

Rules:

1. Never commit production secrets.
2. Local passwords can be simple but must be marked development-only.
3. Application service DSNs should live in Nacos config or untracked local env files, not hardcoded in Go code.
4. Image tags must be pinned before creating the actual Compose file.

## Service Design

### PostgreSQL

Purpose:

- P0 relational source of truth.
- One local instance, one `bbs` database, multiple service schemas.

Ports:

```text
host 5432 -> container 5432
```

Volume:

```text
postgres-data:/var/lib/postgresql/data
```

Planned init:

```sql
CREATE DATABASE bbs;
```

Then create service schemas and users inside `bbs`:

```sql
CREATE SCHEMA IF NOT EXISTS bbs_auth;
CREATE SCHEMA IF NOT EXISTS bbs_user;
CREATE SCHEMA IF NOT EXISTS bbs_content;
CREATE SCHEMA IF NOT EXISTS bbs_reaction;
CREATE SCHEMA IF NOT EXISTS bbs_credit;
CREATE SCHEMA IF NOT EXISTS bbs_notification;
CREATE SCHEMA IF NOT EXISTS bbs_admin;
CREATE SCHEMA IF NOT EXISTS bbs_config;
CREATE SCHEMA IF NOT EXISTS bbs_file;
CREATE SCHEMA IF NOT EXISTS bbs_audit;
```

Implementation note:

- `docker-entrypoint-initdb.d` scripts only run on first volume creation.
- Use a separate idempotent bootstrap script for schema/user adjustments after reset.
- App service users should receive only schema-level privileges for their own schema.

Health check:

```text
pg_isready -U ${POSTGRES_SUPERUSER} -d ${POSTGRES_DB}
```

### Redis

Purpose:

- Sessions.
- Hot counters.
- Rate limits.
- Idempotency keys.
- Later rankings.

Ports:

```text
host 6379 -> container 6379
```

Volume:

```text
redis-data:/data
```

Recommended local command:

```text
redis-server --appendonly yes
```

Health check:

```text
redis-cli ping
```

### MongoDB

Purpose:

- Comment bodies and reply structure.

Profile:

```text
comments
full
```

Ports:

```text
host 27017 -> container 27017
```

Volume:

```text
mongo-data:/data/db
```

Database:

```text
bbs_comment
```

Bootstrap:

- Create `comments` indexes from `09-p0-schema-draft.md`.
- Create `comment_audit_logs` only when moderation audit flow needs it.

Health check:

```text
mongosh --eval "db.adminCommand('ping')"
```

### Kafka

Purpose:

- Domain events.
- Search indexing.
- Notifications.
- Counters and audit projections.

Profile:

```text
events
full
```

Use a single broker in KRaft mode for local development. The final Compose should expose:

```text
host 9092 -> external listener for host Go services
container 29092 -> internal listener for other containers
```

Draft listener model:

```text
KAFKA_CFG_LISTENERS=PLAINTEXT://:29092,EXTERNAL://:9092,CONTROLLER://:9093
KAFKA_CFG_ADVERTISED_LISTENERS=PLAINTEXT://kafka:29092,EXTERNAL://127.0.0.1:9092
KAFKA_CFG_LISTENER_SECURITY_PROTOCOL_MAP=PLAINTEXT:PLAINTEXT,EXTERNAL:PLAINTEXT,CONTROLLER:PLAINTEXT
```

Bootstrap topics from `kafka/topics.txt`:

```text
content.topic.created
content.topic.updated
content.topic.deleted
content.article.created
content.article.updated
content.article.deleted
comment.created
comment.deleted
reaction.liked
reaction.unliked
reaction.favorited
reaction.unfavorited
reaction.reported
user.created
user.updated
user.followed
user.unfollowed
admin.config.changed
admin.forbidden_word.changed
audit.operation.recorded
dlq.search
dlq.notification
dlq.credit
dlq.audit
```

Local topic defaults:

```text
partitions: 1 for normal topics
partitions: 3 for reaction/comment topics if testing load behavior
replication-factor: 1
```

Health check:

```text
kafka-topics.sh --bootstrap-server 127.0.0.1:9092 --list
```

### Kafka UI

Purpose:

- Inspect topics, consumer groups, and messages locally.

Profile:

```text
events
full
```

Ports:

```text
host 8088 -> container 8080
```

Depends on:

```text
kafka
```

Configuration:

```text
KAFKA_CLUSTERS_0_NAME=bbs-local
KAFKA_CLUSTERS_0_BOOTSTRAPSERVERS=kafka:29092
```

### etcd

Purpose:

- Service discovery only.

Ports:

```text
host 2379 -> container 2379
host 2380 -> container 2380
```

Volume:

```text
etcd-data:/bitnami/etcd
```

Local auth:

```text
disabled for initial local development
```

Discovery key shape:

```text
/bbs/local/services/{serviceName}/{instanceId}
```

Health check:

```text
etcdctl endpoint health --endpoints=http://127.0.0.1:2379
```

### Nacos

Purpose:

- Runtime configuration center.

Ports:

```text
host 8848 -> container 8848
host 9848 -> container 9848
host 9849 -> container 9849
```

Volume:

```text
nacos-data:/home/nacos/data
```

Mode:

```text
standalone
```

Draft env:

```text
MODE=standalone
NACOS_AUTH_ENABLE=false
```

Namespace and group:

```text
namespace: bbs-local
group: BBS_LOCAL
```

Bootstrap:

- Create/import common config.
- Create/import service configs.
- Use YAML data IDs listed in `10-local-dev-topology.md`.

Health check:

```text
GET http://127.0.0.1:8848/nacos/actuator/health
```

If the chosen Nacos image does not expose this endpoint, use a simple HTTP readiness check against the Nacos console path.

### Elasticsearch

Purpose:

- Search projections.

Profile:

```text
search
full
```

Ports:

```text
host 9200 -> container 9200
host 9300 -> container 9300
```

Volume:

```text
elasticsearch-data:/usr/share/elasticsearch/data
```

Local settings:

```text
discovery.type=single-node
xpack.security.enabled=false
ES_JAVA_OPTS=-Xms1g -Xmx1g
```

Bootstrap mappings:

```text
bbs_topics
bbs_articles
bbs_users
bbs_tags
```

Health check:

```text
GET http://127.0.0.1:9200/_cluster/health
```

Windows note:

- Elasticsearch can be memory-sensitive. Keep heap configurable through `.env`.
- Document any required Docker Desktop memory setting in `backend/deployments/local/README.md`.

### MinIO

Purpose:

- Local S3-compatible object storage for future file-service work.

Profile:

```text
files
full
```

Ports:

```text
host 9000 -> container 9000
host 9001 -> container 9001
```

Volume:

```text
minio-data:/data
```

Command:

```text
server /data --console-address ":9001"
```

Bootstrap:

- Create bucket `bbs-local`.
- Keep file-service optional until file upload work starts.

Health check:

```text
GET http://127.0.0.1:9000/minio/health/live
```

### Mailpit

Purpose:

- Capture local password reset and email verification messages.

Profile:

```text
mail
full
```

Ports:

```text
host 1025 -> container 1025
host 8025 -> container 8025
```

Usage:

```text
SMTP endpoint for services: 127.0.0.1:1025
Web UI: http://127.0.0.1:8025
```

## Draft Compose Shape

This is intentionally partial. It shows structure and wiring, not final image tags.

```yaml
name: bbs-local

services:
  postgres:
    image: ${POSTGRES_IMAGE}
    ports:
      - "${POSTGRES_PORT}:5432"
    environment:
      POSTGRES_USER: ${POSTGRES_SUPERUSER}
      POSTGRES_PASSWORD: ${POSTGRES_SUPERPASS}
      POSTGRES_DB: ${POSTGRES_DB}
    volumes:
      - postgres-data:/var/lib/postgresql/data
      - ./postgres/init:/docker-entrypoint-initdb.d:ro
    networks:
      - bbs-local-net

  redis:
    image: ${REDIS_IMAGE}
    command: ["redis-server", "--appendonly", "yes"]
    ports:
      - "${REDIS_PORT}:6379"
    volumes:
      - redis-data:/data
    networks:
      - bbs-local-net

  mongodb:
    image: ${MONGO_IMAGE}
    profiles: ["comments", "full"]
    ports:
      - "${MONGO_PORT}:27017"
    volumes:
      - mongo-data:/data/db
    networks:
      - bbs-local-net

  kafka:
    image: ${KAFKA_IMAGE}
    profiles: ["events", "full"]
    ports:
      - "${KAFKA_PORT}:9092"
    volumes:
      - kafka-data:/bitnami/kafka
    networks:
      - bbs-local-net

  kafka-ui:
    image: ${KAFKA_UI_IMAGE}
    profiles: ["events", "full"]
    ports:
      - "${KAFKA_UI_PORT}:8080"
    depends_on:
      - kafka
    networks:
      - bbs-local-net

  etcd:
    image: ${ETCD_IMAGE}
    ports:
      - "${ETCD_CLIENT_PORT}:2379"
      - "${ETCD_PEER_PORT}:2380"
    volumes:
      - etcd-data:/bitnami/etcd
    networks:
      - bbs-local-net

  nacos:
    image: ${NACOS_IMAGE}
    ports:
      - "${NACOS_HTTP_PORT}:8848"
      - "${NACOS_GRPC_PORT}:9848"
      - "${NACOS_RAFT_PORT}:9849"
    environment:
      MODE: standalone
      NACOS_AUTH_ENABLE: "false"
    volumes:
      - nacos-data:/home/nacos/data
    networks:
      - bbs-local-net

  elasticsearch:
    image: ${ELASTICSEARCH_IMAGE}
    profiles: ["search", "full"]
    ports:
      - "${ELASTICSEARCH_HTTP_PORT}:9200"
      - "${ELASTICSEARCH_TRANSPORT_PORT}:9300"
    environment:
      discovery.type: single-node
      xpack.security.enabled: "false"
      ES_JAVA_OPTS: "-Xms1g -Xmx1g"
    volumes:
      - elasticsearch-data:/usr/share/elasticsearch/data
    networks:
      - bbs-local-net

  minio:
    image: ${MINIO_IMAGE}
    profiles: ["files", "full"]
    command: ["server", "/data", "--console-address", ":9001"]
    ports:
      - "${MINIO_API_PORT}:9000"
      - "${MINIO_CONSOLE_PORT}:9001"
    environment:
      MINIO_ROOT_USER: ${MINIO_ROOT_USER}
      MINIO_ROOT_PASSWORD: ${MINIO_ROOT_PASSWORD}
    volumes:
      - minio-data:/data
    networks:
      - bbs-local-net

  mailpit:
    image: ${MAILPIT_IMAGE}
    profiles: ["mail", "full"]
    ports:
      - "${MAILPIT_SMTP_PORT}:1025"
      - "${MAILPIT_HTTP_PORT}:8025"
    networks:
      - bbs-local-net

networks:
  bbs-local-net:
    name: bbs-local-net

volumes:
  postgres-data:
  redis-data:
  mongo-data:
  kafka-data:
  etcd-data:
  nacos-data:
  elasticsearch-data:
  minio-data:
```

Final implementation must add health checks and Kafka/Nacos-specific environment details after image versions are pinned.

## Bootstrap Design

Bootstrap should be explicit, not hidden inside long Compose commands.

Recommended command shape:

```powershell
cd backend/deployments/local
docker compose up -d
./scripts/bootstrap.ps1
```

For full P0:

```powershell
docker compose --profile comments --profile events --profile search --profile mail up -d
./scripts/bootstrap.ps1 -Full
```

Bootstrap steps:

1. Wait for PostgreSQL, Redis, etcd, and Nacos.
2. Create PostgreSQL service schemas and local service users.
3. Import Nacos common and service configs.
4. If `events` profile is active, create Kafka topics.
5. If `comments` profile is active, create MongoDB indexes.
6. If `search` profile is active, create Elasticsearch mappings.
7. If `files` profile is active, create MinIO bucket.
8. Print host connection strings for Go services.

Idempotency rules:

- Running bootstrap multiple times must not fail on existing schemas, topics, indexes, buckets, or configs.
- Bootstrap should not delete local data.
- Reset scripts may delete volumes, but they must ask for explicit confirmation.

## Service Connection Values

Host-run Go services should use:

```text
PostgreSQL: 127.0.0.1:5432
Redis: 127.0.0.1:6379
MongoDB: 127.0.0.1:27017
Kafka: 127.0.0.1:9092
etcd: 127.0.0.1:2379
Nacos: 127.0.0.1:8848
Elasticsearch: http://127.0.0.1:9200
MinIO: http://127.0.0.1:9000
SMTP: 127.0.0.1:1025
```

Container-to-container addresses should use service names:

```text
postgres:5432
redis:6379
mongodb:27017
kafka:29092
etcd:2379
nacos:8848
elasticsearch:9200
minio:9000
mailpit:1025
```

## Nacos Config Draft

`bbs-common.yaml` should include shared local defaults:

```yaml
env: local
log:
  level: debug
discovery:
  etcd:
    endpoints:
      - 127.0.0.1:2379
kafka:
  brokers:
    - 127.0.0.1:9092
redis:
  addr: 127.0.0.1:6379
```

Service-specific configs should own their datastore values:

```yaml
service:
  name: content-service
  grpcPort: 9103
postgres:
  dsn: postgres://bbs_content_app:${BBS_CONTENT_DB_PASSWORD}@127.0.0.1:5432/bbs?search_path=bbs_content
```

Do not commit real service passwords. The actual implementation should choose between:

1. Local-only simple passwords in `.env.example`.
2. Untracked `.env.local`.
3. Nacos configs imported from templates with environment substitution.

## PostgreSQL Schema/User Draft

Recommended local app users:

```text
bbs_auth_app
bbs_user_app
bbs_content_app
bbs_reaction_app
bbs_credit_app
bbs_notification_app
bbs_admin_app
bbs_config_app
bbs_file_app
bbs_audit_app
```

Grant pattern:

```sql
GRANT USAGE ON SCHEMA bbs_content TO bbs_content_app;
GRANT CREATE ON SCHEMA bbs_content TO bbs_content_app;
ALTER DEFAULT PRIVILEGES IN SCHEMA bbs_content
  GRANT SELECT, INSERT, UPDATE, DELETE ON TABLES TO bbs_content_app;
ALTER DEFAULT PRIVILEGES IN SCHEMA bbs_content
  GRANT USAGE, SELECT ON SEQUENCES TO bbs_content_app;
```

No app user should receive privileges on another service schema.

## Health And Smoke Checks

After `docker compose up -d`, the local runbook should verify:

```powershell
docker compose ps
docker compose exec postgres pg_isready -U postgres -d bbs
docker compose exec redis redis-cli ping
docker compose exec etcd etcdctl endpoint health --endpoints=http://127.0.0.1:2379
Invoke-WebRequest http://127.0.0.1:8848/nacos/ -UseBasicParsing
```

When optional profiles are active:

```powershell
docker compose exec mongodb mongosh --eval "db.adminCommand('ping')"
docker compose exec kafka kafka-topics.sh --bootstrap-server 127.0.0.1:9092 --list
Invoke-WebRequest http://127.0.0.1:9200/_cluster/health -UseBasicParsing
Invoke-WebRequest http://127.0.0.1:9000/minio/health/live -UseBasicParsing
Invoke-WebRequest http://127.0.0.1:8025 -UseBasicParsing
```

Adjust these commands to the actual chosen images. Some images use different binary paths.

## Reset Design

Provide two reset levels:

### Soft Reset

Purpose:

- Re-run bootstrap without deleting data.
- Useful after adding mappings/topics/config templates.

Command:

```powershell
./scripts/bootstrap.ps1
```

### Hard Reset

Purpose:

- Delete local volumes and return to a fresh environment.

Command:

```powershell
./scripts/reset.ps1 -Confirm
docker compose up -d
./scripts/bootstrap.ps1 -Full
```

Hard reset must:

- Stop Compose services.
- Remove only volumes owned by the `bbs-local` project.
- Refuse to run if project name is empty or not `bbs-local`.
- Print the volume names before deletion.

## Implementation Checklist

Before creating the actual Compose file:

1. Pin image versions in `.env.example`.
2. Confirm each chosen image's health check command.
3. Confirm Kafka listener environment for the chosen Kafka image.
4. Confirm Nacos standalone persistence path and health endpoint.
5. Confirm Elasticsearch image version and memory requirement.
6. Create idempotent bootstrap scripts.
7. Keep Go services out of the infra Compose unless a later decision changes local development workflow.
8. Add `backend/deployments/local/README.md` with Windows-first commands and equivalent Bash commands.
