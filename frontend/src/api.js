const API_BASE = (import.meta.env.VITE_API_BASE || "http://127.0.0.1:8080/api/v1").replace(/\/$/, "");

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
  if (body !== undefined) {
    headers["Content-Type"] = "application/json";
  }
  if (token) {
    headers.Authorization = `Bearer ${token}`;
  }

  const response = await fetch(`${API_BASE}${path}`, {
    method,
    headers,
    body: body === undefined ? undefined : JSON.stringify(body)
  });

  const text = await response.text();
  const data = text ? parseJsonPreservingLargeInts(text) : null;
  const isEnvelope = data && typeof data === "object" && "http_code" in data && "code" in data && "data" in data;

  if (!response.ok) {
    const message = data?.message || data?.reason || data?.error?.message || `请求失败 (${response.status})`;
    throw new Error(message);
  }
  if (isEnvelope) {
    if (data.code !== 0) {
      throw new Error(data.message || data.reason || `请求失败 (${data.http_code || response.status})`);
    }
    return data.data;
  }
  return data;
}

function parseJsonPreservingLargeInts(text) {
  return JSON.parse(text.replace(/(:\s*)(-?\d{16,})(?=[,}\]])/g, '$1"$2"'));
}

export const bbsApi = {
  login(payload) {
    return request("/auth/login", { method: "POST", body: payload });
  },
  register(payload) {
    return request("/auth/register", { method: "POST", body: payload });
  },
  me(token) {
    return request("/users/me", { token });
  },
  updateMe(payload, token) {
    return request("/users/me", { method: "PUT", body: payload, token });
  },
  changePassword(payload, token) {
    return request("/users/me/password", { method: "POST", body: payload, token });
  },
  getUser(userId) {
    return request(`/users/${userId}`);
  },
  followUser(userId, token) {
    return request(`/users/${userId}/follow`, { method: "POST", token });
  },
  unfollowUser(userId, token) {
    return request(`/users/${userId}/follow`, { method: "DELETE", token });
  },
  followingState(userId, token) {
    return request(`/users/${userId}/following-state`, { token });
  },
  followers(userId, params = {}) {
    return request(`/users/${userId}/followers${buildQuery({ page: 1, page_size: 20, ...params })}`);
  },
  following(userId, params = {}) {
    return request(`/users/${userId}/following${buildQuery({ page: 1, page_size: 20, ...params })}`);
  },
  userBadges(userId, params = {}) {
    return request(`/users/${userId}/badges${buildQuery({ limit: 20, offset: 0, ...params })}`);
  },
  likes(params = {}, token) {
    return request(`/users/current/likes${buildQuery({ limit: 20, offset: 0, ...params })}`, { token });
  },
  favorites(params = {}, token) {
    return request(`/users/current/favorites${buildQuery({ limit: 20, offset: 0, ...params })}`, { token });
  },
  feed(params = {}) {
    return request(`/feed${buildQuery({ limit: 20, offset: 0, ...params })}`);
  },
  listTopics(params = {}) {
    return request(`/topics${buildQuery({ status: 2, limit: 20, offset: 0, ...params })}`);
  },
  listArticles(params = {}) {
    return request(`/articles${buildQuery({ status: 2, limit: 20, offset: 0, ...params })}`);
  },
  categories(params = {}) {
    return request(`/categories${buildQuery({ status: 2, limit: 20, offset: 0, ...params })}`);
  },
  links(params = {}) {
    return request(`/links${buildQuery({ status: 2, limit: 20, offset: 0, ...params })}`);
  },
  tasks(params = {}, token) {
    return request(`/tasks${buildQuery({ status: 2, limit: 20, offset: 0, ...params })}`, { token });
  },
  getCategory(categoryId) {
    return request(`/categories/${categoryId}`);
  },
  tags(params = {}) {
    return request(`/tags${buildQuery({ limit: 12, ...params })}`);
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
  createArticle(payload, token) {
    return request("/articles", { method: "POST", body: payload, token });
  },
  updateArticle(articleId, payload, token) {
    return request(`/articles/${articleId}`, { method: "PUT", body: payload, token });
  },
  publishArticle(articleId, token) {
    return request(`/articles/${articleId}/publish`, { method: "POST", token });
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
  getTopic(topicId) {
    return request(`/topics/${topicId}`);
  },
  getArticle(articleId) {
    return request(`/articles/${articleId}`);
  },
  listTopicComments(topicId, params = {}) {
    return request(`/topics/${topicId}/comments${buildQuery({ page: 1, page_size: 20, ...params })}`);
  },
  listComments(articleId, params = {}) {
    return request(`/articles/${articleId}/comments${buildQuery({ page: 1, page_size: 20, ...params })}`);
  },
  createTopicComment(topicId, payload, token) {
    return request(`/topics/${topicId}/comments`, { method: "POST", body: payload, token });
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
  reportTopic(topicId, payload, token) {
    return request(`/topics/${topicId}/report`, { method: "POST", body: payload, token });
  },
  reportArticle(articleId, payload, token) {
    return request(`/articles/${articleId}/report`, { method: "POST", body: payload, token });
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
  creditBalance(token) {
    return request("/credits/balance", { token });
  },
  creditLedger(params = {}, token) {
    return request(`/credits/ledger${buildQuery({ limit: 20, offset: 0, ...params })}`, { token });
  }
};
