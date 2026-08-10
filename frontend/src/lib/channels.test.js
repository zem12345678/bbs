import assert from "node:assert/strict";
import test from "node:test";

import { channelCategories, channelColor, channelFromResponse, channelList, ownsChannel } from "./channels.js";

test("normalizes channel ids, counts, colors, and relationship state", () => {
  const result = channelList({
    items: [{
      id: "9223372036854775807",
      owner_id: "9223372036854775806",
      category_id: "42",
      name: "  工程实践  ",
      color: "not-a-color",
      followers_count: "12",
      topics_count: 7,
      is_featured: true,
      is_following: true,
      is_favorited: false
    }],
    total: "31"
  });

  assert.equal(result.total, 31);
  assert.deepEqual(result.items[0], {
    id: "9223372036854775807",
    owner_id: "9223372036854775806",
    category_id: "42",
    name: "工程实践",
    color: "#1683f7",
    followers_count: 12,
    topics_count: 7,
    is_following: true,
    is_favorited: false,
    description: "",
    is_archived: false,
    is_featured: true
  });
});

test("unwraps channel mutations and channel category ids", () => {
  const channel = channelFromResponse({ channel: { id: "99", owner_id: "7", name: "前端", color: "#0A84FF" } });
  const categories = channelCategories({ items: [
    { category_id: "5", name: " 技术 ", channels_count: "3" },
    { id: "6", name: " 产品 " }
  ] });

  assert.equal(channel.color, "#0A84FF");
  assert.equal(ownsChannel(channel, { id: 7 }), true);
  assert.deepEqual(categories, [
    { category_id: "5", id: "5", name: "技术", channels_count: 3, followers_count: 0, topics_count: 0 },
    { id: "6", name: "产品", channels_count: 0, followers_count: 0, topics_count: 0 }
  ]);
  assert.equal(channelColor("#abc"), "#1683f7");
});
