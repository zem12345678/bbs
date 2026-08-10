export function webPushSupported(scope = globalThis) {
  return Boolean(
    scope?.Notification
    && scope?.PushManager
    && scope?.navigator?.serviceWorker?.register
    && scope?.navigator?.serviceWorker?.getRegistration
  );
}

export function base64URLToUint8Array(value) {
  const text = String(value || "").trim();
  if (!text) return new Uint8Array();
  const base64 = text.replace(/-/g, "+").replace(/_/g, "/").padEnd(Math.ceil(text.length / 4) * 4, "=");
  const binary = globalThis.atob(base64);
  return Uint8Array.from(binary, (character) => character.charCodeAt(0));
}

export async function existingWebPushSubscription(serviceWorker = globalThis?.navigator?.serviceWorker) {
  if (!serviceWorker?.getRegistration) return null;
  const registration = await serviceWorker.getRegistration("/");
  if (!registration?.pushManager?.getSubscription) return null;
  return registration.pushManager.getSubscription();
}

export async function registerWebPushServiceWorker(serviceWorker = globalThis?.navigator?.serviceWorker) {
  if (!serviceWorker?.register) throw new Error("当前浏览器不支持浏览器推送。");
  return serviceWorker.register("/sw.js", { scope: "/" });
}

export async function requestWebPushPermission(notificationAPI = globalThis?.Notification) {
  if (!notificationAPI?.requestPermission) throw new Error("当前浏览器不支持通知权限。");
  return notificationAPI.requestPermission();
}

export async function subscribeWebPush(registration, publicKey) {
  if (!registration?.pushManager?.subscribe) throw new Error("浏览器推送服务不可用。");
  const applicationServerKey = base64URLToUint8Array(publicKey);
  if (applicationServerKey.length === 0) throw new Error("服务器推送公钥无效。");
  return registration.pushManager.subscribe({ userVisibleOnly: true, applicationServerKey });
}

export async function ensureWebPushSubscription(registration, publicKey) {
  if (!registration?.pushManager?.getSubscription) throw new Error("浏览器推送服务不可用。");
  const applicationServerKey = base64URLToUint8Array(publicKey);
  if (applicationServerKey.length === 0) throw new Error("服务器推送公钥无效。");

  const existing = await registration.pushManager.getSubscription();
  if (existing && subscriptionUsesApplicationServerKey(existing, applicationServerKey)) return existing;
  if (existing) await unsubscribeWebPush(existing);
  return subscribeWebPush(registration, publicKey);
}

export function serializeWebPushSubscription(subscription) {
  if (!subscription?.endpoint) throw new Error("浏览器推送订阅无效。");
  const json = typeof subscription.toJSON === "function" ? subscription.toJSON() : {};
  const auth = json?.keys?.auth || keyToBase64URL(subscription.getKey?.("auth"));
  const publickey = json?.keys?.p256dh || keyToBase64URL(subscription.getKey?.("p256dh"));
  if (!auth || !publickey) throw new Error("浏览器推送订阅密钥无效。");
  return {
    endpoint: String(subscription.endpoint),
    auth,
    publickey,
    sendReadMessage: false
  };
}

export async function unsubscribeWebPush(subscription) {
  if (!subscription?.unsubscribe) return false;
  return subscription.unsubscribe();
}

export async function bestEffortRemoveWebPushSubscription(accessToken, unregister, serviceWorker = globalThis?.navigator?.serviceWorker) {
  let subscription;
  try {
    subscription = await existingWebPushSubscription(serviceWorker);
  } catch {
    return false;
  }
  if (!subscription) return false;

  const tasks = [Promise.resolve().then(() => unsubscribeWebPush(subscription))];
  if (accessToken && typeof unregister === "function") {
    tasks.push(Promise.resolve().then(() => unregister(subscription.endpoint, accessToken)));
  }
  await Promise.allSettled(tasks);
  return true;
}

function subscriptionUsesApplicationServerKey(subscription, expected) {
  const value = subscription?.options?.applicationServerKey;
  if (!value) return false;
  const actual = ArrayBuffer.isView(value)
    ? new Uint8Array(value.buffer, value.byteOffset, value.byteLength)
    : new Uint8Array(value);
  return actual.length === expected.length && actual.every((byte, index) => byte === expected[index]);
}

function keyToBase64URL(value) {
  if (!value) return "";
  const bytes = value instanceof Uint8Array ? value : new Uint8Array(value);
  let binary = "";
  for (const byte of bytes) binary += String.fromCharCode(byte);
  return globalThis.btoa(binary).replace(/\+/g, "-").replace(/\//g, "_").replace(/=+$/g, "");
}
