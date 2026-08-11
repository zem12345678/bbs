import { bbsApi } from "../api.js";

const emojiTokenPattern = /:([\p{L}\p{N}\p{M}_+\-]{1,128}):/gu;
const highlightTagPattern = /(<mark>|<\/mark>)/gi;

export const PUBLIC_EMOJI_CACHE_TTL_MS = 60_000;

let cachedPublicEmojis = null;
let cachedPublicEmojisAt = 0;
let publicEmojiRequest = null;

export function normalizePublicEmojisResponse(data) {
  const items = Array.isArray(data?.emojis)
    ? data.emojis
    : Array.isArray(data?.items)
      ? data.items
      : Array.isArray(data)
        ? data
        : [];
  return items.map(normalizePublicEmoji).filter(Boolean);
}

export function buildEmojiLookup(emojis = []) {
  const lookup = new Map();
  for (const emoji of emojis) {
    if (!emoji?.name || !emoji?.url) continue;
    lookup.set(emoji.name.toLocaleLowerCase(), emoji);
  }
  for (const emoji of emojis) {
    for (const alias of emoji?.aliases || []) {
      const key = alias.toLocaleLowerCase();
      if (!lookup.has(key)) lookup.set(key, emoji);
    }
  }
  return lookup;
}

export function emojiTextParts(text, emojis = []) {
  const value = String(text ?? "");
  const lookup = emojis instanceof Map ? emojis : buildEmojiLookup(emojis);
  const parts = [];
  let offset = 0;

  for (const match of value.matchAll(emojiTokenPattern)) {
    const emoji = lookup.get(match[1].toLocaleLowerCase());
    if (!emoji) continue;
    appendTextPart(parts, value.slice(offset, match.index));
    parts.push({ type: "emoji", value: match[0], name: match[1], emoji });
    offset = match.index + match[0].length;
  }
  appendTextPart(parts, value.slice(offset));
  return parts;
}

export function normalizeEmojiHighlightMarkup(text) {
  const source = String(text ?? "");
  const chunks = source.split(highlightTagPattern);
  const highlighted = [];
  let active = false;
  let value = "";

  for (const chunk of chunks) {
    const tag = chunk.toLocaleLowerCase();
    if (tag === "<mark>") {
      active = true;
      continue;
    }
    if (tag === "</mark>") {
      active = false;
      continue;
    }
    value += chunk;
    highlighted.push(...new Array(chunk.length).fill(active));
  }

  for (const match of value.matchAll(emojiTokenPattern)) {
    const start = match.index;
    const end = start + match[0].length;
    if (highlighted.slice(start, end).some(Boolean)) {
      highlighted.fill(true, start, end);
    }
  }

  let normalized = "";
  let markOpen = false;
  for (let index = 0; index < value.length; index += 1) {
    if (highlighted[index] && !markOpen) {
      normalized += "<mark>";
      markOpen = true;
    } else if (!highlighted[index] && markOpen) {
      normalized += "</mark>";
      markOpen = false;
    }
    normalized += value[index];
  }
  if (markOpen) normalized += "</mark>";
  return normalized;
}

export function insertEmojiToken(text, name, selectionStart, selectionEnd = selectionStart, maxLength) {
  const value = String(text ?? "");
  const start = clampSelection(selectionStart, value.length);
  const end = Math.max(start, clampSelection(selectionEnd, value.length));
  const token = `:${String(name || "").trim()}:`;
  const nextValue = `${value.slice(0, start)}${token}${value.slice(end)}`;
  const limit = Number(maxLength);
  if (maxLength !== undefined && Number.isFinite(limit) && limit >= 0 && nextValue.length > Math.trunc(limit)) {
    return null;
  }
  return {
    value: nextValue,
    selection: start + token.length
  };
}

export function captureEmojiSelection(input) {
  const start = Number.isFinite(input?.selectionStart) ? input.selectionStart : undefined;
  const end = Number.isFinite(input?.selectionEnd) ? input.selectionEnd : start;
  return { start, end };
}

export function getCachedPublicEmojis() {
  return cachedPublicEmojis || [];
}

export function loadPublicEmojis(loader = () => bbsApi.emojis()) {
  if (cachedPublicEmojis && Date.now() - cachedPublicEmojisAt < PUBLIC_EMOJI_CACHE_TTL_MS) {
    return Promise.resolve(cachedPublicEmojis);
  }
  if (!publicEmojiRequest) {
    const request = Promise.resolve()
      .then(loader)
      .then((data) => {
        cachedPublicEmojis = normalizePublicEmojisResponse(data);
        cachedPublicEmojisAt = Date.now();
        return cachedPublicEmojis;
      })
      .finally(() => {
        if (publicEmojiRequest === request) publicEmojiRequest = null;
      });
    publicEmojiRequest = request;
  }
  return publicEmojiRequest;
}

export function clearPublicEmojiCache() {
  cachedPublicEmojis = null;
  cachedPublicEmojisAt = 0;
  publicEmojiRequest = null;
}

function normalizePublicEmoji(item) {
  if (!item || typeof item !== "object") return null;
  const name = String(item.name || "").trim();
  const url = safeEmojiURL(item.url ?? item.publicUrl ?? item.public_url);
  if (!name || !url) return null;
  const aliases = Array.isArray(item.aliases)
    ? [...new Set(item.aliases.map((alias) => String(alias || "").trim()).filter(Boolean))]
    : [];
  return {
    ...item,
    name,
    url,
    aliases,
    category: String(item.category || "").trim()
  };
}

function safeEmojiURL(value) {
  const url = String(value || "").trim();
  if (!url || url.includes("\\")) return "";
  if (url.startsWith("/") && !url.startsWith("//")) return url;
  try {
    const parsed = new URL(url);
    if ((parsed.protocol === "http:" || parsed.protocol === "https:") && !parsed.username && !parsed.password) {
      return parsed.href;
    }
  } catch {
    return "";
  }
  return "";
}

function appendTextPart(parts, value) {
  if (!value) return;
  const previous = parts[parts.length - 1];
  if (previous?.type === "text") {
    previous.value += value;
  } else {
    parts.push({ type: "text", value });
  }
}

function clampSelection(value, length) {
  const number = Number(value);
  if (!Number.isFinite(number)) return length;
  return Math.min(length, Math.max(0, Math.trunc(number)));
}
