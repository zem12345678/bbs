import assert from "node:assert/strict";
import { afterEach, test } from "node:test";

import { ApiError, bbsApi } from "./api.js";

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

  await bbsApi.attachmentDownloads({ limit: 6, offset: 4 }, "access-token");

  const url = new URL(requestedUrl);
  assert.equal(url.pathname, "/api/v1/attachments/downloads");
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
      return true;
    }
  );
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
