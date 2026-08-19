import { bbsApi } from "../api.js";
import { listItems } from "./apiShapes.js";
import { sameId, timeAgoMillis, toId, toNumber } from "./formatters.js";
import { markdownImageUrls, textWithoutMarkdownImages } from "./markdownMedia.js";

const POST_AUTHOR_BATCH_SIZE = 100;

function fallbackAvatar(seed = "V") {
  const raw = String(seed || "V");
  const label = (raw.replace(/[^\p{L}\p{N}]/gu, "").slice(-2) || "V").toUpperCase();
  let hash = 0;
  for (const char of raw) {
    hash = (hash * 31 + char.codePointAt(0)) % 360;
  }
  const svg = `<svg xmlns="http://www.w3.org/2000/svg" width="140" height="140" viewBox="0 0 140 140"><rect width="140" height="140" rx="70" fill="hsl(${hash},68%,44%)"/><text x="50%" y="53%" text-anchor="middle" dominant-baseline="middle" fill="white" font-family="Inter,Arial,sans-serif" font-size="42" font-weight="700">${label}</text></svg>`;
  return `data:image/svg+xml;utf8,${encodeURIComponent(svg)}`;
}

export function normalizeProfileTheme(value) {
  const theme = String(value || "").trim().toLowerCase();
  return theme === "theme-pro" ? "theme-pro" : "default";
}

export function profileThemeClass(value) {
  return normalizeProfileTheme(value) === "theme-pro" ? "profile-theme-pro" : "profile-theme-default";
}

export function fallbackPerson(authorId, fallback = {}) {
  const id = toId(authorId ?? fallback.id);
  const name = id ? `用户 #${id}` : fallback.name || "社区成员";
  const handle = id ? `u${id}` : fallback.handle || "user-unknown";
  return {
    ...fallback,
    id: id || fallback.id || "",
    name,
    handle,
    role: "社区成员",
    bio: fallback.bio || "正在参与社区讨论",
    avatar: fallback.avatar || fallbackAvatar(id || handle || name),
    background: fallback.background || "",
    backgroundUrl: fallback.backgroundUrl || "",
    profileTheme: normalizeProfileTheme(fallback.profileTheme),
    followApprovalRequired: Boolean(fallback.followApprovalRequired),
    followerCount: toNumber(fallback.followerCount),
    followingCount: toNumber(fallback.followingCount)
  };
}

export function userAvatar(user) {
  return user?.avatar_url || user?.avatarUrl || fallbackAvatar(user?.id || user?.username || user?.nickname || "user");
}

export function userDisplayName(user) {
  return user?.nickname || user?.username || "社区成员";
}

export function userToPerson(user, fallback = {}) {
  const fallbackProfile = fallbackPerson(user?.id ?? fallback.id, fallback);
  const id = user?.id ?? fallbackProfile.id;
  return {
    id,
    username: String(user?.username || fallbackProfile.username || "").trim(),
    name: userDisplayName(user),
    handle: user?.username || fallbackProfile.handle || `user-${id || "unknown"}`,
    role: "社区成员",
    bio: user?.bio || fallbackProfile.bio || "正在参与社区讨论",
    avatar: user?.avatar_url || user?.avatarUrl || fallbackProfile.avatar,
    background: user?.background_url || user?.backgroundUrl || fallbackProfile.background || "",
    backgroundUrl: user?.background_url || user?.backgroundUrl || fallbackProfile.backgroundUrl || "",
    profileTheme: normalizeProfileTheme(user?.profile_theme || user?.profileTheme || fallbackProfile.profileTheme),
    followApprovalRequired: Boolean(user?.follow_approval_required ?? user?.followApprovalRequired ?? fallbackProfile.followApprovalRequired),
    followerCount: toNumber(user?.follower_count ?? user?.followerCount),
    followingCount: toNumber(user?.following_count ?? user?.followingCount)
  };
}

export function authProfileThemeNeedsVerification(auth) {
  return normalizeProfileTheme(auth?.user?.profile_theme || auth?.user?.profileTheme) === "theme-pro";
}

export function authProfileAppearanceNeedsVerification(auth) {
  return authProfileThemeNeedsVerification(auth) || Boolean(String(auth?.user?.background_url || auth?.user?.backgroundUrl || "").trim());
}

export function authToPerson(auth, options = {}) {
  const person = userToPerson(auth?.user);
  if (!options.trustAppearance && authProfileAppearanceNeedsVerification(auth)) {
    return { ...person, background: "", backgroundUrl: "", profileTheme: "default" };
  }
  return person;
}

function uniqueImages(...groups) {
  const seen = new Set();
  return groups
    .flat()
    .filter(Boolean)
    .filter((url) => {
      if (seen.has(url)) return false;
      seen.add(url);
      return true;
    });
}

function optionalNumber(...values) {
  const value = values.find((item) => item !== undefined && item !== null && item !== "");
  return value === undefined ? null : toNumber(value);
}

function optionalId(value) {
  const id = toId(value);
  return id && id !== "0" ? id : "";
}

function articleAuthor(article, auth) {
  const authorId = toId(article?.author_id ?? article?.authorId);
  if (sameId(authorId, auth?.user?.id)) {
    return authToPerson(auth);
  }
  return fallbackPerson(authorId);
}

export function articleToPost(article, auth) {
  const coverUrl = article?.cover_url || article?.coverUrl;
  const body = article?.body || article?.content_excerpt || article?.contentExcerpt || article?.summary || article?.title || "";
  const images = uniqueImages([coverUrl], markdownImageUrls(body));
  const timestamp = article?.published_at || article?.publishedAt || article?.created_at || article?.createdAt;
  const activeTimestamp = article?.updated_at || article?.updatedAt || timestamp;
  return {
    id: article?.id,
    kind: "article",
    authorId: toId(article?.author_id ?? article?.authorId),
    title: article?.title || "未命名帖子",
    author: articleAuthor(article, auth),
    level: "LV.1",
    time: timeAgoMillis(timestamp),
    sortAt: toNumber(timestamp),
    activeAt: toNumber(activeTimestamp),
    text: textWithoutMarkdownImages(body),
    images: images.length > 0 ? images : undefined,
    tags: article?.tags || article?.tag_names || article?.tagNames || [],
    likes: toNumber(article?.like_count ?? article?.likeCount),
    favorites: toNumber(article?.favorite_count ?? article?.favoriteCount),
    comments: optionalNumber(article?.comment_count, article?.commentCount),
    views: optionalNumber(article?.view_count, article?.viewCount),
    liked: false,
    favorited: false,
    pinned: Boolean(article?.is_pinned ?? article?.isPinned)
  };
}

export function topicToPost(topic, auth) {
  const timestamp = topic?.published_at || topic?.publishedAt || topic?.created_at || topic?.createdAt;
  const activeTimestamp = topic?.updated_at || topic?.updatedAt || timestamp;
  const body = topic?.body || topic?.content_excerpt || topic?.contentExcerpt || topic?.title || "";
  const images = uniqueImages(markdownImageUrls(body));
  const topicType = topic?.type || topic?.summary || "topic";
  return {
    id: topic?.id,
    kind: "topic",
    topicType,
    authorId: toId(topic?.author_id ?? topic?.authorId),
    title: topic?.title || "未命名帖子",
    author: articleAuthor(topic, auth),
    level: topicType === "tweet" ? "动态" : topicType === "qa" ? "问答" : "话题",
    time: timeAgoMillis(timestamp),
    sortAt: toNumber(timestamp),
    activeAt: toNumber(activeTimestamp),
    text: textWithoutMarkdownImages(body),
    images: images.length > 0 ? images : undefined,
    tags: topic?.tags || topic?.tag_names || topic?.tagNames || [],
    categoryId: optionalId(topic?.category_id ?? topic?.categoryId),
    bountyScore: toNumber(topic?.bounty_score ?? topic?.bountyScore),
    qaStatus: topic?.qa_status || topic?.qaStatus || "",
    acceptedCommentId: optionalId(topic?.accepted_comment_id ?? topic?.acceptedCommentId),
    likes: toNumber(topic?.like_count ?? topic?.likeCount),
    favorites: toNumber(topic?.favorite_count ?? topic?.favoriteCount),
    comments: optionalNumber(topic?.comment_count, topic?.commentCount),
    views: optionalNumber(topic?.view_count, topic?.viewCount),
    liked: false,
    favorited: false,
    pinned: Boolean(topic?.is_pinned ?? topic?.isPinned)
  };
}

export function searchHitToPost(hit, auth) {
  const article = hit?.article || hit;
  const highlight = hit?.highlight || {};
  const post = articleToPost(
    {
      ...article,
      body: article?.content_excerpt || article?.contentExcerpt || article?.summary,
      tags: article?.tag_names || article?.tagNames || article?.tags
    },
    auth
  );
  return {
    ...post,
    highlight: {
      title: highlight.title || [],
      text: highlight.content_excerpt || highlight.contentExcerpt || highlight.summary || []
    }
  };
}

export function topicSearchHitToPost(hit, auth) {
  const topic = hit?.topic || hit;
  const highlight = hit?.highlight || {};
  const post = topicToPost(
    {
      ...topic,
      body: topic?.content_excerpt || topic?.contentExcerpt || topic?.body,
      tags: topic?.tag_names || topic?.tagNames || topic?.tags
    },
    auth
  );
  return {
    ...post,
    highlight: {
      title: highlight.title || [],
      text: highlight.content_excerpt || highlight.contentExcerpt || highlight.tag_names || highlight.tagNames || []
    }
  };
}

export function feedItemToPost(item, auth) {
  const entityType = item?.entity_type || item?.entityType;
  return entityType === "topic" ? topicToPost(item, auth) : articleToPost(item, auth);
}

export function uniquePosts(items) {
  const byKey = new Map();
  items.forEach((item) => {
    const key = `${item.kind || "article"}:${item.id}`;
    if (!item.id) {
      return;
    }
    const existing = byKey.get(key);
    byKey.set(key, existing ? mergePost(existing, item) : item);
  });
  return Array.from(byKey.values());
}

function mergePost(base, next) {
  return {
    ...base,
    ...next,
    author: next.author || base.author,
    images: next.images?.length ? next.images : base.images,
    tags: next.tags?.length ? next.tags : base.tags,
    text: next.text || base.text,
    views: next.views ?? base.views,
    categoryId: next.categoryId || base.categoryId,
    likes: Math.max(toNumber(base.likes), toNumber(next.likes)),
    favorites: Math.max(toNumber(base.favorites), toNumber(next.favorites)),
    comments: Math.max(toNumber(base.comments), toNumber(next.comments)),
    activeAt: Math.max(toNumber(base.activeAt), toNumber(next.activeAt)),
    sortAt: Math.max(toNumber(base.sortAt), toNumber(next.sortAt))
  };
}

function hasPersistedPostId(post) {
  return Boolean(post?.id);
}

async function hydratePostCounts(post) {
  if (!hasPersistedPostId(post)) {
    return post;
  }
  const topicPost = post.kind === "topic";
  const [reactionData, commentsData] = await Promise.all([
    (topicPost ? bbsApi.topicReactions(post.id) : bbsApi.articleReactions(post.id)).catch(() => null),
    (topicPost ? bbsApi.listTopicComments(post.id, { page: 1, page_size: 1 }) : bbsApi.listComments(post.id, { page: 1, page_size: 1 })).catch(() => null)
  ]);

  return {
    ...post,
    likes: toNumber(reactionData?.like_count ?? reactionData?.likeCount, post.likes),
    favorites: toNumber(reactionData?.favorite_count ?? reactionData?.favoriteCount, post.favorites),
    comments:
      typeof commentsData?.total !== "undefined" ? toNumber(commentsData.total, post.comments) : toNumber(post.comments)
  };
}

async function loadPostAuthors(items, auth) {
  const authorIDs = new Set();
  items.forEach((post) => {
    const authorID = toId(post?.authorId);
    if (!/^[1-9]\d*$/.test(authorID)) return;
    const currentUserAuthor = sameId(authorID, auth?.user?.id);
    if (!currentUserAuthor || authProfileAppearanceNeedsVerification(auth)) {
      authorIDs.add(authorID);
    }
  });

  const usersByID = new Map();
  const ids = [...authorIDs];
  for (let start = 0; start < ids.length; start += POST_AUTHOR_BATCH_SIZE) {
    const data = await bbsApi.getUsers(ids.slice(start, start + POST_AUTHOR_BATCH_SIZE)).catch(() => null);
    listItems(data).forEach((user) => {
      const userID = toId(user?.id);
      if (userID) usersByID.set(userID, user);
    });
  }
  return usersByID;
}

async function hydratePostAuthor(post, auth, authorsPromise) {
  if (!post?.authorId) {
    return post;
  }
  const currentUserAuthor = sameId(post.authorId, auth?.user?.id);
  if (currentUserAuthor && !authProfileAppearanceNeedsVerification(auth)) {
    return { ...post, author: authToPerson(auth, { trustAppearance: true }) };
  }
  const authorsByID = await authorsPromise;
  const author = authorsByID.get(toId(post.authorId));
  if (!author) {
    if (currentUserAuthor) {
      return { ...post, author: authToPerson(auth) };
    }
    return post;
  }
  return { ...post, author: userToPerson(author, post.author) };
}

async function hydratePostMeta(post, auth, options, authorsPromise) {
  const [withCounts, withAuthor] = await Promise.all([
    options.skipCounts ? Promise.resolve(post) : hydratePostCounts(post),
    hydratePostAuthor(post, auth, authorsPromise)
  ]);
  return { ...withCounts, author: withAuthor.author };
}

export async function hydratePostsMeta(items, auth, options = {}) {
  const authorsPromise = loadPostAuthors(items, auth);
  return Promise.all(items.map((item) => hydratePostMeta(item, auth, options, authorsPromise)));
}

function interactionEntity(item) {
  const entity = item?.entity || {};
  return {
    type: entity.entity_type || entity.entityType || "",
    id: toId(entity.entity_id ?? entity.entityId)
  };
}

export async function interactionToPost(item, auth, mode) {
  const entity = interactionEntity(item);
  if (!entity.type || !entity.id) {
    return null;
  }
  try {
    const data = entity.type === "topic" ? await bbsApi.getTopic(entity.id) : await bbsApi.getArticle(entity.id);
    const post = data?.topic ? topicToPost(data.topic, auth) : data?.article ? articleToPost(data.article, auth) : null;
    if (!post) {
      return null;
    }
    return {
      ...post,
      liked: mode === "likes" ? true : post.liked,
      favorited: mode === "favorites" ? true : post.favorited,
      interactionAt: item?.created_at || item?.createdAt
    };
  } catch {
    return null;
  }
}
