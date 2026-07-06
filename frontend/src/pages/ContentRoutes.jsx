import React from "react";
import { useLocation, useNavigate, useParams, useSearchParams } from "react-router-dom";
import { Clock3, Edit3, FileText, Flame, Hash, MessageCircle, Plus, Search } from "lucide-react";
import { bbsApi } from "../api";
import PostCard from "../components/post/PostCard.jsx";
import { listItems } from "../lib/apiShapes";
import { sameId, toNumber } from "../lib/formatters";
import { articleToPost, hydratePostsMeta, searchHitToPost, topicSearchHitToPost, topicToPost, uniquePosts } from "../lib/postMappers";
import { makeSlug } from "../lib/slugs";
import { EmptyState, PillTabs, RouteHeader } from "./RouteBlocks.jsx";

const sortTabs = [
  { value: "latest", label: "最新", icon: Clock3 },
  { value: "hot", label: "热门", icon: Flame }
];

const CONTENT_PAGE_SIZE = 20;
const SEARCH_PAGE_SIZE = 20;

export function ContentListPage({ auth, categories = [], filter = "all", kind = "topic" }) {
  const params = useParams();
  const navigate = useNavigate();
  const [sort, setSort] = React.useState("latest");
  const [reloadKey, setReloadKey] = React.useState(0);
  const [state, setState] = React.useState({
    posts: [],
    loading: false,
    loadingMore: false,
    hasMore: false,
    offset: CONTENT_PAGE_SIZE,
    message: "",
    footerMessage: "",
    error: false
  });
  const isArticle = kind === "article";
  const routeTitle = isArticle ? "文章" : "话题";

  const loadPage = React.useCallback(
    async (offset) => {
      const query = {
        limit: CONTENT_PAGE_SIZE,
        offset,
        sort: sort === "hot" ? "hot" : undefined
      };
      if (filter === "category") {
        query.category_id = toNumber(params.id);
      }
      if (filter === "tag") {
        query.tag = decodeURIComponent(params.id || "");
      }
      const loader = isArticle ? bbsApi.listArticles : bbsApi.listTopics;
      const data = await loader(query);
      const mapper = isArticle ? articleToPost : topicToPost;
      const rawItems = listItems(data);
      const items = await hydratePostsMeta(rawItems.map((item) => mapper(item, auth)), auth, {
        skipCounts: true
      });
      return { hasMore: rawItems.length >= CONTENT_PAGE_SIZE, items };
    },
    [auth, filter, isArticle, params.id, sort]
  );

  React.useEffect(() => {
    let alive = true;
    setState((current) => ({
      ...current,
      loading: true,
      loadingMore: false,
      hasMore: false,
      offset: CONTENT_PAGE_SIZE,
      message: "",
      footerMessage: "",
      error: false
    }));
    loadPage(0)
      .then(({ hasMore, items }) => {
        if (!alive) return;
        setState({
          posts: items,
          loading: false,
          loadingMore: false,
          hasMore,
          offset: CONTENT_PAGE_SIZE,
          message: items.length > 0 ? "" : `暂无${routeTitle}内容。`,
          footerMessage: "",
          error: false
        });
      })
      .catch((error) => {
        if (!alive) return;
        setState({
          posts: [],
          loading: false,
          loadingMore: false,
          hasMore: false,
          offset: CONTENT_PAGE_SIZE,
          message: `${routeTitle}加载失败，请稍后重试。${error.message ? `(${error.message})` : ""}`,
          footerMessage: "",
          error: true
        });
      });
    return () => {
      alive = false;
    };
  }, [loadPage, reloadKey, routeTitle]);

  async function loadMore() {
    if (state.loading || state.loadingMore || !state.hasMore) {
      return;
    }
    setState((current) => ({ ...current, loadingMore: true, footerMessage: "" }));
    try {
      const { hasMore, items } = await loadPage(state.offset);
      setState((current) => {
        const posts = uniquePosts([...current.posts, ...items]);
        const appendedCount = Math.max(0, posts.length - current.posts.length);
        return {
          ...current,
          posts,
          loadingMore: false,
          hasMore: appendedCount > 0 ? hasMore : false,
          offset: current.offset + CONTENT_PAGE_SIZE,
          footerMessage: appendedCount > 0 ? "" : "没有更多内容了。"
        };
      });
    } catch (error) {
      setState((current) => ({
        ...current,
        loadingMore: false,
        footerMessage: `更多${routeTitle}加载失败，请稍后重试。${error.message ? `(${error.message})` : ""}`
      }));
    }
  }

  function updatePostStats(postId, stats) {
    setState((current) => ({
      ...current,
      posts: current.posts.map((post) => (String(post.id) === String(postId) ? { ...post, ...stats } : post))
    }));
  }

  function handlePostArchived(postId, postKind) {
    setState((current) => ({
      ...current,
      posts: current.posts.filter((post) => String(post.id) !== String(postId) || (postKind && post.kind !== postKind)),
      message: "内容已归档。"
    }));
  }

  return (
    <>
      <RouteHeader
        icon={isArticle ? FileText : MessageCircle}
        eyebrow={filter === "all" ? "社区内容" : filter === "category" ? "分类内容" : "标签内容"}
        title={`${routeTitle}${filter === "all" ? "列表" : "聚合"}`}
        description={
          isArticle
            ? "沉淀长内容、教程、复盘和资源说明，适合结构化阅读与收藏。"
            : "承接日常讨论、求助、分享和圈子动态，按最新与热度快速浏览。"
        }
        actions={
          <button type="button" onClick={() => navigate(isArticle ? "/article/create" : "/topic/create")}>
            <Plus size={18} aria-hidden="true" />
            发布{routeTitle}
          </button>
        }
      />
      <PillTabs items={sortTabs} label={`${routeTitle}排序`} value={sort} onChange={setSort} />
      {!isArticle && categories.length > 0 && (
        <div className="category-strip panel" aria-label="分类快捷入口">
          <button className={filter === "all" ? "is-active" : ""} type="button" onClick={() => navigate(isArticle ? "/articles" : "/topics")}>
            全部
          </button>
          {categories.slice(0, 8).map((category) => (
            <button
              className={filter === "category" && toNumber(params.id) === category.id ? "is-active" : ""}
              key={category.id}
              type="button"
              onClick={() => navigate(`/topics/category/${category.id}`)}
            >
              <Hash size={14} aria-hidden="true" />
              {category.name}
            </button>
          ))}
        </div>
      )}
      {state.loading && <EmptyState title={`正在加载${routeTitle}...`} description="请稍候" />}
      {!state.loading && state.message && (
        <EmptyState
          title={state.message}
          description={state.error ? "如果后端服务刚启动完成，可以直接重试。" : "发布内容后会自动进入列表。"}
          action={
            state.error ? (
              <button type="button" onClick={() => setReloadKey((value) => value + 1)}>
                重试
              </button>
            ) : null
          }
        />
      )}
      {state.posts.map((post, index) => (
        <PostCard
          auth={auth}
          index={index}
          key={`${post.kind}-${post.id}`}
          post={post}
          onPostArchived={handlePostArchived}
          onPostStatsChange={updatePostStats}
        />
      ))}
      {state.posts.length > 0 && state.hasMore && !state.loading && (
        <EmptyState
          title={state.loadingMore ? `正在加载更多${routeTitle}...` : `继续浏览更早的${routeTitle}。`}
          description={state.loadingMore ? "请稍候" : ""}
          action={
            state.loadingMore ? null : (
              <button type="button" onClick={loadMore}>
                加载更多
              </button>
            )
          }
        />
      )}
      {state.posts.length > 0 && state.footerMessage && !state.loading && (
        <EmptyState title={state.footerMessage} description="" />
      )}
    </>
  );
}

export function ContentDetailPage({ auth, kind = "topic" }) {
  const params = useParams();
  const location = useLocation();
  const navigate = useNavigate();
  const [state, setState] = React.useState({
    post: null,
    loading: false,
    error: ""
  });
  const isArticle = kind === "article";
  const routeTitle = isArticle ? "文章详情" : "话题详情";
  const ownerPost = state.post && sameId(auth?.user?.id, state.post.authorId);
  const focusedCommentId = commentIdFromHash(location.hash);

  React.useEffect(() => {
    let alive = true;
    setState({ post: null, loading: true, error: "" });
    const loader = isArticle ? bbsApi.getArticle : bbsApi.getTopic;
    loader(params.id)
      .then(async (data) => {
        const post = data?.article ? articleToPost(data.article, auth) : data?.topic ? topicToPost(data.topic, auth) : null;
        const hydrated = post ? await hydratePostsMeta([post], auth) : [];
        if (!alive) return;
        setState({
          post: hydrated[0] || null,
          loading: false,
          error: hydrated[0] ? "" : "没有找到对应内容。"
        });
      })
      .catch((error) => {
        if (!alive) return;
        setState({ post: null, loading: false, error: error.message || "详情加载失败" });
      });
    return () => {
      alive = false;
    };
  }, [auth, isArticle, params.id]);

  function updatePostStats(postId, stats) {
    setState((current) => ({
      ...current,
      post: String(current.post?.id) === String(postId) ? { ...current.post, ...stats } : current.post
    }));
  }

  function handlePostArchived() {
    navigate(isArticle ? "/articles" : "/topics");
  }

  return (
    <>
      <RouteHeader
        icon={isArticle ? FileText : MessageCircle}
        eyebrow="内容详情"
        title={routeTitle}
        description="详情页聚合正文、互动、评论和举报入口。"
        actions={
          ownerPost ? (
            <button type="button" onClick={() => navigate(isArticle ? `/article/edit/${params.id}` : `/topic/edit/${params.id}`)}>
              <Edit3 size={18} aria-hidden="true" />
              编辑
            </button>
          ) : null
        }
      />
      {state.loading && <EmptyState title="正在加载详情..." description="请稍候" />}
      {state.error && <EmptyState title={state.error} description="可以返回列表重新选择内容。" />}
      {state.post && (
        <PostCard
          auth={auth}
          focusCommentId={focusedCommentId}
          index={0}
          post={state.post}
          onPostArchived={handlePostArchived}
          onPostStatsChange={updatePostStats}
        />
      )}
    </>
  );
}

function commentIdFromHash(hash) {
  const match = /^#comment-(.+)$/.exec(hash || "");
  return match ? decodeURIComponent(match[1]) : "";
}

export function EditorPage({ auth, categories = [], edit = false, kind = "topic" }) {
  const params = useParams();
  const navigate = useNavigate();
  const isArticle = kind === "article";
  const routeTitle = `${edit ? "编辑" : "发布"}${isArticle ? "文章" : "话题"}`;
  const [form, setForm] = React.useState({
    title: "",
    body: "",
    tags: "",
    cover_url: "",
    category_id: 0,
    publish: true
  });
  const [state, setState] = React.useState({
    loading: false,
    saving: false,
    error: "",
    message: "",
    loadedStatus: 0
  });

  React.useEffect(() => {
    if (categories.length === 0 || form.category_id) {
      return;
    }
    setForm((current) => ({ ...current, category_id: categories[0].id || 0 }));
  }, [categories, form.category_id]);

  React.useEffect(() => {
    if (!edit || !params.id) {
      return;
    }
    let alive = true;
    setState((current) => ({ ...current, loading: true, error: "" }));
    const loader = isArticle ? bbsApi.getArticle : bbsApi.getTopic;
    loader(params.id)
      .then((data) => {
        if (!alive) return;
        const item = data?.article || data?.topic;
        if (!item) {
          setState((current) => ({ ...current, loading: false, error: "没有找到可编辑内容。" }));
          return;
        }
        const status = toNumber(item.status, 1);
        setForm({
          title: item.title || "",
          body: item.body || item.content || "",
          tags: (item.tags || item.tag_names || item.tagNames || []).join(" "),
          cover_url: item.cover_url || item.coverUrl || "",
          category_id: toNumber(item.category_id ?? item.categoryId),
          publish: status === 2
        });
        setState((current) => ({ ...current, loading: false, loadedStatus: status }));
      })
      .catch((error) => {
        if (!alive) return;
        setState((current) => ({ ...current, loading: false, error: error.message || "内容加载失败" }));
      });
    return () => {
      alive = false;
    };
  }, [edit, isArticle, params.id]);

  function updateField(field, value) {
    setForm((current) => ({ ...current, [field]: value }));
  }

  async function submit(event) {
    event.preventDefault();
    if (!auth?.accessToken) {
      setState((current) => ({ ...current, error: "请先登录后再发布内容。" }));
      return;
    }
    const title = form.title.trim();
    const body = form.body.trim();
    if (!title || !body) {
      setState((current) => ({ ...current, error: "标题和正文不能为空。" }));
      return;
    }
    const tags = form.tags
      .split(/[,，\s#]+/)
      .map((tag) => tag.trim())
      .filter(Boolean)
      .slice(0, 8);
    const payload = {
      slug: makeSlug(title),
      type: isArticle ? "article" : "topic",
      title,
      body,
      tags,
      category_id: form.category_id || undefined,
      cover_url: isArticle ? form.cover_url.trim() || undefined : undefined,
      publish: form.publish,
      status: form.publish ? 2 : 1
    };
    setState((current) => ({ ...current, saving: true, error: "", message: "" }));
    try {
      const data = edit
        ? isArticle
          ? await bbsApi.updateArticle(params.id, payload, auth.accessToken)
          : await bbsApi.updateTopic(params.id, payload, auth.accessToken)
        : isArticle
          ? await bbsApi.createArticle(payload, auth.accessToken)
          : await bbsApi.createTopic(payload, auth.accessToken);
      let item = data?.article || data?.topic;
      const id = item?.id || params.id;
      if (edit && form.publish && state.loadedStatus !== 2) {
        const publishedData = isArticle
          ? await bbsApi.publishArticle(id, auth.accessToken)
          : await bbsApi.publishTopic(id, auth.accessToken);
        item = publishedData?.article || publishedData?.topic || item;
      }
      setState((current) => ({
        ...current,
        saving: false,
        loadedStatus: toNumber(item?.status, form.publish ? 2 : current.loadedStatus),
        message: form.publish ? "已发布。" : "已保存为草稿。"
      }));
      navigate(isArticle ? `/article/${id}` : `/topic/${id}`);
    } catch (error) {
      setState((current) => ({ ...current, saving: false, error: error.message || "保存失败" }));
    }
  }

  return (
    <>
      <RouteHeader
        icon={Edit3}
        eyebrow="创作中心"
        title={routeTitle}
        description="支持标题、正文、标签、分类和发布状态。"
      />
      {!auth && <EmptyState title="请先登录" description="登录后可以发布和编辑社区内容。" />}
      {state.loading && <EmptyState title="正在加载内容..." description="请稍候" />}
      <form className="editor-form panel" onSubmit={submit}>
        <input
          className="compose-title"
          placeholder="标题"
          value={form.title}
          onChange={(event) => updateField("title", event.target.value)}
        />
        <textarea
          className="editor-body"
          placeholder="正文内容"
          value={form.body}
          onChange={(event) => updateField("body", event.target.value)}
        />
        <div className="editor-grid">
          <input
            className="compose-tags"
            placeholder="标签，用空格或逗号分隔"
            value={form.tags}
            onChange={(event) => updateField("tags", event.target.value)}
          />
          <select value={form.category_id} onChange={(event) => updateField("category_id", toNumber(event.target.value))}>
            <option value={0}>不关联分类</option>
            {categories.map((category) => (
              <option key={category.id} value={category.id}>
                {category.name}
              </option>
            ))}
          </select>
        </div>
        {isArticle && (
          <input
            className="compose-tags"
            placeholder="封面图片 URL"
            value={form.cover_url}
            onChange={(event) => updateField("cover_url", event.target.value)}
          />
        )}
        <label className="publish-toggle">
          <input
            checked={form.publish}
            disabled={edit && state.loadedStatus === 2}
            type="checkbox"
            onChange={(event) => updateField("publish", event.target.checked)}
          />
          {edit && state.loadedStatus === 2 ? "已发布，保存后保持发布" : "立即发布"}
        </label>
        {state.error && <p className="form-error">{state.error}</p>}
        {state.message && <p className="form-success">{state.message}</p>}
        <div className="editor-actions">
          <button type="button" onClick={() => navigate(-1)}>
            取消
          </button>
          <button type="submit" disabled={!auth || state.saving}>
            {state.saving ? "保存中..." : form.publish ? "发布" : "保存草稿"}
          </button>
        </div>
      </form>
    </>
  );
}

export function SearchPage({ auth }) {
  const navigate = useNavigate();
  const [searchParams] = useSearchParams();
  const query = searchParams.get("q") || "";
  const [input, setInput] = React.useState(query);
  const [state, setState] = React.useState({
    posts: [],
    loading: false,
    loadingMore: false,
    hasMore: false,
    page: 2,
    footerMessage: "",
    error: ""
  });

  const loadSearchPage = React.useCallback(
    async (page) => {
      const [topicData, articleData] = await Promise.all([
        bbsApi.searchTopics(query, { page, page_size: SEARCH_PAGE_SIZE }).catch(() => ({ items: [] })),
        bbsApi.searchArticles(query, { page, page_size: SEARCH_PAGE_SIZE }).catch(() => ({ items: [] }))
      ]);
      const topicItems = listItems(topicData);
      const articleItems = listItems(articleData);
      const posts = uniquePosts([
        ...topicItems.map((item) => topicSearchHitToPost(item, auth)),
        ...articleItems.map((item) => searchHitToPost(item, auth))
      ]);
      const hydrated = await hydratePostsMeta(posts, auth);
      return {
        hasMore: topicItems.length >= SEARCH_PAGE_SIZE || articleItems.length >= SEARCH_PAGE_SIZE,
        posts: hydrated
      };
    },
    [auth, query]
  );

  React.useEffect(() => {
    setInput(query);
    if (!query.trim()) {
      setState({ posts: [], loading: false, loadingMore: false, hasMore: false, page: 2, footerMessage: "", error: "" });
      return;
    }
    let alive = true;
    setState({ posts: [], loading: true, loadingMore: false, hasMore: false, page: 2, footerMessage: "", error: "" });
    loadSearchPage(1)
      .then(({ hasMore, posts }) => {
        if (!alive) return;
        setState({ posts, loading: false, loadingMore: false, hasMore, page: 2, footerMessage: "", error: "" });
      })
      .catch((error) => {
        if (!alive) return;
        setState({ posts: [], loading: false, loadingMore: false, hasMore: false, page: 2, footerMessage: "", error: error.message || "搜索失败" });
      });
    return () => {
      alive = false;
    };
  }, [loadSearchPage, query]);

  async function loadMoreSearchResults() {
    if (state.loading || state.loadingMore || !state.hasMore) {
      return;
    }
    setState((current) => ({ ...current, loadingMore: true, footerMessage: "" }));
    try {
      const { hasMore, posts: nextPosts } = await loadSearchPage(state.page);
      setState((current) => {
        const posts = uniquePosts([...current.posts, ...nextPosts]);
        const appendedCount = Math.max(0, posts.length - current.posts.length);
        return {
          ...current,
          posts,
          loadingMore: false,
          hasMore: appendedCount > 0 ? hasMore : false,
          page: current.page + 1,
          footerMessage: appendedCount > 0 ? "" : "没有更多搜索结果了。"
        };
      });
    } catch (error) {
      setState((current) => ({
        ...current,
        loadingMore: false,
        footerMessage: `更多搜索结果加载失败。${error.message ? `(${error.message})` : ""}`
      }));
    }
  }

  function submit(event) {
    event.preventDefault();
    const keyword = input.trim();
    navigate(keyword ? `/search?q=${encodeURIComponent(keyword)}` : "/search");
  }

  function updatePostStats(postId, stats) {
    setState((current) => ({
      ...current,
      posts: current.posts.map((post) => (String(post.id) === String(postId) ? { ...post, ...stats } : post))
    }));
  }

  function handlePostArchived(postId, postKind) {
    setState((current) => ({
      ...current,
      posts: current.posts.filter((post) => String(post.id) !== String(postId) || (postKind && post.kind !== postKind))
    }));
  }

  return (
    <>
      <RouteHeader
        icon={Search}
        eyebrow="全站搜索"
        title="搜索帖子、文章和话题"
        description="输入关键词查找社区里的文章、话题和讨论内容。"
      />
      <form className="search-page-form panel" onSubmit={submit}>
        <Search size={22} aria-hidden="true" />
        <input placeholder="输入关键词" value={input} onChange={(event) => setInput(event.target.value)} />
        <button type="submit">搜索</button>
      </form>
      {state.loading && <EmptyState title="正在搜索..." description={query} />}
      {state.error && <EmptyState title={state.error} />}
      {!state.loading && query && state.posts.length === 0 && <EmptyState title="没有找到内容" description="换个关键词再试试。" />}
      {state.posts.map((post, index) => (
        <PostCard
          auth={auth}
          index={index}
          key={`${post.kind}-${post.id}`}
          post={post}
          onPostArchived={handlePostArchived}
          onPostStatsChange={updatePostStats}
        />
      ))}
      {state.posts.length > 0 && state.hasMore && !state.loading && (
        <EmptyState
          title={state.loadingMore ? "正在加载更多搜索结果..." : "继续查看更多搜索结果。"}
          description={state.loadingMore ? "请稍候" : ""}
          action={
            state.loadingMore ? null : (
              <button type="button" onClick={loadMoreSearchResults}>
                加载更多
              </button>
            )
          }
        />
      )}
      {state.posts.length > 0 && state.footerMessage && !state.loading && (
        <EmptyState title={state.footerMessage} description="" />
      )}
    </>
  );
}
