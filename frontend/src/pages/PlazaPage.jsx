import React from "react";
import { bbsApi } from "../api";
import Composer from "../components/feed/Composer.jsx";
import { FeedStatus, FeedToolbar, SearchResultBar } from "../components/feed/FeedChrome.jsx";
import PostCard from "../components/post/PostCard.jsx";
import ProfilePreview from "../components/ProfilePreview.jsx";
import { people } from "../data/communityData";
import { toNumber } from "../lib/formatters";
import { feedItemToPost, hydratePostsMeta, topicToPost, uniquePosts } from "../lib/postMappers";

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
  const [message, setMessage] = React.useState("");
  const [loadFailed, setLoadFailed] = React.useState(false);
  const [reloadKey, setReloadKey] = React.useState(0);

  React.useEffect(() => {
    let alive = true;
    setLoading(true);
    setLoadFailed(false);
    Promise.allSettled([
      bbsApi.feed({ limit: 20, offset: 0, sort: feedSort === "hot" ? "hot" : undefined }),
      bbsApi.listTopics({
          limit: 20,
          offset: 0,
          type: feedSort === "hot" ? "" : "topic",
          category_id: categoryFilter || undefined
      })
    ])
      .then(async ([feedResult, topicResult]) => {
        if (!alive) return;
        const failures = [feedResult, topicResult].filter((result) => result.status === "rejected");
        const feedData = feedResult.status === "fulfilled" ? feedResult.value : { items: [] };
        const topicData = topicResult.status === "fulfilled" ? topicResult.value : { items: [] };
        const projected = (feedData?.items || []).map((item) => feedItemToPost(item, auth));
        const topics = (topicData?.items || []).map((item) => topicToPost(item, auth));
        const items = await hydratePostsMeta(uniquePosts([...projected, ...topics].sort((a, b) => toNumber(b.sortAt) - toNumber(a.sortAt))), auth, {
          skipCounts: true
        });
        if (!alive) return;
        setFeedPosts(items);
        setLoadFailed(failures.length === 2);
        setMessage(
          failures.length === 2
            ? "社区动态加载失败，请检查后端服务后重试。"
            : failures.length > 0
              ? "部分动态加载失败，已展示可用内容。"
              : items.length > 0
                ? ""
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
  }, [auth, categoryFilter, feedSort, reloadKey]);

  function handlePublished(topic) {
    if (feedSort === "latest") {
      setFeedPosts((current) => [topicToPost(topic, auth), ...current]);
    }
    setMessage(feedSort === "latest" ? "帖子已发布，已进入话题流。" : "帖子已发布，可切换最新查看。");
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
            key={post.id || `${post.author.handle}-${index}`}
            post={post}
            index={index}
            onPostArchived={handlePostArchived}
            onPostStatsChange={handlePostStatsChange}
          />
        ))}
        {visiblePosts.length === 0 && !searchState?.loading && !message && <FeedStatus text="没有找到内容。" />}
        <ProfilePreview person={people[0]} />
      </section>
      <RightColumn activePage={activePage} categories={categories} hotTags={hotTags} />
    </main>
  );
}
