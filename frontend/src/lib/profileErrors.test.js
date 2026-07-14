import assert from "node:assert/strict";
import test from "node:test";

import { friendlyProfileUpdateError, isProfileThemeEntitlementError } from "./profileErrors.js";

test("isProfileThemeEntitlementError recognizes profile theme permission failures", () => {
  assert.equal(
    isProfileThemeEntitlementError({
      status: 403,
      message: "profile theme entitlement required"
    }),
    true
  );
});

test("isProfileThemeEntitlementError ignores unrelated errors", () => {
  assert.equal(
    isProfileThemeEntitlementError({
      status: 403,
      message: "membership entitlement required for bounty QA topics"
    }),
    false
  );
  assert.equal(
    isProfileThemeEntitlementError({
      status: 400,
      message: "profile theme entitlement required"
    }),
    false
  );
});

test("friendlyProfileUpdateError maps theme entitlement failures", () => {
  assert.equal(
    friendlyProfileUpdateError({
      httpCode: 403,
      message: "profile theme entitlement required"
    }),
    "高级主题需要 theme-pro 权益，请先购买或切回默认主题。"
  );
  assert.equal(friendlyProfileUpdateError({ message: "" }), "资料保存失败");
});
