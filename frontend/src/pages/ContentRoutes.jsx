import React from "react";
import { Link, useLocation, useNavigate, useParams, useSearchParams } from "react-router-dom";
import { Clock3, Crown, Edit3, Eye, FileText, Flame, Hash, ImagePlus, ListChecks, MessageCircle, Plus, Search, Trash2, UserRound } from "lucide-react";
import { bbsApi } from "../api";
import MarkdownPreview from "../components/content/MarkdownPreview.jsx";
import TagAssist from "../components/content/TagAssist.jsx";
import ThreadReader from "../components/content/ThreadReader.jsx";
import TopicAttachments from "../components/content/TopicAttachments.jsx";
import PostCard from "../components/post/PostCard.jsx";
import { listItems } from "../lib/apiShapes";
import { clampBountyScore, publishedBountyMinimum } from "../lib/bounty";
import { isBountyCreditInsufficientError, isMembershipBountyError } from "../lib/contentErrors";
import { clearDraft, readDraft, writeDraft } from "../lib/drafts";
import { digitalEntitlementLookupLimit, isActiveMembershipEntitlement } from "../lib/entitlements";
import { loadListForFocus } from "../lib/focusedLists";
import { compactNumber, sameId, timeAgoMillis, toId, toNumber } from "../lib/formatters";
import { bountyRequiresMembershipForSubmit, membershipBountyGateState } from "../lib/membershipBountyGate";
import { articleToPost, hydratePostsMeta, searchHitToPost, topicSearchHitToPost, topicToPost, uniquePosts, userToPerson } from "../lib/postMappers";
import { hasSearchResults } from "../lib/searchResults";
import { makeSlug } from "../lib/slugs";
import { MAX_POLL_CHOICES, emptyPollDraft, pollDraftFromApi, pollPayloadFromDraft } from "../lib/topicPoll";
import { EmptyState, PillTabs, RouteHeader } from "./RouteBlocks.jsx";

const sortTabs = [
  { value: "latest", label: "最新", icon: Clock3 },
  { value: "hot", label: "热门", icon: Flame }
];

const CONTENT_PAGE_SIZE = 20;
const SEARCH_PAGE_SIZE = 20;
const MEMBERSHIP_BOUNTY_ERROR = "悬赏问答需要会员权益，请先兑换会员月卡。";
const BOUNTY_CREDIT_ERROR = "悬赏积分不足，请先补足积分余额。";

function emptyEditorForm() {
  return {
    title: "",
    body: "",
    tags: "",
    cover_url: "",
    category_id: "",
    bounty_score: 0,
    poll: emptyPollDraft(),
    publish: true
  };
}

function hasEditorDraftContent(form) {
  return Boolean(
    form?.title?.trim() ||
      form?.body?.trim() ||
      form?.tags?.trim() ||
      form?.cover_url?.trim() ||
      form?.poll?.enabled
  );
}

function emptyMembershipGate() {
  return {
    loading: false,
    checked: false,
    active: false,
    count: 0,
    error: ""
  };
}

export function ContentListPage({ auth, categories = [], filter = "all", kind = "topic" }) {
  const params = useParams();
  const navigate = useNavigate();
  const [searchParams, setSearchParams] = useSearchParams();
  const keyword = searchParams.get("q")?.trim() || "";
  const [keywordInput, setKeywordInput] = React.useState(keyword);
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
  const listSearchEnabled = filter === "all";
  const activeKeyword = listSearchEnabled ? keyword : "";

  const loadPage = React.useCallback(
    async (offset) => {
      if (activeKeyword) {
        const page = Math.floor(offset / CONTENT_PAGE_SIZE) + 1;
        const data = isArticle
          ? await bbsApi.searchArticles(activeKeyword, { page, page_size: CONTENT_PAGE_SIZE })
          : await bbsApi.searchTopics(activeKeyword, { page, page_size: CONTENT_PAGE_SIZE });
        const mapper = isArticle ? searchHitToPost : topicSearchHitToPost;
        const rawItems = listItems(data);
        const items = await hydratePostsMeta(rawItems.map((item) => mapper(item, auth)), auth);
        return { hasMore: rawItems.length >= CONTENT_PAGE_SIZE, items };
      }
      const query = {
        limit: CONTENT_PAGE_SIZE,
        offset,
        sort: sort === "hot" ? "hot" : undefined
      };
      if (filter === "category") {
        query.category_id = toId(params.id);
      }
      if (filter === "tag") {
        query.tag = decodeURIComponent(params.id || "");
      }
      if (!isArticle) {
        query.type = kind === "question" ? "qa" : "topic";
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
    [activeKeyword, auth, filter, isArticle, kind, params.id, sort]
  );

  React.useEffect(() => {
    setKeywordInput(keyword);
  }, [keyword]);

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
          message: items.length > 0 ? "" : activeKeyword ? `没有找到匹配的${routeTitle}。` : `暂无${routeTitle}内容。`,
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
          message: `${routeTitle}${activeKeyword ? "搜索" : "加载"}失败，请稍后重试。${error.message ? `(${error.message})` : ""}`,
          footerMessage: "",
          error: true
        });
      });
    return () => {
      alive = false;
    };
  }, [activeKeyword, loadPage, reloadKey, routeTitle]);

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

  function submitListSearch(event) {
    event.preventDefault();
    const next = new URLSearchParams(searchParams);
    const value = keywordInput.trim();
    if (value) {
      next.set("q", value);
    } else {
      next.delete("q");
    }
    setSearchParams(next);
  }

  function clearListSearch() {
    setKeywordInput("");
    const next = new URLSearchParams(searchParams);
    next.delete("q");
    setSearchParams(next, { replace: true });
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
      {listSearchEnabled && (
        <form className="search-page-form panel" role="search" onSubmit={submitListSearch}>
          <Search size={22} aria-hidden="true" />
          <input
            aria-label={`搜索${routeTitle}`}
            placeholder={`搜索${routeTitle}关键词`}
            value={keywordInput}
            onChange={(event) => setKeywordInput(event.target.value)}
          />
          <button type="submit">搜索</button>
          {activeKeyword && <button type="button" onClick={clearListSearch}>清除</button>}
        </form>
      )}
      {!activeKeyword && <PillTabs items={sortTabs} label={`${routeTitle}排序`} value={sort} onChange={setSort} />}
      {!isArticle && categories.length > 0 && (
        <div className="category-strip panel" aria-label="分类快捷入口">
          <button className={filter === "all" ? "is-active" : ""} type="button" onClick={() => navigate(isArticle ? "/articles" : "/topics")}>
            全部
          </button>
          {categories.slice(0, 8).map((category) => (
            <button
              className={filter === "category" && sameId(params.id, category.id) ? "is-active" : ""}
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
      {state.loading && <EmptyState title={`正在${activeKeyword ? "搜索" : "加载"}${routeTitle}...`} description="请稍候" />}
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
      {!isArticle && state.posts.length > 0 ? (
        <TopicDirectory categories={categories} posts={state.posts} />
      ) : (
        state.posts.map((post, index) => (
          <PostCard
            auth={auth}
            categories={categories}
            index={index}
            key={`${post.kind}-${post.id}`}
            post={post}
            onPostArchived={handlePostArchived}
            onPostStatsChange={updatePostStats}
          />
        ))
      )}
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

function TopicDirectory({ categories = [], posts = [] }) {
  return (
    <section className="topic-directory panel" aria-label="话题目录">
      <header className="topic-directory-head">
        <span>话题</span>
        <div className="topic-directory-metrics" aria-hidden="true">
          <span>回复</span>
          <span>浏览</span>
          <span>活跃</span>
        </div>
      </header>
      <div className="topic-directory-list">
        {posts.map((post) => {
          const categoryId = toId(post.categoryId);
          const category = categoryId ? categories.find((item) => sameId(item.id, categoryId)) : null;
          const detailPath = `/topic/${post.id}`;
          const activityAt = toNumber(post.activeAt || post.sortAt);
          return (
            <article className="topic-directory-row" key={`${post.kind}-${post.id}`}>
              <div className="topic-directory-main">
                <h2>
                  <Link to={detailPath}>{post.title || "未命名话题"}</Link>
                </h2>
                {post.text && <p>{post.text}</p>}
                <div className="topic-directory-meta">
                  {categoryId ? (
                    <Link className="topic-directory-category" to={`/topics/category/${categoryId}`}>
                      <Hash size={13} aria-hidden="true" />
                      {category?.name || `分类 #${categoryId}`}
                    </Link>
                  ) : null}
                  {(post.tags || []).slice(0, 4).map((tag) => (
                    <Link to={`/topics/tag/${encodeURIComponent(tag)}`} key={tag}>
                      #{tag}
                    </Link>
                  ))}
                  <span>{post.author?.name || "社区成员"} · {post.time}</span>
                </div>
              </div>
              <div className="topic-directory-metrics topic-directory-row-metrics">
                <span>
                  <strong>{post.comments === null || post.comments === undefined ? "-" : compactNumber(post.comments)}</strong>
                  <em>回复</em>
                </span>
                <span>
                  <strong>{post.views === null || post.views === undefined ? "-" : compactNumber(post.views)}</strong>
                  <em>浏览</em>
                </span>
                <span>
                  <strong>{activityAt ? timeAgoMillis(activityAt) : post.time}</strong>
                  <em>活跃</em>
                </span>
              </div>
            </article>
          );
        })}
      </div>
    </section>
  );
}

export function ContentDetailPage({ auth, kind = "topic" }) {
  const params = useParams();
  const location = useLocation();
  const navigate = useNavigate();
  const [state, setState] = React.useState({
    item: null,
    post: null,
    loading: false,
    error: ""
  });
  const isArticle = kind === "article";
  const routeTitle = isArticle ? "文章详情" : "话题详情";
  const ownerPost = state.post && sameId(auth?.user?.id, state.post.authorId);
  const focusedCommentId = commentIdFromHash(location.hash);
  const editPath = isArticle
    ? `/article/edit/${params.id}`
    : String(state.item?.type || state.post?.topicType || "").toLowerCase() === "qa"
      ? `/question/edit/${params.id}`
      : `/topic/edit/${params.id}`;

  React.useEffect(() => {
    let alive = true;
    setState({ item: null, post: null, loading: true, error: "" });
    const loader = isArticle ? bbsApi.getArticle : bbsApi.getTopic;
    loader(params.id, isArticle ? undefined : auth?.accessToken)
      .then(async (data) => {
        const item = data?.article || data?.topic || null;
        const post = data?.article ? articleToPost(data.article, auth) : data?.topic ? topicToPost(data.topic, auth) : null;
        const hydrated = post ? await hydratePostsMeta([post], auth) : [];
        if (!alive) return;
        setState({
          item,
          post: hydrated[0] || null,
          loading: false,
          error: hydrated[0] ? "" : "没有找到对应内容。"
        });
      })
      .catch((error) => {
        if (!alive) return;
        setState({ item: null, post: null, loading: false, error: error.message || "详情加载失败" });
      });
    return () => {
      alive = false;
    };
  }, [auth, isArticle, params.id]);

  const updatePostStats = React.useCallback((postId, stats) => {
    setState((current) => ({
      ...current,
      post: String(current.post?.id) === String(postId) ? { ...current.post, ...stats } : current.post
    }));
  }, []);

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
            <button type="button" onClick={() => navigate(editPath)}>
              <Edit3 size={18} aria-hidden="true" />
              编辑
            </button>
          ) : null
        }
      />
      {state.loading && <EmptyState title="正在加载详情..." description="请稍候" />}
      {state.error && <EmptyState title={state.error} description="可以返回列表重新选择内容。" />}
      {state.post && (
        <ThreadReader
          auth={auth}
          focusedCommentId={focusedCommentId}
          item={state.item}
          kind={kind}
          post={state.post}
          onEdit={() => navigate(editPath)}
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
  const isQuestion = kind === "question";
  const contentLabel = isArticle ? "文章" : isQuestion ? "求助" : "话题";
  const routeTitle = `${edit ? "编辑" : "发布"}${contentLabel}`;
  const [form, setForm] = React.useState(emptyEditorForm);
  const [state, setState] = React.useState({
    loading: false,
    saving: false,
    error: "",
    message: "",
    loadedStatus: 0,
    loadedBountyScore: 0,
    loadedPollVoters: 0
  });
  const [imageUpload, setImageUpload] = React.useState({ loading: "", error: "", message: "" });
  const [membershipGate, setMembershipGate] = React.useState(emptyMembershipGate);
  const [previewOpen, setPreviewOpen] = React.useState(false);
  const [draftReady, setDraftReady] = React.useState(false);
  const publishedBountyFloor = publishedBountyMinimum({
    isQuestion,
    status: state.loadedStatus,
    bountyScore: state.loadedBountyScore
  });
  const bountyScore = isQuestion ? clampBountyScore(form.bounty_score, publishedBountyFloor) : 0;
  const bountyNeedsMembership = isQuestion && bountyScore > 0;
  const bountyGateState = membershipBountyGateState(bountyNeedsMembership, membershipGate);
  const bountyRequiresMembershipForCurrentSubmit = bountyRequiresMembershipForSubmit({
    needsMembership: bountyNeedsMembership,
    edit,
    loadedStatus: state.loadedStatus,
    publish: form.publish
  });
  const bountySubmissionBlocked =
    bountyRequiresMembershipForCurrentSubmit && bountyGateState.blocked;
  const draftDirtyRef = React.useRef(false);
  const draftKey = React.useMemo(
    () => `bbs:editor:${kind}:${edit ? params.id || "unknown" : "new"}:${auth?.user?.id || "guest"}:v1`,
    [auth?.user?.id, edit, kind, params.id]
  );

  React.useEffect(() => {
    if (!isQuestion || !auth?.accessToken) {
      setMembershipGate(emptyMembershipGate());
      return;
    }
    let alive = true;
    setMembershipGate((current) => ({ ...current, loading: true, error: "" }));
    loadListForFocus(
      bbsApi.mallDigitalEntitlements,
      { status: "ACTIVE", grant_type: "membership", limit: digitalEntitlementLookupLimit, offset: 0 },
      auth.accessToken,
      "membership",
      (entitlement) => isActiveMembershipEntitlement(entitlement),
      null,
      { focusLimit: digitalEntitlementLookupLimit }
    )
      .then((data) => {
        if (!alive) return;
        const memberships = listItems(data).filter(isActiveMembershipEntitlement);
        setMembershipGate({
          loading: false,
          checked: true,
          active: memberships.length > 0,
          count: memberships.length,
          error: ""
        });
      })
      .catch((error) => {
        if (!alive) return;
        setMembershipGate({
          loading: false,
          checked: false,
          active: false,
          count: 0,
          error: error.message || "会员权益同步失败"
        });
      });
    return () => {
      alive = false;
    };
  }, [auth?.accessToken, isQuestion]);

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
    if (!auth?.accessToken) {
      setDraftReady(true);
      setState((current) => ({ ...current, loading: false, error: "请先登录后再编辑内容。", message: "" }));
      return;
    }
    let alive = true;
    setDraftReady(false);
    setState((current) => ({ ...current, loading: true, error: "", message: "" }));
    const loader = isArticle ? bbsApi.getEditableArticle : bbsApi.getEditableTopic;
    loader(params.id, auth.accessToken)
      .then((data) => {
        if (!alive) return;
        const item = data?.article || data?.topic;
        if (!item) {
          setDraftReady(true);
          setState((current) => ({ ...current, loading: false, error: "没有找到可编辑内容。" }));
          return;
        }
        const status = toNumber(item.status, 1);
        const loadedBountyScore = toNumber(item.bounty_score ?? item.bountyScore);
        const loadedPollVoters = toNumber(item.poll?.total_voters ?? item.poll?.totalVoters);
        const loadedForm = {
          title: item.title || "",
          body: item.body || item.content || "",
          tags: (item.tags || item.tag_names || item.tagNames || []).join(" "),
          cover_url: item.cover_url || item.coverUrl || "",
          category_id: toId(item.category_id ?? item.categoryId),
          bounty_score: loadedBountyScore,
          poll: pollDraftFromApi(item.poll),
          publish: status === 2
        };
        const draft = readDraft(draftKey);
        const draftForm = draft?.form && hasEditorDraftContent(draft.form) ? { ...loadedForm, ...draft.form } : null;
        const nextForm = draftForm || loadedForm;
        if (loadedPollVoters > 0) {
          nextForm.poll = loadedForm.poll;
        }
        const bountyFloor = publishedBountyMinimum({ isQuestion, status, bountyScore: loadedBountyScore });
        setForm({
          ...nextForm,
          bounty_score: isQuestion ? clampBountyScore(nextForm.bounty_score, bountyFloor) : nextForm.bounty_score
        });
        draftDirtyRef.current = false;
        setDraftReady(true);
        setState((current) => ({
          ...current,
          loading: false,
          loadedStatus: status,
          loadedBountyScore,
          loadedPollVoters,
          message: draftForm ? "已恢复本地草稿。" : ""
        }));
      })
      .catch((error) => {
        if (!alive) return;
        setDraftReady(true);
        setState((current) => ({ ...current, loading: false, error: error.message || "内容加载失败" }));
      });
    return () => {
      alive = false;
    };
  }, [auth?.accessToken, draftKey, edit, isArticle, params.id]);

  React.useEffect(() => {
    if (edit) return;
    setDraftReady(false);
    const draft = readDraft(draftKey);
    if (draft?.form && hasEditorDraftContent(draft.form)) {
      setForm({ ...emptyEditorForm(), ...draft.form });
      setState((current) => ({ ...current, message: "已恢复本地草稿。" }));
    } else {
      setForm(emptyEditorForm());
    }
    setState((current) => ({ ...current, loadedStatus: 0, loadedBountyScore: 0, loadedPollVoters: 0 }));
    draftDirtyRef.current = false;
    setDraftReady(true);
  }, [draftKey, edit]);

  React.useEffect(() => {
    if (!draftReady || !draftDirtyRef.current) return undefined;
    const timer = window.setTimeout(() => {
      if (!draftDirtyRef.current) return;
      if (hasEditorDraftContent(form)) {
        writeDraft(draftKey, { form });
      } else {
        clearDraft(draftKey);
      }
    }, 350);
    return () => window.clearTimeout(timer);
  }, [draftKey, draftReady, form]);

  function updateField(field, value) {
    draftDirtyRef.current = true;
    setForm((current) => ({
      ...current,
      [field]: field === "bounty_score" ? clampBountyScore(value, publishedBountyFloor) : value
    }));
  }

  function updatePollField(field, value) {
    if (state.loadedPollVoters > 0) return;
    draftDirtyRef.current = true;
    setForm((current) => ({
      ...current,
      poll: { ...current.poll, [field]: value, dirty: true }
    }));
  }

  function updatePollChoice(index, value) {
    if (state.loadedPollVoters > 0) return;
    draftDirtyRef.current = true;
    setForm((current) => ({
      ...current,
      poll: {
        ...current.poll,
        choices: current.poll.choices.map((choice, position) => (position === index ? value : choice)),
        dirty: true
      }
    }));
  }

  function addPollChoice() {
    if (form.poll.choices.length >= MAX_POLL_CHOICES) return;
    updatePollField("choices", [...form.poll.choices, ""]);
  }

  function removePollChoice(index) {
    if (form.poll.choices.length <= 2) return;
    updatePollField("choices", form.poll.choices.filter((_, position) => position !== index));
  }

  async function uploadEditorImage(event, target) {
    const file = event.target.files?.[0];
    event.target.value = "";
    if (!file) return;
    if (!auth?.accessToken) {
      setImageUpload({ loading: "", error: "请先登录后再上传图片。", message: "" });
      return;
    }
    setImageUpload({ loading: target, error: "", message: "" });
    setState((current) => ({ ...current, error: "", message: "" }));
    try {
      const data = await bbsApi.uploadImage(file, auth.accessToken);
      const imageUrl = data?.image_url || data?.imageUrl || data?.url || "";
      if (!imageUrl) {
        throw new Error("图片上传成功但未返回地址");
      }
      if (target === "cover") {
        updateField("cover_url", imageUrl);
        setImageUpload({ loading: "", error: "", message: "封面图片已更新。" });
        return;
      }
      draftDirtyRef.current = true;
      setForm((current) => ({ ...current, body: `${current.body.trimEnd()}\n\n![图片](${imageUrl})\n` }));
      setImageUpload({ loading: "", error: "", message: "图片已插入正文。" });
    } catch (error) {
      setImageUpload({ loading: "", error: error.message || "图片上传失败", message: "" });
    }
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
    if (bountySubmissionBlocked) {
      setState((current) => ({
        ...current,
        error:
          bountyGateState.reason === "checking"
            ? "正在校验会员权益，请稍后再发布悬赏。"
            : MEMBERSHIP_BOUNTY_ERROR
      }));
      return;
    }
    const shouldSendPoll = !isArticle && (edit ? Boolean(form.poll?.dirty) : Boolean(form.poll?.enabled));
    const pollResult = shouldSendPoll ? pollPayloadFromDraft(form.poll) : { payload: null, error: "" };
    if (pollResult.error) {
      setState((current) => ({ ...current, error: pollResult.error }));
      return;
    }
    const tags = form.tags
      .split(/[,，\s#]+/)
      .map((tag) => tag.trim())
      .filter(Boolean)
      .slice(0, 8);
    const payload = {
      slug: makeSlug(title),
      type: isArticle ? "article" : isQuestion ? "qa" : "topic",
      title,
      body,
      tags,
      category_id: form.category_id || undefined,
      bounty_score: isQuestion ? bountyScore : undefined,
      cover_url: isArticle ? form.cover_url.trim() || undefined : undefined,
      publish: form.publish,
      status: form.publish ? 2 : 1
    };
    if (shouldSendPoll) {
      payload.poll = pollResult.payload;
    }
    setState((current) => ({ ...current, saving: true, error: "", message: "" }));
    setImageUpload((current) => ({ ...current, message: "" }));
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
        loadedBountyScore: isQuestion ? toNumber(item?.bounty_score ?? item?.bountyScore, bountyScore) : current.loadedBountyScore,
        message: form.publish ? "已发布。" : "已保存为草稿。"
      }));
      clearDraft(draftKey);
      draftDirtyRef.current = false;
      const detailPath = isArticle ? `/article/${id}` : `/topic/${id}`;
      const editPath = isArticle ? `/article/edit/${id}` : isQuestion ? `/question/edit/${id}` : `/topic/edit/${id}`;
      if (form.publish) {
        navigate(detailPath);
      } else {
        navigate(editPath);
      }
    } catch (error) {
      const membershipError = isMembershipBountyError(error);
      const bountyCreditError = isBountyCreditInsufficientError(error);
      if (membershipError) {
        setMembershipGate((current) => ({ ...current, checked: true, active: false, count: 0 }));
      }
      setState((current) => ({
        ...current,
        saving: false,
        error: membershipError ? MEMBERSHIP_BOUNTY_ERROR : bountyCreditError ? BOUNTY_CREDIT_ERROR : error.message || "保存失败"
      }));
    }
  }

  return (
    <>
      <RouteHeader
        icon={Edit3}
        eyebrow="创作中心"
        title={routeTitle}
        description={isQuestion ? "描述问题背景、已尝试方案和期望结果，可设置积分悬赏方便后续采纳结算。" : "支持标题、正文、标签、分类和发布状态。"}
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
        <div className="editor-toolbar">
          <div className="editor-mode-tabs" role="tablist" aria-label="正文编辑模式">
            <button className={!previewOpen ? "is-active" : ""} type="button" onClick={() => setPreviewOpen(false)}>
              <Edit3 size={16} aria-hidden="true" />
              编辑
            </button>
            <button className={previewOpen ? "is-active" : ""} type="button" onClick={() => setPreviewOpen(true)}>
              <Eye size={16} aria-hidden="true" />
              预览
            </button>
          </div>
          <span>{form.body.length} 字</span>
        </div>
        {previewOpen ? (
          <MarkdownPreview className="editor-preview" text={form.body} />
        ) : (
          <textarea
            className="editor-body"
            placeholder="正文内容"
            value={form.body}
            onChange={(event) => updateField("body", event.target.value)}
          />
        )}
        <div className="editor-media-tools">
          <label>
            <input accept="image/jpeg,image/png,image/gif,image/webp" className="sr-only" disabled={Boolean(imageUpload.loading)} type="file" onChange={(event) => uploadEditorImage(event, "body")} />
            <ImagePlus size={17} aria-hidden="true" />
            <span>{imageUpload.loading === "body" ? "上传中..." : "插入正文图片"}</span>
          </label>
          {isArticle && (
            <label>
              <input accept="image/jpeg,image/png,image/gif,image/webp" className="sr-only" disabled={Boolean(imageUpload.loading)} type="file" onChange={(event) => uploadEditorImage(event, "cover")} />
              <ImagePlus size={17} aria-hidden="true" />
              <span>{imageUpload.loading === "cover" ? "上传中..." : "设为封面"}</span>
            </label>
          )}
        </div>
        {!isArticle && edit && state.loadedStatus === 2 && <TopicAttachments auth={auth} canManage topicId={params.id} />}
        <div className="editor-grid">
          <TagAssist
            className="compose-tags"
            placeholder="标签，用空格或逗号分隔"
            value={form.tags}
            onChange={(value) => updateField("tags", value)}
          />
          <select value={form.category_id} onChange={(event) => updateField("category_id", toId(event.target.value))}>
            <option value="">不关联分类</option>
            {categories.map((category) => (
              <option key={category.id} value={category.id}>
                {category.name}
              </option>
            ))}
          </select>
        </div>
        {!isArticle && (
          <section className={`editor-poll ${form.poll.enabled ? "is-enabled" : ""}`.trim()}>
            <header>
              <span><ListChecks size={18} aria-hidden="true" />投票</span>
              <label>
                <input
                  checked={form.poll.enabled}
                  disabled={state.loadedPollVoters > 0}
                  type="checkbox"
                  onChange={(event) => updatePollField("enabled", event.target.checked)}
                />
                添加投票
              </label>
            </header>
            {form.poll.enabled && (
              <div className="editor-poll__body">
                <div className="editor-poll__choices">
                  {form.poll.choices.map((choice, index) => (
                    <div key={index}>
                      <input
                        maxLength="80"
                        placeholder={`选项 ${index + 1}`}
                        value={choice}
                        disabled={state.loadedPollVoters > 0}
                        onChange={(event) => updatePollChoice(index, event.target.value)}
                      />
                      <button
                        aria-label={`删除选项 ${index + 1}`}
                        disabled={state.loadedPollVoters > 0 || form.poll.choices.length <= 2}
                        title="删除选项"
                        type="button"
                        onClick={() => removePollChoice(index)}
                      >
                        <Trash2 size={16} aria-hidden="true" />
                      </button>
                    </div>
                  ))}
                </div>
                <div className="editor-poll__settings">
                  <button disabled={state.loadedPollVoters > 0 || form.poll.choices.length >= MAX_POLL_CHOICES} type="button" onClick={addPollChoice}>
                    <Plus size={16} aria-hidden="true" />增加选项
                  </button>
                  <label>
                    <input
                      checked={form.poll.multiple}
                      disabled={state.loadedPollVoters > 0}
                      type="checkbox"
                      onChange={(event) => updatePollField("multiple", event.target.checked)}
                    />
                    允许多选
                  </label>
                  <label>
                    <span>截止时间</span>
                    <input
                      disabled={state.loadedPollVoters > 0}
                      type="datetime-local"
                      value={form.poll.expires_at}
                      onChange={(event) => updatePollField("expires_at", event.target.value)}
                    />
                  </label>
                </div>
                {state.loadedPollVoters > 0 && <small>已有 {state.loadedPollVoters} 人投票，选项已锁定。</small>}
              </div>
            )}
          </section>
        )}
        {isArticle && (
          <input
            className="compose-tags"
            placeholder="封面图片 URL"
            value={form.cover_url}
            onChange={(event) => updateField("cover_url", event.target.value)}
          />
        )}
        {isQuestion && (
          <div className={`editor-bounty-field ${membershipGate.active ? "has-membership" : bountyNeedsMembership ? "needs-membership" : ""}`.trim()}>
            <label>
              <span>悬赏积分</span>
              <input
                min={publishedBountyFloor}
                placeholder="0 表示不设置悬赏"
                type="number"
                value={form.bounty_score}
                onChange={(event) => updateField("bounty_score", event.target.value)}
              />
            </label>
            <small>
              {publishedBountyFloor > 0
                ? `已发布悬赏最低 ${publishedBountyFloor} 积分`
                : "采纳答案后按悬赏积分奖励答主；未设置悬赏时发布后冻结 10 积分作为基础采纳奖励。"}
            </small>
            <div className={`editor-membership-gate ${membershipGate.active ? "is-active" : bountyNeedsMembership ? "is-warning" : ""}`.trim()}>
              <span>
                <Crown size={16} aria-hidden="true" />
                {membershipGate.loading
                  ? "正在校验会员权益"
                  : membershipGate.active
                    ? `会员权益可用${membershipGate.count > 1 ? ` · ${membershipGate.count} 项` : ""}`
                    : membershipGate.error
                      ? "会员权益暂未同步"
                      : "悬赏需会员权益"}
              </span>
              <button
                type="button"
                disabled={membershipGate.loading}
                onClick={() => navigate(membershipGate.active ? "/member" : "/shop?category=digital&keyword=vip")}
              >
                {membershipGate.active ? "查看权益" : "兑换会员月卡"}
              </button>
            </div>
          </div>
        )}
        {imageUpload.error && <p className="form-error">{imageUpload.error}</p>}
        {imageUpload.message && <p className="form-success">{imageUpload.message}</p>}
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
          <button type="submit" disabled={!auth || state.saving || bountySubmissionBlocked}>
            {state.saving
              ? "保存中..."
              : bountySubmissionBlocked
                ? "会员权益未就绪"
                : form.publish
                  ? "发布"
                  : "保存草稿"}
          </button>
        </div>
      </form>
    </>
  );
}

export function SearchPage({ auth, categories = [] }) {
  const navigate = useNavigate();
  const [searchParams] = useSearchParams();
  const query = searchParams.get("q") || "";
  const [input, setInput] = React.useState(query);
  const [state, setState] = React.useState({
    posts: [],
    users: [],
    hashtags: [],
    loading: false,
    loadingMore: false,
    hasMore: false,
    page: 2,
    notice: "",
    footerMessage: "",
    error: ""
  });

  const loadSearchPage = React.useCallback(
    async (page) => {
      const [topicData, articleData, userData, hashtagData] = await Promise.all([
        bbsApi.searchTopics(query, { page, page_size: SEARCH_PAGE_SIZE }),
        bbsApi.searchArticles(query, { page, page_size: SEARCH_PAGE_SIZE }),
        bbsApi.searchUsers(query, { page, page_size: SEARCH_PAGE_SIZE }),
        bbsApi.searchHashtags(query, { limit: SEARCH_PAGE_SIZE, offset: (page - 1) * SEARCH_PAGE_SIZE })
      ]);
      const topicItems = listItems(topicData);
      const articleItems = listItems(articleData);
      const userItems = listItems(userData);
      const hashtagItems = normalizeSearchHashtags(listItems(hashtagData));
      const posts = uniquePosts([
        ...topicItems.map((item) => topicSearchHitToPost(item, auth)),
        ...articleItems.map((item) => searchHitToPost(item, auth))
      ]);
      const hydrated = await hydratePostsMeta(posts, auth);
      return {
        hasMore:
          topicItems.length >= SEARCH_PAGE_SIZE ||
          articleItems.length >= SEARCH_PAGE_SIZE ||
          userItems.length >= SEARCH_PAGE_SIZE ||
          hashtagItems.length >= SEARCH_PAGE_SIZE,
        posts: hydrated,
        users: userItems.map((item) => userToPerson(item)),
        hashtags: hashtagItems,
        notice: ""
      };
    },
    [auth, query]
  );

  React.useEffect(() => {
    setInput(query);
    if (!query.trim()) {
      setState({ posts: [], users: [], hashtags: [], loading: false, loadingMore: false, hasMore: false, page: 2, notice: "", footerMessage: "", error: "" });
      return;
    }
    let alive = true;
    setState({ posts: [], users: [], hashtags: [], loading: true, loadingMore: false, hasMore: false, page: 2, notice: "", footerMessage: "", error: "" });
    loadSearchPage(1)
      .then(({ hasMore, notice, posts, users, hashtags }) => {
        if (!alive) return;
        setState({ posts, users, hashtags, loading: false, loadingMore: false, hasMore, page: 2, notice: notice || "", footerMessage: "", error: "" });
      })
      .catch((error) => {
        if (!alive) return;
        setState({ posts: [], users: [], hashtags: [], loading: false, loadingMore: false, hasMore: false, page: 2, notice: "", footerMessage: "", error: error.message || "搜索失败" });
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
      const { hasMore, posts: nextPosts, users: nextUsers, hashtags: nextHashtags } = await loadSearchPage(state.page);
      setState((current) => {
        const posts = uniquePosts([...current.posts, ...nextPosts]);
        const users = uniqueSearchUsers([...current.users, ...nextUsers]);
        const hashtags = uniqueSearchHashtags([...current.hashtags, ...nextHashtags]);
        const appendedCount =
          Math.max(0, posts.length - current.posts.length) +
          Math.max(0, users.length - current.users.length) +
          Math.max(0, hashtags.length - current.hashtags.length);
        return {
          ...current,
          posts,
          users,
          hashtags,
          loadingMore: false,
          hasMore: appendedCount > 0 ? hasMore : false,
          page: current.page + 1,
          notice: "",
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

  const hasResults = hasSearchResults(state.posts, state.users) || state.hashtags.length > 0;

  return (
    <>
      <RouteHeader
        icon={Search}
        eyebrow="全站搜索"
        title="搜索帖子、文章和用户"
        description="输入关键词查找社区里的文章、话题、讨论内容和成员。"
      />
      <form className="search-page-form panel" onSubmit={submit}>
        <Search size={22} aria-hidden="true" />
        <input placeholder="输入关键词" value={input} onChange={(event) => setInput(event.target.value)} />
        <button type="submit">搜索</button>
      </form>
      {state.loading && <EmptyState title="正在搜索..." description={query} />}
      {state.error && <EmptyState title={state.error} />}
      {state.notice && !state.loading && <EmptyState title={state.notice} />}
      {!state.loading && query && state.posts.length === 0 && state.users.length === 0 && state.hashtags.length === 0 && <EmptyState title="没有找到内容" description="换个关键词再试试。" />}
      {state.hashtags.length > 0 && (
        <section className="search-hashtag-results panel">
          <header>
            <div>
              <span>相关标签</span>
              <strong>{state.hashtags.length} 个标签</strong>
            </div>
            <Hash size={22} aria-hidden="true" />
          </header>
          <div className="search-hashtag-list">
            {state.hashtags.map((hashtag) => (
              <Link className="search-hashtag-chip" key={hashtag.tag} to={`/topics/tag/${encodeURIComponent(hashtag.tag)}`}>
                <span>#{hashtag.tag}</span>
                <small>{hashtag.count} 条内容</small>
              </Link>
            ))}
          </div>
        </section>
      )}
      {state.users.length > 0 && (
        <section className="search-user-results panel">
          <header>
            <div>
              <span>相关用户</span>
              <strong>{state.users.length} 位成员</strong>
            </div>
            <UserRound size={22} aria-hidden="true" />
          </header>
          <div className="search-user-list">
            {state.users.map((person) => (
              <Link className="search-user-row" key={person.id} to={`/user/${person.id}`}>
                <img src={person.avatar} alt="" />
                <span>
                  <strong>{person.name}</strong>
                  <small>@{person.handle || person.id} · {person.bio || person.role || "社区成员"}</small>
                </span>
                <em>查看主页</em>
              </Link>
            ))}
          </div>
        </section>
      )}
      {state.posts.map((post, index) => (
        <PostCard
          auth={auth}
          categories={categories}
          index={index}
          key={`${post.kind}-${post.id}`}
          post={post}
          onPostArchived={handlePostArchived}
          onPostStatsChange={updatePostStats}
        />
      ))}
      {hasResults && state.hasMore && !state.loading && (
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
      {hasResults && state.footerMessage && !state.loading && (
        <EmptyState title={state.footerMessage} description="" />
      )}
    </>
  );
}

function uniqueSearchUsers(users = []) {
  const seen = new Set();
  const result = [];
  users.forEach((user) => {
    const key = String(user?.id || user?.handle || user?.name || "");
    if (!key || seen.has(key)) return;
    seen.add(key);
    result.push(user);
  });
  return result;
}

function normalizeSearchHashtags(items = []) {
  return uniqueSearchHashtags(
    items
      .map((item) => {
        const tag = String(item?.tag ?? item?.name ?? "").trim().replace(/^#/, "");
        if (!tag) return null;
        return { ...item, tag, count: Number(item?.count ?? item?.mentionedUsersCount ?? 0) || 0 };
      })
      .filter(Boolean)
  );
}

function uniqueSearchHashtags(items = []) {
  const seen = new Set();
  return items.filter((item) => {
    const key = String(item?.tag || "").toLowerCase();
    if (!key || seen.has(key)) return false;
    seen.add(key);
    return true;
  });
}
