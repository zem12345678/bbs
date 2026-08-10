import React from "react";
import { Link, useNavigate, useParams, useSearchParams } from "react-router-dom";
import {
  Archive,
  Clock3,
  Compass,
  FolderKanban,
  Hash,
  MessageCircle,
  Pencil,
  Plus,
  Search,
  Sparkles,
  Star,
  UserCheck,
  Users
} from "lucide-react";
import { bbsApi } from "../api";
import { listItems, listTotal } from "../lib/apiShapes";
import {
  channelCategories,
  channelFromResponse,
  channelList,
  normalizeChannel,
  ownsChannel
} from "../lib/channels";
import { normalizeCategoriesResponse } from "../lib/catalog";
import { compactNumber, sameId, timeAgoMillis, toId } from "../lib/formatters";
import { EmptyState, PillTabs, RouteHeader } from "./RouteBlocks.jsx";
import { CircleCard, PageHero } from "./SectionBlocks.jsx";
import { pageImages } from "./sectionData";

const CHANNEL_PAGE_SIZE = 20;
const CHANNEL_VIEWS = [
  { value: "all", label: "全部", icon: Compass },
  { value: "featured", label: "精选", icon: Sparkles },
  { value: "followed", label: "已关注", icon: UserCheck, auth: true },
  { value: "favorites", label: "已收藏", icon: Star, auth: true },
  { value: "owned", label: "我创建", icon: FolderKanban, auth: true }
];
const PRIVATE_CHANNEL_VIEWS = new Set(CHANNEL_VIEWS.filter((item) => item.auth).map((item) => item.value));

function channelLoader(view) {
  if (view === "featured") return bbsApi.featuredChannels;
  if (view === "followed") return bbsApi.followedChannels;
  if (view === "favorites") return bbsApi.favoriteChannels;
  if (view === "owned") return bbsApi.ownedChannels;
  return bbsApi.channels;
}

function loginPath(target) {
  return `/user/signin?redirect=${encodeURIComponent(target)}`;
}

function channelMutationPatch(channel, kind) {
  if (kind === "follow") {
    const isFollowing = !channel.is_following;
    return {
      is_following: isFollowing,
      followers_count: Math.max(0, channel.followers_count + (isFollowing ? 1 : -1))
    };
  }
  return { is_favorited: !channel.is_favorited };
}

export function CirclesPage({ auth }) {
  const navigate = useNavigate();
  const [searchParams, setSearchParams] = useSearchParams();
  const view = CHANNEL_VIEWS.some((item) => item.value === searchParams.get("view")) ? searchParams.get("view") : "all";
  const query = searchParams.get("q")?.trim() || "";
  const categoryId = toId(searchParams.get("category_id"));
  const [searchDraft, setSearchDraft] = React.useState(query);
  const [categoriesState, setCategoriesState] = React.useState({ items: [], loading: true });
  const [state, setState] = React.useState({ items: [], total: 0, loading: true, loadingMore: false, error: "", notice: "" });
  const [pending, setPending] = React.useState({ id: "", action: "" });

  React.useEffect(() => setSearchDraft(query), [query]);

  React.useEffect(() => {
    let alive = true;
    bbsApi.channelCategories()
      .then((data) => {
        if (alive) setCategoriesState({ items: channelCategories(data), loading: false });
      })
      .catch(() => {
        if (alive) setCategoriesState({ items: [], loading: false });
      });
    return () => {
      alive = false;
    };
  }, []);

  React.useEffect(() => {
    if (PRIVATE_CHANNEL_VIEWS.has(view) && !auth?.accessToken) {
      setState({ items: [], total: 0, loading: false, loadingMore: false, error: "", notice: "" });
      return undefined;
    }
    let alive = true;
    setState((current) => ({ ...current, items: [], total: 0, loading: true, loadingMore: false, error: "", notice: "" }));
    const params = {
      limit: CHANNEL_PAGE_SIZE,
      offset: 0,
      q: query || undefined,
      category_id: categoryId || undefined
    };
    channelLoader(view)(params, auth?.accessToken)
      .then((data) => {
        if (!alive) return;
        const result = channelList(data);
        setState({ ...result, loading: false, loadingMore: false, error: "", notice: "" });
      })
      .catch((error) => {
        if (alive) setState({ items: [], total: 0, loading: false, loadingMore: false, error: error.message || "圈子加载失败", notice: "" });
      });
    return () => {
      alive = false;
    };
  }, [auth?.accessToken, categoryId, query, view]);

  function updateSearchParams(patch) {
    const next = new URLSearchParams(searchParams);
    Object.entries(patch).forEach(([key, value]) => {
      if (value) next.set(key, value);
      else next.delete(key);
    });
    setSearchParams(next);
  }

  function submitSearch(event) {
    event.preventDefault();
    updateSearchParams({ q: searchDraft.trim() });
  }

  async function loadMore() {
    if (state.loadingMore || state.items.length >= state.total) return;
    setState((current) => ({ ...current, loadingMore: true, error: "" }));
    try {
      const data = await channelLoader(view)({
        limit: CHANNEL_PAGE_SIZE,
        offset: state.items.length,
        q: query || undefined,
        category_id: categoryId || undefined
      }, auth?.accessToken);
      const page = channelList(data);
      setState((current) => {
        const seen = new Set(current.items.map((item) => item.id));
        return {
          ...current,
          items: [...current.items, ...page.items.filter((item) => !seen.has(item.id))],
          total: page.total,
          loadingMore: false
        };
      });
    } catch (error) {
      setState((current) => ({ ...current, loadingMore: false, error: error.message || "更多圈子加载失败" }));
    }
  }

  async function mutateRelationship(channel, action) {
    if (!auth?.accessToken) {
      navigate(loginPath(`/circles/${channel.id}`));
      return;
    }
    setPending({ id: channel.id, action });
    setState((current) => ({ ...current, error: "", notice: "" }));
    try {
      const active = action === "follow" ? channel.is_following : channel.is_favorited;
      const method = action === "follow"
        ? active ? bbsApi.unfollowChannel : bbsApi.followChannel
        : active ? bbsApi.unfavoriteChannel : bbsApi.favoriteChannel;
      const data = await method(channel.id, auth.accessToken);
      const responseChannel = channelFromResponse(data);
      const fallback = channelMutationPatch(channel, action);
      setState((current) => ({
        ...current,
        items: current.items.map((item) => sameId(item.id, channel.id) ? responseChannel || { ...item, ...fallback } : item),
        notice: action === "follow" ? (active ? "已取消关注。" : "已关注圈子。") : (active ? "已取消收藏。" : "已收藏圈子。")
      }));
    } catch (error) {
      setState((current) => ({ ...current, error: error.message || "圈子状态更新失败" }));
    } finally {
      setPending({ id: "", action: "" });
    }
  }

  const privateViewLocked = PRIVATE_CHANNEL_VIEWS.has(view) && !auth?.accessToken;
  const activeCategory = categoriesState.items.find((item) => sameId(item.id, categoryId));
  const canLoadMore = !state.loading && state.items.length < state.total;

  return (
    <>
      <PageHero
        icon={Users}
        eyebrow="圈子"
        title="找到长期关注的讨论空间"
        description="围绕稳定主题关注圈子、收藏入口，并在同一条时间线里持续参与讨论。"
        image={pageImages.圈子}
        stats={[
          [state.loading ? "..." : compactNumber(state.total), "当前视图"],
          [categoriesState.loading ? "..." : String(categoriesState.items.length), "圈子分类"],
          [auth?.accessToken ? "已登录" : "访客", "浏览身份"]
        ]}
      />
      <div className="channel-directory-head">
        <PillTabs
          items={CHANNEL_VIEWS}
          label="圈子视图"
          value={view}
          onChange={(nextView) => updateSearchParams({ view: nextView === "all" ? "" : nextView })}
        />
        {auth?.accessToken ? (
          <Link className="channel-primary-link" to="/circles/new"><Plus size={17} aria-hidden="true" />创建圈子</Link>
        ) : (
          <Link className="channel-secondary-link" to={loginPath("/circles/new")}>登录后创建</Link>
        )}
      </div>
      <form className="channel-filter panel" role="search" onSubmit={submitSearch}>
        <label className="channel-search-field">
          <span className="sr-only">搜索圈子</span>
          <Search size={18} aria-hidden="true" />
          <input placeholder="搜索圈子名称或简介" value={searchDraft} onChange={(event) => setSearchDraft(event.target.value)} />
        </label>
        <label>
          <span className="sr-only">筛选圈子分类</span>
          <select value={categoryId} onChange={(event) => updateSearchParams({ category_id: toId(event.target.value) })}>
            <option value="">全部分类</option>
            {categoriesState.items.map((category) => <option key={category.id} value={category.id}>{category.name}</option>)}
          </select>
        </label>
        <button type="submit"><Search size={17} aria-hidden="true" />搜索</button>
      </form>
      {(query || activeCategory) && (
        <div className="channel-active-filter panel">
          <span>正在查看{activeCategory ? `“${activeCategory.name}”分类` : "全部分类"}{query ? `中与“${query}”相关的圈子` : ""}</span>
          <button type="button" onClick={() => { setSearchDraft(""); updateSearchParams({ q: "", category_id: "" }); }}>清除筛选</button>
        </div>
      )}
      {privateViewLocked && (
        <EmptyState
          title="登录后查看个人圈子"
          description="已关注、已收藏和我创建的圈子只对当前账号可见。"
          action={<Link className="channel-primary-link" to={loginPath(`/circles?view=${view}`)}>登录</Link>}
        />
      )}
      {!privateViewLocked && state.loading && <EmptyState title="正在加载圈子..." description="请稍候" />}
      {!privateViewLocked && !state.loading && state.error && <EmptyState title={state.error} description="调整筛选条件或稍后重试。" />}
      {!privateViewLocked && !state.loading && !state.error && state.items.length === 0 && (
        <EmptyState title="没有找到圈子" description={query || categoryId ? "可以清除搜索或分类筛选后重试。" : "这个视图暂时还没有圈子。"} />
      )}
      {!privateViewLocked && state.items.length > 0 && (
        <div className="circle-grid channel-grid">
          {state.items.map((channel) => (
            <CircleCard
              channel={channel}
              key={channel.id}
              pendingAction={sameId(pending.id, channel.id) ? pending.action : ""}
              onFavorite={(item) => mutateRelationship(item, "favorite")}
              onFollow={(item) => mutateRelationship(item, "follow")}
            />
          ))}
        </div>
      )}
      {state.notice && <p className="form-success channel-feedback" role="status">{state.notice}</p>}
      {canLoadMore && <button className="channel-load-more panel" disabled={state.loadingMore} type="button" onClick={loadMore}>{state.loadingMore ? "加载中..." : "加载更多圈子"}</button>}
    </>
  );
}

export function ChannelDetailPage({ auth }) {
  const params = useParams();
  const navigate = useNavigate();
  const [state, setState] = React.useState({ channel: null, topics: [], topicTotal: 0, loading: true, loadingMore: false, error: "", notice: "" });
  const [pending, setPending] = React.useState("");

  React.useEffect(() => {
    let alive = true;
    setState((current) => ({ ...current, loading: true, error: "", notice: "" }));
    Promise.all([
      bbsApi.getChannel(params.id, auth?.accessToken),
      bbsApi.channelTopics(params.id, { limit: CHANNEL_PAGE_SIZE, offset: 0 })
    ]).then(([channelData, topicsData]) => {
      if (!alive) return;
      setState({
        channel: channelFromResponse(channelData),
        topics: listItems(topicsData),
        topicTotal: listTotal(topicsData),
        loading: false,
        loadingMore: false,
        error: "",
        notice: ""
      });
    }).catch((error) => {
      if (alive) setState((current) => ({ ...current, loading: false, error: error.message || "圈子详情加载失败" }));
    });
    return () => {
      alive = false;
    };
  }, [auth?.accessToken, params.id]);

  async function mutateRelationship(action) {
    const channel = state.channel;
    if (!channel) return;
    if (!auth?.accessToken) {
      navigate(loginPath(`/circles/${channel.id}`));
      return;
    }
    setPending(action);
    setState((current) => ({ ...current, error: "", notice: "" }));
    try {
      const active = action === "follow" ? channel.is_following : channel.is_favorited;
      const method = action === "follow"
        ? active ? bbsApi.unfollowChannel : bbsApi.followChannel
        : active ? bbsApi.unfavoriteChannel : bbsApi.favoriteChannel;
      const data = await method(channel.id, auth.accessToken);
      const updated = channelFromResponse(data) || normalizeChannel({ ...channel, ...channelMutationPatch(channel, action) });
      setState((current) => ({ ...current, channel: updated, notice: active ? "状态已取消。" : "状态已更新。" }));
    } catch (error) {
      setState((current) => ({ ...current, error: error.message || "圈子状态更新失败" }));
    } finally {
      setPending("");
    }
  }

  async function archiveChannel() {
    if (!state.channel || !auth?.accessToken) return;
    if (typeof window !== "undefined" && !window.confirm(`确认归档圈子“${state.channel.name}”吗？`)) return;
    setPending("archive");
    try {
      await bbsApi.archiveChannel(state.channel.id, auth.accessToken);
      navigate("/circles?view=owned", { replace: true });
    } catch (error) {
      setState((current) => ({ ...current, error: error.message || "圈子归档失败" }));
      setPending("");
    }
  }

  async function loadMoreTopics() {
    if (!state.channel || state.loadingMore) return;
    setState((current) => ({ ...current, loadingMore: true, error: "" }));
    try {
      const data = await bbsApi.channelTopics(state.channel.id, { limit: CHANNEL_PAGE_SIZE, offset: state.topics.length });
      const nextTopics = listItems(data);
      setState((current) => {
        const seen = new Set(current.topics.map((topic) => toId(topic.id)));
        return {
          ...current,
          topics: [...current.topics, ...nextTopics.filter((topic) => !seen.has(toId(topic.id)))],
          topicTotal: listTotal(data),
          loadingMore: false
        };
      });
    } catch (error) {
      setState((current) => ({ ...current, loadingMore: false, error: error.message || "更多主题加载失败" }));
    }
  }

  if (state.loading) return <EmptyState title="正在加载圈子..." description="请稍候" />;
  if (!state.channel) return <EmptyState title={state.error || "没有找到圈子"} description="可以返回圈子列表重新选择。" action={<Link className="channel-secondary-link" to="/circles">返回圈子</Link>} />;

  const channel = state.channel;
  const isOwner = ownsChannel(channel, auth?.user);
  const canFollow = !channel.is_archived || channel.is_following;
  const canFavorite = !channel.is_archived || channel.is_favorited;
  const canLoadMore = state.topics.length < state.topicTotal;
  return (
    <>
      <section className="channel-detail-hero panel" style={{ "--channel-color": channel.color }}>
        <span className="channel-detail-color" aria-hidden="true" />
        <div>
          <div className="channel-detail-labels">
            <span className="eyebrow"><Hash size={18} aria-hidden="true" />圈子</span>
            {channel.is_featured && !channel.is_archived && <span className="channel-state-badge"><Sparkles size={14} aria-hidden="true" />精选</span>}
            {channel.is_archived && <span className="channel-archive-badge"><Archive size={14} aria-hidden="true" />已归档</span>}
          </div>
          <h1>{channel.name}</h1>
          <p>{channel.description || "这个圈子暂时还没有简介。"}</p>
          <div className="channel-detail-stats">
            <span><strong>{compactNumber(channel.topics_count)}</strong>主题</span>
            <span><strong>{compactNumber(channel.followers_count)}</strong>关注者</span>
          </div>
        </div>
        <div className="channel-detail-actions">
          {canFollow && <button aria-pressed={channel.is_following} disabled={Boolean(pending)} type="button" onClick={() => mutateRelationship("follow")}><UserCheck size={17} aria-hidden="true" />{pending === "follow" ? "处理中" : channel.is_following ? "已关注" : "关注"}</button>}
          {canFavorite && <button aria-pressed={channel.is_favorited} className={channel.is_favorited ? "is-active" : ""} disabled={Boolean(pending)} type="button" onClick={() => mutateRelationship("favorite")}><Star fill={channel.is_favorited ? "currentColor" : "none"} size={17} aria-hidden="true" />{pending === "favorite" ? "处理中" : channel.is_favorited ? "已收藏" : "收藏"}</button>}
          {channel.is_archived ? (
            <span className="channel-archive-note"><Archive size={16} aria-hidden="true" />已归档，仅保留历史内容</span>
          ) : (
            <Link className="channel-primary-link" to={`/topic/create?channel_id=${encodeURIComponent(channel.id)}`}><Plus size={17} aria-hidden="true" />在圈子发帖</Link>
          )}
        </div>
      </section>
      {isOwner && !channel.is_archived && (
        <div className="channel-owner-actions panel">
          <span>你是这个圈子的创建者</span>
          <Link to={`/circles/${encodeURIComponent(channel.id)}/edit`}><Pencil size={16} aria-hidden="true" />编辑</Link>
          <button disabled={pending === "archive"} type="button" onClick={archiveChannel}><Archive size={16} aria-hidden="true" />{pending === "archive" ? "归档中" : "归档"}</button>
        </div>
      )}
      {state.notice && <p className="form-success channel-feedback" role="status">{state.notice}</p>}
      {state.error && <p className="form-error channel-feedback" role="alert">{state.error}</p>}
      <section className="panel channel-topic-panel">
        <header>
          <h2><MessageCircle size={20} aria-hidden="true" />圈子主题</h2>
          <span>{state.topicTotal} 条</span>
        </header>
        {state.topics.length === 0 ? (
          <div className="channel-topic-empty">
            <strong>还没有主题</strong>
            <p>发布第一条内容，开启这个圈子的讨论。</p>
          </div>
        ) : (
          <div className="channel-topic-list">
            {state.topics.map((topic) => (
              <article key={toId(topic.id)}>
                <div>
                  <h3><Link to={`/topic/${encodeURIComponent(topic.id)}`}>{topic.title || "未命名主题"}</Link></h3>
                  <p>{String(topic.body || topic.content || "暂无摘要").slice(0, 120)}</p>
                </div>
                <span><Clock3 size={14} aria-hidden="true" />{timeAgoMillis(topic.last_posted_at || topic.updated_at || topic.created_at)}</span>
              </article>
            ))}
          </div>
        )}
        {canLoadMore && <button className="channel-load-more" disabled={state.loadingMore} type="button" onClick={loadMoreTopics}>{state.loadingMore ? "加载中..." : "加载更多主题"}</button>}
      </section>
    </>
  );
}

export function ChannelEditorPage({ auth, edit = false }) {
  const params = useParams();
  const navigate = useNavigate();
  const [categoriesState, setCategoriesState] = React.useState([]);
  const [form, setForm] = React.useState({ name: "", description: "", color: "#1683f7", category_id: "" });
  const [state, setState] = React.useState({ loading: edit, saving: false, error: "", blocked: false });

  React.useEffect(() => {
    if (!auth?.accessToken) {
      setState({ loading: false, saving: false, error: "", blocked: false });
      return undefined;
    }
    let alive = true;
    const requests = [bbsApi.categories()];
    if (edit && params.id) requests.push(bbsApi.getChannel(params.id, auth.accessToken));
    Promise.all(requests).then(([categoriesData, channelData]) => {
      if (!alive) return;
      const categories = normalizeCategoriesResponse(categoriesData);
      setCategoriesState(categories);
      if (edit) {
        const channel = channelFromResponse(channelData);
        if (channel?.is_archived) {
          setState({ loading: false, saving: false, error: "已归档的圈子不能编辑。", blocked: true });
          return;
        }
        if (!channel || !ownsChannel(channel, auth.user)) {
          setState({ loading: false, saving: false, error: "只有圈子创建者可以编辑。", blocked: false });
          return;
        }
        setForm({ name: channel.name, description: channel.description, color: channel.color, category_id: channel.category_id });
      }
      setState({ loading: false, saving: false, error: "", blocked: false });
    }).catch((error) => {
      if (alive) setState({ loading: false, saving: false, error: error.message || "圈子表单加载失败", blocked: false });
    });
    return () => {
      alive = false;
    };
  }, [auth?.accessToken, auth?.user, edit, params.id]);

  function updateField(field, value) {
    setForm((current) => ({ ...current, [field]: value }));
  }

  async function submit(event) {
    event.preventDefault();
    if (!auth?.accessToken) return;
    const name = form.name.trim();
    if (!name) {
      setState((current) => ({ ...current, error: "请输入圈子名称。" }));
      return;
    }
    const payload = {
      name,
      description: form.description.trim(),
      color: form.color,
      category_id: form.category_id || undefined
    };
    setState((current) => ({ ...current, saving: true, error: "" }));
    try {
      const data = edit
        ? await bbsApi.updateChannel(params.id, payload, auth.accessToken)
        : await bbsApi.createChannel(payload, auth.accessToken);
      const channel = channelFromResponse(data);
      navigate(`/circles/${encodeURIComponent(channel?.id || params.id)}`, { replace: true });
    } catch (error) {
      setState((current) => ({ ...current, saving: false, error: error.message || "圈子保存失败" }));
    }
  }

  if (!auth?.accessToken) {
    const target = edit ? `/circles/${params.id}/edit` : "/circles/new";
    return <EmptyState title="请先登录" description="登录后可以创建和管理圈子。" action={<Link className="channel-primary-link" to={loginPath(target)}>登录</Link>} />;
  }

  return (
    <>
      <RouteHeader
        icon={edit ? Pencil : Plus}
        eyebrow="圈子管理"
        title={edit ? "编辑圈子" : "创建圈子"}
        description="设置清晰的名称、简介和识别色，帮助成员快速理解讨论边界。"
      />
      {state.loading ? <EmptyState title="正在加载圈子..." description="请稍候" /> : state.blocked ? (
        <EmptyState
          title={state.error}
          description="归档后仅保留历史内容。"
          action={<Link className="channel-secondary-link" to={`/circles/${params.id}`}>返回圈子</Link>}
        />
      ) : (
        <form className="channel-editor panel" onSubmit={submit}>
          <label>
            <span>圈子名称</span>
            <input maxLength={128} placeholder="例如：前端工程实践" required value={form.name} onChange={(event) => updateField("name", event.target.value)} />
          </label>
          <label>
            <span>圈子简介</span>
            <textarea maxLength={2048} placeholder="说明适合在这里讨论的主题" rows={5} value={form.description} onChange={(event) => updateField("description", event.target.value)} />
          </label>
          <div className="channel-editor-grid">
            <label>
              <span>识别色</span>
              <div className="channel-color-input">
                <input aria-label="圈子识别色" type="color" value={form.color} onChange={(event) => updateField("color", event.target.value)} />
                <code>{form.color.toUpperCase()}</code>
              </div>
            </label>
            <label>
              <span>圈子分类</span>
              <select value={form.category_id} onChange={(event) => updateField("category_id", toId(event.target.value))}>
                <option value="">不设分类</option>
                {categoriesState.map((category) => <option key={category.id} value={category.id}>{category.name}</option>)}
              </select>
            </label>
          </div>
          {state.error && <p className="form-error" role="alert">{state.error}</p>}
          <footer>
            <Link className="channel-secondary-link" to={edit ? `/circles/${params.id}` : "/circles"}>取消</Link>
            <button className="channel-primary-button" disabled={state.saving} type="submit">{state.saving ? "保存中..." : edit ? "保存修改" : "创建圈子"}</button>
          </footer>
        </form>
      )}
    </>
  );
}
