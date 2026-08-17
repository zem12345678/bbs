import assert from "node:assert/strict";
import test from "node:test";

import {
  NOTIFICATION_PREFERENCES,
  defaultNotificationPreferences,
  mergeNotificationPreferences
} from "./notificationPreferences.js";

test("exposes export completion as a user-visible notification preference", () => {
  const definition = NOTIFICATION_PREFERENCES.find((item) => item.type === "export_completed");

  assert.deepEqual(definition, {
    type: "export_completed",
    label: "数据导出",
    description: "个人数据导出文件生成完成"
  });
  assert.deepEqual(
    defaultNotificationPreferences().find((item) => item.type === "export_completed"),
    { type: "export_completed", enabled: true }
  );
});

test("preserves disabled and newer server preferences when another setting is saved", () => {
  const merged = mergeNotificationPreferences([
    { type: "export_completed", enabled: false },
    { notification_type: "comment", enabled: false },
    { type: "future_server_notification", enabled: false }
  ]);

  assert.deepEqual(
    merged.filter((item) => ["export_completed", "comment", "future_server_notification"].includes(item.type)),
    [
      { type: "export_completed", enabled: false },
      { type: "comment", enabled: false },
      { type: "future_server_notification", enabled: false }
    ]
  );
});
