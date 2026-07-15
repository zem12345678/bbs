import assert from "node:assert/strict";
import test from "node:test";

import { mallOrderReviewableProductIds } from "./mallOrders.js";

test("mallOrderReviewableProductIds preserves unique order item product ids", () => {
  assert.deepEqual(
    mallOrderReviewableProductIds({
      items: [
        { product_id: 101, title: "会员月卡" },
        { productId: "102", title: "高级主题" },
        { product: { id: 103 }, title: "创始徽章" },
        { product_id: 101, title: "会员月卡加购" },
        { title: "缺少商品 ID" }
      ]
    }),
    ["101", "102", "103"]
  );
});

test("mallOrderReviewableProductIds handles empty orders", () => {
  assert.deepEqual(mallOrderReviewableProductIds({}), []);
  assert.deepEqual(mallOrderReviewableProductIds({ items: null }), []);
});
