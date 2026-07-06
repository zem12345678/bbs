export const NOTIFICATIONS_CHANGED_EVENT = "bbs:notifications-changed";

export function emitNotificationsChanged() {
  window.dispatchEvent(new Event(NOTIFICATIONS_CHANGED_EVENT));
}
