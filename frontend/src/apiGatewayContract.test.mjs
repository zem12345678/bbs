import assert from "node:assert/strict";
import fs from "node:fs";
import path from "node:path";
import test from "node:test";
import { fileURLToPath } from "node:url";

const repoRoot = path.resolve(
  path.dirname(fileURLToPath(import.meta.url)),
  "../.."
);

const gatewayRouteFiles = [
  {
    path: "backend/services/api-gateway/internal/interfaces/http/handler.go",
    prefix: ""
  },
  {
    path: "backend/services/api-gateway/internal/interfaces/http/handler_chat.go",
    prefix: "/chat"
  }
];

test("frontend API routes are registered in api-gateway", () => {
  const gatewayRoutes = extractGatewayRoutes();
  const frontendRequests = extractFrontendRequests();
  const missing = frontendRequests
    .filter(request => !gatewayRoutes.has(request.key))
    .map(
      request =>
        `${request.method.toUpperCase()} ${request.raw} (${request.file}:${
          request.line
        })`
    );

  assert.ok(
    frontendRequests.length >= 100,
    `expected to extract the frontend API surface, got ${frontendRequests.length}`
  );
  assert.deepEqual(missing, []);
});

test("production user frontend defaults to its same-origin API proxy", () => {
  const productionEnv = read("frontend/.env.production");
  const dockerfile = read("frontend/Dockerfile");

  assert.match(productionEnv, /^VITE_API_BASE_URL=\/api\/v1$/m);
  assert.match(dockerfile, /ARG VITE_API_BASE_URL=\/api\/v1/);
});

test("production WebSocket proxies preserve the chat heartbeat window", () => {
  const ingress = read(
    "deployments/kubernetes/overlays/production-example/ingress.yaml"
  );
  const readTimeoutSeconds = ingressTimeoutSeconds(ingress, "proxy-read-timeout");
  const sendTimeoutSeconds = ingressTimeoutSeconds(ingress, "proxy-send-timeout");
  const pingSeconds = gatewayPingSeconds(
    read("backend/services/api-gateway/internal/realtime/chat/connection.go")
  );

  assert.equal(readTimeoutSeconds, 3600);
  assert.equal(sendTimeoutSeconds, 3600);
  assert.ok(
    pingSeconds < Math.min(readTimeoutSeconds, sendTimeoutSeconds),
    "Gateway heartbeats must arrive before either production proxy timeout"
  );
  assert.match(
    ingress,
    /host:\s+bbs\.example\.com[\s\S]*?path:\s+\/api[\s\S]*?name:\s+bbs-api-gateway/,
    "the public API (including /api/v1/chat/ws) must reach the Gateway"
  );

  for (const file of ["frontend/nginx.conf", "vue-pure-admin/nginx.conf"]) {
    const location = chatWebSocketLocation(read(file));
    assert.match(location, /proxy_http_version 1\.1;/, `${file} must use HTTP/1.1`);
    assert.match(location, /proxy_set_header Upgrade \$http_upgrade;/, `${file} must forward Upgrade`);
    assert.match(location, /proxy_set_header Connection "upgrade";/, `${file} must forward Connection`);
    assert.match(location, /proxy_read_timeout 3600s;/, `${file} must retain a long read timeout`);
    assert.match(location, /proxy_send_timeout 3600s;/, `${file} must retain a long send timeout`);
  }
});

function extractGatewayRoutes() {
  const routes = new Set();
  for (const item of gatewayRouteFiles) {
    const source = read(item.path);
    const routePattern =
      /(?:api|chat)\.(GET|POST|PUT|PATCH|DELETE)\("([^"]+)"/g;
    for (const match of source.matchAll(routePattern)) {
      routes.add(
        routeKey(match[1].toLowerCase(), `${item.prefix}${match[2]}`)
      );
    }
  }
  return routes;
}

function ingressTimeoutSeconds(source, name) {
  const match = source.match(
    new RegExp(`nginx\\.ingress\\.kubernetes\\.io/${name}:\\s*"(\\d+)"`)
  );
  assert.ok(match, `production Ingress must configure ${name}`);
  return Number(match[1]);
}

function gatewayPingSeconds(source) {
  const match = source.match(/pingPeriod\s*=\s*(\d+)\s*\*\s*time\.Second/);
  assert.ok(match, "Gateway chat ping period must remain explicit");
  return Number(match[1]);
}

function chatWebSocketLocation(source) {
  const match = source.match(
    /location = \/api\/v1\/chat\/ws\s*\{([\s\S]*?)\n    \}/
  );
  assert.ok(match, "Nginx must define an exact chat WebSocket location");
  return match[1];
}

function extractFrontendRequests() {
  const file = "frontend/src/api.js";
  const source = read(file);
  const apiStart = source.indexOf("export const bbsApi");
  const requests = [
    ...extractTemplateRequests(file, source, apiStart),
    ...extractStringRequests(file, source, apiStart),
    ...extractConcatenatedStringRequests(file, source, apiStart),
    ...extractDownloadRequests(file, source, apiStart),
    ...specialApiHelpers(file, source)
  ];
  const seen = new Set();
  return requests.filter(request => {
    request.key = routeKey(request.method, `/api/v1${request.raw}`);
    if (seen.has(request.key)) return false;
    seen.add(request.key);
    return true;
  });
}

function extractTemplateRequests(file, source, apiStart) {
  return extractRequestCalls(
    file,
    source,
    apiStart,
    /\brequest\(\s*`([^`]*)`\s*(?:,\s*\{([\s\S]*?)\}\s*)?\)/g
  );
}

function extractStringRequests(file, source, apiStart) {
  return extractRequestCalls(
    file,
    source,
    apiStart,
    /\brequest\(\s*"([^"]+)"\s*(?:,\s*\{([\s\S]*?)\}\s*)?\)/g
  );
}

function extractRequestCalls(file, source, apiStart, pattern) {
  const requests = [];
  for (const match of source.matchAll(pattern)) {
    if (match.index < apiStart) continue;
    addRequest(requests, file, source, match, methodFromOptions(match[2]), match[1]);
  }
  return requests;
}

function extractConcatenatedStringRequests(file, source, apiStart) {
  const requests = [];
  const pattern = /\brequest\(\s*"([^"]+)"\s*\+/g;
  for (const match of source.matchAll(pattern)) {
    if (match.index < apiStart) continue;
    addRequest(requests, file, source, match, "get", `${match[1]}${"${param}"}`);
  }
  return requests;
}

function extractDownloadRequests(file, source, apiStart) {
  const requests = [];
  const pattern = /\bdownloadAttachment\(\s*`([^`]*)`/g;
  for (const match of source.matchAll(pattern)) {
    if (match.index < apiStart) continue;
    addRequest(requests, file, source, match, "get", match[1]);
  }
  return requests;
}

function specialApiHelpers(file, source) {
  return [
    {
      file,
      line: lineNumber(source, source.indexOf("oauthStartUrl(")),
      method: "get",
      raw: "/auth/oauth/:param/start"
    },
    {
      file,
      line: lineNumber(source, source.indexOf("chatWebSocketUrlForBase(")),
      method: "get",
      raw: "/chat/ws"
    }
  ];
}

function addRequest(requests, file, source, match, method, rawPath) {
  requests.push({
    file,
    line: lineNumber(source, match.index),
    method,
    raw: normalizeClientPath(rawPath)
  });
}

function methodFromOptions(options) {
  const match = /\bmethod\s*:\s*["`](GET|POST|PUT|PATCH|DELETE)["`]/i.exec(
    options || ""
  );
  return (match?.[1] || "GET").toLowerCase();
}

function routeKey(method, apiPath) {
  return `${method} ${routeShape(apiPath)}`;
}

function routeShape(apiPath) {
  return apiPath
    .replace(/^\/api\/v1/, "")
    .split("?")[0]
    .split("/")
    .filter(Boolean)
    .map(segment =>
      segment.startsWith(":") || segment === ":param" ? ":" : segment
    )
    .join("/");
}

function normalizeClientPath(rawPath) {
  return rawPath
    .replace(/\$\{buildQuery\([\s\S]*?\)\}/g, "")
    .replace(/\s+/g, "")
    .replace(/\$\{[^}]+}/g, ":param")
    .replace(/^\/+/, "/")
    .split("?")[0];
}

function lineNumber(source, index) {
  if (index < 0) return 1;
  return source.slice(0, index).split(/\r?\n/).length;
}

function read(relativePath) {
  return fs.readFileSync(path.join(repoRoot, relativePath), "utf8");
}
