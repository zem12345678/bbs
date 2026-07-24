import assert from "node:assert/strict";
import { test } from "node:test";

import { ChatRealtimeClient } from "./chatRealtime.js";

test("authenticates, subscribes, sends commands, and replaces room subscriptions", async () => {
  const states = [];
  const events = [];
  const client = new ChatRealtimeClient({
    issueTicket: async () => ({ ticket: "ticket-1" }),
    websocketUrl: (ticket) => `ws://example.test/chat?ticket=${ticket}`,
    WebSocketImpl: FakeWebSocket,
    onEvent: (event) => events.push(event),
    onState: (state) => states.push(state),
    delays: [1]
  });

  client.start(["ab12cd3e", "AB12CD3E"]);
  await tick();
  const socket = FakeWebSocket.instances.at(-1);
  assert.equal(socket.url, "ws://example.test/chat?ticket=ticket-1");
  socket.open();
  assert.equal(client.send("message.send", {}), false);
  socket.receive({ type: "session.ready", payload: { user_id: "42" } });
  assert.equal(states.at(-1), "connected");
  assert.deepEqual(JSON.parse(socket.sent[0]).payload.room_numbers, ["AB12CD3E"]);

  client.setRooms(["YZ83T019"]);
  assert.deepEqual(JSON.parse(socket.sent[1]).payload.room_numbers, ["YZ83T019"]);
  assert.equal(client.send("read.advance", { room_no: "YZ83T019", read_seq: "4" }, "req-1"), true);
  assert.equal(JSON.parse(socket.sent[2]).request_id, "req-1");
  assert.equal(events[0].type, "session.ready");
  client.stop();
  assert.equal(states.at(-1), "disconnected");
});

test("requests a fresh ticket after a websocket close", async () => {
  let tickets = 0;
  const client = new ChatRealtimeClient({
    issueTicket: async () => ({ ticket: `ticket-${++tickets}` }),
    websocketUrl: (ticket) => `ws://example.test/${ticket}`,
    WebSocketImpl: FakeWebSocket,
    delays: [1]
  });

  client.start([]);
  await tick();
  const first = FakeWebSocket.instances.at(-1);
  first.open();
  first.receive({ type: "session.ready", payload: {} });
  first.serverClose();
  await tick(5);
  assert.equal(tickets, 2);
  assert.equal(FakeWebSocket.instances.at(-1).url, "ws://example.test/ticket-2");
  client.stop();
});

test("can stop silently during component cleanup", async () => {
  const states = [];
  const client = new ChatRealtimeClient({
    issueTicket: async () => ({ ticket: "ticket-1" }),
    websocketUrl: (ticket) => `ws://example.test/${ticket}`,
    WebSocketImpl: FakeWebSocket,
    onState: (state) => states.push(state)
  });

  client.start([]);
  await tick();
  client.stop({ notify: false });

  assert.notEqual(states.at(-1), "disconnected");
});

class FakeWebSocket {
  static instances = [];

  constructor(url) {
    this.url = url;
    this.readyState = 0;
    this.sent = [];
    FakeWebSocket.instances.push(this);
  }

  open() {
    this.readyState = 1;
    this.onopen?.();
  }

  receive(value) {
    this.onmessage?.({ data: JSON.stringify(value) });
  }

  send(value) {
    this.sent.push(value);
  }

  serverClose() {
    this.readyState = 3;
    this.onclose?.();
  }

  close() {
    this.serverClose();
  }
}

function tick(delay = 0) {
  return new Promise((resolve) => setTimeout(resolve, delay));
}
