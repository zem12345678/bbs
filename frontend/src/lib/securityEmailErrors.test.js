import assert from "node:assert/strict";
import test from "node:test";

import { friendlySecurityEmailError } from "./securityEmailErrors.js";

test("friendlySecurityEmailError maps unavailable delivery to an actionable message", () => {
  assert.equal(
    friendlySecurityEmailError({ message: "security email delivery unavailable" }, "发送失败"),
    "邮件服务暂不可用，请稍后重试。"
  );
});

test("friendlySecurityEmailError preserves unrelated API messages", () => {
  assert.equal(friendlySecurityEmailError({ message: "token expired" }, "发送失败"), "token expired");
  assert.equal(friendlySecurityEmailError({}, "发送失败"), "发送失败");
});
