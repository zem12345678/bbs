# Production overlay example

Copy this directory to the environment-specific deployment repository or
replace every example value before applying it.

Required changes:

- Replace `ghcr.io/your-org/*:replace-me` with immutable image digests.
- Replace the two example hostnames and provide the referenced TLS Secret.
- Create a separate `bbs-<service>-runtime` Secret for every backend service.
- Provision every Kafka topic in
  [`backend/deployments/local/kafka/topics.txt`](../../../backend/deployments/local/kafka/topics.txt)
  through the Kafka platform before deploying application Pods. BBS producers
  deliberately disable automatic topic creation.
- Create the three internal mTLS Secrets required by the base:
  `bbs-api-gateway-grpc-client-tls`, `bbs-admin-service-grpc-server-tls`, and
  `bbs-chat-service-grpc-server-tls`. Each must contain `ca.crt`, `tls.crt`,
  and `tls.key`; see the Kubernetes base README for the trust and DNS-SAN
  requirements.
- Set production CORS origins and the real ingress proxy CIDR in the Gateway
  runtime configuration; set `http.publicBaseURL` to the canonical public
  HTTPS origin (normally `https://bbs.example.com`) so media and OAuth URLs do
  not depend on request headers. Do not use the example local values.
- Keep the Gateway object-store bucket private. The Ingress exposes only
  `/uploads/avatars` and `/uploads/images` through the Gateway; it must never
  expose topic attachment object keys directly.
- Select a deployment-unique Chat Snowflake worker ID and patch both the Chat
  Deployment and Chat migration Job consistently. Do not add an HPA or a
  surge-based rollout for Chat, User, Content, or Comment before an allocator
  is available.
- Verify database migrations as release Jobs before applying this overlay.

The `base/jobs` package is intentionally not referenced here. Migration Job
names must be unique per release, so generate a release-specific overlay or
use the deployment controller's pre-deploy Job support.
