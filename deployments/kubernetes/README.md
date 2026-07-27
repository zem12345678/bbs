# Kubernetes delivery baseline

This directory provides a vendor-neutral deployment baseline for the complete
BBS product. It intentionally contains no credentials and no executable
production configuration.

Do not apply `base` directly. Copy `overlays/production-example`, replace the
example image names with immutable digests, set real domains, and create the
per-service runtime Secrets before applying it.

Each backend Pod expects a Secret named `bbs-<service>-runtime` with a
`config.yaml` key mounted at `/app/config/config.yaml`. The backend image does
not contain its local development configuration, so a missing Secret fails
closed rather than booting with a local JWT, database password, or Nacos value.

The configuration Secret must contain the complete production configuration
for exactly one service. Keep passwords, JWT keys, internal tokens, object
storage credentials, and TLS paths out of Nacos and ConfigMaps. Use an
external secret manager, SOPS, or the deployment platform's encrypted-secret
mechanism to create those Secrets.

Provision every topic listed in
[`backend/deployments/local/kafka/topics.txt`](../../backend/deployments/local/kafka/topics.txt)
before deploying the BBS services. Topic provisioning is owned by the Kafka
platform, not the application manifests: producers disable automatic topic
creation, and Chat additionally verifies that its configured `chat.events`
topic has readable partitions during startup.

The production manifests pin Gateway, Admin, User, Content, Comment, Reaction,
Search, Credit, Notification, File, Feed, Mall, and Chat to use the mounted runtime
Secret directly and skip Nacos. Treat that Secret as their immutable startup
configuration; do not make production availability depend on a Nacos instance
or place production credentials in Nacos.

## Public media contract

Avatar and editor-image uploads are stored in the Gateway's private object
store bucket under `uploads/avatars/` and `uploads/images/`. They are served
only through the Gateway's corresponding public routes; paid topic attachments
remain on their authenticated download route. Keep the bucket private and do
not add an anonymous-read policy as a shortcut.

Create the bucket and its private policy before deploying the Gateway. The
Gateway validates that the bucket is reachable at startup but does not create
it or change its policy. Grant its runtime identity only the bucket discovery
and object operations it needs for public media and authenticated attachments.

The Gateway runtime configuration must set `http.publicBaseURL` to the
canonical externally reachable origin (for example,
`https://bbs.example.com`). It is used for persisted media URLs and OAuth
callback URLs, so it must not be derived from incoming forwarding headers.
The production Ingress sends `/uploads/*` to the Gateway before the web SPA
fallback; the Gateway itself white-lists only the avatar and editor-image
public routes. Any CDN must preserve these paths and cache only the public
media response, never the private object-store endpoint.

## Internal mTLS contract

The base manifests enable the first production mTLS slice: API Gateway to
Admin and Chat. The required TLS Secrets are mounted read-only at `/app/tls`
in the long-running Pods and their migration Jobs.

| Workload | Secret name | Required keys |
| --- | --- | --- |
| API Gateway client | `bbs-api-gateway-grpc-client-tls` | `ca.crt`, `tls.crt`, `tls.key` |
| Admin server | `bbs-admin-service-grpc-server-tls` | `ca.crt`, `tls.crt`, `tls.key` |
| Chat server | `bbs-chat-service-grpc-server-tls` | `ca.crt`, `tls.crt`, `tls.key` |

The gateway client certificate must be accepted by both server Secrets, and
its CA bundle must trust both server certificates. Server certificates must
include the Kubernetes service DNS name used by the gateway: respectively
`bbs-admin-service` and `bbs-chat-service`. Do not set a shared
`grpc.client.tls.serverName`: the gateway deliberately verifies each service
name independently.

The manifests pin the TLS enablement flags and file paths through environment
variables, so a runtime `config.yaml` cannot accidentally downgrade this
slice to plaintext. Private keys remain only in the named TLS Secrets; the
runtime config Secret provides the production tokens and other application
configuration. User, Mall, and Credit remain explicitly plaintext until their
servers receive their own mTLS rollout. For a non-baseline deployment, set
`upstreams.adminInternalAuthSecure` and `upstreams.chatInternalAuthSecure` to
`true`, set the User/Mall/Credit equivalents to `false`, and do not rely on
the legacy global `grpc.client.secure` switch.

This is not a full-mesh mTLS deployment: every other gRPC edge remains outside
this slice. Do not enable TLS for an additional server in isolation, because
it can also receive calls from other services that still use plaintext. Migrate
the server, every caller, their certificates, and the corresponding per-edge
client configuration together.

The processes load certificate material at startup. After rotating any of
these Secrets, perform a controlled rollout so Gateway, Admin, and Chat load
the new key material; updating only the mounted Secret is not sufficient.

Apply in this order:

1. Create the namespace and all per-service runtime/TLS Secrets.
2. Apply the migration package for the release and wait for every Job to
   succeed.
3. Apply the application overlay.
4. Verify Gateway and web readiness, then enable the Ingress.

For the initial file-service internal-token rollout, deploy Gateway with the
matching token first, then deploy file-service enforcement. Token rotation is
not a zero-downtime operation while file-service accepts one token, so
coordinate both workloads in a release window.

`base/jobs` is deliberately separate from the long-running application base.
Do not include it in a continuous Deployment sync: create a fresh, uniquely
named migration Job for every release and block rollout on its success.

The baseline uses one replica and `Recreate` rollout strategy for stateful
Snowflake services (user, content, comment, and chat) until worker IDs are
allocated uniquely per Pod. This intentionally permits a brief rollout
outage rather than allowing old and new Pods to issue duplicate IDs. Do not
enable an HPA or change these services back to a surge-based rollout until an
instance-aware allocator is in place.

Chat also requires an explicit `BBS_CHAT_SNOWFLAKE_WORKER_ID`. The base uses
`16` for its single Chat replica, including its migration Job. Change both
values together in an environment overlay and choose an ID unique within the
deployment domain before using a separate BBS installation.

The gateway is intentionally configured with `BBS_GATEWAY_HTTP_HOST=0.0.0.0`
in its Deployment. Its local config remains loopback-only for developer
machines.

## Edge and network assumptions

The production overlay targets ingress-nginx in a namespace named
`ingress-nginx`. Its NetworkPolicy restricts ingress to BBS Pods and that
controller namespace; it intentionally does not restrict egress because the
database, cache, Kafka, object-storage, DNS, and service-discovery endpoints
are environment-specific. Adjust the namespace selector and add explicit
egress rules in the environment overlay when those endpoints are known, and
confirm that the cluster CNI enforces NetworkPolicy.

The Ingress requires HTTPS redirect, HSTS, a 52 MiB edge body limit, and no
Nginx version token. It uses `server-snippet` to disable `server_tokens`. If
the controller disallows snippet annotations, set `server-tokens: "false"` in
the ingress-nginx controller configuration before removing that annotation in
the environment overlay; do not silently drop the hardening setting.
