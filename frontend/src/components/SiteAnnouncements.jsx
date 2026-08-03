import React from "react";
import { AlertTriangle, CheckCircle2, Info, Megaphone, X } from "lucide-react";
import { bbsApi } from "../api";
import {
  announcementDismissalKey,
  normalizeAnnouncementsResponse,
  visibleAnnouncements
} from "../lib/announcements";

const DISMISSED_STORAGE_KEY = "bbs:announcements:dismissed";

export default function SiteAnnouncements() {
  const [announcements, setAnnouncements] = React.useState([]);
  const [dismissed, setDismissed] = React.useState(readDismissedAnnouncements);

  React.useEffect(() => {
    let alive = true;
    bbsApi
      .announcements({ limit: 20 })
      .then((data) => {
        if (alive) setAnnouncements(normalizeAnnouncementsResponse(data));
      })
      .catch(() => {
        if (alive) setAnnouncements([]);
      });
    return () => {
      alive = false;
    };
  }, []);

  const visible = visibleAnnouncements(announcements).filter((item) => !dismissed.has(announcementDismissalKey(item)));
  if (visible.length === 0) return null;

  const announcement = visible[0];
  const Icon = iconForAnnouncement(announcement.icon);
  const dismiss = () => {
    const key = announcementDismissalKey(announcement);
    if (!key) return;
    const next = new Set(dismissed);
    next.add(key);
    setDismissed(next);
    persistDismissedAnnouncements(next);
  };

  return (
    <section className="site-announcements" aria-label="社区公告">
      <article className={`site-announcement site-announcement--${announcement.display}`} role="status" aria-live="polite">
        <div className="site-announcement__icon" aria-hidden="true">
          <Icon size={18} />
        </div>
        <div className="site-announcement__body">
          <strong>{announcement.title}</strong>
          <p>{announcement.text}</p>
          {visible.length > 1 && <small>还有 {visible.length - 1} 条公告</small>}
        </div>
        {announcement.imageUrl && <img src={announcement.imageUrl} alt="" className="site-announcement__image" />}
        <button className="site-announcement__dismiss" type="button" aria-label="关闭公告" onClick={dismiss}>
          <X size={17} aria-hidden="true" />
        </button>
      </article>
    </section>
  );
}

function iconForAnnouncement(icon) {
  if (icon === "warning" || icon === "error") return AlertTriangle;
  if (icon === "success") return CheckCircle2;
  return icon === "info" ? Info : Megaphone;
}

function readDismissedAnnouncements() {
  try {
    const storage = typeof window !== "undefined" ? window.localStorage : null;
    if (!storage) return new Set();
    const value = JSON.parse(storage.getItem(DISMISSED_STORAGE_KEY) || "[]");
    return new Set(Array.isArray(value) ? value.filter((item) => typeof item === "string") : []);
  } catch {
    return new Set();
  }
}

function persistDismissedAnnouncements(value) {
  try {
    const storage = typeof window !== "undefined" ? window.localStorage : null;
    if (!storage) return;
    storage.setItem(DISMISSED_STORAGE_KEY, JSON.stringify(Array.from(value).slice(-100)));
  } catch {
    // Local storage can be disabled without affecting announcement delivery.
  }
}
