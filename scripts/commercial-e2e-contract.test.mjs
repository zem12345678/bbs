import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import path from "node:path";
import { fileURLToPath } from "node:url";
import test from "node:test";

const scriptDir = path.dirname(fileURLToPath(import.meta.url));

async function readScript(name) {
  return readFile(path.join(scriptDir, name), "utf8");
}

function countMatches(text, pattern) {
  return [...text.matchAll(pattern)].length;
}

function assertNoAutoFrontendGuard(source, functionName, flagName, reachableCall) {
  const start = source.indexOf(`async function ${functionName}`);
  assert.notEqual(start, -1, `${functionName} should exist`);
  const chunk = source.slice(start, source.indexOf("\n}\n", start) + 3);
  const guard = chunk.indexOf(`truthyEnv("${flagName}")`);
  const reachable = chunk.indexOf(reachableCall);
  const spawn = chunk.indexOf("spawn(");
  assert.ok(guard > -1, `${functionName} should read ${flagName}`);
  assert.ok(reachable > guard, `${functionName} should assert the existing server when ${flagName} is set`);
  assert.ok(spawn > reachable, `${functionName} should only spawn Vite after the no-auto guard`);
}

function functionChunk(source, functionName) {
  const start = source.indexOf(`function ${functionName}`);
  assert.notEqual(start, -1, `${functionName} should exist`);
  return source.slice(start, source.indexOf("\n}\n", start) + 3);
}

test("commercial e2e can reuse already-running backend services", async () => {
  const source = await readScript("commercial-e2e.ps1");
  assert.match(source, /\[switch\]\$ReuseRunningBackend/);
  assert.equal(countMatches(source, /\$smokeArgs\.RefreshRunningServices/g), 1);
  assert.equal(countMatches(source, /\$smokeArgs\.ReuseRunningServicesOnly/g), 1);
  assert.match(
    source,
    /if\s*\(\s*-not\s+\$ReuseRunningBackend\s*\)\s*\{\s*\$smokeArgs\.RefreshRunningServices\s*=\s*\$true\s*\}/s
  );
  assert.match(source, /\$smokeArgs\.ReuseRunningServicesOnly\s*=\s*\$true/);
  assert.match(source, /KeepRunning\s*=\s*\$true/);
});

test("commercial e2e loads an isolated environment file before resolving api base", async () => {
  const source = await readScript("commercial-e2e.ps1");
  assert.match(source, /\[string\]\$EnvironmentFile/);
  assert.match(source, /Import-ProcessEnvironmentFile\s+-Path\s+\$EnvironmentFile\s+-Required/);
  assert.match(source, /BBS_GATEWAY_SERVICE_HTTP_PORT/);
  assert.ok(
    source.indexOf("Import-ProcessEnvironmentFile -Path $EnvironmentFile -Required") <
      source.indexOf('$ApiBase = "http://127.0.0.1:$GatewayPort/api/v1"'),
    "environment file should be loaded before API_BASE is derived"
  );
});

test("backend smoke reuse-only mode refuses to start missing services", async () => {
  const source = await readScript("../backend/scripts/smoke-local.ps1");
  assert.match(source, /\[switch\]\$ReuseRunningServicesOnly/);
  assert.match(source, /if\s*\(\s*\$ReuseRunningServicesOnly\s*\)\s*\{\s*throw\s+"ReuseRunningServicesOnly requires \$ServiceName to already listen on port \$Port\."/s);
  assert.match(source, /if\s*\(\s*\$ReuseRunningServicesOnly\s*\)\s*\{[\s\S]*Assert-PortReusableOrFree \$service\.Name \$service\.Port[\s\S]*api-gateway to already listen/s);
  assert.ok(
    source.indexOf("if ($ReuseRunningServicesOnly)") < source.indexOf("if (-not $SkipBuild)"),
    "reuse-only readiness check should run before build and migrations"
  );
});

test("visible backend launcher only restarts verified BBS listener processes", async () => {
  const source = await readScript("../backend/scripts/start-local-visible.ps1");
  const stopChunk = functionChunk(source, "Stop-ServiceProcess");
  const verifyChunk = functionChunk(source, "Stop-VerifiedProcess");

  assert.doesNotMatch(source, /\bStop-Process\b|taskkill/i);
  assert.match(stopChunk, /\$expectedExe\s*=\s*Join-Path\s+\(Join-Path\s+\$ServicesRoot\s+\$ServiceName\)\s+"bin\\\$ServiceName\.exe"/);
  assert.match(stopChunk, /\$listeningProcessIds\s*=\s*@\(Get-ListeningProcessIds\s+\$Port\)/);
  assert.match(stopChunk, /\$processes\s*=\s*@\(Get-ServiceProcess\s+\$ServiceName\)/);
  assert.match(stopChunk, /\$listeningProcessIds\s+-contains\s+\[int\]\$process\.ProcessId\s+-and\s+\(Stop-VerifiedProcess\s+-ProcessId\s+\$process\.ProcessId\s+-ExpectedPath\s+\$expectedExe\)/);
  assert.match(verifyChunk, /OrdinalIgnoreCase\.Equals\(\$actualPath,\s*\$expectedPath\)/);
  assert.ok(
    verifyChunk.indexOf("OrdinalIgnoreCase.Equals($actualPath, $expectedPath)") <
      verifyChunk.indexOf("$process.Kill()"),
    "path verification should happen before killing a process"
  );
});

test("chat cluster e2e is opt-in and only stops its own verified child processes", async () => {
  const source = await readScript("../backend/scripts/chat-cluster-e2e.ps1");
  const stopChunk = functionChunk(source, "Stop-OwnedService");

  assert.match(source, /\[switch\]\$Run/);
  assert.match(
    source,
    /if\s*\(\s*-not\s+\$Run\s+-and\s+-not\s+\$Preflight\s*\)\s*\{[\s\S]*?no processes or API writes were performed\.[\s\S]*?return/s,
    "the cluster test must require an explicit run or preflight mode"
  );
  assert.doesNotMatch(source, /\bStop-Process\b|taskkill/i);
  assert.match(stopChunk, /\$expectedPath\s*=\s*\[System\.IO\.Path\]::GetFullPath\(\[string\]\$OwnedService\.ExecutablePath\)/);
  assert.match(stopChunk, /Get-CimInstance Win32_Process -Filter "ProcessId=\$\(\$OwnedService\.Process\.Id\)"/);
  assert.match(stopChunk, /OrdinalIgnoreCase/);
  assert.match(stopChunk, /\$OwnedService\.Process\.Kill\(\)/);
  assert.ok(
    stopChunk.indexOf("OrdinalIgnoreCase") < stopChunk.indexOf("$OwnedService.Process.Kill()"),
    "the child executable path must be verified before cleanup"
  );
});

test("browser commerce E2E cleans up only the frontend and Chrome children it starts", async () => {
  for (const scriptName of ["frontend-mall-e2e.mjs", "admin-mall-e2e.mjs"]) {
    const source = await readScript(scriptName);

    assert.doesNotMatch(source, /\bStop-Process\b|taskkill/i);
    assert.match(source, /const server = spawn\(/);
    assert.match(source, /stop:\s*\(\)\s*=>\s*stopProcess\(server\)/);
    assert.match(source, /const chrome = spawn\(/);
    assert.match(source, /await stopProcess\(chrome\)/);
    assert.doesNotMatch(source, /chrome\.kill\(\)/);
  }
});

test("chat cluster e2e verifies cross-gateway message retry idempotency", async () => {
  const source = await readScript("../backend/scripts/chat-cluster-e2e.ps1");

  assert.match(
    source,
    /\$idempotentRetry\s*=\s*Invoke-BbsApi\s+-Uri\s+"\$gatewayBUrl\/api\/v1\/chat\/rooms\/\$roomNo\/messages"\s+-Method\s+Post\s+-Headers\s+\$sender\.Headers/s
  );
  assert.match(source, /client_message_id\s*=\s+\$clientMessageID/);
  assert.match(source, /\[int64\]\$idempotentRetry\.message\.seq\s+-ne\s+\$messageSequence/);
  assert.match(source, /\$persisted\.Count\s+-ne\s+1/);
});

test("chat cluster e2e verifies deletion propagation and durable tombstones", async () => {
  const source = await readScript("../backend/scripts/chat-cluster-e2e.ps1");

  assert.match(
    source,
    /\$deleted\s*=\s*Invoke-BbsApi\s+-Uri\s+"\$gatewayBUrl\/api\/v1\/chat\/rooms\/\$roomNo\/messages\/\$messageID"\s+-Method\s+Delete\s+-Headers\s+\$sender\.Headers/s
  );
  assert.match(source, /Receive-WebSocketEventUntil\s+-Socket\s+\$receiverSocket\s+-ExpectedType\s+"message\.deleted"/);
  assert.match(source, /\$deletedEvent\.payload\.payload\.message_id\s+-ne\s+\$messageID/);
  assert.match(source, /\$deletedPersisted\s*=\s+@\(\$deletedHistory\.messages\s+\|\s+Where-Object\s+\{\s*\[string\]\$_\.id\s+-eq\s+\$messageID\s*\}\)/);
  assert.match(source, /\[int\]\$deletedPersisted\[0\]\.status\s+-ne\s+2/);
});

test("chat cluster e2e verifies announcement delivery and seen state across gateways", async () => {
  const source = await readScript("../backend/scripts/chat-cluster-e2e.ps1");

  assert.match(
    source,
    /\$announcement\s*=\s*Invoke-BbsApi\s+-Uri\s+"\$gatewayAUrl\/api\/v1\/chat\/rooms\/\$roomNo\/announcement"\s+-Method\s+Patch\s+-Headers\s+\$sender\.Headers/s
  );
  assert.match(source, /Receive-WebSocketEventUntil\s+-Socket\s+\$receiverSocket\s+-ExpectedType\s+"announcement\.updated"/);
  assert.match(
    source,
    /\$announcementSeen\s*=\s*Invoke-BbsApi\s+-Uri\s+"\$gatewayBUrl\/api\/v1\/chat\/rooms\/\$roomNo\/announcement-seen"\s+-Method\s+Put\s+-Headers\s+\$receiver\.Headers/s
  );
  assert.match(source, /last_seen_announcement_version\s+-ne\s+\$announcementVersion/);
});

test("commercial e2e forwards manual frontend mode to both browser flows", async () => {
  const source = await readScript("commercial-e2e.ps1");
  assert.match(source, /if\s*\(\s*\$NoAutoFrontend\s*\)\s*\{\s*\$frontendEnv\.MALL_E2E_NO_AUTO_FRONTEND\s*=\s*"1"\s*\}/s);
  assert.match(source, /if\s*\(\s*\$NoAutoFrontend\s*\)\s*\{\s*\$adminEnv\.ADMIN_MALL_E2E_NO_AUTO_FRONTEND\s*=\s*"1"\s*\}/s);
});

test("browser e2e scripts honor no-auto frontend flags before spawning vite", async () => {
  assertNoAutoFrontendGuard(
    await readScript("frontend-mall-e2e.mjs"),
    "ensureFrontendServer",
    "MALL_E2E_NO_AUTO_FRONTEND",
    'assertHttpReachable(shopUrl, "frontend")'
  );
  assertNoAutoFrontendGuard(
    await readScript("admin-mall-e2e.mjs"),
    "ensureAdminServer",
    "ADMIN_MALL_E2E_NO_AUTO_FRONTEND",
    'assertHttpReachable(ADMIN_BASE, "admin frontend")'
  );
});
