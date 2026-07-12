const API_BASE = (import.meta.env.VITE_API_BASE || "http://127.0.0.1:18080/api/v1").replace(/\/$/, "");

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

  const response = await fetch(`${API_BASE}${path}`, {
    method,
    headers,
    body: body === undefined ? undefined : isFormData ? body : JSON.stringify(body)
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
  authConfig() {
    return request("/auth/config");
  },
  oauthStartUrl(provider, redirect) {
    return `${API_BASE}/auth/oauth/${encodeURIComponent(provider)}/start${buildQuery({ redirect })}`;
  },
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
  levels(params = {}) {
    return request(`/levels${buildQuery({ limit: 20, offset: 0, ...params })}`);
  },
  likes(params = {}, token) {
    return request(`/users/current/likes${buildQuery({ limit: 20, offset: 0, ...params })}`, { token });
  },
  favorites(params = {}, token) {
    return request(`/users/current/favorites${buildQuery({ limit: 20, offset: 0, ...params })}`, { token });
  },
  feed(params = {}, token) {
    return request(`/feed${buildQuery({ limit: 20, offset: 0, ...params })}`, { token });
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
  searchUsers(keyword, params = {}) {
    return request(`/search/users${buildQuery({ q: keyword, page: 1, page_size: 20, ...params })}`);
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
  getComment(commentId) {
    return request(`/comments/${commentId}`);
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
  creditBalance(token) {
    return request("/credits/balance", { token });
  },
  creditLedger(params = {}, token) {
    return request(`/credits/ledger${buildQuery({ limit: 20, offset: 0, ...params })}`, { token });
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
  mallRefunds(params = {}, token) {
    return request(`/mall/refunds${buildQuery({ limit: 20, offset: 0, ...params })}`, { token });
  }
};
