import assert from "node:assert/strict";
import test from "node:test";

import {
  describeUserAgent,
  loginFailureLabel,
  loginMethodLabel,
  normalizeLoginEventList,
  normalizeSessionList,
  sessionStatus,
  sessionStatusLabel
} from "./sessions.js";

test("normalizes session lists and drops entries without a session id", () => {
  assert.deepEqual(
    normalizeSessionList({
      items: [
        {
          session_id: "session-abcdef0123456789",
          user_id: 42,
          ip_address: "203.0.113.7",
          user_agent: "Mozilla/5.0 (Windows NT 10.0) Chrome/120.0",
          login_method: "password",
          created_at: 1_800_000_000,
          expires_at: 1_800_086_400
        },
        { user_id: 42, ip_address: "203.0.113.8" }
      ],
      total: 2
    }),
    {
      items: [
        {
          sessionId: "session-abcdef0123456789",
          ipAddress: "203.0.113.7",
          userAgent: "Mozilla/5.0 (Windows NT 10.0) Chrome/120.0",
          loginMethod: "password",
          createdAt: 1_800_000_000,
          expiresAt: 1_800_086_400,
          revokedAt: 0,
          current: false
        }
      ],
      total: 2
    }
  );
  assert.deepEqual(normalizeSessionList(null), { items: [], total: 0 });
});

// The gateway serializes protobuf structs with omitempty, so an active
// session omits revoked_at entirely and a failed login omits success.
test("treats omitted protobuf fields as unset rather than missing data", () => {
  const [session] = normalizeSessionList({
    items: [{ session_id: "session-abcdef0123456789", login_method: "passkey", created_at: 10, expires_at: 20 }]
  }).items;
  assert.equal(session.revokedAt, 0);
  assert.equal(session.current, false);

  const [event] = normalizeLoginEventList({
    items: [{ id: "907", user_id: 42, failure_reason: "invalid_password", created_at: 10 }]
  }).items;
  assert.equal(event.success, false);
  assert.equal(event.sessionId, "");
});

// Login event ids are snowflakes that exceed Number.MAX_SAFE_INTEGER, so
// they must survive normalization as exact strings.
test("keeps login event ids as lossless strings", () => {
  const { items, total } = normalizeLoginEventList({
    items: [
      {
        id: "9007199254740993",
        session_id: "session-abcdef0123456789",
        ip_address: "203.0.113.7",
        user_agent: "Mozilla/5.0 (iPhone) Safari/605.1",
        success: true,
        created_at: 1_800_000_000
      }
    ],
    total: 1
  });
  assert.equal(items[0].id, "9007199254740993");
  assert.equal(items[0].success, true);
  assert.equal(items[0].failureReason, "");
  assert.equal(total, 1);
});

test("derives session status from revocation and expiry", () => {
  const now = 1_800_000_000;
  assert.equal(sessionStatus({ expiresAt: now + 60 }, now), "active");
  assert.equal(sessionStatus({ expiresAt: now + 60, revokedAt: now - 10 }, now), "revoked");
  assert.equal(sessionStatus({ expiresAt: now - 1 }, now), "expired");
  assert.equal(sessionStatus({ expiresAt: now }, now), "expired");
  assert.equal(sessionStatus({}, now), "active");
  assert.equal(sessionStatusLabel("revoked"), "已退出");
  assert.equal(sessionStatusLabel("nonsense"), "未知");
});

test("summarizes user agents and labels login methods", () => {
  assert.equal(describeUserAgent("Mozilla/5.0 (Windows NT 10.0; Win64) Chrome/120.0 Safari/537.36"), "Windows · Chrome");
  assert.equal(describeUserAgent("Mozilla/5.0 (iPhone; CPU iPhone OS 17_0) Version/17.0 Safari/605.1"), "iOS · Safari");
  assert.equal(describeUserAgent("Mozilla/5.0 (Macintosh) Edg/120.0"), "macOS · Edge");
  assert.equal(describeUserAgent(""), "未知设备");
  assert.equal(describeUserAgent("curl/8.4.0"), "curl/8.4.0");
  assert.equal(loginMethodLabel("PASSWORD"), "密码登录");
  assert.equal(loginMethodLabel("custom_sso"), "custom_sso");
  assert.equal(loginMethodLabel(""), "未知方式");
  assert.equal(loginFailureLabel("invalid_password"), "密码错误");
  assert.equal(loginFailureLabel(""), "登录失败");
});