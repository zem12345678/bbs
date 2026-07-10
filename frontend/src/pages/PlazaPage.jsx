import React from "react";
import { bbsApi } from "../api";
import Composer from "../components/feed/Composer.jsx";
import { FeedStatus, FeedToolbar, SearchResultBar } from "../components/feed/FeedChrome.jsx";
import PostCard from "../components/post/PostCard.jsx";
import ProfilePreview from "../components/ProfilePreview.jsx";
import { people } from "../data/communityData";
import { toNumber } from "../lib/formatters";
import { feedItemToPost, hydratePostsMeta, topicToPost, uniquePosts } from "../lib/postMappers";

const FEED_PAGE_SIZE = 20;

export default function PlazaPage({
  activePage,
  auth,
  categories,
  hotTags,
  LeftColumn,
  onClearSearch,
  RightColumn,
  searchState
}) {
  const [feedPosts, setFeedPosts] = React.useState([]);
  const [feedSort, setFeedSort] = React.useState("latest");
  const [categoryFilter, setCategoryFilter] = React.useState(0);
  const [loading, setLoading] = React.useState(false);
  const [loadingMore, setLoadingMore] = React.useState(false);
  const [hasMore, setHasMore] = React.useState(false);
  const [pageOffset, setPageOffset] = React.useState(FEED_PAGE_SIZE);
  const [message, setMessage] = React.useState("");
  const [loadFailed, setLoadFailed] = React.useState(false);
  const [reloadKey, setReloadKey] = React.useState(0);

  const loadFeedPage = React.useCallback(
    async (offset) => {
      const requests = [];
      if (feedSort === "follow") {
        requests.push({
          kind: "feed",
          promise: bbsApi.feed({ limit: FEED_PAGE_SIZE, offset, sort: "follow" }, auth?.accessToken)
        });
      } else {
        if (!categoryFilter) {
          const feedSortParam = feedSort === "hot" || feedSort === "active" ? feedSort : undefined;
          requests.push({
            kind: "feed",
            promise: bbsApi.feed({ limit: FEED_PAGE_SIZE, offset, sort: feedSortParam })
          });
        }
        requests.push({
          kind: "topics",
          promise: bbsApi.listTopics({
            limit: FEED_PAGE_SIZE,
            offset,
            type: feedSort === "hot" || feedSort === "active" ? "" : "topic",
            category_id: categoryFilter || undefined
          })
        });
      }

      const results = await Promise.allSettled(requests.map((request) => request.promise));
      const failures = results.filter((result) => result.status === "rejected");
      const projected = [];
      const hasMoreData = results.some((result) => result.status === "fulfilled" && (result.value?.items || []).length >= FEED_PAGE_SIZE);
      results.forEach((result, index) => {
        if (result.status !== "fulfilled") return;
        const mapper = requests[index].kind === "topics" ? topicToPost : feedItemToPost;
        projected.push(...(result.value?.items || []).map((item) => mapper(item, auth)));
      });
      const sortField = feedSort === "active" ? "activeAt" : "sortAt";
      const items = await hydratePostsMeta(
        uniquePosts(projected.sort((a, b) => toNumber(b[sortField] ?? b.sortAt) - toNumber(a[sortField] ?? a.sortAt))),
        auth,
        {
          skipCounts: true
        }
      );
      return { failures, hasMoreData, items, requestCount: requests.length };
    },
    [auth, categoryFilter, feedSort]
  );

  React.useEffect(() => {
    let alive = true;
    setLoading(true);
    setLoadingMore(false);
    setLoadFailed(false);
    setHasMore(false);
    setPageOffset(FEED_PAGE_SIZE);
    if (feedSort === "follow" && !auth?.accessToken) {
      setFeedPosts([]);
      setMessage("登录后查看关注动态。");
      setLoading(false);
      return () => {
        alive = false;
      };
    }

    loadFeedPage(0)
      .then(({ failures, hasMoreData, items, requestCount }) => {
        if (!alive) return;
        setFeedPosts(items);
        setHasMore(hasMoreData);
        setPageOffset(FEED_PAGE_SIZE);
        setLoadFailed(failures.length === requestCount);
        setMessage(
          failures.length === requestCount
            ? "社区动态加载失败，请检查后端服务后重试。"
            : failures.length > 0
              ? "部分动态加载失败，已展示可用内容。"
              : items.length > 0
                ? ""
                : feedSort === "follow"
                  ? "暂无关注动态，先关注感兴趣的作者。"
                  : feedSort === "active"
                    ? "暂无活跃讨论，回复或发布内容后会出现在这里。"
                    : "暂无帖子，发布第一条内容。"
        );
      })
      .catch((error) => {
        if (!alive) return;
        setFeedPosts([]);
        setLoadFailed(true);
        setMessage(`社区动态加载失败，请稍后重试。${error.message ? `(${error.message})` : ""}`);
      })
      .finally(() => {
        if (alive) setLoading(false);
      });

    return () => {
      alive = false;
    };
  }, [auth, categoryFilter, feedSort, loadFeedPage, reloadKey]);

  async function handleLoadMore() {
    if (loading || loadingMore || !hasMore || searchState?.query) {
      return;
    }
    setLoadingMore(true);
    setMessage("");
    try {
      const { failures, hasMoreData, items, requestCount } = await loadFeedPage(pageOffset);
      if (failures.length === requestCount) {
        setMessage("更多动态加载失败，请稍后重试。");
        return;
      }
      const mergedItems = uniquePosts([...feedPosts, ...items]);
      const appendedCount = Math.max(0, mergedItems.length - feedPosts.length);
      setFeedPosts(mergedItems);
      setHasMore(hasMoreData);
      setPageOffset((offset) => offset + FEED_PAGE_SIZE);
      setMessage(
        failures.length > 0
          ? "部分更多动态加载失败，已追加可用内容。"
          : appendedCount > 0
            ? ""
            : "没有更多内容了。"
      );
    } catch (error) {
      setMessage(`更多动态加载失败，请稍后重试。${error.message ? `(${error.message})` : ""}`);
    } finally {
      setLoadingMore(false);
    }
  }

  function handlePublished(topic) {
    if (feedSort === "latest" || feedSort === "active") {
      setFeedPosts((current) => [topicToPost(topic, auth), ...current]);
    }
    setMessage(feedSort === "latest" || feedSort === "active" ? "帖子已发布，已进入当前动态。" : "帖子已发布，可切换最新查看。");
  }

  function handlePostStatsChange(postId, stats) {
    setFeedPosts((current) => current.map((item) => (String(item.id) === String(postId) ? { ...item, ...stats } : item)));
  }

  function handlePostArchived(postId, postKind) {
    setFeedPosts((current) => current.filter((item) => String(item.id) !== String(postId) || (postKind && item.kind !== postKind)));
    setMessage("内容已归档。");
  }

  const visiblePosts = searchState?.query ? searchState.items || [] : feedPosts;

  return (
    <main className="page-grid">
      <LeftColumn
        activeCategoryId={categoryFilter}
        activePage={activePage}
        categories={categories}
        feedSort={feedSort}
        hotTags={hotTags}
        onCategoryChange={setCategoryFilter}
        onFeedSortChange={setFeedSort}
      />
      <section className="feed" aria-label="社区动态">
        {!searchState?.query && <FeedToolbar loading={loading} sort={feedSort} onSortChange={setFeedSort} />}
        <Composer auth={auth} categories={categories} onPublished={handlePublished} />
        {searchState?.query && (
          <SearchResultBar
            error={searchState.error}
            loading={searchState.loading}
            query={searchState.query}
            total={visiblePosts.length}
            onClear={onClearSearch}
          />
        )}
        {!searchState?.query && loading && <FeedStatus text="正在从后端加载社区动态..." />}
        {!searchState?.query && message && (
          <FeedStatus
            text={message}
            actionLabel={loadFailed ? "重试" : undefined}
            onAction={loadFailed ? () => setReloadKey((value) => value + 1) : undefined}
          />
        )}
        {visiblePosts.map((post, index) => (
          <PostCard
            auth={auth}
            categories={categories}
            key={post.id || `${post.author.handle}-${index}`}
            post={post}
            index={index}
            onPostArchived={handlePostArchived}
            onPostStatsChange={handlePostStatsChange}
          />
        ))}
        {!searchState?.query && visiblePosts.length > 0 && hasMore && !loading && (
          <FeedStatus
            text={loadingMore ? "正在加载更多动态..." : "继续浏览更早的社区动态。"}
            actionLabel={loadingMore ? undefined : "加载更多"}
            onAction={loadingMore ? undefined : handleLoadMore}
          />
        )}
        {visiblePosts.length === 0 && !searchState?.loading && !message && <FeedStatus text="没有找到内容。" />}
        <ProfilePreview person={people[0]} />
      </section>
      <RightColumn activePage={activePage} categories={categories} hotTags={hotTags} />
    </main>
  );
}
