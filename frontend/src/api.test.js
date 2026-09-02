import assert from "node:assert/strict";
import { afterEach, beforeEach, test } from "node:test";

import { AUTH_INVALIDATED_EVENT, ApiError, bbsApi, chatWebSocketUrl, chatWebSocketUrlForBase, configuredAPIBase, isUnauthorizedError, parseRetryAfterSeconds } from "./api.js";

beforeEach(() => {
  globalThis.__BBS_API_ORIGIN__ = "http://127.0.0.1:18080";
});

afterEach(() => {
  delete globalThis.fetch;
  delete globalThis.__BBS_API_ORIGIN__;
});

test("reads a normalized API URL from runtime config with explicit dev overrides", () => {
  assert.equal(configuredAPIBase({ apiBaseUrl: "https://bbs.example.com/api/v1/" }, { VITE_API_BASE_URL: "/ignored" }), "https://bbs.example.com/api/v1");
  assert.equal(configuredAPIBase({ api_base_url: "/configured/api///" }, {}), "/configured/api");
  assert.equal(configuredAPIBase({ apiBaseUrl: "http://127.0.0.1:18080/api/v1" }, { DEV: true, VITE_API_BASE: "http://127.0.0.1:28080/api/v1" }), "http://127.0.0.1:28080/api/v1");
  assert.equal(configuredAPIBase({ apiBaseUrl: "   " }, { VITE_API_BASE_URL: "/build/api/" }), "/build/api");
  assert.equal(configuredAPIBase({}, { VITE_API_BASE_URL: "/build/api" }), "/build/api");
  assert.equal(configuredAPIBase({}, {}), "/api/v1");
});

test("unwraps successful API envelopes and preserves large integer ids", async () => {
  let requestedUrl = "";
  globalThis.fetch = async (url) => {
    requestedUrl = url;
    return textResponse(
      200,
      '{"service":"api-gateway","http_code":200,"code":0,"reason":"成功","message":"success","data":{"id":9223372036854775807,"name":"demo"}}'
    );
  };

  const data = await bbsApi.authConfig();

  assert.equal(requestedUrl, "http://127.0.0.1:18080/api/v1/auth/config");
  assert.deepEqual(data, { id: "9223372036854775807", name: "demo" });
});

test("loads public metadata without authentication", async () => {
  let captured;
  globalThis.fetch = async (url, options) => {
    captured = { url, options };
    return jsonResponse(200, {
      service: "api-gateway",
      http_code: 200,
      code: 0,
      message: "success",
      data: { ads: [], notesPerOneAd: 0 }
    });
  };

  const data = await bbsApi.meta(false);

  assert.equal(captured.url, "http://127.0.0.1:18080/api/v1/meta");
  assert.equal(captured.options.method, "POST");
  assert.equal(captured.options.headers.Authorization, undefined);
  assert.deepEqual(JSON.parse(captured.options.body), { detail: false });
  assert.deepEqual(data, { ads: [], notesPerOneAd: 0 });
});

test("loads public custom emojis without authentication", async () => {
  let captured;
  globalThis.fetch = async (url, options) => {
    captured = { url, options };
    return jsonResponse(200, {
      service: "api-gateway",
      http_code: 200,
      code: 0,
      message: "success",
      data: { emojis: [{ name: "party", url: "https://cdn.example.test/party.webp" }] }
    });
  };

  const data = await bbsApi.emojis();

  assert.equal(captured.url, "http://127.0.0.1:18080/api/v1/emojis");
  assert.equal(captured.options.method, "GET");
  assert.equal(captured.options.headers.Authorization, undefined);
  assert.equal(data.emojis[0].name, "party");
});

test("requests Misskey-compatible notification feeds with the bearer token", async () => {
  const requests = [];
  globalThis.fetch = async (url, options) => {
    requests.push({ url, options });
    if (url.endsWith("/notifications/flush")) return textResponse(204, "");
    return jsonResponse(200, []);
  };

  await bbsApi.misskeyNotifications({ limit: 10, sinceId: "9007199254740993", includeTypes: ["reaction"] }, "access-token");
  await bbsApi.misskeyGroupedNotifications({ markAsRead: false }, "access-token");
  await bbsApi.flushNotifications("access-token");

  assert.deepEqual(
    requests.map(({ url, options }) => [url, options.method, options.headers.Authorization, options.body === undefined ? undefined : JSON.parse(options.body)]),
    [
      ["http://127.0.0.1:18080/api/v1/i/notifications", "POST", "Bearer access-token", { limit: 10, sinceId: "9007199254740993", includeTypes: ["reaction"] }],
      ["http://127.0.0.1:18080/api/v1/i/notifications-grouped", "POST", "Bearer access-token", { markAsRead: false }],
      ["http://127.0.0.1:18080/api/v1/notifications/flush", "POST", "Bearer access-token", undefined]
    ]
  );
});

test("maps registry requests without normalizing arbitrary JSON values", async () => {
  const requests = [];
  globalThis.fetch = async (url, options) => {
    requests.push({ url, options });
    if (url.endsWith("/i/registry/get")) {
      return jsonResponse(200, { http_code: 418, code: 0, data: { nested: true } });
    }
    if (url.endsWith("/i/registry/get-unsecure")) {
      return textResponse(200, '[9223372036854775807,{"nested":-9223372036854775808}]');
    }
    return jsonResponse(200, null);
  };

  const address = { key: "editor", scope: ["client", "preferences"], domain: null };
  const value = { layout: ["wide", { columns: 3 }], externalId: "9223372036854775807" };

  await bbsApi.registrySet({ ...address, value }, "access-token");
  const received = await bbsApi.registryGet(address, "access-token");
  await bbsApi.registryGetAll({ scope: [], domain: "" }, "access-token");
  await bbsApi.registryGetDetail(address, "access-token");
  const unsecure = await bbsApi.registryGetUnsecure({ key: "reactions", scope: [] }, "access-token");
  await bbsApi.registryKeys({ scope: [], domain: null }, "access-token");
  await bbsApi.registryKeysWithType({ scope: [], domain: "client-token" }, "access-token");
  await bbsApi.registryRemove(address, "access-token");
  await bbsApi.registryScopesWithDomain("access-token");

  assert.deepEqual(received, { http_code: 418, code: 0, data: { nested: true } });
  assert.deepEqual(unsecure, ["9223372036854775807", { nested: "-9223372036854775808" }]);
  assert.deepEqual(
    requests.map(({ url }) => new URL(url).pathname),
    [
      "/api/v1/i/registry/set",
      "/api/v1/i/registry/get",
      "/api/v1/i/registry/get-all",
      "/api/v1/i/registry/get-detail",
      "/api/v1/i/registry/get-unsecure",
      "/api/v1/i/registry/keys",
      "/api/v1/i/registry/keys-with-type",
      "/api/v1/i/registry/remove",
      "/api/v1/i/registry/scopes-with-domain"
    ]
  );
  assert.equal(requests.every(({ options }) => options.method === "POST"), true);
  assert.equal(requests.every(({ options }) => options.headers.Authorization === "Bearer access-token"), true);
  assert.deepEqual(JSON.parse(requests[0].options.body), { ...address, value });
  assert.deepEqual(JSON.parse(requests[2].options.body), { scope: [], domain: "" });
  assert.deepEqual(JSON.parse(requests[4].options.body), { key: "reactions", scope: [] });
  assert.deepEqual(JSON.parse(requests[8].options.body), {});
});

test("requests favorite export with the interactive bearer token", async () => {
  let captured;
  globalThis.fetch = async (url, options) => {
    captured = { url, options };
    return new Response(null, { status: 204 });
  };

  await bbsApi.exportFavorites("access-token");

  assert.equal(captured.url, "http://127.0.0.1:18080/api/v1/i/export-favorites");
  assert.equal(captured.options.method, "POST");
  assert.equal(captured.options.headers.Authorization, "Bearer access-token");
  assert.deepEqual(JSON.parse(captured.options.body), {});
});

test("requests note export with the interactive bearer token", async () => {
  let captured;
  globalThis.fetch = async (url, options) => {
    captured = { url, options };
    return new Response(null, { status: 204 });
  };

  await bbsApi.exportNotes("access-token");

  assert.equal(captured.url, "http://127.0.0.1:18080/api/v1/i/export-notes");
  assert.equal(captured.options.method, "POST");
  assert.equal(captured.options.headers.Authorization, "Bearer access-token");
  assert.deepEqual(JSON.parse(captured.options.body), {});
});

test("maps personal content pin mutations and profile queries with string-safe ids", async () => {
  const requests = [];
  globalThis.fetch = async (url, options) => {
    requests.push({ url, options });
    return jsonResponse(200, {
      service: "api-gateway",
      http_code: 200,
      code: 0,
      message: "success",
      data: { items: [] }
    });
  };

  await bbsApi.pinNote("9223372036854775807", "access-token");
  await bbsApi.unpinNote("9223372036854775807", "access-token");
  await bbsApi.currentPinnedContent("access-token");
  await bbsApi.userPinnedContent("9223372036854775807", "access-token");

  assert.deepEqual(
    requests.map((request) => [new URL(request.url).pathname, request.options.method]),
    [
      ["/api/v1/i/pin", "POST"],
      ["/api/v1/i/unpin", "POST"],
      ["/api/v1/users/me/pinned", "GET"],
      ["/api/v1/users/9223372036854775807/pinned", "GET"]
    ]
  );
  assert.equal(requests[0].options.headers.Authorization, "Bearer access-token");
  assert.deepEqual(JSON.parse(requests[0].options.body), { noteId: "9223372036854775807" });
  assert.deepEqual(JSON.parse(requests[1].options.body), { noteId: "9223372036854775807" });
});

test("loads and updates a private user memo with string-safe ids", async () => {
  const requests = [];
  globalThis.fetch = async (url, options) => {
    requests.push({ url, options });
    if (options.method === "GET") {
      return jsonResponse(200, {
        service: "api-gateway",
        http_code: 200,
        code: 0,
        message: "success",
        data: { memo: "project lead" }
      });
    }
    return new Response(null, { status: 204 });
  };

  const loaded = await bbsApi.getUserMemo("9223372036854775807", "access-token");
  await bbsApi.updateUserMemo("9223372036854775807", "project lead", "access-token");
  await bbsApi.updateUserMemo("9223372036854775807", null, "access-token");

  assert.equal(loaded.memo, "project lead");
  assert.deepEqual(
    requests.map((request) => [new URL(request.url).pathname, request.options.method]),
    [
      ["/api/v1/users/9223372036854775807/memo", "GET"],
      ["/api/v1/users/update-memo", "POST"],
      ["/api/v1/users/update-memo", "POST"]
    ]
  );
  assert.deepEqual(JSON.parse(requests[1].options.body), { userId: "9223372036854775807", memo: "project lead" });
  assert.deepEqual(JSON.parse(requests[2].options.body), { userId: "9223372036854775807", memo: null });
  assert.equal(requests[1].options.headers.Authorization, "Bearer access-token");
});

test("requests note import with the selected source and interactive bearer token", async () => {
  let captured;
  globalThis.fetch = async (url, options) => {
    captured = { url, options };
    return jsonResponse(200, { imported: 2, drafts: 1, skipped: 0 });
  };

  await bbsApi.importNotes("9223372036854773000", "Mastodon", "access-token");

  assert.equal(captured.url, "http://127.0.0.1:18080/api/v1/i/import-notes");
  assert.equal(captured.options.method, "POST");
  assert.equal(captured.options.headers.Authorization, "Bearer access-token");
  assert.deepEqual(JSON.parse(captured.options.body), { fileId: "9223372036854773000", type: "Mastodon" });
});

test("requests account data export with the interactive bearer token", async () => {
  let captured;
  globalThis.fetch = async (url, options) => {
    captured = { url, options };
    return new Response(null, { status: 204 });
  };

  await bbsApi.exportData("access-token");

  assert.equal(captured.url, "http://127.0.0.1:18080/api/v1/i/export-data");
  assert.equal(captured.options.method, "POST");
  assert.equal(captured.options.headers.Authorization, "Bearer access-token");
  assert.deepEqual(JSON.parse(captured.options.body), {});
});

test("keeps int64 mall ids quoted in mutation payloads", async () => {
  const requests = [];
  globalThis.fetch = async (url, options) => {
    requests.push({ url, options });
    return jsonResponse(200, {
      service: "api-gateway",
      http_code: 200,
      code: 0,
      message: "success",
      data: {}
    });
  };

  await bbsApi.createMallOrder(
    { items: [{ product_id: "339000000000000011", quantity: 1 }] },
    "access-token"
  );
  await bbsApi.createMallProductReview(
    "339000000000000021",
    { order_id: "339000000000000012", rating: 5, content: "体验很好" },
    "access-token"
  );

  assert.equal(JSON.parse(requests[0].options.body).items[0].product_id, "339000000000000011");
  assert.equal(requests[1].url, "http://127.0.0.1:18080/api/v1/mall/products/339000000000000021/reviews");
  assert.equal(JSON.parse(requests[1].options.body).order_id, "339000000000000012");
});

test("calls the follow approval workflow endpoints with authentication", async () => {
  const requests = [];
  globalThis.fetch = async (url, options = {}) => {
    requests.push({ url, options });
    return jsonResponse(200, {
      service: "api-gateway",
      http_code: 200,
      code: 0,
      message: "success",
      data: { items: [], total: 0 }
    });
  };

  await bbsApi.receivedFollowRequests({ page: 2, page_size: 10 }, "access-token");
  await bbsApi.sentFollowRequests({}, "access-token");
  await bbsApi.acceptFollowRequest("9223372036854775807", "access-token");
  await bbsApi.rejectFollowRequest("9223372036854775806", "access-token");
  await bbsApi.cancelFollowRequest("9223372036854775805", "access-token");
  await bbsApi.setFollowApprovalRequired(true, "access-token");

  assert.equal(requests[0].url, "http://127.0.0.1:18080/api/v1/users/me/follow-requests?page=2&page_size=10");
  assert.equal(requests[1].url, "http://127.0.0.1:18080/api/v1/users/me/follow-requests/sent?page=1&page_size=20");
  assert.equal(requests[2].url, "http://127.0.0.1:18080/api/v1/users/me/follow-requests/9223372036854775807/accept");
  assert.equal(requests[2].options.method, "POST");
  assert.equal(requests[3].url, "http://127.0.0.1:18080/api/v1/users/me/follow-requests/9223372036854775806/reject");
  assert.equal(requests[4].url, "http://127.0.0.1:18080/api/v1/users/9223372036854775805/follow/cancel");
  assert.equal(requests[5].url, "http://127.0.0.1:18080/api/v1/users/me/settings/follow-approval");
  assert.equal(requests[5].options.method, "PUT");
  assert.deepEqual(JSON.parse(requests[5].options.body), { required: true });
  requests.forEach(({ options }) => assert.equal(options.headers.Authorization, "Bearer access-token"));
});

test("submits topic poll ballots with authentication", async () => {
  let captured;
  globalThis.fetch = async (url, options) => {
    captured = { url, options };
    return jsonResponse(200, {
      service: "api-gateway",
      http_code: 200,
      code: 0,
      message: "success",
      data: { poll: { has_voted: true } }
    });
  };

  const data = await bbsApi.voteTopicPoll("9223372036854775807", { choices: [0, 2] }, "access-token");

  assert.equal(captured.url, "http://127.0.0.1:18080/api/v1/topics/9223372036854775807/poll/votes");
  assert.equal(captured.options.method, "POST");
  assert.equal(captured.options.headers.Authorization, "Bearer access-token");
  assert.deepEqual(JSON.parse(captured.options.body), { choices: [0, 2] });
  assert.equal(data.poll.has_voted, true);
});

test("maps authenticated user-list management and timeline requests", async () => {
  const requests = [];
  globalThis.fetch = async (url, options = {}) => {
    requests.push({ url, options });
    return jsonResponse(200, {
      service: "api-gateway",
      http_code: 200,
      code: 0,
      message: "success",
      data: { items: [] }
    });
  };

  await bbsApi.createUserList({ name: "Editors", is_public: true }, "access-token");
  await bbsApi.addUserListMember("9223372036854775000", "9223372036854774000", "access-token");
  await bbsApi.userListFeed("9223372036854775000", { limit: 10, offset: 20 }, "access-token");
  await bbsApi.exportUserLists("access-token");
  await bbsApi.importUserLists("9223372036854773000", "access-token");

  assert.equal(requests[0].url, "http://127.0.0.1:18080/api/v1/users/me/lists");
  assert.equal(requests[0].options.method, "POST");
  assert.equal(requests[0].options.headers.Authorization, "Bearer access-token");
  assert.deepEqual(JSON.parse(requests[0].options.body), { name: "Editors", is_public: true });
  assert.equal(requests[1].url, "http://127.0.0.1:18080/api/v1/user-lists/9223372036854775000/members");
  assert.deepEqual(JSON.parse(requests[1].options.body), { user_id: "9223372036854774000" });
  assert.equal(requests[2].url, "http://127.0.0.1:18080/api/v1/user-lists/9223372036854775000/feed?limit=10&offset=20");
  assert.equal(requests[3].url, "http://127.0.0.1:18080/api/v1/i/export-user-lists");
  assert.equal(requests[3].options.method, "POST");
  assert.equal(requests[3].options.headers.Authorization, "Bearer access-token");
  assert.deepEqual(JSON.parse(requests[3].options.body), {});
  assert.equal(requests[4].url, "http://127.0.0.1:18080/api/v1/i/import-user-lists");
  assert.equal(requests[4].options.method, "POST");
  assert.equal(requests[4].options.headers.Authorization, "Bearer access-token");
  assert.deepEqual(JSON.parse(requests[4].options.body), { fileId: "9223372036854773000" });
});

test("maps MFA login and account-security requests", async () => {
  const requests = [];
  globalThis.fetch = async (url, options = {}) => {
    requests.push({ url, options });
    return jsonResponse(200, {
      service: "api-gateway",
      http_code: 200,
      code: 0,
      message: "success",
      data: {}
    });
  };

  await bbsApi.completeMfaLogin({ challenge: "challenge-token", code: "123456" });
  await bbsApi.mfaStatus("access-token");
  await bbsApi.beginTotpEnrollment({ password: "secret", current_code: "old-code" }, "access-token");
  await bbsApi.confirmTotpEnrollment({ code: "654321" }, "access-token");
  await bbsApi.regenerateMfaRecoveryCodes({ password: "secret", code: "backup-code" }, "access-token");
  await bbsApi.disableTotp({ password: "secret", code: "backup-code" }, "access-token");

  assert.equal(requests[0].url, "http://127.0.0.1:18080/api/v1/auth/login/mfa");
  assert.equal(requests[0].options.method, "POST");
  assert.equal(requests[0].options.headers.Authorization, undefined);
  assert.deepEqual(JSON.parse(requests[0].options.body), { challenge: "challenge-token", code: "123456" });
  assert.equal(requests[1].url, "http://127.0.0.1:18080/api/v1/users/me/mfa");
  assert.equal(requests[1].options.headers.Authorization, "Bearer access-token");
  assert.equal(requests[2].url, "http://127.0.0.1:18080/api/v1/users/me/mfa/totp/enrollment");
  assert.deepEqual(JSON.parse(requests[2].options.body), { password: "secret", current_code: "old-code" });
  assert.equal(requests[3].url, "http://127.0.0.1:18080/api/v1/users/me/mfa/totp/confirm");
  assert.equal(requests[4].url, "http://127.0.0.1:18080/api/v1/users/me/mfa/recovery-codes");
  assert.equal(requests[5].url, "http://127.0.0.1:18080/api/v1/users/me/mfa/totp");
  assert.equal(requests[5].options.method, "DELETE");
  assert.equal(requests.slice(1).every(({ options }) => options.headers.Authorization === "Bearer access-token"), true);
});

test("maps account lifecycle and sensitive account requests", async () => {
  const requests = [];
  globalThis.fetch = async (url, options = {}) => {
    requests.push({ url, options });
    if (requests.length > 1) return textResponse(204, "");
    return jsonResponse(200, { service: "api-gateway", http_code: 200, code: 0, message: "success", data: { state: "active" } });
  };

  await bbsApi.accountLifecycle("access-token");
  await bbsApi.changePassword({ old_password: "old-secret", new_password: "new-secret", mfa_code: "  123456  " }, "access-token");
  await bbsApi.requestAccountDeletion({ password: "secret", code: "123456" }, "access-token");

  assert.equal(requests[0].url, "http://127.0.0.1:18080/api/v1/users/me/account-lifecycle");
  assert.equal(requests[0].options.method, "GET");
  assert.equal(requests[1].url, "http://127.0.0.1:18080/api/v1/i/change-password");
  assert.equal(requests[1].options.method, "POST");
  assert.deepEqual(JSON.parse(requests[1].options.body), { currentPassword: "old-secret", newPassword: "new-secret", token: "123456" });
  assert.equal(requests[2].url, "http://127.0.0.1:18080/api/v1/i/delete-account");
  assert.equal(requests[2].options.method, "POST");
  assert.deepEqual(JSON.parse(requests[2].options.body), { password: "secret", token: "123456" });
  assert.equal(requests.every(({ options }) => options.headers.Authorization === "Bearer access-token"), true);
});

test("maps notification preference read and update requests", async () => {
  const requests = [];
  globalThis.fetch = async (url, options = {}) => {
    requests.push({ url, options });
    return jsonResponse(200, {
      service: "api-gateway",
      http_code: 200,
      code: 0,
      message: "success",
      data: { items: [{ type: "comment", enabled: false }] }
    });
  };

  await bbsApi.notificationPreferences("access-token");
  await bbsApi.updateNotificationPreferences([{ type: "comment", enabled: false }], "access-token");

  assert.equal(requests[0].url, "http://127.0.0.1:18080/api/v1/users/me/notification-preferences");
  assert.equal(requests[0].options.method, "GET");
  assert.equal(requests[1].url, requests[0].url);
  assert.equal(requests[1].options.method, "PUT");
  assert.deepEqual(JSON.parse(requests[1].options.body), { items: [{ type: "comment", enabled: false }] });
  assert.equal(requests.every(({ options }) => options.headers.Authorization === "Bearer access-token"), true);
});

test("maps browser push config and subscription requests", async () => {
  const requests = [];
  globalThis.fetch = async (url, options = {}) => {
    requests.push({ url, options });
    return jsonResponse(200, {
      service: "api-gateway",
      http_code: 200,
      code: 0,
      message: "success",
      data: requests.length === 1 ? { enabled: true, public_key: "vapid-key" } : { registered: true }
    });
  };

  const subscription = { endpoint: "https://push.example.test/1", auth: "auth-key", publickey: "public-key", sendReadMessage: false };
  await bbsApi.webPushConfig();
  await bbsApi.registerWebPush(subscription, "access-token");
  await bbsApi.webPushRegistration(subscription.endpoint, "access-token");
  await bbsApi.unregisterWebPush(subscription.endpoint, "access-token");

  assert.equal(requests[0].url, "http://127.0.0.1:18080/api/v1/sw/config");
  assert.equal(requests[0].options.method, "GET");
  assert.equal(requests[0].options.headers.Authorization, undefined);
  assert.equal(requests[1].url, "http://127.0.0.1:18080/api/v1/sw/register");
  assert.deepEqual(JSON.parse(requests[1].options.body), subscription);
  assert.equal(requests[2].url, "http://127.0.0.1:18080/api/v1/sw/show-registration");
  assert.deepEqual(JSON.parse(requests[2].options.body), { endpoint: subscription.endpoint });
  assert.equal(requests[3].url, "http://127.0.0.1:18080/api/v1/sw/unregister");
  assert.deepEqual(JSON.parse(requests[3].options.body), { endpoint: subscription.endpoint });
  assert.equal(requests.slice(1).every(({ options }) => options.method === "POST" && options.headers.Authorization === "Bearer access-token"), true);
});

test("maps passkey registration, MFA, passwordless, and management requests", async () => {
  const requests = [];
  globalThis.fetch = async (url, options = {}) => {
    requests.push({ url, options });
    return jsonResponse(200, { service: "api-gateway", code: 0, message: "ok", data: {} });
  };

  await bbsApi.beginPasswordlessPasskeyLogin();
  await bbsApi.completePasswordlessPasskeyLogin({ challenge: "public-challenge", credential: { id: "cred" } });
  await bbsApi.beginPasskeyMfaLogin({ mfa_challenge: "mfa-challenge" });
  await bbsApi.completePasskeyMfaLogin({ challenge: "passkey-challenge", credential: { id: "cred" } });
  await bbsApi.passkeys("access-token");
  await bbsApi.beginPasskeyRegistration({ name: "Laptop", password: "secret", code: "123456" }, "access-token");
  await bbsApi.finishPasskeyRegistration({ challenge: "registration-challenge", credential: { id: "cred" } }, "access-token");
  await bbsApi.updatePasskey("credential/id", { name: "Phone" }, "access-token");
  await bbsApi.deletePasskey("credential/id", { password: "secret", code: "123456" }, "access-token");
  await bbsApi.setPasskeyPasswordless({ enabled: true, password: "secret", code: "123456" }, "access-token");

  assert.equal(requests[0].url, "http://127.0.0.1:18080/api/v1/auth/passkeys/options");
  assert.equal(requests[0].options.method, "POST");
  assert.equal(requests[1].url, "http://127.0.0.1:18080/api/v1/auth/passkeys/login");
  assert.deepEqual(JSON.parse(requests[1].options.body), { challenge: "public-challenge", credential: { id: "cred" } });
  assert.equal(requests[2].url, "http://127.0.0.1:18080/api/v1/auth/login/mfa/passkey/options");
  assert.equal(requests[3].url, "http://127.0.0.1:18080/api/v1/auth/login/mfa/passkey");
  assert.equal(requests[4].url, "http://127.0.0.1:18080/api/v1/users/me/passkeys");
  assert.equal(requests[5].url, "http://127.0.0.1:18080/api/v1/users/me/passkeys/registration/options");
  assert.equal(requests[6].url, "http://127.0.0.1:18080/api/v1/users/me/passkeys/registration/verify");
  assert.equal(requests[7].url, "http://127.0.0.1:18080/api/v1/users/me/passkeys/credential%2Fid");
  assert.equal(requests[7].options.method, "PUT");
  assert.equal(requests[8].options.method, "DELETE");
  assert.equal(requests[9].url, "http://127.0.0.1:18080/api/v1/users/me/passkeys/passwordless");
  assert.equal(requests.slice(4).every(({ options }) => options.headers.Authorization === "Bearer access-token"), true);
});

test("maps API token listing, creation, and revocation requests", async () => {
  const requests = [];
  globalThis.fetch = async (url, options = {}) => {
    requests.push({ url, options });
    return jsonResponse(200, { service: "api-gateway", code: 0, message: "ok", data: requests.length === 1 ? { items: [], total: 0 } : requests.length === 2 ? { token: "secret", id: "tok-1", api_token: { id: "tok-1" } } : { api_token: { id: "tok-1", revoked_at: 1 } } });
  };

  await bbsApi.listAPITokens("access-token");
  await bbsApi.createAPIToken({ name: "Deploy", scopes: ["read", "write"], expires_in_days: 90 }, "access-token");
  await bbsApi.revokeAPIToken("tok/id", "access-token");

  assert.equal(requests[0].url, "http://127.0.0.1:18080/api/v1/users/me/api-tokens");
  assert.equal(requests[0].options.method, "GET");
  assert.equal(requests[1].url, "http://127.0.0.1:18080/api/v1/users/me/api-tokens");
  assert.equal(requests[1].options.method, "POST");
  assert.deepEqual(JSON.parse(requests[1].options.body), { name: "Deploy", scopes: ["read", "write"], expires_in_days: 90 });
  assert.equal(requests[2].url, "http://127.0.0.1:18080/api/v1/users/me/api-tokens/tok%2Fid");
  assert.equal(requests[2].options.method, "DELETE");
  assert.equal(requests.slice(0, 3).every(({ options }) => options.headers.Authorization === "Bearer access-token"), true);
});

test("maps personal webhook management and test requests", async () => {
  const requests = [];
  globalThis.fetch = async (url, options = {}) => {
    requests.push({ url, options });
    return jsonResponse(200, { service: "api-gateway", code: 0, message: "ok", data: {} });
  };

  const createPayload = { name: "发布通知", url: "https://hooks.example.test/bbs", secret: "hook-secret", on: ["note", "reply"] };
  const updatePayload = { name: "互动通知", url: "https://hooks.example.test/events", on: ["reaction"] };
  await bbsApi.listWebhooks("access-token");
  await bbsApi.createWebhook(createPayload, "access-token");
  await bbsApi.showWebhook("hook/id", "access-token");
  await bbsApi.updateWebhook("hook/id", updatePayload, "access-token");
  await bbsApi.testWebhook("hook/id", "reaction", "access-token");
  await bbsApi.deleteWebhook("hook/id", "access-token");

  assert.equal(requests[0].url, "http://127.0.0.1:18080/api/v1/users/me/webhooks");
  assert.equal(requests[0].options.method, "GET");
  assert.equal(requests[1].options.method, "POST");
  assert.deepEqual(JSON.parse(requests[1].options.body), createPayload);
  assert.equal(requests[2].url, "http://127.0.0.1:18080/api/v1/users/me/webhooks/hook%2Fid");
  assert.equal(requests[2].options.method, "GET");
  assert.equal(requests[3].options.method, "PUT");
  assert.deepEqual(JSON.parse(requests[3].options.body), updatePayload);
  assert.equal(requests[4].url, "http://127.0.0.1:18080/api/v1/users/me/webhooks/hook%2Fid/test");
  assert.equal(requests[4].options.method, "POST");
  assert.deepEqual(JSON.parse(requests[4].options.body), { type: "reaction" });
  assert.equal(requests[5].options.method, "DELETE");
  assert.equal(requests.every(({ options }) => options.headers.Authorization === "Bearer access-token"), true);
});

test("maps antenna management and filtered timeline requests", async () => {
  const requests = [];
  globalThis.fetch = async (url, options = {}) => {
    requests.push({ url, options });
    return jsonResponse(200, { service: "api-gateway", code: 0, message: "ok", data: {} });
  };

  const payload = { name: "后端", src: "all", keywords: [["go", "redis"]], excludeKeywords: [], users: [], caseSensitive: false, withReplies: false, withFile: false };
  await bbsApi.listAntennas("access-token");
  await bbsApi.createAntenna(payload, "access-token");
  await bbsApi.updateAntenna("antenna/id", { name: "服务端" }, "access-token");
  await bbsApi.antennaNotes("antenna/id", { limit: 30, offset: 10 }, "access-token");
  await bbsApi.deleteAntenna("antenna/id", "access-token");
  await bbsApi.exportAntennas("access-token");
  await bbsApi.importAntennas("9001", "access-token");

  assert.equal(requests[0].url, "http://127.0.0.1:18080/api/v1/users/me/antennas");
  assert.equal(requests[0].options.method, "GET");
  assert.equal(requests[1].options.method, "POST");
  assert.deepEqual(JSON.parse(requests[1].options.body), payload);
  assert.equal(requests[2].url, "http://127.0.0.1:18080/api/v1/users/me/antennas/antenna%2Fid");
  assert.equal(requests[2].options.method, "PUT");
  assert.equal(new URL(requests[3].url).pathname, "/api/v1/users/me/antennas/antenna%2Fid/notes");
  assert.equal(new URL(requests[3].url).searchParams.get("limit"), "30");
  assert.equal(new URL(requests[3].url).searchParams.get("offset"), "10");
  assert.equal(requests[4].options.method, "DELETE");
  assert.equal(new URL(requests[5].url).pathname, "/api/v1/i/export-antennas");
  assert.equal(requests[5].options.method, "POST");
  assert.deepEqual(JSON.parse(requests[5].options.body), {});
  assert.equal(new URL(requests[6].url).pathname, "/api/v1/i/import-antennas");
  assert.equal(requests[6].options.method, "POST");
  assert.deepEqual(JSON.parse(requests[6].options.body), { fileId: "9001" });
  assert.equal(requests.every(({ options }) => options.headers.Authorization === "Bearer access-token"), true);
});

test("loads public site config without authentication", async () => {
  let requestedUrl = "";
  globalThis.fetch = async (url) => {
    requestedUrl = url;
    return jsonResponse(200, {
      service: "api-gateway",
      http_code: 200,
      code: 0,
      message: "success",
      data: { site_name: "示例社区" }
    });
  };

  assert.deepEqual(await bbsApi.siteConfig(), { site_name: "示例社区" });
  assert.equal(requestedUrl, "http://127.0.0.1:18080/api/v1/site-config");
});

test("loads public announcements and individual announcement details", async () => {
  const requests = [];
  globalThis.fetch = async (url, options) => {
    requests.push({ url, options });
    return jsonResponse(200, {
      service: "api-gateway",
      http_code: 200,
      code: 0,
      message: "success",
      data: requests.length === 1 ? { items: [], total: 0 } : { announcement: { id: "launch" } }
    });
  };

  await bbsApi.announcements({ limit: 20 });
  await bbsApi.announcement("launch");
  await bbsApi.readAnnouncement("launch");

  const listURL = new URL(requests[0].url);
  assert.equal(listURL.pathname, "/api/v1/announcements");
  assert.equal(listURL.searchParams.get("limit"), "20");
  assert.equal(new URL(requests[1].url).pathname, "/api/v1/announcements/launch");
	assert.equal(new URL(requests[2].url).pathname, "/api/v1/i/read-announcement");
  assert.equal(requests[2].options.method, "POST");
  assert.equal(requests[2].options.body, JSON.stringify({ announcementId: "launch" }));
  assert.equal(requests[0].options.headers.Authorization, undefined);
  assert.equal(requests[2].options.headers.Authorization, undefined);
});

test("passes the access token for personalized announcements and read acknowledgements", async () => {
  const requests = [];
  globalThis.fetch = async (url, options) => {
    requests.push({ url, options });
    return jsonResponse(200, {
      service: "api-gateway",
      http_code: 200,
      code: 0,
      message: "success",
      data: { items: [] }
    });
  };

  await bbsApi.announcements({ limit: 20 }, "access-token");
  await bbsApi.readAnnouncement("launch", "access-token");

  assert.equal(requests[0].options.headers.Authorization, "Bearer access-token");
  assert.equal(requests[1].options.headers.Authorization, "Bearer access-token");
});

test("passes search keywords and pagination through query params", async () => {
  const requests = [];
  globalThis.fetch = async (url, options) => {
    requests.push({ url, options });
    return jsonResponse(200, {
      service: "api-gateway",
      http_code: 200,
      code: 0,
      message: "success",
      data: { items: [], total: 0 }
    });
  };

  await bbsApi.searchTopics("paymnt", { page: 2, page_size: 7 });
  await bbsApi.searchArticles("codx", { page: 3, page_size: 9 });
  await bbsApi.searchUsers("ali", { page: 4, page_size: 5 });

  const topicURL = new URL(requests[0].url);
  assert.equal(topicURL.pathname, "/api/v1/search/topics");
  assert.equal(topicURL.searchParams.get("q"), "paymnt");
  assert.equal(topicURL.searchParams.get("page"), "2");
  assert.equal(topicURL.searchParams.get("page_size"), "7");
  assert.equal(requests[0].options.headers.Authorization, undefined);

  const articleURL = new URL(requests[1].url);
  assert.equal(articleURL.pathname, "/api/v1/search/articles");
  assert.equal(articleURL.searchParams.get("q"), "codx");
  assert.equal(articleURL.searchParams.get("page"), "3");
  assert.equal(articleURL.searchParams.get("page_size"), "9");
  assert.equal(requests[1].options.headers.Authorization, undefined);

  const userURL = new URL(requests[2].url);
  assert.equal(userURL.pathname, "/api/v1/search/users");
  assert.equal(userURL.searchParams.get("q"), "ali");
  assert.equal(userURL.searchParams.get("page"), "4");
  assert.equal(userURL.searchParams.get("page_size"), "5");
  assert.equal(requests[2].options.headers.Authorization, undefined);
});

test("searches public hashtags without authentication", async () => {
  let requestedUrl = "";
  globalThis.fetch = async (url) => {
    requestedUrl = url;
    return jsonResponse(200, {
      service: "api-gateway",
      http_code: 200,
      code: 0,
      message: "success",
      data: { items: [], total: 0 }
    });
  };

  await bbsApi.searchHashtags("go", { limit: 6, offset: 12 });
  const url = new URL(requestedUrl);
  assert.equal(url.pathname, "/api/v1/hashtags/search");
  assert.equal(url.searchParams.get("q"), "go");
  assert.equal(url.searchParams.get("limit"), "6");
  assert.equal(url.searchParams.get("offset"), "12");
});

test("searches mixed public content by tag with a lossless cursor", async () => {
  let captured;
  globalThis.fetch = async (url, options) => {
    captured = { url, options };
    return textResponse(200, '[{"kind":"topic","topic":{"id":9223372036854775807}}]');
  };

  const items = await bbsApi.searchNotesByTag({
    query: [["go", "backend"], ["community"]],
    limit: 20,
    untilId: "9223372036854775807"
  });

  assert.equal(new URL(captured.url).pathname, "/api/v1/notes/search-by-tag");
  assert.equal(captured.options.method, "POST");
  assert.equal(captured.options.headers.Authorization, undefined);
  assert.deepEqual(JSON.parse(captured.options.body), {
    query: [["go", "backend"], ["community"]],
    limit: 20,
    untilId: "9223372036854775807"
  });
  assert.equal(items[0].topic.id, "9223372036854775807");
});

test("searches public notes with optional bearer authentication", async () => {
  let captured;
  globalThis.fetch = async (url, options) => {
    captured = { url, options };
    return textResponse(200, "[]");
  };

  const items = await bbsApi.searchNotes({ query: "database", sinceId: "10", limit: 5 }, "read-token");
  assert.deepEqual(items, []);
  assert.equal(new URL(captured.url).pathname, "/api/v1/notes/search");
  assert.equal(captured.options.method, "POST");
  assert.equal(captured.options.headers.Authorization, "Bearer read-token");
  assert.deepEqual(JSON.parse(captured.options.body), { query: "database", sinceId: "10", limit: 5 });
});

test("loads trending hashtags without authentication", async () => {
  let requestedUrl = "";
  globalThis.fetch = async (url) => {
    requestedUrl = url;
    return jsonResponse(200, {
      service: "api-gateway",
      http_code: 200,
      code: 0,
      message: "success",
      data: { items: [{ tag: "go", count: 4 }] }
    });
  };

  await bbsApi.trendingHashtags({ limit: 6, offset: 2 });
  const url = new URL(requestedUrl);
  assert.equal(url.pathname, "/api/v1/hashtags/trend");
  assert.equal(url.searchParams.get("limit"), "6");
  assert.equal(url.searchParams.get("offset"), "2");
});

test("exposes public hashtag detail and author APIs", async () => {
  const requests = [];
  globalThis.fetch = async (url, options) => {
    requests.push({ url, options });
    return jsonResponse(200, {
      service: "api-gateway",
      http_code: 200,
      code: 0,
      message: "success",
      data: { items: [] }
    });
  };

  await bbsApi.hashtags({ limit: 5, offset: 1 });
  await bbsApi.hashtag("#go");
  await bbsApi.hashtagUsers("#go", { limit: 6, offset: 2, sort: "-follower" });

  const listURL = new URL(requests[0].url);
  assert.equal(listURL.pathname, "/api/v1/hashtags/list");
  assert.equal(listURL.searchParams.get("limit"), "5");
  assert.equal(listURL.searchParams.get("offset"), "1");
  const detailURL = new URL(requests[1].url);
  assert.equal(detailURL.pathname, "/api/v1/hashtags/show");
  assert.equal(detailURL.searchParams.get("tag"), "#go");
  const usersURL = new URL(requests[2].url);
  assert.equal(usersURL.pathname, "/api/v1/hashtags/users");
  assert.equal(usersURL.searchParams.get("tag"), "#go");
  assert.equal(usersURL.searchParams.get("sort"), "-follower");
  assert.equal(requests.every(({ options }) => options.headers.Authorization === undefined), true);
});

test("logs out with a bearer-authenticated POST request", async () => {
  let requestedUrl = "";
  let requestOptions;
  globalThis.fetch = async (url, options) => {
    requestedUrl = url;
    requestOptions = options;
    return jsonResponse(200, {
      service: "api-gateway",
      http_code: 200,
      code: 0,
      message: "success",
      data: null
    });
  };

  await bbsApi.logout("access-token");

  assert.equal(requestedUrl, "http://127.0.0.1:18080/api/v1/auth/logout");
  assert.equal(requestOptions.method, "POST");
  assert.equal(requestOptions.headers.Authorization, "Bearer access-token");
});

test("hides an article with authorization", async () => {
  let requestedUrl = "";
  let requestOptions;
  globalThis.fetch = async (url, options) => {
    requestedUrl = url;
    requestOptions = options;
    return jsonResponse(200, {
      service: "api-gateway",
      http_code: 200,
      code: 0,
      message: "success",
      data: {}
    });
  };

  await bbsApi.hideArticle("123", "access-token");

  assert.equal(requestedUrl, "http://127.0.0.1:18080/api/v1/articles/123/hide");
  assert.equal(requestOptions.method, "POST");
  assert.equal(requestOptions.headers.Authorization, "Bearer access-token");
});

test("passes digital entitlement grant filters through query params", async () => {
  let requestedUrl = "";
  let authorization = "";
  globalThis.fetch = async (url, options) => {
    requestedUrl = url;
    authorization = options.headers.Authorization;
    return jsonResponse(200, {
      service: "api-gateway",
      http_code: 200,
      code: 0,
      message: "success",
      data: { items: [], total: 0 }
    });
  };

  await bbsApi.mallDigitalEntitlements(
    { limit: 50, offset: 0, status: "ACTIVE", grant_type: "theme", grant_key: "theme-pro" },
    "access-token"
  );

  const url = new URL(requestedUrl);
  assert.equal(url.pathname, "/api/v1/mall/digital-entitlements");
  assert.equal(url.searchParams.get("limit"), "50");
  assert.equal(url.searchParams.get("offset"), "0");
  assert.equal(url.searchParams.get("status"), "ACTIVE");
  assert.equal(url.searchParams.get("grant_type"), "theme");
  assert.equal(url.searchParams.get("grant_key"), "theme-pro");
  assert.equal(authorization, "Bearer access-token");
});

test("loads public users in one deduplicated batch request", async () => {
  let requestedUrl = "";
  globalThis.fetch = async (url) => {
    requestedUrl = url;
    return jsonResponse(200, {
      service: "api-gateway",
      http_code: 200,
      code: 0,
      message: "success",
      data: { items: [{ id: 42 }], total: 1 }
    });
  };

  await bbsApi.getUsers([42, "7", 42]);

  const url = new URL(requestedUrl);
  assert.equal(url.pathname, "/api/v1/users/batch");
  assert.equal(url.searchParams.get("ids"), "42,7");
});

test("calls Misskey-compatible user directory and current-user endpoints", async () => {
  const requests = [];
  globalThis.fetch = async (url, options) => {
    requests.push({ url, options });
    if (new URL(url).pathname.endsWith("/i")) return textResponse(200, '{"id":"42","username":"alice"}');
    return textResponse(200, '[{"id":"42","username":"alice"}]');
  };

  const current = await bbsApi.currentUserCompat("access-token");
  const users = await bbsApi.usersCompat({ limit: 20, offset: 10, sort: "-createdAt" });
  const shown = await bbsApi.showUserCompat({ userIds: ["9223372036854775807"] });

  assert.equal(new URL(requests[0].url).pathname, "/api/v1/i");
  assert.equal(requests[0].options.method, "POST");
  assert.equal(requests[0].options.headers.Authorization, "Bearer access-token");
  assert.equal(current.id, "42");

  assert.equal(new URL(requests[1].url).pathname, "/api/v1/users");
  assert.deepEqual(JSON.parse(requests[1].options.body), { limit: 20, offset: 10, sort: "-createdAt" });
  assert.equal(requests[1].options.headers.Authorization, undefined);
  assert.equal(users[0].id, "42");

  assert.equal(new URL(requests[2].url).pathname, "/api/v1/users/show");
  assert.equal(shown[0].id, "42");
});

test("calls Misskey-compatible note create, show, timeline, user notes, and delete endpoints", async () => {
  const requests = [];
  globalThis.fetch = async (url, options) => {
    requests.push({ url, options });
    return textResponse(200, options.body === "{}" ? "[]" : "{}", options.method === "POST" ? { "content-type": "application/json" } : undefined);
  };

  await bbsApi.createNote({ text: "hello" }, "access-token");
  await bbsApi.showNote("9007199254740993", "access-token");
  await bbsApi.noteTimeline({ limit: 10, untilId: "9007199254740994" }, "access-token");
  await bbsApi.userNotes("9007199254740995", { withFiles: true });
  await bbsApi.deleteNote("9007199254740996", "access-token");

  assert.deepEqual(
    requests.map(({ url, options }) => [new URL(url).pathname, options.method, options.headers.Authorization, JSON.parse(options.body)]),
    [
      ["/api/v1/notes/create", "POST", "Bearer access-token", { text: "hello" }],
      ["/api/v1/notes/show", "POST", "Bearer access-token", { noteId: "9007199254740993" }],
      ["/api/v1/notes/timeline", "POST", "Bearer access-token", { limit: 10, untilId: "9007199254740994" }],
      ["/api/v1/users/notes", "POST", undefined, { userId: "9007199254740995", withFiles: true }],
      ["/api/v1/notes/delete", "POST", "Bearer access-token", { noteId: "9007199254740996" }]
    ]
  );
});

test("calls Misskey-compatible note feed and reply endpoints", async () => {
  const requests = [];
  globalThis.fetch = async (url, options = {}) => {
    requests.push({ url, options });
    return textResponse(200, "[]");
  };

  await bbsApi.globalNoteTimeline({ limit: 10 });
  await bbsApi.localNoteTimeline({ limit: 9 });
  await bbsApi.followingNoteTimeline({ limit: 8 }, "access-token");
  await bbsApi.featuredNotes({ limit: 7 });
  await bbsApi.userFeaturedNotes("9223372036854775807", { limit: 6 });
  await bbsApi.noteChildren("9223372036854775806", { limit: 5 });
  await bbsApi.noteReplies("9223372036854775805", { limit: 4 });

  assert.deepEqual(requests.map(({ url }) => new URL(url).pathname), [
    "/api/v1/notes/global-timeline",
    "/api/v1/notes/local-timeline",
    "/api/v1/notes/following",
    "/api/v1/notes/featured",
    "/api/v1/users/featured-notes",
    "/api/v1/notes/children",
    "/api/v1/notes/replies"
  ]);
  assert.deepEqual(JSON.parse(requests[4].options.body), { userId: "9223372036854775807", limit: 6 });
  assert.deepEqual(JSON.parse(requests[6].options.body), { noteId: "9223372036854775805", limit: 4 });
  assert.equal(requests[2].options.headers.Authorization, "Bearer access-token");
});

test("calls Misskey-compatible reaction and abuse-report endpoints", async () => {
  const requests = [];
  globalThis.fetch = async (url, options) => {
    requests.push({ url, options });
    return textResponse(204, "");
  };

  await bbsApi.createNoteReaction("9007199254740993", "👍", "access-token");
  await bbsApi.deleteNoteReaction("9007199254740994", "access-token");
  await bbsApi.userReactions("9007199254740995", { sinceId: "9007199254740996" });
  await bbsApi.reportUserAbuse("9007199254740997", "spam", "access-token");

  assert.deepEqual(
    requests.map(({ url, options }) => [new URL(url).pathname, options.method, options.headers.Authorization, JSON.parse(options.body)]),
    [
      ["/api/v1/notes/reactions/create", "POST", "Bearer access-token", { noteId: "9007199254740993", reaction: "👍" }],
      ["/api/v1/notes/reactions/delete", "POST", "Bearer access-token", { noteId: "9007199254740994" }],
      ["/api/v1/users/reactions", "POST", undefined, { userId: "9007199254740995", sinceId: "9007199254740996" }],
      ["/api/v1/users/report-abuse", "POST", "Bearer access-token", { userId: "9007199254740997", comment: "spam" }]
    ]
  );
});

test("calls Misskey-compatible note reaction, favorite, like, and state endpoints", async () => {
  const requests = [];
  globalThis.fetch = async (url, options = {}) => {
    requests.push({ url, options });
    return textResponse(204, "");
  };

  await bbsApi.noteReactions("9223372036854775807", { type: "🔥", limit: 5 });
  await bbsApi.likeNote("9223372036854775806", "access-token");
  await bbsApi.favoriteNote("9223372036854775805", "access-token");
  await bbsApi.unfavoriteNote("9223372036854775804", "access-token");
  await bbsApi.noteState("9223372036854775803", "access-token");

  assert.deepEqual(requests.map(({ url, options }) => [new URL(url).pathname, options.method]), [
    ["/api/v1/notes/reactions", "POST"],
    ["/api/v1/notes/like", "POST"],
    ["/api/v1/notes/favorites/create", "POST"],
    ["/api/v1/notes/favorites/delete", "POST"],
    ["/api/v1/notes/state", "POST"]
  ]);
  assert.deepEqual(JSON.parse(requests[0].options.body), { noteId: "9223372036854775807", type: "🔥", limit: 5 });
  assert.equal(requests[0].options.headers.Authorization, undefined);
  assert.deepEqual(JSON.parse(requests[4].options.body), { noteId: "9223372036854775803" });
  assert.equal(requests.slice(1).every(({ options }) => options.headers.Authorization === "Bearer access-token"), true);
});
test("maps following preference compatibility requests without breaking legacy follow calls", async () => {
  const requests = [];
  globalThis.fetch = async (url, options) => {
    requests.push({ url, options });
    return jsonResponse(200, {
      service: "api-gateway",
      http_code: 200,
      code: 0,
      message: "success",
      data: { items: [] }
    });
  };

  await bbsApi.followUser("9007199254740993", "legacy-token");
  await bbsApi.followUser("9007199254740994", { withReplies: true }, "access-token");
  await bbsApi.followUser("9007199254740995", false, "access-token");
  await bbsApi.updateFollowing("9007199254740994", { notify: "normal" }, "access-token");
  await bbsApi.listFollowingEdges("9007199254740993", {
    sinceId: "9007199254741000",
    untilId: "9007199254742000",
    limit: 30
  }, "access-token");

  assert.deepEqual(
    requests.map(({ url, options }) => [new URL(url).pathname, options.method, options.headers.Authorization]),
    [
      ["/api/v1/users/9007199254740993/follow", "POST", "Bearer legacy-token"],
      ["/api/v1/users/9007199254740994/follow", "POST", "Bearer access-token"],
      ["/api/v1/users/9007199254740995/follow", "POST", "Bearer access-token"],
      ["/api/v1/following/update", "POST", "Bearer access-token"],
      ["/api/v1/users/following", "POST", "Bearer access-token"]
    ]
  );
  assert.equal(requests[0].options.body, undefined);
  assert.deepEqual(JSON.parse(requests[1].options.body), { withReplies: true });
  assert.deepEqual(JSON.parse(requests[2].options.body), { withReplies: false });
  assert.deepEqual(JSON.parse(requests[3].options.body), { userId: "9007199254740994", notify: "normal" });
  assert.deepEqual(JSON.parse(requests[4].options.body), {
    userId: "9007199254740993",
    sinceId: "9007199254741000",
    untilId: "9007199254742000",
    limit: 30
  });
});

test("loads a public user by username without authentication", async () => {
  let requestedUrl = "";
  let requestOptions;
  globalThis.fetch = async (url, options) => {
    requestedUrl = url;
    requestOptions = options;
    return jsonResponse(200, {
      service: "api-gateway",
      http_code: 200,
      code: 0,
      message: "success",
      data: { user: { id: 42, username: "alice" } }
    });
  };

  const data = await bbsApi.getUserByUsername("alice");

  assert.deepEqual(data, { user: { id: 42, username: "alice" } });
  assert.equal(requestedUrl, "http://127.0.0.1:18080/api/v1/users/by-username/alice");
  assert.equal(requestOptions?.headers?.Authorization, undefined);
});

test("manages authenticated user safety relationships", async () => {
  const requests = [];
  globalThis.fetch = async (url, options) => {
    requests.push({ url, options });
    return jsonResponse(200, {
      service: "api-gateway",
      http_code: 200,
      code: 0,
      message: "success",
      data: { items: [], total: 0 }
    });
  };

  await bbsApi.userSafetyState("42", "access-token");
  await bbsApi.blockUser("42", "access-token");
  await bbsApi.unblockUser("42", "access-token");
  await bbsApi.muteUser("42", "access-token");
  await bbsApi.unmuteUser("42", "access-token");
  await bbsApi.blockedUsers({ page: 2, page_size: 8 }, "access-token");
  await bbsApi.mutedUsers({ page: 3, page_size: 6 }, "access-token");
  await bbsApi.exportFollowing({ excludeMuting: true, excludeInactive: true }, "access-token");
  await bbsApi.exportBlocking("access-token");
  await bbsApi.exportMute("access-token");
  await bbsApi.importBlocking("9001", "access-token");
  await bbsApi.importMuting("9002", "access-token");
  await bbsApi.importFollowing("9003", true, "access-token");

  assert.deepEqual(
    requests.map(({ url, options }) => [new URL(url).pathname, options.method || "GET"]),
    [
      ["/api/v1/users/42/safety-state", "GET"],
      ["/api/v1/users/42/block", "POST"],
      ["/api/v1/users/42/block", "DELETE"],
      ["/api/v1/users/42/mute", "POST"],
      ["/api/v1/users/42/mute", "DELETE"],
      ["/api/v1/users/me/blocked", "GET"],
      ["/api/v1/users/me/muted", "GET"],
      ["/api/v1/i/export-following", "POST"],
      ["/api/v1/i/export-blocking", "POST"],
      ["/api/v1/i/export-mute", "POST"],
      ["/api/v1/i/import-blocking", "POST"],
      ["/api/v1/i/import-muting", "POST"],
      ["/api/v1/i/import-following", "POST"]
    ]
  );
  assert.equal(new URL(requests[5].url).searchParams.get("page"), "2");
  assert.equal(new URL(requests[6].url).searchParams.get("page_size"), "6");
  assert.deepEqual(JSON.parse(requests[7].options.body), { excludeMuting: true, excludeInactive: true });
  assert.deepEqual(JSON.parse(requests[8].options.body), {});
  assert.deepEqual(JSON.parse(requests[9].options.body), {});
  assert.deepEqual(JSON.parse(requests[10].options.body), { fileId: "9001" });
  assert.deepEqual(JSON.parse(requests[11].options.body), { fileId: "9002" });
  assert.deepEqual(JSON.parse(requests[12].options.body), { fileId: "9003", withReplies: true });
  assert.equal(requests.every(({ options }) => options.headers.Authorization === "Bearer access-token"), true);
});

test("removes a follower from the authenticated user's audience", async () => {
  let requestRecord;
  globalThis.fetch = async (url, options) => {
    requestRecord = { url, options };
    return jsonResponse(200, {
      service: "api-gateway",
      http_code: 200,
      code: 0,
      message: "success",
      data: { user: { id: "42", username: "alice" } }
    });
  };

  await bbsApi.removeFollower("9223372036854775000", "access-token");

  assert.equal(new URL(requestRecord.url).pathname, "/api/v1/users/me/followers/9223372036854775000");
  assert.equal(requestRecord.options.method, "DELETE");
  assert.equal(requestRecord.options.headers.Authorization, "Bearer access-token");
});

test("manages named collections with string-safe ids", async () => {
  const requests = [];
  globalThis.fetch = async (url, options) => {
    requests.push({ url, options });
    return jsonResponse(200, {
      service: "api-gateway",
      http_code: 200,
      code: 0,
      message: "success",
      data: { items: [], total: 0, changed: true }
    });
  };

  const collectionId = "9007199254740993";
  const entityId = "9007199254740999";
  await bbsApi.collections({ limit: 100, offset: 20 }, "access-token");
  await bbsApi.createCollection({ name: "阅读", description: "", is_public: false }, "access-token");
  await bbsApi.updateCollection(collectionId, { name: "稍后阅读", description: "", is_public: true }, "access-token");
  await bbsApi.collectionItems(collectionId, { entity_type: "article", limit: 20 }, "access-token");
  await bbsApi.addCollectionItem(collectionId, { entity_type: "article", entity_id: entityId }, "access-token");
  await bbsApi.removeCollectionItem(collectionId, { entity_type: "article", entity_id: entityId }, "access-token");
  await bbsApi.deleteCollection(collectionId, "access-token");

  assert.deepEqual(
    requests.map(({ url, options }) => [new URL(url).pathname, options.method || "GET"]),
    [
      ["/api/v1/users/me/collections", "GET"],
      ["/api/v1/users/me/collections", "POST"],
      [`/api/v1/users/me/collections/${collectionId}`, "PUT"],
      [`/api/v1/users/me/collections/${collectionId}/items`, "GET"],
      [`/api/v1/users/me/collections/${collectionId}/items`, "POST"],
      [`/api/v1/users/me/collections/${collectionId}/items`, "DELETE"],
      [`/api/v1/users/me/collections/${collectionId}`, "DELETE"]
    ]
  );
  assert.equal(new URL(requests[0].url).searchParams.get("offset"), "20");
  assert.equal(new URL(requests[3].url).searchParams.get("entity_type"), "article");
  assert.equal(JSON.parse(requests[4].options.body).entity_id, entityId);
  assert.equal(JSON.parse(requests[5].options.body).entity_id, entityId);
  assert.equal(requests.every(({ options }) => options.headers.Authorization === "Bearer access-token"), true);
});

test("uses Misskey-compatible clip endpoints with string-safe ids", async () => {
  const requests = [];
  globalThis.fetch = async (url, options) => { requests.push({ url, options }); return jsonResponse(204, null); };
  await bbsApi.clips("access-token");
  await bbsApi.exportClips("access-token");
  await bbsApi.createClip({ name: "阅读", description: null, isPublic: false }, "access-token");
  await bbsApi.updateClip({ clipId: "9007199254740993", description: null, isPublic: true }, "access-token");
  await bbsApi.showClip("9007199254740993", "access-token");
  await bbsApi.addClipNote("9007199254740993", "9007199254740999", "access-token");
  await bbsApi.removeClipNote("9007199254740993", "9007199254740999", "access-token");
  await bbsApi.clipNotes("9007199254740993", { limit: 10, sinceId: "7", untilId: "3" }, "access-token");
  await bbsApi.favoriteClip("9007199254740993", "access-token");
  await bbsApi.unfavoriteClip("9007199254740993", "access-token");
  await bbsApi.myFavoriteClips("access-token");
  await bbsApi.userClips("9007199254740993", { limit: 10, sinceId: "7", untilId: "3" }, "access-token");
  await bbsApi.noteClips("9007199254740999", "access-token");
  assert.deepEqual(requests.map(({ url, options }) => [new URL(url).pathname, options.method]), [
    ["/api/v1/clips/list", "POST"], ["/api/v1/i/export-clips", "POST"], ["/api/v1/clips/create", "POST"], ["/api/v1/clips/update", "POST"], ["/api/v1/clips/show", "POST"],
    ["/api/v1/clips/add-note", "POST"], ["/api/v1/clips/remove-note", "POST"], ["/api/v1/clips/notes", "POST"], ["/api/v1/clips/favorite", "POST"], ["/api/v1/clips/unfavorite", "POST"], ["/api/v1/clips/my-favorites", "POST"], ["/api/v1/users/clips", "POST"], ["/api/v1/notes/clips", "POST"]
  ]);
  assert.deepEqual(JSON.parse(requests[1].options.body), {});
  assert.equal(JSON.parse(requests[2].options.body).description, null);
  assert.equal(JSON.parse(requests[3].options.body).description, null);
  assert.equal(JSON.parse(requests[7].options.body).sinceId, "7");
  assert.equal(JSON.parse(requests[7].options.body).untilId, "3");
  assert.equal(JSON.parse(requests[11].options.body).userId, "9007199254740993");
  assert.equal(JSON.parse(requests[11].options.body).untilId, "3");
  assert.equal(JSON.parse(requests[12].options.body).noteId, "9007199254740999");
  assert.equal(requests.every(({ options }) => options.headers.Authorization === "Bearer access-token"), true);
});

test("preserves a zero chat repair cursor and sends bearer auth", async () => {
  let requestedUrl = "";
  let authorization = "";
  globalThis.fetch = async (url, options) => {
    requestedUrl = url;
    authorization = options.headers.Authorization;
    return jsonResponse(200, {
      service: "api-gateway",
      http_code: 200,
      code: 0,
      message: "success",
      data: { messages: [], latest_seq: 0, has_newer: false }
    });
  };

  await bbsApi.chatMessages("AB12CD3E", { after_seq: 0, limit: 100 }, "access-token");

  const url = new URL(requestedUrl);
  assert.equal(url.pathname, "/api/v1/chat/rooms/AB12CD3E/messages");
  assert.equal(url.searchParams.get("after_seq"), "0");
  assert.equal(url.searchParams.get("limit"), "100");
  assert.equal(authorization, "Bearer access-token");
});

test("soft-deletes a chat message with authorization", async () => {
  let requestedUrl = "";
  let requestOptions;
  globalThis.fetch = async (url, options) => {
    requestedUrl = url;
    requestOptions = options;
    return jsonResponse(200, {
      service: "api-gateway",
      http_code: 200,
      code: 0,
      message: "success",
      data: { id: "9223372036854775807", status: 2, body: "" }
    });
  };

  const message = await bbsApi.deleteChatMessage("AB12CD3E", "9223372036854775807", "access-token");

  assert.deepEqual(message, { id: "9223372036854775807", status: 2, body: "" });
  assert.equal(requestedUrl, "http://127.0.0.1:18080/api/v1/chat/rooms/AB12CD3E/messages/9223372036854775807");
  assert.equal(requestOptions.method, "DELETE");
  assert.equal(requestOptions.headers.Authorization, "Bearer access-token");
});

test("leaves a chat room with authorization", async () => {
  let requestedUrl = "";
  let requestOptions;
  globalThis.fetch = async (url, options) => {
    requestedUrl = url;
    requestOptions = options;
    return jsonResponse(200, {
      service: "api-gateway",
      http_code: 200,
      code: 0,
      message: "success",
      data: { membership: { room_id: "9", status: 2 } }
    });
  };

  const data = await bbsApi.leaveChatRoom("AB12CD3E", "access-token");

  assert.deepEqual(data, { membership: { room_id: "9", status: 2 } });
  assert.equal(requestedUrl, "http://127.0.0.1:18080/api/v1/chat/rooms/AB12CD3E/membership");
  assert.equal(requestOptions.method, "DELETE");
  assert.equal(requestOptions.headers.Authorization, "Bearer access-token");
});

test("maps chat room member governance requests", async () => {
  const calls = [];
  globalThis.fetch = async (url, options = {}) => {
    calls.push({ url, options });
    return jsonResponse(200, {
      service: "api-gateway",
      http_code: 200,
      code: 0,
      message: "success",
      data: { items: [], total: 0 }
    });
  };

  await bbsApi.chatRoomMembers("AB12CD3E", { limit: 50, offset: 10, role: "manager", user_id: "9223372036854775807" }, "access-token");
  await bbsApi.updateChatRoomMemberRole("AB12CD3E", "9223372036854775807", "manager", "access-token");
  await bbsApi.muteChatRoomMember("AB12CD3E", "9223372036854775807", 1800000000000, "access-token");
  await bbsApi.muteChatRoomMember("AB12CD3E", "8", null, "access-token");
  await bbsApi.unmuteChatRoomMember("AB12CD3E", "9223372036854775807", "access-token");

  const listUrl = new URL(calls[0].url);
  assert.equal(listUrl.pathname, "/api/v1/chat/rooms/AB12CD3E/members");
  assert.deepEqual(Object.fromEntries(listUrl.searchParams), {
    limit: "50", offset: "10", role: "manager", user_id: "9223372036854775807"
  });
  assert.equal(calls[1].options.method, "PUT");
  assert.deepEqual(JSON.parse(calls[1].options.body), { role: "manager" });
  assert.equal(calls[2].options.method, "PUT");
  assert.deepEqual(JSON.parse(calls[2].options.body), { expires_at: 1800000000000 });
  assert.deepEqual(JSON.parse(calls[3].options.body), { expires_at: null });
  assert.equal(calls[4].options.method, "DELETE");
  assert.equal(calls[4].url, "http://127.0.0.1:18080/api/v1/chat/rooms/AB12CD3E/members/9223372036854775807/mute");
  calls.forEach(({ options }) => assert.equal(options.headers.Authorization, "Bearer access-token"));
});

test("updates and deletes a chat group with authorization", async () => {
  const calls = [];
  globalThis.fetch = async (url, options) => {
    calls.push({ url, options });
    return jsonResponse(200, {
      service: "api-gateway",
      http_code: 200,
      code: 0,
      message: "success",
      data: {}
    });
  };

  await bbsApi.updateChatGroup("900", { name: "常用", sort_order: 2 }, "access-token");
  await bbsApi.deleteChatGroup("900", "access-token");

  assert.equal(calls[0].url, "http://127.0.0.1:18080/api/v1/chat/groups/900");
  assert.equal(calls[0].options.method, "PATCH");
  assert.equal(calls[0].options.headers.Authorization, "Bearer access-token");
  assert.deepEqual(JSON.parse(calls[0].options.body), { name: "常用", sort_order: 2 });
  assert.equal(calls[1].url, "http://127.0.0.1:18080/api/v1/chat/groups/900");
  assert.equal(calls[1].options.method, "DELETE");
  assert.equal(calls[1].options.headers.Authorization, "Bearer access-token");
});

test("moves a chat group atomically with authorization", async () => {
  let requestedUrl = "";
  let requestOptions;
  globalThis.fetch = async (url, options) => {
    requestedUrl = url;
    requestOptions = options;
    return jsonResponse(200, {
      service: "api-gateway",
      http_code: 200,
      code: 0,
      message: "success",
      data: { success: true }
    });
  };

  await bbsApi.moveChatGroup("900", -1, "access-token");

  assert.equal(requestedUrl, "http://127.0.0.1:18080/api/v1/chat/groups/900/move");
  assert.equal(requestOptions.method, "POST");
  assert.equal(requestOptions.headers.Authorization, "Bearer access-token");
  assert.deepEqual(JSON.parse(requestOptions.body), { direction: -1 });
});

test("builds the websocket URL under the configured API path", () => {
  assert.equal(chatWebSocketUrl("a/b?c"), "ws://127.0.0.1:18080/api/v1/chat/ws?ticket=a%2Fb%3Fc");
});

test("resolves a same-origin production API base before opening a websocket", () => {
  assert.equal(
    chatWebSocketUrlForBase("/api/v1", "ticket", "https://bbs.example.com"),
    "wss://bbs.example.com/api/v1/chat/ws?ticket=ticket"
  );
});

test("loads the public credit leaderboard without authentication", async () => {
  let requestedUrl = "";
  let requestOptions;
  globalThis.fetch = async (url, options) => {
    requestedUrl = url;
    requestOptions = options;
    return jsonResponse(200, {
      service: "api-gateway",
      http_code: 200,
      code: 0,
      message: "success",
      data: { items: [{ rank: 1, user_id: 42, total: 120 }] }
    });
  };

  const data = await bbsApi.creditLeaderboard({ limit: 6 });

  const url = new URL(requestedUrl);
  assert.equal(url.pathname, "/api/v1/credits/leaderboard");
  assert.equal(url.searchParams.get("limit"), "6");
  assert.equal(requestOptions.headers.Authorization, undefined);
  assert.equal(data.items[0].total, 120);
});

test("loads popular chat rooms and resources without authentication", async () => {
  const calls = [];
  globalThis.fetch = async (url, options) => {
    calls.push({ url, options });
    return jsonResponse(200, {
      service: "api-gateway",
      http_code: 200,
      code: 0,
      message: "success",
      data: { items: [] }
    });
  };

  await bbsApi.popularChatRooms({ limit: 4 });
  await bbsApi.popularResources({ limit: 3 });

  assert.equal(new URL(calls[0].url).pathname, "/api/v1/chat/popular");
  assert.equal(new URL(calls[0].url).searchParams.get("limit"), "4");
  assert.equal(calls[0].options.headers.Authorization, undefined);
  assert.equal(new URL(calls[1].url).pathname, "/api/v1/links/popular");
  assert.equal(new URL(calls[1].url).searchParams.get("limit"), "3");
  assert.equal(calls[1].options.headers.Authorization, undefined);
});

test("records resource visits without authentication", async () => {
  let requestedUrl = "";
  let requestOptions;
  globalThis.fetch = async (url, options) => {
    requestedUrl = url;
    requestOptions = options;
    return jsonResponse(200, {
      service: "api-gateway",
      http_code: 200,
      code: 0,
      message: "success",
      data: { success: true }
    });
  };

  await bbsApi.recordResourceVisit("42");

  assert.equal(requestedUrl, "http://127.0.0.1:18080/api/v1/links/42/visit");
  assert.equal(requestOptions.method, "POST");
  assert.equal(requestOptions.headers.Authorization, undefined);
});

test("cancels a mall refund with authorization", async () => {
  let requestedUrl = "";
  let options;
  globalThis.fetch = async (url, requestOptions) => {
    requestedUrl = url;
    options = requestOptions;
    return jsonResponse(200, {
      service: "api-gateway",
      http_code: 200,
      code: 0,
      message: "success",
      data: { refund: { id: "700", status: 5 } }
    });
  };

  await bbsApi.cancelMallRefund("700", "access-token");

  assert.equal(requestedUrl, "http://127.0.0.1:18080/api/v1/mall/refunds/700/cancel");
  assert.equal(options.method, "POST");
  assert.equal(options.headers.Authorization, "Bearer access-token");
});

test("revokes an accepted topic comment with authorization", async () => {
  let requestedUrl = "";
  let options;
  globalThis.fetch = async (url, requestOptions) => {
    requestedUrl = url;
    options = requestOptions;
    return jsonResponse(200, {
      service: "api-gateway",
      http_code: 200,
      code: 0,
      message: "success",
      data: { topic: { id: "1001", qa_status: "open", accepted_comment_id: 0 } }
    });
  };

  await bbsApi.unacceptTopicComment("1001", "9001", "access-token");

  assert.equal(requestedUrl, "http://127.0.0.1:18080/api/v1/topics/1001/comments/9001/unaccept");
  assert.equal(options.method, "POST");
  assert.equal(options.headers.Authorization, "Bearer access-token");
});

test("loads a bounded comment ancestor conversation without authentication", async () => {
  let requestedUrl = "";
  let options;
  globalThis.fetch = async (url, requestOptions) => {
    requestedUrl = url;
    options = requestOptions;
    return jsonResponse(200, {
      service: "api-gateway",
      http_code: 200,
      code: 0,
      message: "success",
      data: { items: [{ id: "202" }, { id: "101" }] }
    });
  };

  const data = await bbsApi.commentConversation("303", { limit: 7, offset: 2 });

  assert.equal(requestedUrl, "http://127.0.0.1:18080/api/v1/comments/303/conversation?limit=7&offset=2");
  assert.equal(options.method, "GET");
  assert.equal(options.headers.Authorization, undefined);
  assert.deepEqual(data.items, [{ id: "202" }, { id: "101" }]);
});

test("uploads topic attachments as multipart form data without exposing a JSON content type", async () => {
  let requestedUrl = "";
  let options;
  globalThis.fetch = async (url, requestOptions) => {
    requestedUrl = url;
    options = requestOptions;
    return jsonResponse(200, {
      service: "api-gateway",
      http_code: 200,
      code: 0,
      message: "success",
      data: { id: "1001", original_name: "guide.txt", price_credits: 8 }
    });
  };

  const attachment = new Blob(["attachment"], { type: "text/plain" });
  const data = await bbsApi.uploadTopicAttachment("9223372036854775807", attachment, 8, "access-token");

  assert.equal(requestedUrl, "http://127.0.0.1:18080/api/v1/topics/9223372036854775807/attachments");
  assert.equal(options.method, "POST");
  assert.equal(options.headers.Authorization, "Bearer access-token");
  assert.equal(options.headers["Content-Type"], undefined);
  assert.equal(options.body.get("price_credits"), "8");
  assert.equal(options.body.get("file").size, attachment.size);
  assert.equal(options.body.get("file").type, "text/plain");
  assert.equal(data.id, "1001");
});

test("uploads generic files as multipart form data with a library source and folder", async () => {
  let requestedUrl = "";
  let options;
  globalThis.fetch = async (url, requestOptions) => {
    requestedUrl = url;
    options = requestOptions;
    return jsonResponse(200, {
      service: "api-gateway",
      http_code: 200,
      code: 0,
      message: "success",
      data: { id: "9223372036854775807", original_name: "release.zip", biz_type: "files" }
    });
  };

  const file = new Blob(["archive"], { type: "application/zip" });
  const data = await bbsApi.uploadFile(file, "access-token", "files", "9223372036854775000");

  assert.equal(requestedUrl, "http://127.0.0.1:18080/api/v1/files");
  assert.equal(options.method, "POST");
  assert.equal(options.headers.Authorization, "Bearer access-token");
  assert.equal(options.headers["Content-Type"], undefined);
  assert.equal(options.body.get("biz_type"), "files");
  assert.equal(options.body.get("folder_id"), "9223372036854775000");
  assert.equal(options.body.get("file").size, file.size);
  assert.equal(options.body.get("file").type, "application/zip");
  assert.equal(data.id, "9223372036854775807");
});

test("manages file folders and updates file metadata with precise ids", async () => {
  const requests = [];
  globalThis.fetch = async (url, options = {}) => {
    requests.push({ url, options });
    return jsonResponse(200, {
      service: "api-gateway",
      http_code: 200,
      code: 0,
      message: "success",
      data: requests.length === 1 ? { items: [], total: 0 } : { id: "9223372036854775807" }
    });
  };

  await bbsApi.fileFolders({ parent_id: "9223372036854775000", limit: 50, offset: 25 }, "access-token");
  await bbsApi.createFileFolder({ name: "资料", parent_id: null }, "access-token");
  await bbsApi.updateFileFolder("9223372036854775807", { name: "归档", parent_id: null }, "access-token");
  await bbsApi.updateFile("9223372036854775806", { name: "说明.txt", folder_id: "9223372036854775807", is_sensitive: false, comment: "版本说明" }, "access-token");
  await bbsApi.deleteFileFolder("9223372036854775807", "access-token");

  const listUrl = new URL(requests[0].url);
  assert.equal(listUrl.pathname, "/api/v1/file-folders");
  assert.equal(listUrl.searchParams.get("parent_id"), "9223372036854775000");
  assert.equal(listUrl.searchParams.get("limit"), "50");
  assert.equal(listUrl.searchParams.get("offset"), "25");
  assert.equal(requests[1].options.method, "POST");
  assert.equal(requests[2].options.method, "PUT");
  assert.equal(requests[3].options.method, "PATCH");
  assert.deepEqual(JSON.parse(requests[3].options.body), {
    name: "说明.txt",
    folder_id: "9223372036854775807",
    is_sensitive: false,
    comment: "版本说明"
  });
  assert.equal(requests[4].options.method, "DELETE");
  assert.equal(requests.every(({ options }) => options.headers.Authorization === "Bearer access-token"), true);
});

test("lists, reads, and deletes files with pagination and authorization", async () => {
  const requests = [];
  globalThis.fetch = async (url, options = {}) => {
    requests.push({ url, options });
    return jsonResponse(200, {
      service: "api-gateway",
      http_code: 200,
      code: 0,
      message: "success",
      data: requests.length === 1 ? { items: [], total: 0 } : { id: "9223372036854775807" }
    });
  };

  await bbsApi.listFiles({ limit: 50, offset: 25 }, "access-token");
  await bbsApi.getFile("9223372036854775807", "access-token");
  await bbsApi.deleteFile("9223372036854775807", "access-token");

  const listUrl = new URL(requests[0].url);
  assert.equal(listUrl.pathname, "/api/v1/files");
  assert.equal(listUrl.searchParams.get("limit"), "50");
  assert.equal(listUrl.searchParams.get("offset"), "25");
  assert.equal(requests[1].url, "http://127.0.0.1:18080/api/v1/files/9223372036854775807");
  assert.equal(requests[1].options.method, "GET");
  assert.equal(requests[2].url, "http://127.0.0.1:18080/api/v1/files/9223372036854775807");
  assert.equal(requests[2].options.method, "DELETE");
  assert.equal(requests.every(({ options }) => options.headers.Authorization === "Bearer access-token"), true);
});

test("reads authenticated file storage usage", async () => {
  let requestedUrl = "";
  let authorization = "";
  globalThis.fetch = async (url, options) => {
    requestedUrl = url;
    authorization = options.headers.Authorization;
    return jsonResponse(200, {
      service: "api-gateway",
      http_code: 200,
      code: 0,
      message: "success",
      data: {
        used_bytes: 1048576,
        capacity_bytes: 1073741824,
        remaining_bytes: 1072693248
      }
    });
  };

  const data = await bbsApi.getFileUsage("access-token");

  assert.equal(requestedUrl, "http://127.0.0.1:18080/api/v1/files/usage");
  assert.equal(authorization, "Bearer access-token");
  assert.deepEqual(data, {
    used_bytes: 1048576,
    capacity_bytes: 1073741824,
    remaining_bytes: 1072693248
  });
});

test("downloads generic files with authorization and preserves the response filename", async () => {
  let requestedUrl = "";
  let authorization = "";
  globalThis.fetch = async (url, options) => {
    requestedUrl = url;
    authorization = options.headers.Authorization;
    return {
      ok: true,
      status: 200,
      headers: new Headers({ "Content-Disposition": "attachment; filename*=UTF-8''release%20notes.txt" }),
      blob: async () => new Blob(["notes"], { type: "text/plain" })
    };
  };

  const data = await bbsApi.downloadFile("9223372036854775807", "access-token");

  assert.equal(requestedUrl, "http://127.0.0.1:18080/api/v1/files/9223372036854775807/download");
  assert.equal(authorization, "Bearer access-token");
  assert.equal(data.filename, "release notes.txt");
  assert.equal(await data.blob.text(), "notes");
});

test("updates a topic attachment price with authorization", async () => {
  let requestedUrl = "";
  let options;
  globalThis.fetch = async (url, requestOptions) => {
    requestedUrl = url;
    options = requestOptions;
    return jsonResponse(200, {
      service: "api-gateway",
      http_code: 200,
      code: 0,
      message: "success",
      data: { id: "1001", price_credits: 13 }
    });
  };

  const data = await bbsApi.updateTopicAttachmentPrice("9223372036854775807", 13, "access-token");

  assert.equal(requestedUrl, "http://127.0.0.1:18080/api/v1/attachments/9223372036854775807");
  assert.equal(options.method, "PATCH");
  assert.equal(options.headers.Authorization, "Bearer access-token");
  assert.equal(options.headers["Content-Type"], "application/json");
  assert.deepEqual(JSON.parse(options.body), { price_credits: 13 });
  assert.equal(data.price_credits, 13);
});

test("downloads protected topic attachments with authorization and extracts the response filename", async () => {
  let requestedUrl = "";
  let authorization = "";
  globalThis.fetch = async (url, options) => {
    requestedUrl = url;
    authorization = options.headers.Authorization;
    return {
      ok: true,
      status: 200,
      headers: new Headers({ "Content-Disposition": "attachment; filename*=UTF-8''guide%20notes.txt" }),
      blob: async () => new Blob(["attachment"], { type: "text/plain" })
    };
  };

  const data = await bbsApi.downloadTopicAttachment("9223372036854775807", "access-token");

  assert.equal(requestedUrl, "http://127.0.0.1:18080/api/v1/attachments/9223372036854775807/download");
  assert.equal(authorization, "Bearer access-token");
  assert.equal(data.filename, "guide notes.txt");
  assert.equal(await data.blob.text(), "attachment");
});

test("lists the current user's attachment downloads with pagination and authorization", async () => {
  let requestedUrl = "";
  let authorization = "";
  globalThis.fetch = async (url, options) => {
    requestedUrl = url;
    authorization = options.headers.Authorization;
    return jsonResponse(200, {
      service: "api-gateway",
      http_code: 200,
      code: 0,
      message: "success",
      data: { items: [] }
    });
  };

  await bbsApi.attachmentDownloads({ topic_id: "336853987166789633", limit: 6, offset: 4 }, "access-token");

  const url = new URL(requestedUrl);
  assert.equal(url.pathname, "/api/v1/attachments/downloads");
  assert.equal(url.searchParams.get("limit"), "6");
  assert.equal(url.searchParams.get("offset"), "4");
  assert.equal(url.searchParams.get("topic_id"), "336853987166789633");
  assert.equal(authorization, "Bearer access-token");
});

test("lists the current user's attachment sales with pagination and authorization", async () => {
  let requestedUrl = "";
  let authorization = "";
  globalThis.fetch = async (url, options) => {
    requestedUrl = url;
    authorization = options.headers.Authorization;
    return jsonResponse(200, {
      service: "api-gateway",
      http_code: 200,
      code: 0,
      message: "success",
      data: { items: [] }
    });
  };

  await bbsApi.attachmentSales({ limit: 6, offset: 4 }, "access-token");

  const url = new URL(requestedUrl);
  assert.equal(url.pathname, "/api/v1/attachments/sales");
  assert.equal(url.searchParams.get("limit"), "6");
  assert.equal(url.searchParams.get("offset"), "4");
  assert.equal(authorization, "Bearer access-token");
});

test("loads and submits the authenticated daily check-in", async () => {
  const requests = [];
  globalThis.fetch = async (url, options) => {
    requests.push({ url, options });
    return jsonResponse(200, {
      service: "api-gateway",
      http_code: 200,
      code: 0,
      message: "success",
      data: { checked_in: options.method !== "POST", reward_credits: 5 }
    });
  };

  await bbsApi.checkInStatus("access-token");
  await bbsApi.checkIn("access-token");

  assert.equal(requests[0].url, "http://127.0.0.1:18080/api/v1/credits/check-in");
  assert.equal(requests[0].options.method, "GET");
  assert.equal(requests[0].options.headers.Authorization, "Bearer access-token");
  assert.equal(requests[1].url, "http://127.0.0.1:18080/api/v1/credits/check-in");
  assert.equal(requests[1].options.method, "POST");
  assert.equal(requests[1].options.headers.Authorization, "Bearer access-token");
  assert.equal(requests[1].options.headers["Content-Type"], undefined);
});

test("loads task claim state and claims configured task rewards with authorization", async () => {
  const requests = [];
  globalThis.fetch = async (url, options) => {
    requests.push({ url, options });
    return jsonResponse(200, {
      service: "api-gateway",
      http_code: 200,
      code: 0,
      message: "success",
      data: { items: [], total: 0 }
    });
  };

  await bbsApi.myTasks({ limit: 6, offset: 2 }, "access-token");
  await bbsApi.claimTask("8", "access-token");

  const taskListURL = new URL(requests[0].url);
  assert.equal(taskListURL.pathname, "/api/v1/tasks/me");
  assert.equal(taskListURL.searchParams.get("limit"), "6");
  assert.equal(taskListURL.searchParams.get("offset"), "2");
  assert.equal(requests[0].options.method, "GET");
  assert.equal(requests[0].options.headers.Authorization, "Bearer access-token");
  assert.equal(requests[1].url, "http://127.0.0.1:18080/api/v1/tasks/8/claim");
  assert.equal(requests[1].options.method, "POST");
  assert.equal(requests[1].options.headers.Authorization, "Bearer access-token");
  assert.equal(requests[1].options.headers["Content-Type"], undefined);
});

test("throws ApiError with gateway envelope metadata for HTTP failures", async () => {
  globalThis.fetch = async () =>
    jsonResponse(
      412,
      {
        service: "api-gateway",
        trace_id: "trace-1",
        request_id: "req-1",
        http_code: 412,
        code: 412,
        reason: "Precondition Failed",
        message: "商品库存不足",
        meta: { legacy_code: "FailedPrecondition" },
        data: null
      },
      { "X-Request-ID": "req-header" }
    );

  await assert.rejects(
    () => bbsApi.createMallOrder({ items: [{ product_id: 1001, quantity: 2 }] }, "token"),
    (error) => {
      assert.ok(error instanceof ApiError);
      assert.equal(error.message, "商品库存不足");
      assert.equal(error.status, 412);
      assert.equal(error.httpCode, 412);
      assert.equal(error.service, "api-gateway");
      assert.equal(error.traceId, "trace-1");
      assert.equal(error.requestId, "req-1");
      assert.deepEqual(error.meta, { legacy_code: "FailedPrecondition" });
      assert.equal(error.retryAfterSeconds, 0);
      return true;
    }
  );
});

test("preserves Retry-After metadata on API errors", async () => {
  globalThis.fetch = async () => jsonResponse(429, {
    service: "api-gateway",
    http_code: 429,
    code: 429,
    message: "too many requests",
    meta: { legacy_code: "rate_limited" },
    data: null
  }, { "Retry-After": "75" });

  await assert.rejects(
    () => bbsApi.createChatWebSocketTicket("token"),
    (error) => {
      assert.ok(error instanceof ApiError);
      assert.equal(error.retryAfterSeconds, 75);
      return true;
    }
  );
  assert.equal(parseRetryAfterSeconds("Wed, 21 Oct 2015 07:28:00 GMT", Date.parse("Wed, 21 Oct 2015 07:27:45 GMT")), 15);
  assert.equal(parseRetryAfterSeconds("invalid"), 0);
});

test("notifies the app only when a token-authenticated request is unauthorized", async () => {
  const originalWindow = globalThis.window;
  const events = [];
  class FakeCustomEvent {
    constructor(type, options) {
      this.type = type;
      this.detail = options?.detail;
    }
  }
  globalThis.window = {
    CustomEvent: FakeCustomEvent,
    dispatchEvent: (event) => {
      events.push(event);
      return true;
    }
  };

  try {
    globalThis.fetch = async () =>
      jsonResponse(401, { service: "api-gateway", http_code: 401, code: 401, message: "unauthorized", data: null });
    await assert.rejects(() => bbsApi.me("stale-token"));

    globalThis.fetch = async () =>
      jsonResponse(200, { service: "api-gateway", http_code: 401, code: 401, message: "unauthorized", data: null });
    await assert.rejects(() => bbsApi.me("stale-token"));

    globalThis.fetch = async () =>
      jsonResponse(401, { service: "api-gateway", http_code: 401, code: 401, message: "unauthorized", data: null });
    await assert.rejects(() => bbsApi.downloadTopicAttachment("123", "stale-token"));

    globalThis.fetch = async () =>
      jsonResponse(401, { service: "api-gateway", http_code: 401, code: 401, message: "unauthorized", data: null });
    await assert.rejects(() => bbsApi.login({ account: "demo", password: "bad" }));

    globalThis.fetch = async () =>
      jsonResponse(429, { service: "api-gateway", http_code: 429, code: 429, message: "rate limited", data: null });
    await assert.rejects(() => bbsApi.createChatWebSocketTicket("stale-token"));

    assert.deepEqual(events.map(({ type, detail }) => ({ type, detail })), [
      { type: AUTH_INVALIDATED_EVENT, detail: { accessToken: "stale-token" } },
      { type: AUTH_INVALIDATED_EVENT, detail: { accessToken: "stale-token" } },
      { type: AUTH_INVALIDATED_EVENT, detail: { accessToken: "stale-token" } }
    ]);
    assert.equal(isUnauthorizedError({ status: 200, httpCode: 401 }), true);
    assert.equal(isUnauthorizedError({ status: 403, httpCode: 403 }), false);
  } finally {
    globalThis.window = originalWindow;
  }
});

test("normalizes non-JSON HTTP failures into ApiError", async () => {
  globalThis.fetch = async () => textResponse(502, "<html>bad gateway</html>");

  await assert.rejects(
    () => bbsApi.authConfig(),
    (error) => {
      assert.ok(error instanceof ApiError);
      assert.equal(error.message, "<html>bad gateway</html>");
      assert.equal(error.status, 502);
      assert.equal(error.httpCode, 502);
      assert.equal(error.rawBody, "<html>bad gateway</html>");
      return true;
    }
  );
});

test("throws ApiError for business-error envelopes even when HTTP status is ok", async () => {
  globalThis.fetch = async () =>
    jsonResponse(200, {
      service: "api-gateway",
      http_code: 409,
      code: 409,
      reason: "Conflict",
      message: "duplicate reference",
      meta: { legacy_code: "AlreadyExists" },
      data: null
    });

  await assert.rejects(
    () => bbsApi.createMallProductReview(77, { order_id: 88, rating: 5 }, "token"),
    (error) => {
      assert.ok(error instanceof ApiError);
      assert.equal(error.message, "duplicate reference");
      assert.equal(error.status, 200);
      assert.equal(error.httpCode, 409);
      assert.deepEqual(error.meta, { legacy_code: "AlreadyExists" });
      return true;
    }
  );
});

test("maps channel discovery, management, and relationship requests", async () => {
  const requests = [];
  globalThis.fetch = async (url, options = {}) => {
    requests.push({ url, options });
    return textResponse(
      200,
      '{"service":"api-gateway","http_code":200,"code":0,"message":"success","data":{"channel":{"id":9223372036854775807}}}'
    );
  };

  await bbsApi.channels({ q: "工程", category_id: "9223372036854775806" }, "access-token");
  await bbsApi.featuredChannels({ limit: 6 }, "access-token");
  await bbsApi.channelCategories();
  await bbsApi.ownedChannels({}, "access-token");
  await bbsApi.followedChannels({ offset: 20 }, "access-token");
  await bbsApi.favoriteChannels({}, "access-token");
  const detail = await bbsApi.getChannel("9223372036854775807", "access-token");
  await bbsApi.createChannel({ name: "工程实践", color: "#1683f7", category_id: "9" }, "access-token");
  await bbsApi.updateChannel("9223372036854775807", { description: "经验沉淀" }, "access-token");
  await bbsApi.archiveChannel("9223372036854775807", "access-token");
  await bbsApi.followChannel("9223372036854775807", "access-token");
  await bbsApi.unfollowChannel("9223372036854775807", "access-token");
  await bbsApi.favoriteChannel("9223372036854775807", "access-token");
  await bbsApi.unfavoriteChannel("9223372036854775807", "access-token");
  await bbsApi.channelTopics("9223372036854775807", { limit: 10, offset: 10 });

  assert.equal(requests[0].url, "http://127.0.0.1:18080/api/v1/channels?limit=20&offset=0&q=%E5%B7%A5%E7%A8%8B&category_id=9223372036854775806");
  assert.equal(requests[0].options.headers.Authorization, "Bearer access-token");
  assert.equal(requests[1].url, "http://127.0.0.1:18080/api/v1/channels/featured?limit=6&offset=0");
  assert.equal(requests[2].url, "http://127.0.0.1:18080/api/v1/channels/categories");
  assert.equal(requests[3].url, "http://127.0.0.1:18080/api/v1/channels/owned?limit=20&offset=0");
  assert.equal(requests[4].url, "http://127.0.0.1:18080/api/v1/channels/followed?limit=20&offset=20");
  assert.equal(requests[5].url, "http://127.0.0.1:18080/api/v1/channels/favorites?limit=20&offset=0");
  assert.equal(detail.channel.id, "9223372036854775807");
  assert.equal(requests[7].options.method, "POST");
  assert.deepEqual(JSON.parse(requests[7].options.body), { name: "工程实践", color: "#1683f7", category_id: "9" });
  assert.equal(requests[8].options.method, "PUT");
  assert.equal(requests[9].options.method, "DELETE");
  assert.equal(requests[10].url.endsWith("/channels/9223372036854775807/follow"), true);
  assert.equal(requests[10].options.method, "POST");
  assert.equal(requests[11].options.method, "DELETE");
  assert.equal(requests[12].url.endsWith("/channels/9223372036854775807/favorite"), true);
  assert.equal(requests[12].options.method, "POST");
  assert.equal(requests[13].options.method, "DELETE");
  assert.equal(requests[14].url, "http://127.0.0.1:18080/api/v1/channels/9223372036854775807/topics?limit=10&offset=10");
});

function jsonResponse(status, body, headers = {}) {
  return textResponse(status, JSON.stringify(body), headers);
}

function textResponse(status, body, headers = {}) {
  return {
    ok: status >= 200 && status < 300,
    status,
    headers: new Headers(headers),
    text: async () => body
  };
}
