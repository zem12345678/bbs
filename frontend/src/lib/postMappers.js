import { bbsApi } from "../api";
import { people } from "../data/communityData";
import { sameId, timeAgoMillis, toId, toNumber } from "./formatters";
import { markdownImageUrls, textWithoutMarkdownImages } from "./markdownMedia";

export function userAvatar(user) {
  return user?.avatar_url || user?.avatarUrl || people[0].avatar;
}

export function userDisplayName(user) {
  return user?.nickname || user?.username || "社区成员";
}

export function userToPerson(user, fallback = people[0]) {
  return {
    id: user?.id,
    name: userDisplayName(user),
    handle: user?.username || fallback.handle || `user-${user?.id || "unknown"}`,
    role: "社区成员",
    bio: user?.bio || fallback.bio || "正在参与社区讨论",
    avatar: user?.avatar_url || user?.avatarUrl || fallback.avatar || people[0].avatar,
    background: user?.background_url || user?.backgroundUrl || fallback.background || "",
    backgroundUrl: user?.background_url || user?.backgroundUrl || fallback.backgroundUrl || "",
    followerCount: toNumber(user?.follower_count ?? user?.followerCount),
    followingCount: toNumber(user?.following_count ?? user?.followingCount)
  };
}

function authToPerson(auth) {
  return userToPerson(auth?.user);
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

function articleAuthor(article, auth) {
  const authorId = toId(article?.author_id ?? article?.authorId);
  if (sameId(authorId, auth?.user?.id)) {
    return authToPerson(auth);
  }
  const numericAuthorId = toNumber(authorId);
  const fallback = people[numericAuthorId ? numericAuthorId % people.length : 0];
  return {
    ...fallback,
    id: authorId || fallback.id,
    name: authorId ? `用户 #${authorId}` : fallback.name,
    handle: authorId ? `u${authorId}` : fallback.handle,
    role: authorId ? "社区成员" : fallback.role
  };
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
    comments: toNumber(article?.comment_count ?? article?.commentCount),
    views: optionalNumber(article?.view_count, article?.viewCount),
    liked: false,
    favorited: false
  };
}

export function topicToPost(topic, auth) {
  const timestamp = topic?.published_at || topic?.publishedAt || topic?.created_at || topic?.createdAt;
  const activeTimestamp = topic?.updated_at || topic?.updatedAt || timestamp;
  const body = topic?.body || topic?.content_excerpt || topic?.contentExcerpt || topic?.title || "";
  const images = uniqueImages(markdownImageUrls(body));
  return {
    id: topic?.id,
    kind: "topic",
    authorId: toId(topic?.author_id ?? topic?.authorId),
    title: topic?.title || "未命名帖子",
    author: articleAuthor(topic, auth),
    level: (topic?.type || topic?.summary) === "tweet" ? "动态" : "话题",
    time: timeAgoMillis(timestamp),
    sortAt: toNumber(timestamp),
    activeAt: toNumber(activeTimestamp),
    text: textWithoutMarkdownImages(body),
    images: images.length > 0 ? images : undefined,
    tags: topic?.tags || topic?.tag_names || topic?.tagNames || [],
    categoryId: toNumber(topic?.category_id ?? topic?.categoryId),
    likes: toNumber(topic?.like_count ?? topic?.likeCount),
    favorites: toNumber(topic?.favorite_count ?? topic?.favoriteCount),
    comments: toNumber(topic?.comment_count ?? topic?.commentCount),
    views: optionalNumber(topic?.view_count, topic?.viewCount),
    liked: false,
    favorited: false
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

async function hydratePostAuthor(post, auth) {
  if (!post?.authorId) {
    return post;
  }
  if (sameId(post.authorId, auth?.user?.id)) {
    return { ...post, author: authToPerson(auth) };
  }
  const data = await bbsApi.getUser(post.authorId).catch(() => null);
  if (!data?.user) {
    return post;
  }
  return { ...post, author: userToPerson(data.user, post.author) };
}

async function hydratePostMeta(post, auth, options = {}) {
  const [withCounts, withAuthor] = await Promise.all([
    options.skipCounts ? Promise.resolve(post) : hydratePostCounts(post),
    hydratePostAuthor(post, auth)
  ]);
  return { ...withCounts, author: withAuthor.author };
}

export async function hydratePostsMeta(items, auth, options = {}) {
  return Promise.all(items.map((item) => hydratePostMeta(item, auth, options)));
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
