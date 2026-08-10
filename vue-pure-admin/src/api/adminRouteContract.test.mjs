import assert from "node:assert/strict";
import fs from "node:fs";
import path from "node:path";
import test from "node:test";
import { fileURLToPath } from "node:url";

const repoRoot = path.resolve(
  path.dirname(fileURLToPath(import.meta.url)),
  "../../.."
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

const adminApiFiles = [
  "vue-pure-admin/src/api/admin.ts",
  "vue-pure-admin/src/api/user.ts",
  "vue-pure-admin/src/api/system.ts",
  "vue-pure-admin/src/api/routes.ts"
];

const systemMenuSeedFile =
  "backend/services/admin-service/internal/infrastructure/persistence/repository_system.go";
const dynamicRouteResolverFile = "vue-pure-admin/src/router/utils.ts";
const gatewayHandlerFile =
  "backend/services/api-gateway/internal/interfaces/http/handler.go";
const adminActionFiles = [
  "backend/services/admin-service/internal/domain/admin/types.go",
  "backend/services/admin-service/internal/domain/admin/system.go"
];

test("admin UI API routes are registered in api-gateway", () => {
  const gatewayRoutes = extractGatewayRoutes();
  const adminRequests = extractAdminRequests();
  const missing = adminRequests
    .filter(request => !gatewayRoutes.has(request.key))
    .map(
      request =>
        `${request.method.toUpperCase()} ${request.raw} (${request.file}:${
          request.line
        })`
    );

  assert.ok(
    adminRequests.length >= 90,
    `expected to extract the admin API surface, got ${adminRequests.length}`
  );
  assert.deepEqual(missing, []);
});

test("channel governance UI exposes list, feature, archive and restore controls", () => {
  const apiSource = read("vue-pure-admin/src/api/admin.ts");
  const routeSource = read("vue-pure-admin/src/router/modules/governance.ts");
  const viewSource = read(
    "vue-pure-admin/src/views/governance/channels/index.vue"
  );

  assert.match(apiSource, /listAdminChannels/);
  assert.match(apiSource, /\/api\/v1\/admin\/channels/);
  assert.match(apiSource, /setAdminChannelFeatured/);
  assert.match(apiSource, /\/featured`/);
  assert.match(apiSource, /setAdminChannelArchived/);
  assert.match(apiSource, /\/archived`/);
  assert.match(routeSource, /name:\s*"GovernanceChannels"/);
  assert.match(viewSource, /governance:list_channels/);
  assert.match(viewSource, /governance:feature_channel/);
  assert.match(viewSource, /governance:archive_channel/);
  assert.match(viewSource, /governance:restore_channel/);
  assert.match(viewSource, /archived_status/);
});

test("welcome active users chart is permission-gated and chronological", () => {
  const apiSource = read("vue-pure-admin/src/api/admin.ts");
  const viewSource = read("vue-pure-admin/src/views/welcome/index.vue");
  const chartSource = read(
    "vue-pure-admin/src/views/welcome/components/charts/ChartBar.vue"
  );

  assert.match(apiSource, /getAdminActiveUsersChart/);
  assert.match(apiSource, /\/api\/v1\/charts\/active-users/);
  assert.match(apiSource, /params:\s*\{\s*span:\s*"day",\s*limit:\s*7,\s*offset\s*\}/);
  assert.match(viewSource, /canViewActiveUsers\s*=\s*computed/);
  assert.match(viewSource, /\["\*",\s*"\*:\*",\s*"governance:\*",\s*"governance:list_users"\]/);
  assert.match(viewSource, /\.some\(permission\s*=>\s*hasPerms\(permission\)\s*\)/);
  assert.match(viewSource, /if\s*\(!canViewActiveUsers\.value\)\s*return/);
  assert.match(
    viewSource,
    /activeUsersChart\.value\?\.read[\s\S]*?\.reverse\(\)/
  );
  assert.match(
    viewSource,
    /activeUsersChart\.value\?\.write[\s\S]*?\.reverse\(\)/
  );
  assert.match(viewSource, /activeUsersError\s*=\s*ref\(""\)/);
  assert.match(viewSource, /Date\.UTC\([\s\S]*getUTCFullYear\(\)[\s\S]*getUTCDate\(\)/);
  assert.match(viewSource, /getAdminActiveUsersChart\(offset\)/);
  assert.match(viewSource, /activeUsersReadData\.value\.some\(count\s*=>\s*count\s*>\s*0\)/);
  assert.match(viewSource, /activeUsersWriteData\.value\.some\(count\s*=>\s*count\s*>\s*0\)/);
  assert.match(viewSource, /role="alert"/);
  assert.match(viewSource, /@click="loadActiveUsers"/);
  assert.match(viewSource, /watch\(\s*canViewActiveUsers,/);
  assert.match(viewSource, /\{\s*immediate:\s*true\s*\}/);
  assert.match(viewSource, /activeUsersRequestVersion/);
  assert.match(chartSource, /container:\s*chartRef/);
  assert.doesNotMatch(chartSource, /container:\s*["']\.bar-card["']/);
});

test("admin static menu does not import template demo modules", () => {
  const source = read("vue-pure-admin/src/router/index.ts");
  const staticModules = extractStaticRouterModules(source);
  assert.deepEqual(staticModules.sort(), [
    "./modules/error.ts",
    "./modules/home.ts"
  ]);
});

test("seeded admin menus resolve to available dynamic frontend views", () => {
  const components = extractSeededMenuComponents(read(systemMenuSeedFile));
  const dynamicRouteRoots = extractDynamicRouteRoots(
    read(dynamicRouteResolverFile)
  );

  assert.ok(
    components.length >= 30,
    `expected the commercial menu seed to expose at least 30 page components, got ${components.length}`
  );

  const unsupportedRoots = components.filter(
    component => !dynamicRouteRoots.has(component.split("/")[0])
  );
  assert.deepEqual(
    unsupportedRoots,
    [],
    "every seeded menu component must be covered by the dynamic route resolver"
  );

  const missingViews = components.filter(
    component =>
      ![".vue", ".tsx"].some(extension =>
        fs.existsSync(
          path.join(repoRoot, "vue-pure-admin/src/views", `${component}${extension}`)
        )
      )
  );
  assert.deepEqual(
    missingViews,
    [],
    "every seeded menu component must have a matching admin view"
  );
});

test("gateway admin permissions are covered by default menu and button seeds", () => {
  const actions = new Map(
    adminActionFiles.flatMap(file => extractAdminActions(read(file)))
  );
  const seededPermissions = extractSeededPermissions(
    read(systemMenuSeedFile),
    actions
  );
  const gatewayPermissions = extractGatewayAdminPermissions(
    read(gatewayHandlerFile)
  );

  assert.ok(
    gatewayPermissions.size >= 90,
    `expected the commercial gateway to require at least 90 admin permissions, got ${gatewayPermissions.size}`
  );
  assert.deepEqual(
    [...gatewayPermissions].sort(),
    [...seededPermissions].sort(),
    "each gateway admin permission must be available through the default menu seed"
  );
});

test("mall overview pages do not render zero metrics after initial load failure", () => {
  for (const file of [
    "vue-pure-admin/src/views/mall/overview/index.vue",
    "vue-pure-admin/src/views/mall/orders/index.vue"
  ]) {
    const source = read(file);
    assert.match(source, /overviewError\s*=\s*ref\(""\)/, `${file} tracks overview load errors`);
    assert.match(source, /showOverviewData\s*=\s*computed/, `${file} gates overview data rendering`);
    assert.match(source, /v-if="overviewError"/, `${file} surfaces the overview load error`);
    assert.match(source, /v-if="showOverviewData"/, `${file} hides zero-value metrics when no overview data exists`);
    assert.match(source, /catch\s*\(error\)/, `${file} handles thrown overview API failures`);
  }
});

test("mall refund details ignore superseded drawer responses", () => {
  const source = read("vue-pure-admin/src/views/mall/refunds/index.vue");

  assert.match(source, /let refundDetailRequestVersion = 0/);
  assert.match(source, /const requestVersion = \+\+refundDetailRequestVersion/);
  assert.match(
    source,
    /const isCurrentRequest = \(\) => requestVersion === refundDetailRequestVersion/
  );
  assert.match(source, /if \(!isCurrentRequest\(\)\) return/);
  assert.match(
    source,
    /if \(isCurrentRequest\(\)\) \{\s*detailLoading\.value = false/
  );
});

test("mall order management ignores superseded list, overview, and record responses", () => {
  const source = read("vue-pure-admin/src/views/mall/orders/index.vue");

  assert.match(source, /let orderListRequestVersion = 0/);
  assert.match(source, /let orderOverviewRequestVersion = 0/);
  assert.match(source, /let orderRecordsRequestVersion = 0/);
  assert.match(source, /const requestVersion = \+\+orderListRequestVersion/);
  assert.match(source, /const requestVersion = \+\+orderOverviewRequestVersion/);
  assert.match(source, /const requestVersion = \+\+orderRecordsRequestVersion/);
  assert.match(source, /requestVersion !== orderListRequestVersion/);
  assert.match(source, /requestVersion !== orderOverviewRequestVersion/);
  assert.match(source, /requestVersion !== orderRecordsRequestVersion/);
});

test("mall review management ignores superseded list responses", () => {
  const source = read("vue-pure-admin/src/views/mall/reviews/index.vue");

  assert.match(source, /let reviewListRequestVersion = 0/);
  assert.match(source, /const requestVersion = \+\+reviewListRequestVersion/);
  assert.match(source, /requestVersion !== reviewListRequestVersion/);
  assert.match(
    source,
    /if \(requestVersion === reviewListRequestVersion\) \{\s*loading\.value = false/
  );
});

test("governance user file capacity UI uses the admin capacity contract", () => {
  const apiSource = read("vue-pure-admin/src/api/admin.ts");
  const viewSource = read(
    "vue-pure-admin/src/views/governance/users/index.vue"
  );

  assert.match(apiSource, /getAdminUserFileCapacity/);
  assert.match(apiSource, /updateAdminUserFileCapacity/);
  assert.match(apiSource, /`\/api\/v1\/admin\/users\/\$\{id\}\/file-capacity`/);
  assert.match(viewSource, /governance:list_user_file_capacity/);
  assert.match(viewSource, /governance:update_user_file_capacity/);
  assert.match(viewSource, /\{ override_mb: overrideMb \}/);
  assert.match(viewSource, /saveFileCapacity\(null\)/);
  assert.match(viewSource, /capacityOverrideMb\.value == null/);
  assert.match(
    viewSource,
    /!fileCapacity \|\|\s*fileCapacity\.override_mb === null/
  );
  assert.match(viewSource, /:before-close="closeFileCapacityDialog"/);
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

function extractAdminRequests() {
  return adminApiFiles.flatMap(file => {
    const source = read(file);
    return [
      ...extractHttpRequests(file, source),
      ...extractGetWrapperRequests(file, source),
      ...extractListWrapperRequests(file, source)
    ];
  });
}

function extractHttpRequests(file, source) {
  const requests = [];
  const requestPattern =
    /http\.request(?:<[\s\S]*?>+)?\s*\(\s*["`](get|post|put|patch|delete)["`]\s*,\s*([`"])([\s\S]*?)\2/g;
  for (const match of source.matchAll(requestPattern)) {
    addRequest(requests, file, source, match, match[1], match[3]);
  }
  return requests;
}

function extractGetWrapperRequests(file, source) {
  const requests = [];
  const getPattern = /\bgetOne(?:<[\s\S]*?>+)?\s*\(\s*([`"])([\s\S]*?)\1/g;
  for (const match of source.matchAll(getPattern)) {
    addRequest(requests, file, source, match, "get", match[2]);
  }
  return requests;
}

function extractListWrapperRequests(file, source) {
  const requests = [];
  const listPattern = /\blist(?:<[\s\S]*?>+)?\s*\(\s*([`"])([\s\S]*?)\1/g;
  for (const match of source.matchAll(listPattern)) {
    addRequest(requests, file, source, match, "get", match[2]);
  }
  return requests;
}

function extractStaticRouterModules(source) {
  const match = source.match(
    /const modules:[\s\S]*?import\.meta\.glob\(\s*\[([\s\S]*?)\]/m
  );
  assert.ok(match, "router static module glob should be explicit");
  return [...match[1].matchAll(/["']([^"']+)["']/g)].map(item => item[1]);
}

function extractSeededMenuComponents(source) {
  return [
    ...new Set(
      [...source.matchAll(/Component:\s*"([^"]+)"/g)]
        .map(match => match[1])
        .filter(Boolean)
    )
  ].sort();
}

function extractDynamicRouteRoots(source) {
  return new Set(
    [...source.matchAll(/"\/src\/views\/([^/]+)\/\*\*\/\*\.\{vue,tsx\}"/g)].map(
      match => match[1]
    )
  );
}

function extractAdminActions(source) {
  return [...source.matchAll(/(Action\w+)\s+Action\s+=\s+"([^"]+)"/g)].map(
    match => [match[1], match[2]]
  );
}

function extractSeededPermissions(source, actions) {
  const matches = [
    ...source.matchAll(
      /(governancePermission|mallPermission|systemPermission)\(domain\.(Action\w+)\)/g
    )
  ];
  const unknownActions = matches
    .map(match => match[2])
    .filter(action => !actions.has(action));
  assert.deepEqual(unknownActions, [], "menu seed references unknown admin actions");
  return new Set(
    matches.map(match => {
      const resource = match[1].replace("Permission", "");
      return `${resource}:${actions.get(match[2])}`;
    })
  );
}

function extractGatewayAdminPermissions(source) {
  const staticPermissions = [
    ...source.matchAll(/requireAdminPermission\("([^"]+)"\)/g)
  ].map(match => match[1]);
  const dynamicPermissions = [
    ...source.matchAll(/\bpermission\s*:?=\s*"((?:governance|mall|system):[^"]+)"/g)
  ].map(match => match[1]);
  return new Set([...staticPermissions, ...dynamicPermissions]);
}

function addRequest(requests, file, source, match, method, rawPath) {
  const normalized = normalizeClientPath(rawPath);
  if (!normalized.startsWith("/api/v1/")) return;
  requests.push({
    file,
    line: lineNumber(source, match.index),
    method,
    raw: normalized,
    key: routeKey(method, normalized)
  });
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
  return rawPath.replace(/\s+/g, "").replace(/\$\{[^}]+}/g, ":param");
}

function lineNumber(source, index) {
  return source.slice(0, index).split(/\r?\n/).length;
}

function read(relativePath) {
  return fs.readFileSync(path.join(repoRoot, relativePath), "utf8");
}
