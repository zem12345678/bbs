import assert from "node:assert/strict";
import test from "node:test";

import {
  base64URLToUint8Array,
  bestEffortRemoveWebPushSubscription,
  ensureWebPushSubscription,
  existingWebPushSubscription,
  registerWebPushServiceWorker,
  requestWebPushPermission,
  serializeWebPushSubscription,
  subscribeWebPush,
  unsubscribeWebPush,
  webPushSupported
} from "./webPush.js";

test("detects complete browser push support", () => {
  const supported = {
    Notification: { requestPermission() {} },
    PushManager: class PushManager {},
    navigator: { serviceWorker: { register() {}, getRegistration() {} } }
  };
  assert.equal(webPushSupported(supported), true);
  assert.equal(webPushSupported({ ...supported, Notification: undefined }), false);
  assert.equal(webPushSupported({ ...supported, PushManager: undefined }), false);
  assert.equal(webPushSupported({ ...supported, navigator: {} }), false);
});

test("converts an unpadded base64url application server key", () => {
  assert.deepEqual([...base64URLToUint8Array("-vv8_Q")], [250, 251, 252, 253]);
  assert.deepEqual([...base64URLToUint8Array("")], []);
});

test("checks an existing subscription without registering or requesting permission", async () => {
  const subscription = { endpoint: "https://push.example.test/existing" };
  let registerCalls = 0;
  let permissionCalls = 0;
  const serviceWorker = {
    async getRegistration(scope) {
      assert.equal(scope, "/");
      return { pushManager: { async getSubscription() { return subscription; } } };
    },
    async register() {
      registerCalls += 1;
    }
  };

  assert.equal(await existingWebPushSubscription(serviceWorker), subscription);
  assert.equal(registerCalls, 0);
  assert.equal(permissionCalls, 0);

  await requestWebPushPermission({ async requestPermission() { permissionCalls += 1; return "granted"; } });
  assert.equal(permissionCalls, 1);
});

test("registers the fixed worker path and subscribes with the VAPID key", async () => {
  const calls = [];
  const registration = {
    pushManager: {
      async subscribe(options) {
        calls.push(options);
        return { endpoint: "https://push.example.test/new" };
      }
    }
  };
  const serviceWorker = {
    async register(path, options) {
      calls.push({ path, options });
      return registration;
    }
  };

  assert.equal(await registerWebPushServiceWorker(serviceWorker), registration);
  await subscribeWebPush(registration, "AQID");
  assert.deepEqual(calls[0], { path: "/sw.js", options: { scope: "/" } });
  assert.equal(calls[1].userVisibleOnly, true);
  assert.deepEqual([...calls[1].applicationServerKey], [1, 2, 3]);
});

test("keeps a subscription only when its application server key still matches", async () => {
  const matching = {
    endpoint: "https://push.example.test/matching",
    options: { applicationServerKey: Uint8Array.from([1, 2, 3]).buffer },
    async unsubscribe() {
      assert.fail("matching subscription must not be removed");
    }
  };
  let subscribeCalls = 0;
  const registration = {
    pushManager: {
      async getSubscription() { return matching; },
      async subscribe() { subscribeCalls += 1; }
    }
  };

  assert.equal(await ensureWebPushSubscription(registration, "AQID"), matching);
  assert.equal(subscribeCalls, 0);
});

test("replaces a subscription after the application server key changes", async () => {
  const calls = [];
  const replacement = { endpoint: "https://push.example.test/replacement" };
  const existing = {
    endpoint: "https://push.example.test/old",
    options: { applicationServerKey: Uint8Array.from([9, 9, 9]) },
    async unsubscribe() {
      calls.push("unsubscribe");
      return true;
    }
  };
  const registration = {
    pushManager: {
      async getSubscription() { return existing; },
      async subscribe(options) {
        calls.push("subscribe");
        assert.deepEqual([...options.applicationServerKey], [1, 2, 3]);
        return replacement;
      }
    }
  };

  assert.equal(await ensureWebPushSubscription(registration, "AQID"), replacement);
  assert.deepEqual(calls, ["unsubscribe", "subscribe"]);
});

test("serializes and unsubscribes a browser push subscription", async () => {
  let unsubscribed = false;
  const subscription = {
    endpoint: "https://push.example.test/serialized",
    toJSON() {
      return { keys: { auth: "auth-key", p256dh: "public-key" } };
    },
    async unsubscribe() {
      unsubscribed = true;
      return true;
    }
  };

  assert.deepEqual(serializeWebPushSubscription(subscription), {
    endpoint: subscription.endpoint,
    auth: "auth-key",
    publickey: "public-key",
    sendReadMessage: false
  });
  assert.equal(await unsubscribeWebPush(subscription), true);
  assert.equal(unsubscribed, true);
});

test("removes the local subscription even when server unregister fails", async () => {
  let localUnsubscribed = false;
  const subscription = {
    endpoint: "https://push.example.test/logout",
    async unsubscribe() {
      localUnsubscribed = true;
      return true;
    }
  };
  const serviceWorker = {
    async getRegistration() {
      return { pushManager: { async getSubscription() { return subscription; } } };
    }
  };
  const unregisterCalls = [];

  assert.equal(await bestEffortRemoveWebPushSubscription("old-token", async (...args) => {
    unregisterCalls.push(args);
    throw new Error("server unavailable");
  }, serviceWorker), true);
  assert.equal(localUnsubscribed, true);
  assert.deepEqual(unregisterCalls, [[subscription.endpoint, "old-token"]]);
});
