import React from "react";
import { AlertTriangle, CheckCircle2, Info, Megaphone, X } from "lucide-react";
import { bbsApi } from "../api";
import {
  announcementDismissalKey,
  normalizeAnnouncementsResponse,
  visibleAnnouncements
} from "../lib/announcements";

const DISMISSED_STORAGE_KEY = "bbs:announcements:dismissed:v2";
const EMPTY_SET = new Set();

export default function SiteAnnouncements({ auth }) {
  const accessToken = String(auth?.accessToken || "");
  const dismissalScope = announcementDismissalScope(auth);
  const [announcements, setAnnouncements] = React.useState([]);
  const [dismissed, setDismissed] = React.useState(() => readDismissedAnnouncements(dismissalScope));
  const [dismissedScope, setDismissedScope] = React.useState(dismissalScope);
  const [pendingReads, setPendingReads] = React.useState(EMPTY_SET);
  const dismissedRef = React.useRef(dismissed);
  const scopeRef = React.useRef(dismissalScope);
  const pendingReadsRef = React.useRef(new Set());

  React.useEffect(() => {
    scopeRef.current = dismissalScope;
    const nextDismissed = readDismissedAnnouncements(dismissalScope);
    dismissedRef.current = nextDismissed;
    setDismissed(nextDismissed);
    setDismissedScope(dismissalScope);
    pendingReadsRef.current = new Set();
    setPendingReads(EMPTY_SET);
    setAnnouncements([]);
    let alive = true;
    bbsApi
      .announcements({ limit: 20 }, accessToken)
      .then((data) => {
        if (alive) setAnnouncements(normalizeAnnouncementsResponse(data));
      })
      .catch(() => {
        if (alive) setAnnouncements([]);
      });
    return () => {
      alive = false;
    };
  }, [accessToken, dismissalScope]);

  const activeDismissed = dismissedScope === dismissalScope ? dismissed : EMPTY_SET;
  const visible = visibleAnnouncements(announcements).filter(
    (item) =>
      !item.isRead &&
      !activeDismissed.has(announcementDismissalKey(item)) &&
      item.display !== "normal"
  );
  const banner = visible.find((item) => item.display === "banner");
  const dialog = visible.find((item) => item.display === "dialog");
  if (!banner && !dialog) return null;

  const updatePendingRead = (announcementId, pending) => {
    const next = new Set(pendingReadsRef.current);
    if (pending) next.add(announcementId);
    else next.delete(announcementId);
    pendingReadsRef.current = next;
    setPendingReads(next);
  };

  const rememberDismissal = (key, scope) => {
    if (scopeRef.current !== scope) return;
    const next = new Set(dismissedRef.current);
    next.add(key);
    dismissedRef.current = next;
    setDismissed(next);
    persistDismissedAnnouncements(next, scope);
  };

  const dismiss = async (announcement, confirmed = false) => {
    if (
      announcement.needConfirmationToRead &&
      !confirmed &&
      !window.confirm(`确认已阅读「${announcement.title}」？`)
    ) {
      return;
    }
    const requestScope = dismissalScope;
    if (accessToken) {
      if (pendingReadsRef.current.has(announcement.id)) return;
      updatePendingRead(announcement.id, true);
      try {
        await bbsApi.readAnnouncement(announcement.id, accessToken);
      } catch {
        if (scopeRef.current === requestScope) updatePendingRead(announcement.id, false);
        return;
      }
      if (scopeRef.current === requestScope) updatePendingRead(announcement.id, false);
    }
    const key = announcementDismissalKey(announcement);
    if (key) rememberDismissal(key, requestScope);
  };

  return (
    <>
      {banner && <AnnouncementBanner announcement={banner} count={visible.length} busy={pendingReads.has(banner.id)} onDismiss={dismiss} />}
      {dialog && <AnnouncementDialog announcement={dialog} busy={pendingReads.has(dialog.id)} onDismiss={dismiss} />}
    </>
  );
}

function AnnouncementBanner({ announcement, count, busy, onDismiss }) {
  const Icon = iconForAnnouncement(announcement.icon);
  return (
    <section className="site-announcements" aria-label="社区公告">
      <article className="site-announcement site-announcement--banner" role="status" aria-live="polite">
        <AnnouncementConfetti active={announcement.confetti} />
        <div className="site-announcement__icon" aria-hidden="true">
          <Icon size={18} />
        </div>
        <div className="site-announcement__body">
          <strong>{announcement.title}</strong>
          <p>{announcement.text}</p>
          {count > 1 && <small>还有 {count - 1} 条公告</small>}
        </div>
        {announcement.imageUrl && <img src={announcement.imageUrl} alt="" className="site-announcement__image" />}
        <button className="site-announcement__dismiss" type="button" aria-label="关闭公告" disabled={busy} onClick={() => onDismiss(announcement)}>
          <X size={17} aria-hidden="true" />
        </button>
      </article>
    </section>
  );
}

function AnnouncementDialog({ announcement, busy, onDismiss }) {
  const Icon = iconForAnnouncement(announcement.icon);
  const actionRef = React.useRef(null);

  React.useEffect(() => {
    actionRef.current?.focus();
  }, [announcement.id]);

  React.useEffect(() => {
    function handleKeyDown(event) {
      if (event.key === "Escape" && !busy && !announcement.needConfirmationToRead) {
        onDismiss(announcement);
      }
    }
    document.addEventListener("keydown", handleKeyDown);
    return () => document.removeEventListener("keydown", handleKeyDown);
  }, [announcement, busy, onDismiss]);

  return (
    <div className="announcement-dialog-backdrop" role="presentation">
      <section className="announcement-dialog" role="dialog" aria-modal="true" aria-labelledby={`announcement-${announcement.id}-title`} aria-describedby={`announcement-${announcement.id}-description`}>
        <AnnouncementConfetti active={announcement.confetti} />
        <div className="announcement-dialog__header">
          <span className="site-announcement__icon" aria-hidden="true"><Icon size={20} /></span>
          <h2 id={`announcement-${announcement.id}-title`}>{announcement.title}</h2>
        </div>
        {announcement.imageUrl && <img src={announcement.imageUrl} alt="" className="announcement-dialog__image" />}
        <p id={`announcement-${announcement.id}-description`}>{announcement.text}</p>
        <div className="announcement-dialog__actions">
          <button ref={actionRef} type="button" className="primary-button" disabled={busy} onClick={() => onDismiss(announcement, true)}>
            {announcement.needConfirmationToRead ? "确认已读" : "关闭"}
          </button>
        </div>
      </section>
    </div>
  );
}

function AnnouncementConfetti({ active }) {
  if (!active) return null;
  return <div className="announcement-confetti" aria-hidden="true">{Array.from({ length: 12 }, (_, index) => <span key={index} />)}</div>;
}

function iconForAnnouncement(icon) {
  if (icon === "warning" || icon === "error") return AlertTriangle;
  if (icon === "success") return CheckCircle2;
  return icon === "info" ? Info : Megaphone;
}

function announcementDismissalScope(auth) {
  const userId = auth?.user?.id;
  if (userId !== undefined && userId !== null && String(userId).trim()) return `user:${String(userId).trim()}`;
  return auth?.accessToken ? "authenticated" : "guest";
}

function readDismissedAnnouncements(scope) {
  try {
    const storage = typeof window !== "undefined" ? window.localStorage : null;
    if (!storage) return new Set();
    const value = JSON.parse(storage.getItem(`${DISMISSED_STORAGE_KEY}:${scope}`) || "[]");
    return new Set(Array.isArray(value) ? value.filter((item) => typeof item === "string") : []);
  } catch {
    return new Set();
  }
}

function persistDismissedAnnouncements(value, scope) {
  try {
    const storage = typeof window !== "undefined" ? window.localStorage : null;
    if (!storage) return;
    storage.setItem(`${DISMISSED_STORAGE_KEY}:${scope}`, JSON.stringify(Array.from(value).slice(-100)));
  } catch {
    // Local storage can be disabled without affecting announcement delivery.
  }
}
