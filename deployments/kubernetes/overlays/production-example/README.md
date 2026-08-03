# Production overlay example

Copy this directory to the environment-specific deployment repository or
replace every example value before applying it.

Required changes:

- Replace `ghcr.io/your-org/*:replace-me` with immutable image digests.
- Replace the two example hostnames and provide the referenced TLS Secret.
- Set `bbs-web-config.js` to the public API base URL. Keep `/api/v1` when the
  frontend and Gateway share the Ingress origin.
- Create a separate `bbs-<service>-runtime` Secret for every backend service.
- Set the User service runtime Secret's `mfa.encryptionKey` to a dedicated,
  stable random value of at least 32 bytes; do not reuse a JWT or internal
  authentication secret.
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
- Preserve the four non-overlapping Snowflake worker-ID ranges in the base
  StatefulSets. If an environment shares a database with another BBS
  installation, patch all four ranges to a disjoint set. Any HPA must cap its
  `maxReplicas` at the assigned range size (192 in the base).
- For an existing legacy installation, perform the documented one-time
  Deployment-to-StatefulSet migration before applying this overlay; Kubernetes
  cannot change the workload kind in place.
- Verify database migrations as release Jobs before applying this overlay.

The `base/jobs` package is intentionally not referenced here. Migration Job
names must be unique per release, so generate a release-specific overlay or
use the deployment controller's pre-deploy Job support.
