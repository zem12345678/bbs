const PAID_ATTACHMENT_ACQUISITIONS_STORAGE_KEY = "bbs.community.paid-attachment-acquisitions.v1";

export function attachmentPriceCredits(attachment) {
  const price = Number(attachment?.price_credits ?? attachment?.priceCredits);
  return Number.isFinite(price) && price > 0 ? price : 0;
}

export function needsPaidAttachmentConfirmation(attachment, canManage, acquired) {
  return attachmentPriceCredits(attachment) > 0 && !canManage && !acquired;
}

export function markPaidAttachmentAcquired(acquiredAttachmentIDs, attachment, canManage) {
  const current = acquiredAttachmentIDs || {};
  const attachmentId = normalizePersistentID(attachment?.id);
  if (!attachmentId || attachmentPriceCredits(attachment) <= 0 || canManage || current[attachmentId]) {
    return current;
  }
  return { ...current, [attachmentId]: true };
}

// The download history is the authority for paid-attachment access. A local
// browser hint can improve a just-completed interaction, but it must never be
// used to decide whether a member has already acquired an attachment.
export function authorizedAttachmentIDsFromDownloads(downloads) {
  const acquired = {};
  const items = Array.isArray(downloads) ? downloads : [];
  for (const download of items) {
    if (String(download?.status || "").trim().toUpperCase() !== "AUTHORIZED") {
      continue;
    }
    const attachmentId = normalizePersistentID(download?.attachment?.id);
    if (attachmentId) {
      acquired[attachmentId] = true;
    }
  }
  return acquired;
}

export function readPaidAttachmentAcquisitionIDs(userId, storage) {
  const key = paidAttachmentAcquisitionsStorageKey(userId);
  const target = resolveStorage(storage);
  if (!key || !target) return {};
  try {
    const raw = target.getItem(key);
    return normalizeAcquiredAttachmentIDs(raw ? JSON.parse(raw) : {});
  } catch {
    return {};
  }
}

export function persistPaidAttachmentAcquisitionIDs(userId, acquiredAttachmentIDs, storage) {
  const key = paidAttachmentAcquisitionsStorageKey(userId);
  const target = resolveStorage(storage);
  if (!key || !target) return;
  try {
    target.setItem(key, JSON.stringify(normalizeAcquiredAttachmentIDs(acquiredAttachmentIDs)));
  } catch {
    // This is only a convenience hint; download authorization stays server-side.
  }
}

export function paidAttachmentAcquisitionsStorageKey(userId) {
  const normalizedUserId = normalizePersistentID(userId);
  return normalizedUserId ? `${PAID_ATTACHMENT_ACQUISITIONS_STORAGE_KEY}:${normalizedUserId}` : "";
}

function normalizeAcquiredAttachmentIDs(value) {
  if (!value || typeof value !== "object" || Array.isArray(value)) return {};
  return Object.fromEntries(
    Object.entries(value)
      .map(([attachmentId, acquired]) => [normalizePersistentID(attachmentId), acquired])
      .filter(([attachmentId, acquired]) => attachmentId && acquired === true)
  );
}

function normalizePersistentID(value) {
  if (typeof value === "string") {
    return /^[1-9]\d*$/.test(value) ? value : "";
  }
  if (typeof value === "number" && Number.isSafeInteger(value) && value > 0) {
    return String(value);
  }
  return "";
}

function resolveStorage(storage) {
  if (storage) return storage;
  try {
    return globalThis.localStorage || null;
  } catch {
    return null;
  }
}
