self.addEventListener("push", (event) => {
  let payload = {};
  try {
    payload = event.data ? event.data.json() : {};
  } catch {
    payload = {};
  }
  const notification = payload?.notification || payload || {};
  const title = String(notification.title || "新消息");
  const options = {
    body: String(notification.body || "你有一条新消息"),
    icon: notification.icon,
    badge: notification.badge,
    tag: notification.tag,
    data: { destination: "/dashboard/messages" }
  };
  Object.keys(options).forEach((key) => options[key] === undefined && delete options[key]);
  event.waitUntil(self.registration.showNotification(title, options));
});

self.addEventListener("notificationclick", (event) => {
  event.notification.close();
  event.waitUntil(openMessages());
});

async function openMessages() {
  const destination = new URL("/dashboard/messages", self.location.origin);
  const windows = await self.clients.matchAll({ type: "window", includeUncontrolled: true });
  const exact = windows.find((client) => {
    const url = new URL(client.url);
    return url.origin === destination.origin && url.pathname === destination.pathname;
  });
  if (exact) return exact.focus();

  const sameOrigin = windows.find((client) => new URL(client.url).origin === destination.origin);
  if (sameOrigin) {
    await sameOrigin.navigate(destination.href);
    return sameOrigin.focus();
  }
  return self.clients.openWindow(destination.href);
}
