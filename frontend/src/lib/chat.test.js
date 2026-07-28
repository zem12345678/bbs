import assert from "node:assert/strict";
import { test } from "node:test";

import {
  compareChatIntegers,
  CoalescedUserLoader,
  createChatComposerSubmissionGuard,
  createChatHistoryRequestTracker,
  createChatSupersededRequestTracker,
  groupedChatRooms,
  isCurrentChatRoomSessionRequest,
  isCurrentChatRoomRequest,
  latestChatSeq,
  mergeChatMessagePage,
  mergeChatMessages,
  moveChatGroup,
  needsChatRepair,
  normalizeChatSidebar,
  orderedChatGroups,
  pendingChatMessagesForRoom,
  realtimeMessage,
  realtimePayload,
  unreadChatIndex
} from "./chat.js";

test("normalizes sidebar data and groups rooms in stable user order", () => {
  const sidebar = normalizeChatSidebar({
    groups: [
      { id: 2, name: "later", sort_order: 2 },
      { id: 1, name: "first", sort_order: 1 }
    ],
    rooms: [
      {
        room: { id: "11", room_no: "ab12cd3e" },
        membership: { group_id: 1, sort_order: 2 },
        last_message: { id: "201", sender_id: "7", body: "latest", created_at: "1710000000000" },
        unread_count: 3
      },
      { room: { id: "12", room_no: "yz83t019" }, membership: { group_id: 1, sort_order: 1 }, unread_count: 0 },
      { room: { id: "13", room_no: "jk45mn6p" }, membership: { group_id: 0, sort_order: 0 } }
    ]
  });
  const sections = groupedChatRooms(sidebar.groups, sidebar.rooms);

  assert.equal(sidebar.rooms[0].room_no, "AB12CD3E");
  assert.equal(sidebar.rooms[0].unread_count, "3");
  assert.deepEqual(sidebar.rooms[0].last_message, {
    id: "201", sender_id: "7", body: "latest", created_at: "1710000000000", room_id: "", room_no: "",
    seq: "", client_message_id: "", updated_at: "", deleted_at: "", sender: undefined
  });
  assert.equal(sections[0].group.name, "first");
  assert.deepEqual(sections[0].rooms.map((item) => item.room_no), ["YZ83T019", "AB12CD3E"]);
  assert.equal(sections.at(-1).group, null);
});

test("reorders chat groups using a stable server-ready sort order", () => {
  const groups = [
    { id: "20", name: "later", sort_order: 2 },
    { id: "10", name: "first", sort_order: 0 },
    { id: "30", name: "middle", sort_order: 1 }
  ];

  assert.deepEqual(orderedChatGroups(groups).map((group) => group.id), ["10", "30", "20"]);
  assert.deepEqual(moveChatGroup(groups, "30", -1).map((group) => group.id), ["30", "10", "20"]);
  assert.deepEqual(moveChatGroup(groups, "10", -1).map((group) => group.id), ["10", "30", "20"]);
});

test("rejects delayed history responses for a room that is no longer active", () => {
  assert.equal(isCurrentChatRoomRequest("ab12cd3e", "AB12CD3E"), true);
  assert.equal(isCurrentChatRoomRequest("AB12CD3E", "YZ83T019"), false);
  assert.equal(isCurrentChatRoomRequest("", "AB12CD3E"), false);
  assert.equal(isCurrentChatRoomSessionRequest("AB12CD3E", 3, "AB12CD3E", 3), true);
  assert.equal(isCurrentChatRoomSessionRequest("AB12CD3E", 2, "AB12CD3E", 3), false);
});

test("reconciles optimistic messages by client id and keeps them at the end", () => {
  const pending = {
    room_id: "8",
    client_message_id: "client-1",
    sender_id: "42",
    body: "hello",
    pending: true
  };
  const existing = [{ id: "10", room_id: "8", seq: "2", client_message_id: "old", sender_id: "7", body: "old" }];
  const optimistic = mergeChatMessages(existing, [pending]);
  assert.equal(optimistic.at(-1).pending, true);

  const acknowledged = mergeChatMessages(optimistic, [{
    id: "11",
    room_id: "8",
    seq: "3",
    client_message_id: "client-1",
    sender_id: "42",
    body: "hello"
  }]);
  assert.equal(acknowledged.length, 2);
  assert.equal(acknowledged.at(-1).id, "11");
  assert.equal(Boolean(acknowledged.at(-1).pending), false);
});

test("keeps each pending message ID once when replaying an active room", () => {
  const messages = pendingChatMessagesForRoom([
    { room_no: "ab12cd3e", client_message_id: "retry-1", body: "first", pending: true },
    { room_no: "AB12CD3E", client_message_id: "retry-1", body: "duplicate", pending: true },
    { room_no: "AB12CD3E", client_message_id: "retry-2", body: "second", pending: true },
    { room_no: "AB12CD3E", client_message_id: "sent", body: "done" },
    { room_no: "YZ83T019", client_message_id: "other-room", body: "other", pending: true }
  ], "AB12CD3E");

  assert.deepEqual(messages.map((message) => [message.client_message_id, message.body]), [
    ["retry-1", "first"],
    ["retry-2", "second"]
  ]);
});

test("blocks one synchronous duplicate composer submission without blocking a later retry", () => {
  const guard = createChatComposerSubmissionGuard();

  assert.equal(guard.claim(" hello ", "request-1"), true);
  assert.equal(guard.claim("hello", "request-2"), false);
  assert.equal(guard.release("different-request"), false);
  assert.equal(guard.release("request-1"), true);
  assert.equal(guard.claim("hello", "request-3"), true);
  guard.reset();
  assert.equal(guard.claim("hello", "request-4"), true);
  assert.equal(guard.claim("   ", "request-5"), false);
});

test("consumes only late errors for superseded chat request IDs", () => {
  const tracker = createChatSupersededRequestTracker(2);

  tracker.remember("old-request");
  assert.equal(tracker.consume("old-request"), true);
  assert.equal(tracker.consume("old-request"), false);
  tracker.remember("first");
  tracker.remember("second");
  tracker.remember("third");
  assert.equal(tracker.consume("first"), false);
  assert.equal(tracker.consume("second"), true);
  assert.equal(tracker.consume("third"), true);
});

test("finds unread boundaries and sequence gaps", () => {
  const messages = [{ seq: "3" }, { seq: 4 }, { seq: "5" }];
  assert.equal(unreadChatIndex(messages, "3"), 1);
  assert.equal(needsChatRepair(messages, "7"), "5");
  assert.equal(needsChatRepair(messages, "6"), null);
});

test("orders and repairs sequences beyond JavaScript's safe number range", () => {
  const messages = mergeChatMessages(
    [{ id: "2", room_id: "8", seq: "9223372036854775806", sender_id: "7", body: "later" }],
    [{ id: "1", room_id: "8", seq: "9223372036854775805", sender_id: "7", body: "earlier" }]
  );

  assert.deepEqual(messages.map((message) => message.seq), ["9223372036854775805", "9223372036854775806"]);
  assert.equal(latestChatSeq(messages), "9223372036854775806");
  assert.equal(needsChatRepair(messages, "9223372036854775808"), "9223372036854775806");
  assert.equal(compareChatIntegers("9223372036854775807", "9223372036854775806"), 1);
});

test("keeps the opposite history boundary while loading directional chat pages", () => {
  const current = { hasOlder: true, hasNewer: true, latestSeq: "9223372036854775806" };

  assert.deepEqual(
    mergeChatMessagePage(current, { has_older: false, has_newer: false, latest_seq: "9223372036854775807" }, "older"),
    { hasOlder: false, hasNewer: true, latestSeq: "9223372036854775807" }
  );
  assert.deepEqual(
    mergeChatMessagePage(current, { has_older: false, has_newer: false, latest_seq: "9223372036854775807" }, "newer"),
    { hasOlder: true, hasNewer: false, latestSeq: "9223372036854775807" }
  );
});

test("allows only one in-flight history page per room and direction", async () => {
  const tracker = createChatHistoryRequestTracker();
  const older = tracker.claim("ab12cd3e", "older", 1);
  const newer = tracker.claim("AB12CD3E", "newer", 1);
  const otherRoom = tracker.claim("YZ83T019", "older", 1);

  assert.ok(older);
  assert.ok(newer);
  assert.ok(otherRoom);
  assert.equal(tracker.claim("AB12CD3E", "older", 1), null);
  const olderPending = tracker.pending("AB12CD3E", "older", 1);
  assert.ok(olderPending);
  assert.equal(tracker.release(older), true);
  await olderPending;

  const replacement = tracker.claim("AB12CD3E", "older", 1);
  assert.ok(replacement);
  assert.equal(tracker.release(older), false);
  assert.equal(tracker.claim("AB12CD3E", "older", 1), null);
  const nextSession = tracker.claim("AB12CD3E", "older", 2);
  assert.ok(nextSession);
  assert.equal(tracker.release(replacement), true);
  assert.equal(tracker.release(newer), true);
  assert.equal(tracker.release(otherRoom), true);
  assert.equal(tracker.release(nextSession), true);
  assert.equal(tracker.pending("AB12CD3E", "older", 1), null);
});

test("unwraps durable websocket payloads and normalizes messages", () => {
  const event = {
    type: "message.created",
    payload: {
      event_id: "event-1",
      event_type: "chat.message.created.v1",
      payload: {
        message_id: "9223372036854775807",
        room_id: "8",
        room_no: "ab12cd3e",
        seq: "4",
        sender_id: "42",
        client_message_id: "client-1",
        body: "hello"
      }
    }
  };

  assert.equal(realtimePayload(event).event_id, "event-1");
  const message = realtimeMessage(event);
  assert.equal(message.id, "9223372036854775807");
  assert.equal(message.room_no, "AB12CD3E");
  assert.equal(message.seq, "4");
});

test("normalizes durable message deletion events", () => {
  const message = realtimeMessage({
    type: "message.deleted",
    payload: {
      event_id: "event-2",
      event_type: "chat.message.deleted.v1",
      payload: {
        message_id: "9223372036854775807",
        room_id: "8",
        room_no: "ab12cd3e",
        seq: "4",
        sender_id: "42",
        status: 2,
        updated_at: "1740000000000",
        deleted_at: "1740000000000"
      }
    }
  });

  assert.equal(message.id, "9223372036854775807");
  assert.equal(message.room_no, "AB12CD3E");
  assert.equal(message.status, 2);
  assert.equal(message.body, "");
  assert.equal(message.deleted_at, "1740000000000");
});

test("coalesces unknown users into one bounded batch and caches results", async () => {
  const calls = [];
  const loader = new CoalescedUserLoader(async (ids) => {
    calls.push(ids);
    return { items: ids.map((id) => ({ id, nickname: `user-${id}` })) };
  }, { delay: 0 });

  const [first, second, duplicate] = await Promise.all([loader.load("7"), loader.load(42), loader.load("7")]);
  assert.deepEqual(calls, [["7", "42"]]);
  assert.equal(first.name, "user-7");
  assert.equal(second.id, "42");
  assert.equal(duplicate.id, "7");
  assert.equal((await loader.load(42)).name, "user-42");
  assert.equal(calls.length, 1);
});
