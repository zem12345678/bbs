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
