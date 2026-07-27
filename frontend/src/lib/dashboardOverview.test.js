import assert from "node:assert/strict";
import { test } from "node:test";

import { dashboardOverviewLoadState, dashboardOverviewMetric } from "./dashboardOverview.js";

test("keeps successful dashboard data visible when only some overview requests fail", () => {
  const state = dashboardOverviewLoadState([{ items: [] }, { error: new Error("mall unavailable") }, { total: 3 }]);

  assert.deepEqual(state, { error: "", notice: "部分数据暂不可用（1/3）" });
  assert.deepEqual(dashboardOverviewMetric({ error: new Error("mall unavailable") }, 0, "商城订单"), {
    value: "—",
    label: "商城订单"
  });
  assert.deepEqual(dashboardOverviewMetric({ total: 3 }, 3, "我的话题"), { value: 3, label: "我的话题" });
});

test("shows an overview failure instead of zero metrics when every request fails", () => {
  const state = dashboardOverviewLoadState([{ error: new Error("content unavailable") }, { error: new Error("credit unavailable") }]);

  assert.deepEqual(state, { error: "个人工作台加载失败，请稍后重试。", notice: "" });
});
