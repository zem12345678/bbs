import assert from "node:assert/strict";
import { afterEach, test } from "node:test";

import { AUTH_INVALIDATED_EVENT, ApiError, bbsApi, chatWebSocketUrl, chatWebSocketUrlForBase, isUnauthorizedError, parseRetryAfterSeconds } from "./api.js";

afterEach(() => {
  delete globalThis.fetch;
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
