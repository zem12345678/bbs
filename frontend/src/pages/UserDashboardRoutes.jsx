import React from "react";
import { useNavigate, useParams } from "react-router-dom";
import { Bell, FileText, Heart, LayoutDashboard, MessageCircle, Plus, Star, Trophy, UserRound } from "lucide-react";
import { bbsApi } from "../api";
import { creditBalance, listItems, listTotal, unreadCount } from "../lib/apiShapes";
import { creditEntryMeta, creditReasonLabel, timeAgoMillis, toId, toNumber } from "../lib/formatters";
import { interactionToPost, userDisplayName } from "../lib/postMappers";
import { DataRows, EmptyState, PillTabs, RouteHeader } from "./RouteBlocks.jsx";

const dashboardSections = [
  { value: "overview", label: "概览", icon: LayoutDashboard },
  { value: "contents", label: "内容", icon: FileText },
  { value: "interactions", label: "互动", icon: Heart },
  { value: "messages", label: "消息", icon: Bell },
  { value: "scores", label: "积分", icon: Trophy },
  { value: "profile", label: "资料", icon: UserRound }
];

const contentStatusTabs = [
  { value: 0, label: "全部" },
  { value: 1, label: "草稿" },
  { value: 2, label: "已发布" },
  { value: 3, label: "已隐藏" },
  { value: 4, label: "已归档" }
];

export function UserDashboardPage({ auth, onAuthUserUpdate }) {
  const params = useParams();
  const navigate = useNavigate();
  const section = normalizeSection(params.section);

  function changeSection(value) {
    navigate(value === "overview" ? "/dashboard" : `/dashboard/${value}`);
  }

  return (
    <>
      <RouteHeader
        icon={LayoutDashboard}
        eyebrow="个人工作台"
        title={auth ? `${userDisplayName(auth.user)} 的创作中心` : "登录后管理你的社区内容"}
        description="这里仅面向当前登录用户，集中管理自己的文章、话题、互动、通知、积分和资料；平台运营管理继续放在独立的 vue-pure-admin。"
        actions={
          auth && (
            <>
              <button type="button" onClick={() => navigate("/topic/create")}>
                <Plus size={16} aria-hidden="true" />
                发话题
              </button>
              <button type="button" onClick={() => navigate("/article/create")}>
                <Plus size={16} aria-hidden="true" />
                写文章
              </button>
            </>
          )
        }
      />
      <PillTabs items={dashboardSections} label="个人工作台分区" value={section} onChange={changeSection} />
      {!auth ? (
        <EmptyState
          title="请先登录"
          description="登录后可以进入个人工作台管理自己的内容和互动记录。"
          action={
            <button className="route-link-button" type="button" onClick={() => navigate("/user/signin")}>
              去登录
            </button>
          }
        />
      ) : (
        <>
          <UserWorkspaceStrip auth={auth} />
          {renderSection(section, auth, onAuthUserUpdate)}
        </>
      )}
    </>
  );
}

function renderSection(section, auth, onAuthUserUpdate) {
  switch (section) {
    case "overview":
      return <OverviewPanel auth={auth} />;
    case "contents":
      return <ContentManagerPanel auth={auth} />;
    case "interactions":
      return <InteractionsPanel auth={auth} />;
    case "messages":
      return <MessagesPanel auth={auth} />;
    case "scores":
      return <ScoresPanel auth={auth} />;
    case "profile":
      return <ProfilePanel auth={auth} onAuthUserUpdate={onAuthUserUpdate} />;
    default:
      return <OverviewPanel auth={auth} />;
  }
}

function UserWorkspaceStrip({ auth }) {
  return (
    <section className="workspace-profile-strip panel">
      <div>
        <strong>{userDisplayName(auth.user)}</strong>
        <span>@{auth.user?.username || auth.user?.id} · 普通社区用户</span>
      </div>
      <p>当前工作台只管理本人数据</p>
    </section>
  );
}

function OverviewPanel({ auth }) {
  const [state, setState] = React.useState({ loading: false, error: "", metrics: [], rows: [] });

  React.useEffect(() => {
    const userId = toId(auth?.user?.id);
    if (!userId) return;
    let alive = true;
    setState({ loading: true, error: "", metrics: [], rows: [] });
    Promise.all([
      bbsApi.listArticles({ author_id: userId, status: 0, limit: 5, offset: 0 }).catch((error) => ({ error })),
      bbsApi.listTopics({ author_id: userId, status: 0, limit: 5, offset: 0 }).catch((error) => ({ error })),
      bbsApi.favorites({ limit: 1, offset: 0 }, auth.accessToken).catch((error) => ({ error })),
      bbsApi.notificationUnreadCount(auth.accessToken).catch((error) => ({ error })),
      bbsApi.creditBalance(auth.accessToken).catch((error) => ({ error }))
    ]).then(([articleData, topicData, favoriteData, unreadData, creditData]) => {
      if (!alive) return;
      const articles = listItems(articleData);
      const topics = listItems(topicData);
      setState({
        loading: false,
        error: "",
        metrics: [
          { value: listTotal(articleData, articles), label: "我的文章" },
          { value: listTotal(topicData, topics), label: "我的话题" },
          { value: listTotal(favoriteData), label: "收藏内容" },
          { value: unreadCount(unreadData), label: "未读通知" },
          { value: toNumber(creditBalance(creditData)?.total), label: "当前积分" }
        ],
        rows: [...topics.map((item) => contentDataRow(item, "topic")), ...articles.map((item) => contentDataRow(item, "article"))]
          .sort((left, right) => toNumber(right.sortAt) - toNumber(left.sortAt))
          .slice(0, 6)
      });
    });
    return () => {
      alive = false;
    };
  }, [auth?.accessToken, auth?.user?.id]);

  if (state.loading) return <EmptyState title="正在汇总个人工作台..." />;
  if (state.error) return <EmptyState title={state.error} />;

  return (
    <section className="dashboard-panel">
      <div className="dashboard-metrics dashboard-metrics--wide">
        {state.metrics.map((item) => (
          <Metric key={item.label} value={item.value} label={item.label} />
        ))}
      </div>
      {state.rows.length > 0 ? <DataRows rows={state.rows} /> : <EmptyState title="还没有创作记录" description="发布第一篇话题或文章后会出现在这里。" />}
    </section>
  );
}

function ContentManagerPanel({ auth }) {
  const navigate = useNavigate();
  const [kind, setKind] = React.useState("topic");
  const [status, setStatus] = React.useState(0);
  const [state, setState] = React.useState({ items: [], total: 0, loading: false, error: "", action: "" });
  const userId = toId(auth?.user?.id);

  const loadItems = React.useCallback(() => {
    if (!userId) return;
    let alive = true;
    setState((current) => ({ ...current, loading: true, error: "" }));
    const loader = kind === "topic" ? bbsApi.listTopics : bbsApi.listArticles;
    loader({ author_id: userId, status, limit: 30, offset: 0 })
      .then((data) => {
        if (!alive) return;
        const items = listItems(data);
        setState({ items, total: listTotal(data, items), loading: false, error: "", action: "" });
      })
      .catch((error) => {
        if (!alive) return;
        setState({ items: [], total: 0, loading: false, error: error.message || "内容列表加载失败", action: "" });
      });
    return () => {
      alive = false;
    };
  }, [kind, status, userId]);

  React.useEffect(loadItems, [loadItems]);

  async function runContentAction(action, item) {
    const id = toId(item.id);
    if (!id) return;
    if (action === "edit") {
      navigate(`/${kind}/edit/${id}`);
      return;
    }
    if (action === "view") {
      navigate(`/${kind}/${id}`);
      return;
    }
    setState((current) => ({ ...current, action: `${action}-${id}`, error: "" }));
    try {
      if (action === "publish") {
        kind === "topic" ? await bbsApi.publishTopic(id, auth.accessToken) : await bbsApi.publishArticle(id, auth.accessToken);
      } else {
        kind === "topic" ? await bbsApi.deleteTopic(id, auth.accessToken) : await bbsApi.deleteArticle(id, auth.accessToken);
      }
      loadItems();
    } catch (error) {
      setState((current) => ({ ...current, action: "", error: error.message || "内容操作失败" }));
    }
  }

  return (
    <ModerationSection
      actionError={state.error}
      emptyText={`暂无${kind === "topic" ? "话题" : "文章"}记录`}
      filters={contentStatusTabs}
      loading={state.loading}
      status={status}
      total={state.total}
      toolbar={
        <div className="feed-switch" role="tablist" aria-label="内容类型">
          <button className={kind === "topic" ? "is-active" : ""} type="button" onClick={() => setKind("topic")}>
            <MessageCircle size={17} aria-hidden="true" />
            话题
          </button>
          <button className={kind === "article" ? "is-active" : ""} type="button" onClick={() => setKind("article")}>
            <FileText size={17} aria-hidden="true" />
            文章
          </button>
        </div>
      }
      onStatusChange={setStatus}
    >
      {state.items.map((item) => (
        <WorkspaceRow
          key={item.id}
          title={item.title || `内容 #${item.id}`}
          description={item.summary || item.body || "暂无摘要"}
          meta={`${kind === "topic" ? "话题" : "文章"} · ${timeAgoMillis(item.published_at || item.publishedAt || item.created_at || item.createdAt)}`}
          status={contentStatusLabel(item.status)}
          tags={item.tags || []}
          actions={
            <>
              <button type="button" onClick={() => runContentAction("view", item)}>
                查看
              </button>
              {item.status !== 4 && (
                <button type="button" onClick={() => runContentAction("edit", item)}>
                  编辑
                </button>
              )}
              {item.status === 1 && (
                <button type="button" disabled={state.action === `publish-${item.id}`} onClick={() => runContentAction("publish", item)}>
                  {state.action === `publish-${item.id}` ? "发布中" : "发布"}
                </button>
              )}
              {item.status !== 4 && (
                <button type="button" disabled={state.action === `archive-${item.id}`} onClick={() => runContentAction("archive", item)}>
                  {state.action === `archive-${item.id}` ? "处理中" : "归档"}
                </button>
              )}
            </>
          }
        />
      ))}
    </ModerationSection>
  );
}

function InteractionsPanel({ auth }) {
  const navigate = useNavigate();
  const [mode, setMode] = React.useState("likes");
  const [state, setState] = React.useState({ rows: [], total: 0, loading: false, error: "", action: "" });

  const loadInteractions = React.useCallback(() => {
    let alive = true;
    setState((current) => ({ ...current, loading: true, error: "" }));
    const loader = mode === "likes" ? bbsApi.likes : bbsApi.favorites;
    loader({ limit: 30, offset: 0 }, auth.accessToken)
      .then(async (data) => {
        const items = listItems(data);
        const posts = (await Promise.all(items.map((item) => interactionToPost(item, auth, mode)))).filter(Boolean);
        if (!alive) return;
        setState({ rows: posts, total: listTotal(data, posts), loading: false, error: "", action: "" });
      })
      .catch((error) => {
        if (!alive) return;
        setState({ rows: [], total: 0, loading: false, error: error.message || "互动记录加载失败", action: "" });
      });
    return () => {
      alive = false;
    };
  }, [auth, mode]);

  React.useEffect(loadInteractions, [loadInteractions]);

  async function removeInteraction(post) {
    const id = toId(post.id);
    if (!id) return;
    setState((current) => ({ ...current, action: `${post.kind}-${id}`, error: "" }));
    try {
      if (mode === "likes") {
        post.kind === "topic" ? await bbsApi.unlikeTopic(id, auth.accessToken) : await bbsApi.unlikeArticle(id, auth.accessToken);
      } else {
        post.kind === "topic" ? await bbsApi.unfavoriteTopic(id, auth.accessToken) : await bbsApi.unfavoriteArticle(id, auth.accessToken);
      }
      loadInteractions();
    } catch (error) {
      setState((current) => ({ ...current, action: "", error: error.message || "互动操作失败" }));
    }
  }

  return (
    <ModerationSection
      actionError={state.error}
      emptyText={`暂无${mode === "likes" ? "点赞" : "收藏"}记录`}
      filters={[
        { value: "likes", label: "点赞" },
        { value: "favorites", label: "收藏" }
      ]}
      loading={state.loading}
      status={mode}
      total={state.total}
      onStatusChange={setMode}
    >
      {state.rows.map((post) => (
        <WorkspaceRow
          key={`${post.kind}-${post.id}`}
          title={post.title}
          description={post.text || "暂无摘要"}
          meta={`${post.kind === "topic" ? "话题" : "文章"} · ${post.time}`}
          status={mode === "likes" ? "已点赞" : "已收藏"}
          tags={post.tags || []}
          actions={
            <>
              <button type="button" onClick={() => navigate(`/${post.kind}/${post.id}`)}>
                查看
              </button>
              <button type="button" disabled={state.action === `${post.kind}-${post.id}`} onClick={() => removeInteraction(post)}>
                {mode === "likes" ? "取消点赞" : "取消收藏"}
              </button>
            </>
          }
        />
      ))}
    </ModerationSection>
  );
}

function MessagesPanel({ auth }) {
  const [state, setState] = React.useState({ items: [], total: 0, loading: false, error: "", action: "" });

  const loadMessages = React.useCallback(() => {
    let alive = true;
    setState((current) => ({ ...current, loading: true, error: "" }));
    bbsApi
      .notifications({ limit: 30, offset: 0 }, auth.accessToken)
      .then((data) => {
        if (!alive) return;
        const items = listItems(data);
        setState({ items, total: listTotal(data, items), loading: false, error: "", action: "" });
      })
      .catch((error) => {
        if (!alive) return;
        setState({ items: [], total: 0, loading: false, error: error.message || "通知加载失败", action: "" });
      });
    return () => {
      alive = false;
    };
  }, [auth.accessToken]);

  React.useEffect(loadMessages, [loadMessages]);

  async function markRead(id) {
    setState((current) => ({ ...current, action: `read-${id}`, error: "" }));
    try {
      await bbsApi.markNotificationRead(id, auth.accessToken);
      loadMessages();
    } catch (error) {
      setState((current) => ({ ...current, action: "", error: error.message || "通知操作失败" }));
    }
  }

  async function markAllRead() {
    setState((current) => ({ ...current, action: "read-all", error: "" }));
    try {
      await bbsApi.markAllNotificationsRead(auth.accessToken);
      loadMessages();
    } catch (error) {
      setState((current) => ({ ...current, action: "", error: error.message || "通知操作失败" }));
    }
  }

  return (
    <ModerationSection
      actionError={state.error}
      emptyText="暂无通知"
      filters={[]}
      loading={state.loading}
      status=""
      total={state.total}
      toolbar={
        <button type="button" disabled={state.action === "read-all"} onClick={markAllRead}>
          全部已读
        </button>
      }
      onStatusChange={() => {}}
    >
      {state.items.map((item) => (
        <WorkspaceRow
          key={item.id}
          title={item.title || "站内通知"}
          description={item.content || "暂无内容"}
          meta={timeAgoMillis(item.created_at || item.createdAt)}
          status={item.read ? "已读" : "未读"}
          actions={
            !item.read && (
              <button type="button" disabled={state.action === `read-${item.id}`} onClick={() => markRead(item.id)}>
                标记已读
              </button>
            )
          }
        />
      ))}
    </ModerationSection>
  );
}

function ScoresPanel({ auth }) {
  const [state, setState] = React.useState({ balance: null, rows: [], loading: false, error: "" });

  React.useEffect(() => {
    let alive = true;
    setState({ balance: null, rows: [], loading: true, error: "" });
    Promise.all([bbsApi.creditBalance(auth.accessToken), bbsApi.creditLedger({ limit: 30, offset: 0 }, auth.accessToken)])
      .then(([balanceData, ledgerData]) => {
        if (!alive) return;
        const balance = creditBalance(balanceData) || creditBalance(ledgerData);
        setState({
          balance,
          rows: listItems(ledgerData).map((entry) => ({
            key: entry.id || `${entry.reason}-${entry.source_event_id}`,
            title: creditReasonLabel(entry.reason),
            description: creditEntryMeta(entry),
            meta: String(toNumber(entry.delta))
          })),
          loading: false,
          error: ""
        });
      })
      .catch((error) => {
        if (!alive) return;
        setState({ balance: null, rows: [], loading: false, error: error.message || "积分加载失败" });
      });
    return () => {
      alive = false;
    };
  }, [auth.accessToken]);

  if (state.loading) return <EmptyState title="正在加载积分..." />;
  if (state.error) return <EmptyState title={state.error} />;

  return (
    <section className="dashboard-panel">
      <section className="score-summary panel">
        <span>当前积分</span>
        <strong>{toNumber(state.balance?.total)}</strong>
        <p>积分由发帖、评论、点赞、收藏和任务事件驱动，后端由积分服务统一结算。</p>
      </section>
      {state.rows.length > 0 ? <DataRows rows={state.rows} /> : <EmptyState title="暂无积分明细" />}
    </section>
  );
}

function ProfilePanel({ auth, onAuthUserUpdate }) {
  const [form, setForm] = React.useState({
    nickname: auth.user?.nickname || "",
    avatar_url: auth.user?.avatar_url || auth.user?.avatarUrl || "",
    bio: auth.user?.bio || ""
  });
  const [state, setState] = React.useState({ saving: false, error: "", message: "" });

  function updateField(field, value) {
    setForm((current) => ({ ...current, [field]: value }));
  }

  async function submit(event) {
    event.preventDefault();
    setState({ saving: true, error: "", message: "" });
    try {
      const data = await bbsApi.updateMe(form, auth.accessToken);
      if (data?.user) {
        onAuthUserUpdate?.(data.user);
      }
      setState({ saving: false, error: "", message: "资料已保存。" });
    } catch (error) {
      setState({ saving: false, error: error.message || "资料保存失败", message: "" });
    }
  }

  return (
    <section className="account-security panel">
      <header>
        <strong>个人资料</strong>
        <p>维护昵称、头像和简介，这些信息会显示在你的内容和个人主页上。</p>
      </header>
      <form onSubmit={submit}>
        <label>
          昵称
          <input value={form.nickname} onChange={(event) => updateField("nickname", event.target.value)} />
        </label>
        <label>
          头像 URL
          <input value={form.avatar_url} onChange={(event) => updateField("avatar_url", event.target.value)} />
        </label>
        <label>
          个人简介
          <textarea value={form.bio} onChange={(event) => updateField("bio", event.target.value)} />
        </label>
        {state.error && <p className="form-error">{state.error}</p>}
        {state.message && <p className="form-success">{state.message}</p>}
        <button type="submit" disabled={state.saving}>
          {state.saving ? "保存中..." : "保存资料"}
        </button>
      </form>
    </section>
  );
}

function ModerationSection({ actionError, children, emptyText, filters, loading, status, toolbar, total, onStatusChange }) {
  return (
    <section className="dashboard-panel">
      <div className="moderation-toolbar panel">
        <div>
          <strong>个人列表</strong>
          <span>{loading ? "正在加载" : `${total} 条记录`}</span>
        </div>
        <div className="workspace-toolbar-actions">
          {toolbar}
          {filters.length > 0 && (
            <div className="moderation-filters">
              {filters.map((item) => (
                <button
                  className={status === item.value ? "is-active" : ""}
                  key={item.value}
                  type="button"
                  onClick={() => onStatusChange(item.value)}
                >
                  {item.label}
                </button>
              ))}
            </div>
          )}
        </div>
      </div>
      {actionError && <EmptyState title={actionError} />}
      {loading && <EmptyState title="正在加载列表..." />}
      {!loading && React.Children.count(children) === 0 && <EmptyState title={emptyText} />}
      {!loading && React.Children.count(children) > 0 && <div className="moderation-list">{children}</div>}
    </section>
  );
}

function WorkspaceRow({ actions, description, meta, status, tags = [], title }) {
  return (
    <article className="moderation-row panel">
      <div>
        <strong>{title}</strong>
        <p>{description}</p>
        {tags.length > 0 && (
          <div className="moderation-tags">
            {tags.slice(0, 5).map((tag) => (
              <span key={tag}>{tag}</span>
            ))}
          </div>
        )}
        <small>{meta}</small>
      </div>
      <aside>
        <span>{status}</span>
        {actions && <div className="moderation-actions">{actions}</div>}
      </aside>
    </article>
  );
}

function Metric({ value, label }) {
  return (
    <div className="dashboard-metric panel">
      <strong>{value}</strong>
      <span>{label}</span>
    </div>
  );
}

function normalizeSection(section) {
  if (!section) return "overview";
  return dashboardSections.some((item) => item.value === section) ? section : "overview";
}

function contentDataRow(item, kind) {
  const timestamp = item.published_at || item.publishedAt || item.created_at || item.createdAt;
  return {
    key: `${kind}-${item.id}`,
    title: item.title || `内容 #${item.id}`,
    description: item.summary || item.body || (kind === "topic" ? "话题" : "文章"),
    meta: `${kind === "topic" ? "话题" : "文章"} · ${contentStatusLabel(item.status)}`,
    sortAt: timestamp
  };
}

function contentStatusLabel(status) {
  const labels = { 1: "草稿", 2: "已发布", 3: "已隐藏", 4: "已归档" };
  return labels[toNumber(status)] || "未知";
}
