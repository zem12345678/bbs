export function configuredAPIBase(runtimeConfig = globalThis.__BBS_CONFIG__, env = import.meta.env) {
	const runtimeConfigured = normalizeAPIBase(runtimeConfig?.apiBaseUrl) || normalizeAPIBase(runtimeConfig?.api_base_url);
	const buildConfigured = normalizeAPIBase(env?.VITE_API_BASE_URL) || normalizeAPIBase(env?.VITE_API_BASE);
	return (env?.DEV ? buildConfigured || runtimeConfigured : runtimeConfigured || buildConfigured) || "/api/v1";
}

function normalizeAPIBase(value) {
	return String(value || "").trim().replace(/\/+$/, "");
}

const API_BASE = configuredAPIBase();

export function chatWebSocketUrl(ticket) {
	return chatWebSocketUrlForBase(API_BASE, ticket);
}

// A production build can intentionally use a same-origin relative API base
// (/api/v1). WebSocket requires an absolute ws(s) URL, so resolve it against
// the page origin before converting HTTP(S) to WS(S).
export function chatWebSocketUrlForBase(apiBase, ticket, pageOrigin = browserOrigin()) {
	const base = new URL(String(apiBase || ""), pageOrigin || "http://localhost");
	if (base.protocol === "https:") {
		base.protocol = "wss:";
	} else if (base.protocol === "http:") {
		base.protocol = "ws:";
	}
	return `${base.toString().replace(/\/$/, "")}/chat/ws?ticket=${encodeURIComponent(String(ticket || ""))}`;
}

function apiURL(path) {
	const value = `${API_BASE}${path}`;
	if (/^https?:\/\//i.test(value)) return value;
	const origin = browserOrigin();
	return origin ? new URL(value, origin).toString() : value;
}

function browserOrigin() {
	if (typeof window !== "undefined" && window.location?.origin) return window.location.origin;
	if (globalThis.location?.origin) return globalThis.location.origin;
	if (globalThis.__BBS_API_ORIGIN__) return String(globalThis.__BBS_API_ORIGIN__);
	return "";
}

export class ApiError extends Error {
  constructor(message, { code, data, httpCode, meta, reason, requestId, responseStatus, retryAfterSeconds, service, traceId, rawBody } = {}) {
    super(message);
    const parsedRetryAfter = Number(retryAfterSeconds);
    this.name = "ApiError";
    this.code = code;
    this.data = data;
    this.httpCode = httpCode;
    this.meta = meta || {};
    this.reason = reason || "";
    this.requestId = requestId || "";
    this.service = service || "";
    this.traceId = traceId || "";
    this.rawBody = rawBody || "";
    this.status = responseStatus || httpCode || 0;
    this.retryAfterSeconds = Number.isFinite(parsedRetryAfter) && parsedRetryAfter > 0 ? parsedRetryAfter : 0;
  }
}

export const AUTH_INVALIDATED_EVENT = "bbs:auth-invalidated";

export function isUnauthorizedError(error) {
  return [error?.status, error?.httpCode].some((status) => Number(status) === 401);
}

function buildQuery(params = {}) {
  const query = new URLSearchParams();
  Object.entries(params).forEach(([key, value]) => {
    if (value !== undefined && value !== null && value !== "") {
      query.set(key, value);
    }
  });
  const text = query.toString();
  return text ? `?${text}` : "";
}

async function request(path, { method = "GET", body, token } = {}) {
  const headers = {};
  const isFormData = typeof FormData !== "undefined" && body instanceof FormData;
  if (body !== undefined && !isFormData) {
    headers["Content-Type"] = "application/json";
  }
  if (token) {
    headers.Authorization = `Bearer ${token}`;
  }

  const response = await fetch(apiURL(path), {
    method,
    headers,
    body: body === undefined ? undefined : isFormData ? body : JSON.stringify(body)
  });

  const text = await response.text();
  const data = parseResponseBody(text);
  const isEnvelope = isApiEnvelope(data);

  if (!response.ok) {
    const error = buildApiError(data, response, text);
    notifyAuthInvalidated(error, token);
    throw error;
  }
  if (isEnvelope) {
    if (data.code !== 0) {
      const error = buildApiError(data, response, text);
      notifyAuthInvalidated(error, token);
      throw error;
    }
    return data.data;
  }
  return data ?? text;
}

async function downloadAttachment(path, token) {
  const headers = {};
  if (token) {
    headers.Authorization = `Bearer ${token}`;
  }

  const response = await fetch(apiURL(path), { headers });
  if (!response.ok) {
    const text = await response.text();
    const error = buildApiError(parseResponseBody(text), response, text);
    notifyAuthInvalidated(error, token);
    throw error;
  }
  return {
    blob: await response.blob(),
    filename: filenameFromContentDisposition(response.headers.get("Content-Disposition"))
  };
}

function notifyAuthInvalidated(error, token) {
  if (!token || !isUnauthorizedError(error) || typeof window === "undefined" || typeof window.dispatchEvent !== "function") {
    return;
  }
  const AuthInvalidatedEvent = window.CustomEvent;
  if (typeof AuthInvalidatedEvent !== "function") return;
  window.dispatchEvent(new AuthInvalidatedEvent(AUTH_INVALIDATED_EVENT, { detail: { accessToken: token } }));
}

function filenameFromContentDisposition(value) {
  const header = String(value || "");
  const encoded = /filename\*\s*=\s*(?:UTF-8'')?([^;]+)/i.exec(header)?.[1];
  if (encoded) {
    try {
      return decodeURIComponent(encoded.trim().replace(/^"|"$/g, ""));
    } catch {
      return encoded.trim().replace(/^"|"$/g, "");
    }
  }
  return /filename\s*=\s*"?([^";]+)"?/i.exec(header)?.[1]?.trim() || "";
}

function parseJsonPreservingLargeInts(text) {
  return JSON.parse(text.replace(/(:\s*)(-?\d{16,})(?=[,}\]])/g, '$1"$2"'));
}

function parseResponseBody(text) {
  if (!text) return null;
  try {
    return parseJsonPreservingLargeInts(text);
  } catch {
    return null;
  }
}

function isApiEnvelope(data) {
  return data && typeof data === "object" && "http_code" in data && "code" in data && "data" in data;
}

function buildApiError(data, response, rawBody) {
  const fallbackStatus = response?.status || data?.http_code || 0;
  const message = data?.message || data?.reason || data?.error?.message || data?.error || rawBody || `请求失败 (${fallbackStatus})`;
  return new ApiError(message, {
    code: data?.code,
    data: data?.data,
    httpCode: data?.http_code || fallbackStatus,
    meta: data?.meta,
    reason: data?.reason,
    requestId: data?.request_id || data?.meta?.request_id || response?.headers?.get?.("X-Request-ID") || response?.headers?.get?.("X-Request-Id"),
    responseStatus: response?.status,
    retryAfterSeconds: parseRetryAfterSeconds(response?.headers?.get?.("Retry-After")),
    service: data?.service,
    traceId: data?.trace_id || data?.meta?.trace_id || response?.headers?.get?.("X-Trace-ID") || response?.headers?.get?.("Traceparent"),
    rawBody
  });
}

export function parseRetryAfterSeconds(value, now = Date.now()) {
  const text = String(value || "").trim();
  if (!text) return 0;
  if (/^\d+$/.test(text)) return Number(text);
  const retryAt = Date.parse(text);
  if (!Number.isFinite(retryAt)) return 0;
  return Math.max(0, Math.ceil((retryAt - now) / 1000));
}

export const bbsApi = {
  authConfig() {
    return request("/auth/config");
  },
  siteConfig() {
    return request("/site-config");
  },
  meta(detail = false) {
    return request("/meta", { method: "POST", body: { detail: Boolean(detail) } });
  },
  announcements(params = {}, token) {
    return request(`/announcements${buildQuery({ limit: 10, ...params })}`, { token });
  },
  announcement(announcementId) {
    return request(`/announcements/${encodeURIComponent(announcementId)}`);
  },
  readAnnouncement(announcementId, token) {
    return request("/i/read-announcement", {
      method: "POST",
      body: { announcementId },
      token
    });
  },
  oauthStartUrl(provider, redirect) {
    return `${API_BASE}/auth/oauth/${encodeURIComponent(provider)}/start${buildQuery({ redirect })}`;
  },
  login(payload) {
    return request("/auth/login", { method: "POST", body: payload });
  },
  completeMfaLogin(payload) {
    return request("/auth/login/mfa", { method: "POST", body: payload });
  },
  beginPasskeyMfaLogin(payload) {
    return request("/auth/login/mfa/passkey/options", { method: "POST", body: payload });
  },
  completePasskeyMfaLogin(payload) {
    return request("/auth/login/mfa/passkey", { method: "POST", body: payload });
  },
  beginPasswordlessPasskeyLogin() {
    return request("/auth/passkeys/options", { method: "POST" });
  },
  completePasswordlessPasskeyLogin(payload) {
    return request("/auth/passkeys/login", { method: "POST", body: payload });
  },
  register(payload) {
    return request("/auth/register", { method: "POST", body: payload });
  },
  logout(token) {
    return request("/auth/logout", { method: "POST", token });
  },
  me(token) {
    return request("/users/me", { token });
  },
  currentPinnedContent(token) {
    return request("/users/me/pinned", { token });
  },
  userPinnedContent(userId, token) {
    return request(`/users/${encodeURIComponent(userId)}/pinned`, { token });
  },
  updateMe(payload, token) {
    return request("/users/me", { method: "PUT", body: payload, token });
  },
  changePassword(payload, token) {
    return request("/users/me/password", { method: "POST", body: payload, token });
  },
  accountLifecycle(token) {
    return request("/users/me/account-lifecycle", { token });
  },
  requestAccountDeletion(payload, token) {
    return request("/users/me/deletion-requests", { method: "POST", body: payload, token });
  },
  mfaStatus(token) {
    return request("/users/me/mfa", { token });
  },
  beginTotpEnrollment(payload, token) {
    return request("/users/me/mfa/totp/enrollment", { method: "POST", body: payload, token });
  },
  confirmTotpEnrollment(payload, token) {
    return request("/users/me/mfa/totp/confirm", { method: "POST", body: payload, token });
  },
  regenerateMfaRecoveryCodes(payload, token) {
    return request("/users/me/mfa/recovery-codes", { method: "POST", body: payload, token });
  },
  disableTotp(payload, token) {
    return request("/users/me/mfa/totp", { method: "DELETE", body: payload, token });
  },
  passkeys(token) {
    return request("/users/me/passkeys", { token });
  },
  beginPasskeyRegistration(payload, token) {
    return request("/users/me/passkeys/registration/options", { method: "POST", body: payload, token });
  },
  finishPasskeyRegistration(payload, token) {
    return request("/users/me/passkeys/registration/verify", { method: "POST", body: payload, token });
  },
  updatePasskey(credentialId, payload, token) {
    return request(`/users/me/passkeys/${encodeURIComponent(credentialId)}`, { method: "PUT", body: payload, token });
  },
  deletePasskey(credentialId, payload, token) {
    return request(`/users/me/passkeys/${encodeURIComponent(credentialId)}`, { method: "DELETE", body: payload, token });
  },
  setPasskeyPasswordless(payload, token) {
    return request("/users/me/passkeys/passwordless", { method: "PUT", body: payload, token });
  },
  userSessions(params, token) {
    return request(`/users/me/sessions${buildQuery({ limit: 20, ...params })}`, { token });
  },
  revokeUserSession(sessionId, token) {
    return request(`/users/me/sessions/${encodeURIComponent(sessionId)}`, { method: "DELETE", token });
  },
  userLoginEvents(params, token) {
    return request(`/users/me/login-events${buildQuery({ limit: 20, ...params })}`, { token });
  },
  listAPITokens(token) {
    return request("/users/me/api-tokens", { token });
  },
  createAPIToken(payload, token) {
    return request("/users/me/api-tokens", { method: "POST", body: payload, token });
  },
  revokeAPIToken(tokenId, token) {
    return request(`/users/me/api-tokens/${encodeURIComponent(tokenId)}`, { method: "DELETE", token });
  },
  listWebhooks(token) {
    return request("/users/me/webhooks", { token });
  },
  createWebhook(payload, token) {
    return request("/users/me/webhooks", { method: "POST", body: payload, token });
  },
  showWebhook(webhookId, token) {
    return request(`/users/me/webhooks/${encodeURIComponent(webhookId)}`, { token });
  },
  updateWebhook(webhookId, payload, token) {
    return request(`/users/me/webhooks/${encodeURIComponent(webhookId)}`, { method: "PUT", body: payload, token });
  },
  deleteWebhook(webhookId, token) {
    return request(`/users/me/webhooks/${encodeURIComponent(webhookId)}`, { method: "DELETE", token });
  },
  testWebhook(webhookId, type, token) {
    return request(`/users/me/webhooks/${encodeURIComponent(webhookId)}/test`, { method: "POST", body: { type }, token });
  },
  listAntennas(token) {
    return request("/users/me/antennas", { token });
  },
  exportAntennas(token) {
    return request("/i/export-antennas", { method: "POST", body: {}, token });
  },
  importAntennas(fileId, token) {
    return request("/i/import-antennas", { method: "POST", body: { fileId: String(fileId) }, token });
  },
  createAntenna(payload, token) {
    return request("/users/me/antennas", { method: "POST", body: payload, token });
  },
  updateAntenna(antennaId, payload, token) {
    return request(`/users/me/antennas/${encodeURIComponent(antennaId)}`, { method: "PUT", body: payload, token });
  },
  deleteAntenna(antennaId, token) {
    return request(`/users/me/antennas/${encodeURIComponent(antennaId)}`, { method: "DELETE", token });
  },
  antennaNotes(antennaId, params = {}, token) {
    return request(`/users/me/antennas/${encodeURIComponent(antennaId)}/notes${buildQuery({ limit: 20, ...params })}`, { token });
  },
  uploadAvatar(file, token) {
    const form = new FormData();
    form.append("file", file);
    return request("/users/me/avatar", { method: "POST", body: form, token });
  },
  uploadImage(file, token) {
    const form = new FormData();
    form.append("file", file);
    return request("/uploads/images", { method: "POST", body: form, token });
  },
  uploadFile(file, token, bizType = "files", folderId) {
    const form = new FormData();
    form.append("file", file);
    form.append("biz_type", bizType);
    if (folderId !== undefined && folderId !== null && folderId !== "") {
      form.append("folder_id", folderId);
    }
    return request("/files", { method: "POST", body: form, token });
  },
  listFiles(params = {}, token) {
    return request(`/files${buildQuery({ limit: 20, offset: 0, ...params })}`, { token });
  },
  getFileUsage(token) {
    return request("/files/usage", { token });
  },
  getFile(fileId, token) {
    return request(`/files/${encodeURIComponent(fileId)}`, { token });
  },
  downloadFile(fileId, token) {
    return downloadAttachment(`/files/${encodeURIComponent(fileId)}/download`, token);
  },
  deleteFile(fileId, token) {
    return request(`/files/${encodeURIComponent(fileId)}`, { method: "DELETE", token });
  },
  updateFile(fileId, payload, token) {
    return request(`/files/${encodeURIComponent(fileId)}`, { method: "PATCH", body: payload, token });
  },
  fileFolders(params = {}, token) {
    return request(`/file-folders${buildQuery({ limit: 100, offset: 0, ...params })}`, { token });
  },
  createFileFolder(payload, token) {
    return request("/file-folders", { method: "POST", body: payload, token });
  },
  updateFileFolder(folderId, payload, token) {
    return request(`/file-folders/${encodeURIComponent(folderId)}`, { method: "PUT", body: payload, token });
  },
  deleteFileFolder(folderId, token) {
    return request(`/file-folders/${encodeURIComponent(folderId)}`, { method: "DELETE", token });
  },
  requestPasswordReset(payload) {
    return request("/auth/password/forgot", { method: "POST", body: payload });
  },
  resetPassword(payload) {
    return request("/auth/password/reset", { method: "POST", body: payload });
  },
  requestEmailVerification(token) {
    return request("/auth/email/verification", { method: "POST", token });
  },
  verifyEmail(payload) {
    return request("/auth/email/verify", { method: "POST", body: payload });
  },
  getUser(userId) {
    return request(`/users/${userId}`);
  },
  getUserByUsername(username) {
    return request("/users/by-username/" + encodeURIComponent(username));
  },
  getUsers(userIds = []) {
    const ids = Array.from(
      new Set(userIds.map((userId) => String(userId).trim()).filter(Boolean))
    ).join(",");
    return request(`/users/batch${buildQuery({ ids })}`);
  },
  followUser(userId, token) {
    return request(`/users/${userId}/follow`, { method: "POST", token });
  },
  cancelFollowRequest(userId, token) {
    return request(`/users/${userId}/follow/cancel`, { method: "POST", token });
  },
  unfollowUser(userId, token) {
    return request(`/users/${userId}/follow`, { method: "DELETE", token });
  },
  followingState(userId, token) {
    return request(`/users/${userId}/following-state`, { token });
  },
  exportFollowing(payload = {}, token) {
    return request("/i/export-following", { method: "POST", body: payload, token });
  },
  receivedFollowRequests(params = {}, token) {
    return request(`/users/me/follow-requests${buildQuery({ page: 1, page_size: 20, ...params })}`, { token });
  },
  sentFollowRequests(params = {}, token) {
    return request(`/users/me/follow-requests/sent${buildQuery({ page: 1, page_size: 20, ...params })}`, { token });
  },
  acceptFollowRequest(requesterId, token) {
    return request(`/users/me/follow-requests/${encodeURIComponent(requesterId)}/accept`, { method: "POST", token });
  },
  rejectFollowRequest(requesterId, token) {
    return request(`/users/me/follow-requests/${encodeURIComponent(requesterId)}/reject`, { method: "POST", token });
  },
  setFollowApprovalRequired(required, token) {
    return request("/users/me/settings/follow-approval", { method: "PUT", body: { required: Boolean(required) }, token });
  },
  userSafetyState(userId, token) {
    return request(`/users/${userId}/safety-state`, { token });
  },
  blockUser(userId, token) {
    return request(`/users/${userId}/block`, { method: "POST", token });
  },
  unblockUser(userId, token) {
    return request(`/users/${userId}/block`, { method: "DELETE", token });
  },
  muteUser(userId, token) {
    return request(`/users/${userId}/mute`, { method: "POST", token });
  },
  unmuteUser(userId, token) {
    return request(`/users/${userId}/mute`, { method: "DELETE", token });
  },
  blockedUsers(params = {}, token) {
    return request(`/users/me/blocked${buildQuery({ page: 1, page_size: 20, ...params })}`, { token });
  },
  mutedUsers(params = {}, token) {
    return request(`/users/me/muted${buildQuery({ page: 1, page_size: 20, ...params })}`, { token });
  },
  exportBlocking(token) {
    return request("/i/export-blocking", { method: "POST", body: {}, token });
  },
  exportMute(token) {
    return request("/i/export-mute", { method: "POST", body: {}, token });
  },
  importBlocking(fileId, token) {
    return request("/i/import-blocking", { method: "POST", body: { fileId: String(fileId) }, token });
  },
  importMuting(fileId, token) {
    return request("/i/import-muting", { method: "POST", body: { fileId: String(fileId) }, token });
  },
  importFollowing(fileId, withReplies = false, token) {
    return request("/i/import-following", {
      method: "POST",
      body: { fileId: String(fileId), withReplies: Boolean(withReplies) },
      token
    });
  },
  exportUserLists(token) {
    return request("/i/export-user-lists", { method: "POST", body: {}, token });
  },
  importUserLists(fileId, token) {
    return request("/i/import-user-lists", { method: "POST", body: { fileId: String(fileId) }, token });
  },
  myUserLists(params = {}, token) {
    return request(`/users/me/lists${buildQuery({ page: 1, page_size: 20, ...params })}`, { token });
  },
  userLists(userId, params = {}, token) {
    return request(`/users/${encodeURIComponent(userId)}/lists${buildQuery({ page: 1, page_size: 20, ...params })}`, { token });
  },
  favoriteUserLists(params = {}, token) {
    return request(`/users/me/favorite-lists${buildQuery({ page: 1, page_size: 20, ...params })}`, { token });
  },
  createUserList(payload, token) {
    return request("/users/me/lists", { method: "POST", body: payload, token });
  },
  userList(listId, token) {
    return request(`/user-lists/${encodeURIComponent(listId)}`, { token });
  },
  updateUserList(listId, payload, token) {
    return request(`/user-lists/${encodeURIComponent(listId)}`, { method: "PUT", body: payload, token });
  },
  deleteUserList(listId, token) {
    return request(`/user-lists/${encodeURIComponent(listId)}`, { method: "DELETE", token });
  },
  userListMembers(listId, params = {}, token) {
    return request(`/user-lists/${encodeURIComponent(listId)}/members${buildQuery({ page: 1, page_size: 100, ...params })}`, { token });
  },
  addUserListMember(listId, userId, token) {
    return request(`/user-lists/${encodeURIComponent(listId)}/members`, { method: "POST", body: { user_id: userId }, token });
  },
  removeUserListMember(listId, userId, token) {
    return request(`/user-lists/${encodeURIComponent(listId)}/members`, { method: "DELETE", body: { user_id: userId }, token });
  },
  copyUserList(listId, name, token) {
    return request(`/user-lists/${encodeURIComponent(listId)}/copy`, { method: "POST", body: { name }, token });
  },
  favoriteUserList(listId, token) {
    return request(`/user-lists/${encodeURIComponent(listId)}/favorite`, { method: "POST", token });
  },
  unfavoriteUserList(listId, token) {
    return request(`/user-lists/${encodeURIComponent(listId)}/favorite`, { method: "DELETE", token });
  },
  userListFeed(listId, params = {}, token) {
    return request(`/user-lists/${encodeURIComponent(listId)}/feed${buildQuery({ limit: 20, offset: 0, ...params })}`, { token });
  },
  followers(userId, params = {}) {
    return request(`/users/${userId}/followers${buildQuery({ page: 1, page_size: 20, ...params })}`);
  },
  removeFollower(followerId, token) {
    return request(`/users/me/followers/${encodeURIComponent(followerId)}`, { method: "DELETE", token });
  },
  following(userId, params = {}) {
    return request(`/users/${userId}/following${buildQuery({ page: 1, page_size: 20, ...params })}`);
  },
  userBadges(userId, params = {}) {
    return request(`/users/${userId}/badges${buildQuery({ limit: 20, offset: 0, ...params })}`);
  },
  levels(params = {}) {
    return request(`/levels${buildQuery({ limit: 20, offset: 0, ...params })}`);
  },
  likes(params = {}, token) {
    return request(`/users/current/likes${buildQuery({ limit: 20, offset: 0, ...params })}`, { token });
  },
  favorites(params = {}, token) {
    return request(`/users/current/favorites${buildQuery({ limit: 20, offset: 0, ...params })}`, { token });
  },
  exportFavorites(token) {
    return request("/i/export-favorites", { method: "POST", body: {}, token });
  },
  exportNotes(token) {
    return request("/i/export-notes", { method: "POST", body: {}, token });
  },
  importNotes(fileId, source, token) {
    return request("/i/import-notes", {
      method: "POST",
      body: { fileId: String(fileId), type: String(source) },
      token
    });
  },
  exportData(token) {
    return request("/i/export-data", { method: "POST", body: {}, token });
  },
  collections(params = {}, token) {
    return request(`/users/me/collections${buildQuery({ limit: 20, offset: 0, ...params })}`, { token });
  },
  createCollection(payload, token) {
    return request("/users/me/collections", { method: "POST", body: payload, token });
  },
  updateCollection(collectionId, payload, token) {
    return request(`/users/me/collections/${encodeURIComponent(collectionId)}`, { method: "PUT", body: payload, token });
  },
  deleteCollection(collectionId, token) {
    return request(`/users/me/collections/${encodeURIComponent(collectionId)}`, { method: "DELETE", token });
  },
  collectionItems(collectionId, params = {}, token) {
    return request(`/users/me/collections/${encodeURIComponent(collectionId)}/items${buildQuery({ limit: 20, offset: 0, ...params })}`, { token });
  },
  addCollectionItem(collectionId, payload, token) {
    return request(`/users/me/collections/${encodeURIComponent(collectionId)}/items`, { method: "POST", body: payload, token });
  },
  removeCollectionItem(collectionId, payload, token) {
    return request(`/users/me/collections/${encodeURIComponent(collectionId)}/items`, { method: "DELETE", body: payload, token });
  },
  clips(token) {
    return request("/clips/list", { method: "POST", body: {}, token });
  },
  exportClips(token) {
    return request("/i/export-clips", { method: "POST", body: {}, token });
  },
  createClip(payload, token) {
    return request("/clips/create", { method: "POST", body: payload, token });
  },
  updateClip(payload, token) {
    return request("/clips/update", { method: "POST", body: payload, token });
  },
  deleteClip(clipId, token) {
    return request("/clips/delete", { method: "POST", body: { clipId: String(clipId) }, token });
  },
  showClip(clipId, token) {
    return request("/clips/show", { method: "POST", body: { clipId: String(clipId) }, token });
  },
  addClipNote(clipId, noteId, token) {
    return request("/clips/add-note", { method: "POST", body: { clipId: String(clipId), noteId: String(noteId) }, token });
  },
  removeClipNote(clipId, noteId, token) {
    return request("/clips/remove-note", { method: "POST", body: { clipId: String(clipId), noteId: String(noteId) }, token });
  },
  clipNotes(clipId, payload = {}, token) {
    return request("/clips/notes", { method: "POST", body: { clipId: String(clipId), ...payload }, token });
  },
  favoriteClip(clipId, token) {
    return request("/clips/favorite", { method: "POST", body: { clipId: String(clipId) }, token });
  },
  unfavoriteClip(clipId, token) {
    return request("/clips/unfavorite", { method: "POST", body: { clipId: String(clipId) }, token });
  },
  myFavoriteClips(token) {
    return request("/clips/my-favorites", { method: "POST", body: {}, token });
  },
  userClips(userId, payload = {}, token) {
    return request("/users/clips", { method: "POST", body: { userId: String(userId), ...payload }, token });
  },
  noteClips(noteId, token) {
    return request("/notes/clips", { method: "POST", body: { noteId: String(noteId) }, token });
  },
  feed(params = {}, token) {
    return request(`/feed${buildQuery({ limit: 20, offset: 0, ...params })}`, { token });
  },
  emojis() {
    return request("/emojis");
  },
  listTopics(params = {}) {
    return request(`/topics${buildQuery({ status: 2, limit: 20, offset: 0, ...params })}`);
  },
  myTopics(params = {}, token) {
    return request(`/users/me/topics${buildQuery({ status: 0, limit: 20, offset: 0, ...params })}`, { token });
  },
  listArticles(params = {}) {
    return request(`/articles${buildQuery({ status: 2, limit: 20, offset: 0, ...params })}`);
  },
  myArticles(params = {}, token) {
    return request(`/users/me/articles${buildQuery({ status: 0, limit: 20, offset: 0, ...params })}`, { token });
  },
  categories(params = {}) {
    return request(`/categories${buildQuery({ status: 2, limit: 20, offset: 0, ...params })}`);
  },
  channels(params = {}, token) {
    return request(`/channels${buildQuery({ limit: 20, offset: 0, ...params })}`, { token });
  },
  featuredChannels(params = {}, token) {
    return request(`/channels/featured${buildQuery({ limit: 20, offset: 0, ...params })}`, { token });
  },
  channelCategories(params = {}) {
    return request(`/channels/categories${buildQuery(params)}`);
  },
  ownedChannels(params = {}, token) {
    return request(`/channels/owned${buildQuery({ limit: 20, offset: 0, ...params })}`, { token });
  },
  followedChannels(params = {}, token) {
    return request(`/channels/followed${buildQuery({ limit: 20, offset: 0, ...params })}`, { token });
  },
  favoriteChannels(params = {}, token) {
    return request(`/channels/favorites${buildQuery({ limit: 20, offset: 0, ...params })}`, { token });
  },
  getChannel(channelId, token) {
    return request(`/channels/${encodeURIComponent(channelId)}`, { token });
  },
  createChannel(payload, token) {
    return request("/channels", { method: "POST", body: payload, token });
  },
  updateChannel(channelId, payload, token) {
    return request(`/channels/${encodeURIComponent(channelId)}`, { method: "PUT", body: payload, token });
  },
  archiveChannel(channelId, token) {
    return request(`/channels/${encodeURIComponent(channelId)}`, { method: "DELETE", token });
  },
  followChannel(channelId, token) {
    return request(`/channels/${encodeURIComponent(channelId)}/follow`, { method: "POST", token });
  },
  unfollowChannel(channelId, token) {
    return request(`/channels/${encodeURIComponent(channelId)}/follow`, { method: "DELETE", token });
  },
  favoriteChannel(channelId, token) {
    return request(`/channels/${encodeURIComponent(channelId)}/favorite`, { method: "POST", token });
  },
  unfavoriteChannel(channelId, token) {
    return request(`/channels/${encodeURIComponent(channelId)}/favorite`, { method: "DELETE", token });
  },
  channelTopics(channelId, params = {}) {
    return request(`/channels/${encodeURIComponent(channelId)}/topics${buildQuery({ limit: 20, offset: 0, ...params })}`);
  },
  links(params = {}) {
    return request(`/links${buildQuery({ status: 2, limit: 20, offset: 0, ...params })}`);
  },
  popularChatRooms(params = {}) {
    return request(`/chat/popular${buildQuery({ limit: 5, ...params })}`);
  },
  popularResources(params = {}) {
    return request(`/links/popular${buildQuery({ limit: 5, ...params })}`);
  },
  recordResourceVisit(resourceId) {
    return request(`/links/${encodeURIComponent(resourceId)}/visit`, { method: "POST" });
  },
  tasks(params = {}, token) {
    return request(`/tasks${buildQuery({ status: 2, limit: 20, offset: 0, ...params })}`, { token });
  },
  myTasks(params = {}, token) {
    return request(`/tasks/me${buildQuery({ limit: 20, offset: 0, ...params })}`, { token });
  },
  claimTask(taskId, token) {
    return request(`/tasks/${taskId}/claim`, { method: "POST", token });
  },
  getCategory(categoryId) {
    return request(`/categories/${categoryId}`);
  },
  tags(params = {}) {
    return request(`/tags${buildQuery({ limit: 12, ...params })}`);
  },
  chatSidebar(token) {
    return request("/chat/sidebar", { token });
  },
  createChatRoom(payload, token) {
    return request("/chat/rooms", { method: "POST", body: payload, token });
  },
  lookupChatRoom(roomNo, token) {
    return request(`/chat/rooms/lookup${buildQuery({ room_no: roomNo })}`, { token });
  },
  getChatRoom(roomNo, token) {
    return request(`/chat/rooms/${encodeURIComponent(roomNo)}`, { token });
  },
  joinChatRoom(roomNo, token) {
    return request(`/chat/rooms/${encodeURIComponent(roomNo)}/join`, { method: "POST", token });
  },
  leaveChatRoom(roomNo, token) {
    return request(`/chat/rooms/${encodeURIComponent(roomNo)}/membership`, { method: "DELETE", token });
  },
  chatRoomMembers(roomNo, params = {}, token) {
    return request(`/chat/rooms/${encodeURIComponent(roomNo)}/members${buildQuery(params)}`, { token });
  },
  updateChatRoomMemberRole(roomNo, userId, role, token) {
    return request(`/chat/rooms/${encodeURIComponent(roomNo)}/members/${encodeURIComponent(userId)}/role`, {
      method: "PUT",
      body: { role },
      token
    });
  },
  muteChatRoomMember(roomNo, userId, expiresAt, token) {
    return request(`/chat/rooms/${encodeURIComponent(roomNo)}/members/${encodeURIComponent(userId)}/mute`, {
      method: "PUT",
      body: { expires_at: expiresAt },
      token
    });
  },
  unmuteChatRoomMember(roomNo, userId, token) {
    return request(`/chat/rooms/${encodeURIComponent(roomNo)}/members/${encodeURIComponent(userId)}/mute`, {
      method: "DELETE",
      token
    });
  },
  chatMessages(roomNo, params = {}, token) {
    return request(`/chat/rooms/${encodeURIComponent(roomNo)}/messages${buildQuery(params)}`, { token });
  },
  sendChatMessage(roomNo, payload, token) {
    return request(`/chat/rooms/${encodeURIComponent(roomNo)}/messages`, { method: "POST", body: payload, token });
  },
  deleteChatMessage(roomNo, messageId, token) {
    return request(`/chat/rooms/${encodeURIComponent(roomNo)}/messages/${encodeURIComponent(messageId)}`, { method: "DELETE", token });
  },
  advanceChatRead(roomNo, readSeq, token) {
    return request(`/chat/rooms/${encodeURIComponent(roomNo)}/read`, { method: "PUT", body: { read_seq: readSeq }, token });
  },
  createChatGroup(payload, token) {
    return request("/chat/groups", { method: "POST", body: payload, token });
  },
  updateChatGroup(groupId, payload, token) {
    return request(`/chat/groups/${encodeURIComponent(groupId)}`, { method: "PATCH", body: payload, token });
  },
  deleteChatGroup(groupId, token) {
    return request(`/chat/groups/${encodeURIComponent(groupId)}`, { method: "DELETE", token });
  },
  moveChatGroup(groupId, direction, token) {
    return request(`/chat/groups/${encodeURIComponent(groupId)}/move`, {
      method: "POST",
      body: { direction },
      token
    });
  },
  placeChatRoom(roomNo, payload, token) {
    return request(`/chat/rooms/${encodeURIComponent(roomNo)}/placement`, { method: "PUT", body: payload, token });
  },
  updateChatAnnouncement(roomNo, announcement, token) {
    return request(`/chat/rooms/${encodeURIComponent(roomNo)}/announcement`, { method: "PATCH", body: { announcement }, token });
  },
  markChatAnnouncementSeen(roomNo, announcementVersion, token) {
    return request(`/chat/rooms/${encodeURIComponent(roomNo)}/announcement-seen`, {
      method: "PUT",
      body: { announcement_version: announcementVersion },
      token
    });
  },
  createChatWebSocketTicket(token) {
    return request("/chat/ws-tickets", { method: "POST", token });
  },
  autocompleteTags(payload = {}) {
    return request("/tags/autocomplete", { method: "POST", body: payload });
  },
  searchArticles(keyword, params = {}) {
    return request(`/search/articles${buildQuery({ q: keyword, page: 1, page_size: 20, ...params })}`);
  },
  searchTopics(keyword, params = {}) {
    return request(`/search/topics${buildQuery({ q: keyword, page: 1, page_size: 20, ...params })}`);
  },
  searchUsers(keyword, params = {}) {
    return request(`/search/users${buildQuery({ q: keyword, page: 1, page_size: 20, ...params })}`);
  },
  searchHashtags(keyword, params = {}) {
    return request(`/hashtags/search${buildQuery({ q: keyword, limit: 20, offset: 0, ...params })}`);
  },
  hashtags(params = {}) {
    return request(`/hashtags/list${buildQuery({ limit: 20, offset: 0, ...params })}`);
  },
  hashtag(tag) {
    return request(`/hashtags/show${buildQuery({ tag })}`);
  },
  hashtagUsers(tag, params = {}) {
    return request(`/hashtags/users${buildQuery({ tag, limit: 20, offset: 0, ...params })}`);
  },
  trendingHashtags(params = {}) {
    return request(`/hashtags/trend${buildQuery({ limit: 10, offset: 0, ...params })}`);
  },
  createArticle(payload, token) {
    return request("/articles", { method: "POST", body: payload, token });
  },
  updateArticle(articleId, payload, token) {
    return request(`/articles/${articleId}`, { method: "PUT", body: payload, token });
  },
  publishArticle(articleId, token) {
    return request(`/articles/${articleId}/publish`, { method: "POST", token });
  },
  hideArticle(articleId, token) {
    return request(`/articles/${articleId}/hide`, { method: "POST", token });
  },
  deleteArticle(articleId, token) {
    return request(`/articles/${articleId}`, { method: "DELETE", token });
  },
  createTopic(payload, token) {
    return request("/topics", { method: "POST", body: payload, token });
  },
  updateTopic(topicId, payload, token) {
    return request(`/topics/${topicId}`, { method: "PUT", body: payload, token });
  },
  publishTopic(topicId, token) {
    return request(`/topics/${topicId}/publish`, { method: "POST", token });
  },
  deleteTopic(topicId, token) {
    return request(`/topics/${topicId}`, { method: "DELETE", token });
  },
  getTopic(topicId, token) {
    return request(`/topics/${topicId}`, { token });
  },
  voteTopicPoll(topicId, payload, token) {
    return request(`/topics/${topicId}/poll/votes`, { method: "POST", body: payload, token });
  },
  listTopicAttachments(topicId) {
    return request(`/topics/${topicId}/attachments`);
  },
  attachmentDownloads(params = {}, token) {
    return request(`/attachments/downloads${buildQuery({ limit: 20, offset: 0, ...params })}`, { token });
  },
  attachmentSales(params = {}, token) {
    return request(`/attachments/sales${buildQuery({ limit: 20, offset: 0, ...params })}`, { token });
  },
  uploadTopicAttachment(topicId, file, priceCredits, token) {
    const form = new FormData();
    form.append("file", file);
    form.append("price_credits", String(priceCredits ?? 0));
    return request(`/topics/${topicId}/attachments`, { method: "POST", body: form, token });
  },
  downloadTopicAttachment(attachmentId, token) {
    return downloadAttachment(`/attachments/${attachmentId}/download`, token);
  },
  archiveTopicAttachment(attachmentId, token) {
    return request(`/attachments/${attachmentId}`, { method: "DELETE", token });
  },
  updateTopicAttachmentPrice(attachmentId, priceCredits, token) {
    return request(`/attachments/${attachmentId}`, { method: "PATCH", body: { price_credits: priceCredits }, token });
  },
  getEditableTopic(topicId, token) {
    return request(`/topics/${topicId}/edit-source`, { token });
  },
  getArticle(articleId) {
    return request(`/articles/${articleId}`);
  },
  getEditableArticle(articleId, token) {
    return request(`/articles/${articleId}/edit-source`, { token });
  },
  listTopicComments(topicId, params = {}) {
    return request(`/topics/${topicId}/comments${buildQuery({ page: 1, page_size: 20, ...params })}`);
  },
  listComments(articleId, params = {}) {
    return request(`/articles/${articleId}/comments${buildQuery({ page: 1, page_size: 20, ...params })}`);
  },
  listReplies(commentId, params = {}) {
    return request(`/comments/${commentId}/replies${buildQuery({ page: 1, page_size: 20, ...params })}`);
  },
  commentConversation(commentId, params = {}) {
    return request(`/comments/${commentId}/conversation${buildQuery({ limit: 10, offset: 0, ...params })}`);
  },
  getComment(commentId) {
    return request(`/comments/${commentId}`);
  },
  createTopicComment(topicId, payload, token) {
    return request(`/topics/${topicId}/comments`, { method: "POST", body: payload, token });
  },
  acceptTopicComment(topicId, commentId, token) {
    return request(`/topics/${topicId}/comments/${commentId}/accept`, { method: "POST", token });
  },
  unacceptTopicComment(topicId, commentId, token) {
    return request(`/topics/${topicId}/comments/${commentId}/unaccept`, { method: "POST", token });
  },
  createComment(articleId, payload, token) {
    return request(`/articles/${articleId}/comments`, { method: "POST", body: payload, token });
  },
  deleteComment(commentId, token) {
    return request(`/comments/${commentId}`, { method: "DELETE", token });
  },
  likeTopic(topicId, token) {
    return request(`/topics/${topicId}/like`, { method: "POST", token });
  },
  unlikeTopic(topicId, token) {
    return request(`/topics/${topicId}/like`, { method: "DELETE", token });
  },
  likeArticle(articleId, token) {
    return request(`/articles/${articleId}/like`, { method: "POST", token });
  },
  unlikeArticle(articleId, token) {
    return request(`/articles/${articleId}/like`, { method: "DELETE", token });
  },
  favoriteArticle(articleId, token) {
    return request(`/articles/${articleId}/favorite`, { method: "POST", token });
  },
  favoriteTopic(topicId, token) {
    return request(`/topics/${topicId}/favorite`, { method: "POST", token });
  },
  unfavoriteTopic(topicId, token) {
    return request(`/topics/${topicId}/favorite`, { method: "DELETE", token });
  },
  unfavoriteArticle(articleId, token) {
    return request(`/articles/${articleId}/favorite`, { method: "DELETE", token });
  },
  pinNote(noteId, token) {
    return request("/i/pin", { method: "POST", body: { noteId: String(noteId) }, token });
  },
  unpinNote(noteId, token) {
    return request("/i/unpin", { method: "POST", body: { noteId: String(noteId) }, token });
  },
  reportTopic(topicId, payload, token) {
    return request(`/topics/${topicId}/report`, { method: "POST", body: payload, token });
  },
  reportArticle(articleId, payload, token) {
    return request(`/articles/${articleId}/report`, { method: "POST", body: payload, token });
  },
  reportComment(commentId, payload, token) {
    return request(`/comments/${commentId}/report`, { method: "POST", body: payload, token });
  },
  topicReactions(topicId) {
    return request(`/topics/${topicId}/reactions`);
  },
  articleReactions(articleId) {
    return request(`/articles/${articleId}/reactions`);
  },
  notifications(params = {}, token) {
    return request(`/notifications${buildQuery({ limit: 20, offset: 0, ...params })}`, { token });
  },
  notificationUnreadCount(token) {
    return request("/notifications/unread-count", { token });
  },
  markNotificationRead(notificationId, token) {
    return request(`/notifications/${notificationId}/read`, { method: "POST", token });
  },
  markAllNotificationsRead(token) {
    return request("/notifications/read-all", { method: "POST", token });
  },
  notificationPreferences(token) {
    return request("/users/me/notification-preferences", { token });
  },
  updateNotificationPreferences(items, token) {
    return request("/users/me/notification-preferences", { method: "PUT", body: { items }, token });
  },
  webPushConfig() {
    return request("/sw/config");
  },
  registerWebPush(payload, token) {
    return request("/sw/register", { method: "POST", body: payload, token });
  },
  webPushRegistration(endpoint, token) {
    return request("/sw/show-registration", { method: "POST", body: { endpoint }, token });
  },
  unregisterWebPush(endpoint, token) {
    return request("/sw/unregister", { method: "POST", body: { endpoint }, token });
  },
  creditBalance(token) {
    return request("/credits/balance", { token });
  },
  creditLedger(params = {}, token) {
    return request(`/credits/ledger${buildQuery({ limit: 20, offset: 0, ...params })}`, { token });
  },
  creditLeaderboard(params = {}) {
    return request(`/credits/leaderboard${buildQuery({ limit: 10, ...params })}`);
  },
  checkInStatus(token) {
    return request("/credits/check-in", { token });
  },
  checkIn(token) {
    return request("/credits/check-in", { method: "POST", token });
  },
  mallProducts(params = {}) {
    return request(`/mall/products${buildQuery({ limit: 20, offset: 0, ...params })}`);
  },
  mallCategories(params = {}) {
    return request(`/mall/categories${buildQuery({ limit: 20, offset: 0, ...params })}`);
  },
  mallProduct(productId) {
    return request(`/mall/products/${productId}`);
  },
  mallProductReviews(productId, params = {}) {
    return request(`/mall/products/${productId}/reviews${buildQuery({ limit: 20, offset: 0, ...params })}`);
  },
  createMallProductReview(productId, payload, token) {
    return request(`/mall/products/${productId}/reviews`, { method: "POST", body: payload, token });
  },
  mallReviewableOrders(productId, params = {}, token) {
    return request(`/mall/products/${productId}/reviewable-orders${buildQuery({ limit: 20, offset: 0, ...params })}`, { token });
  },
  mallReviews(params = {}, token) {
    return request(`/mall/reviews${buildQuery({ limit: 20, offset: 0, ...params })}`, { token });
  },
  mallCoupons(params = {}) {
    return request(`/mall/coupons${buildQuery({ limit: 20, offset: 0, ...params })}`);
  },
  mallMyCoupons(params = {}, token) {
    return request(`/mall/coupons/mine${buildQuery({ limit: 20, offset: 0, ...params })}`, { token });
  },
  claimMallCoupon(couponId, token) {
    return request(`/mall/coupons/${couponId}/claim`, { method: "POST", token });
  },
  mallProductFavorites(params = {}, token) {
    return request(`/mall/favorites${buildQuery({ limit: 20, offset: 0, ...params })}`, { token });
  },
  mallProductFavoriteState(productId, token) {
    return request(`/mall/products/${productId}/favorite`, { token });
  },
  favoriteMallProduct(productId, token) {
    return request(`/mall/products/${productId}/favorite`, { method: "POST", token });
  },
  unfavoriteMallProduct(productId, token) {
    return request(`/mall/products/${productId}/favorite`, { method: "DELETE", token });
  },
  mallCart(token) {
    return request("/mall/cart", { token });
  },
  setMallCartItem(productId, payload, token) {
    return request(`/mall/cart/items/${productId}`, { method: "PUT", body: payload, token });
  },
  removeMallCartItem(productId, token) {
    return request(`/mall/cart/items/${productId}`, { method: "DELETE", token });
  },
  clearMallCart(token) {
    return request("/mall/cart", { method: "DELETE", token });
  },
  checkoutMallCart(payload, token) {
    return request("/mall/cart/checkout", { method: "POST", body: payload, token });
  },
  mallAddresses(params = {}, token) {
    return request(`/mall/addresses${buildQuery({ limit: 20, offset: 0, ...params })}`, { token });
  },
  createMallAddress(payload, token) {
    return request("/mall/addresses", { method: "POST", body: payload, token });
  },
  updateMallAddress(addressId, payload, token) {
    return request(`/mall/addresses/${addressId}`, { method: "PUT", body: payload, token });
  },
  deleteMallAddress(addressId, token) {
    return request(`/mall/addresses/${addressId}`, { method: "DELETE", token });
  },
  setDefaultMallAddress(addressId, token) {
    return request(`/mall/addresses/${addressId}/default`, { method: "POST", token });
  },
  createMallOrder(payload, token) {
    return request("/mall/orders", { method: "POST", body: payload, token });
  },
  mallOrders(params = {}, token) {
    return request(`/mall/orders${buildQuery({ limit: 20, offset: 0, ...params })}`, { token });
  },
  mallDigitalEntitlements(params = {}, token) {
    return request(`/mall/digital-entitlements${buildQuery({ limit: 20, offset: 0, ...params })}`, { token });
  },
  mallOrder(orderId, token) {
    return request(`/mall/orders/${orderId}`, { token });
  },
  mallOrderLogs(orderId, token) {
    return request(`/mall/orders/${orderId}/logs`, { token });
  },
  mallOrderPayments(orderId, token) {
    return request(`/mall/orders/${orderId}/payments`, { token });
  },
  payMallOrder(orderId, payload, token) {
    return request(`/mall/orders/${orderId}/pay`, { method: "POST", body: payload, token });
  },
  cancelMallOrder(orderId, token) {
    return request(`/mall/orders/${orderId}/cancel`, { method: "POST", token });
  },
  confirmMallOrder(orderId, token) {
    return request(`/mall/orders/${orderId}/confirm`, { method: "POST", token });
  },
  createMallRefund(orderId, payload, token) {
    return request(`/mall/orders/${orderId}/refunds`, { method: "POST", body: payload, token });
  },
  cancelMallRefund(refundId, token) {
    return request(`/mall/refunds/${refundId}/cancel`, { method: "POST", token });
  },
  mallRefunds(params = {}, token) {
    return request(`/mall/refunds${buildQuery({ limit: 20, offset: 0, ...params })}`, { token });
  }
};
