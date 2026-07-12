import React from "react";
import { useNavigate, useParams, useSearchParams } from "react-router-dom";
import { BadgePercent, Bell, FileText, Heart, ImagePlus, LayoutDashboard, MailCheck, MapPin, MessageCircle, Plus, RefreshCcw, ShoppingBag, Star, Trophy, UserRound } from "lucide-react";
import { bbsApi } from "../api";
import MessageFilterPanel from "../components/notifications/MessageFilterPanel.jsx";
import { creditBalance, listItems, listTotal, notificationRead, unreadCount } from "../lib/apiShapes";
import { creditEntryMeta, creditReasonLabel, sameId, timeAgoMillis, toId, toNumber } from "../lib/formatters";
import { paymentAttemptKey } from "../lib/idempotencyKeys";
import { friendlyMallOrderActionError } from "../lib/mallErrors";
import { markdownImageUrls, textWithoutMarkdownImages } from "../lib/markdownMedia";
import { emitNotificationsChanged } from "../lib/notificationEvents";
import { filterNotifications, isMallNotification, notificationGroupLabel, notificationTarget, notificationTargetLabel, summarizeNotifications } from "../lib/notificationTargets";
import { interactionToPost, userAvatar, userDisplayName } from "../lib/postMappers";
import { DataRows, EmptyState, PillTabs, RouteHeader } from "./RouteBlocks.jsx";

const dashboardSections = [
  { value: "overview", label: "概览", icon: LayoutDashboard },
  { value: "contents", label: "内容", icon: FileText },
  { value: "interactions", label: "互动", icon: Heart },
  { value: "messages", label: "消息", icon: Bell },
  { value: "orders", label: "订单", icon: ShoppingBag },
  { value: "coupons", label: "优惠券", icon: BadgePercent },
  { value: "addresses", label: "地址", icon: MapPin },
  { value: "refunds", label: "售后", icon: RefreshCcw },
  { value: "reviews", label: "评价", icon: Star },
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

const orderStatusTabs = [
  { value: 0, label: "全部" },
  { value: 1, label: "待支付" },
  { value: 2, label: "支付中" },
  { value: 3, label: "已支付" },
  { value: 4, label: "已取消" },
  { value: 5, label: "已发货" },
  { value: 6, label: "已完成" },
  { value: 7, label: "已关闭" },
  { value: 8, label: "已退款" }
];

const reviewStatusTabs = [
  { value: 0, label: "全部" },
  { value: 1, label: "待审核" },
  { value: 2, label: "已展示" },
  { value: 3, label: "已隐藏" }
];

const refundStatusTabs = [
  { value: 0, label: "全部" },
  { value: 1, label: "待审核" },
  { value: 2, label: "处理中" },
  { value: 3, label: "已退款" },
  { value: 4, label: "已拒绝" }
];

const couponUsageStatusTabs = [
  { value: 0, label: "全部" },
  { value: 4, label: "可使用" },
  { value: 1, label: "已锁定" },
  { value: 2, label: "已使用" },
  { value: 3, label: "已释放" }
];

const refundReasons = [
  { value: "quality_issue", label: "商品或权益问题" },
  { value: "wrong_item", label: "兑换内容不符" },
  { value: "delivery_issue", label: "履约或物流问题" },
  { value: "other", label: "其他原因" }
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
    case "orders":
      return <OrdersPanel auth={auth} />;
    case "coupons":
      return <CouponsPanel auth={auth} />;
    case "addresses":
      return <AddressesPanel auth={auth} />;
    case "refunds":
      return <RefundsPanel auth={auth} />;
    case "reviews":
      return <ReviewsPanel auth={auth} />;
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
      bbsApi.myArticles({ status: 0, limit: 5, offset: 0 }, auth.accessToken).catch((error) => ({ error })),
      bbsApi.myTopics({ status: 0, limit: 5, offset: 0 }, auth.accessToken).catch((error) => ({ error })),
      bbsApi.favorites({ limit: 1, offset: 0 }, auth.accessToken).catch((error) => ({ error })),
      bbsApi.notificationUnreadCount(auth.accessToken).catch((error) => ({ error })),
      bbsApi.mallOrders({ limit: 1, offset: 0 }, auth.accessToken).catch((error) => ({ error })),
      bbsApi.mallMyCoupons({ limit: 1, offset: 0, status: 4 }, auth.accessToken).catch((error) => ({ error })),
      bbsApi.mallRefunds({ limit: 1, offset: 0 }, auth.accessToken).catch((error) => ({ error })),
      bbsApi.creditBalance(auth.accessToken).catch((error) => ({ error }))
    ]).then(([articleData, topicData, favoriteData, unreadData, orderData, couponData, refundData, creditData]) => {
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
          { value: listTotal(orderData), label: "商城订单" },
          { value: listTotal(couponData), label: "可用优惠券" },
          { value: listTotal(refundData), label: "售后申请" },
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
    const loader = kind === "topic" ? bbsApi.myTopics : bbsApi.myArticles;
    loader({ status, limit: 30, offset: 0 }, auth.accessToken)
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
  }, [auth.accessToken, kind, status, userId]);

  React.useEffect(loadItems, [loadItems]);

  async function runContentAction(action, item) {
    const id = toId(item.id);
    if (!id) return;
    if (action === "edit") {
      navigate(`/${kind}/edit/${id}`);
      return;
    }
    if (action === "view") {
      navigate(toNumber(item.status) === 2 ? `/${kind}/${id}` : `/${kind}/edit/${id}`);
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
              {(item.status === 1 || item.status === 2) && (
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
  const navigate = useNavigate();
  const [state, setState] = React.useState({ items: [], total: 0, unread: 0, loading: false, error: "", action: "" });
  const [filter, setFilter] = React.useState("all");

  const loadMessages = React.useCallback(() => {
    let alive = true;
    setState((current) => ({ ...current, loading: true, error: "" }));
    bbsApi
      .notifications({ limit: 30, offset: 0 }, auth.accessToken)
      .then((data) => {
        if (!alive) return;
        const items = listItems(data);
        setState({ items, total: listTotal(data, items), unread: unreadCount(data), loading: false, error: "", action: "" });
      })
      .catch((error) => {
        if (!alive) return;
        setState({ items: [], total: 0, unread: 0, loading: false, error: error.message || "通知加载失败", action: "" });
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
      emitNotificationsChanged();
      loadMessages();
    } catch (error) {
      setState((current) => ({ ...current, action: "", error: error.message || "通知操作失败" }));
    }
  }

  async function markAllRead() {
    setState((current) => ({ ...current, action: "read-all", error: "" }));
    try {
      await bbsApi.markAllNotificationsRead(auth.accessToken);
      emitNotificationsChanged();
      loadMessages();
    } catch (error) {
      setState((current) => ({ ...current, action: "", error: error.message || "通知操作失败" }));
    }
  }

  async function openNotification(item) {
    const target = notificationTarget(item);
    if (!target) return;
    setState((current) => ({ ...current, action: `open-${item.id}`, error: "" }));
    try {
      if (!notificationRead(item)) {
        await bbsApi.markNotificationRead(item.id, auth.accessToken);
        emitNotificationsChanged();
      }
      navigate(target);
    } catch (error) {
      setState((current) => ({ ...current, action: "", error: error.message || "通知操作失败" }));
    }
  }

  const summary = React.useMemo(() => summarizeNotifications(state.items), [state.items]);
  const visibleItems = React.useMemo(() => filterNotifications(state.items, filter), [state.items, filter]);

  if (state.loading) return <EmptyState title="正在加载通知..." />;
  if (state.error) return <EmptyState title={state.error} />;
  if (state.items.length === 0) return <EmptyState title="暂无通知" description="评论、点赞、收藏、关注和商城通知会出现在这里。" />;

  return (
    <section className="messages-panel">
      <div className="message-toolbar panel">
        <div>
          <strong>站内通知</strong>
          <span>
            {state.total} 条通知 · {state.unread} 条未读
            {summary.mall.total > 0 ? ` · 商城 ${summary.mall.total} 条` : ""}
          </span>
        </div>
        <button type="button" disabled={state.unread === 0 || state.action === "read-all"} onClick={markAllRead}>
          {state.action === "read-all" ? "处理中..." : "全部已读"}
        </button>
      </div>
      <MessageFilterPanel filter={filter} noun="通知" summary={summary} onFilterChange={setFilter} />
      {visibleItems.length === 0 && <EmptyState title="暂无商城通知" description="订单、售后和商品评价通知会归到这里。" />}
      {visibleItems.length > 0 && (
        <div className="data-rows">
          {visibleItems.map((item) => {
            const read = notificationRead(item);
            const target = notificationTarget(item);
            const mallNotification = isMallNotification(item);
            return (
              <article className={`data-row message-row ${read ? "" : "is-unread"}`} key={item.id}>
                <div>
                  <strong>
                    {item.title || "站内通知"}
                    <em className={mallNotification ? "is-mall" : ""}>{notificationGroupLabel(item)}</em>
                  </strong>
                  {item.content && <p>{item.content}</p>}
                  <small>{timeAgoMillis(item.created_at || item.createdAt)}</small>
                </div>
                <aside className="message-actions">
                  <span>{read ? "已读" : "未读"}</span>
                  {target && (
                    <button type="button" disabled={state.action === `open-${item.id}`} onClick={() => openNotification(item)}>
                      {state.action === `open-${item.id}` ? "打开中..." : notificationTargetLabel(item)}
                    </button>
                  )}
                  {!read && (
                    <button type="button" disabled={state.action === `read-${item.id}`} onClick={() => markRead(item.id)}>
                      {state.action === `read-${item.id}` ? "处理中..." : "标记已读"}
                    </button>
                  )}
                </aside>
              </article>
            );
          })}
        </div>
      )}
    </section>
  );
}

function OrdersPanel({ auth }) {
  const navigate = useNavigate();
  const [searchParams, setSearchParams] = useSearchParams();
  const focusedOrderId = toId(searchParams.get("order_id"));
  const [status, setStatus] = React.useState(0);
  const [refundForm, setRefundForm] = React.useState(null);
  const [selectedOrderId, setSelectedOrderId] = React.useState(focusedOrderId || "");
  const [state, setState] = React.useState({
    items: [],
    total: 0,
    logsByOrder: {},
    paymentsByOrder: {},
    refundsByOrder: {},
    loading: false,
    error: "",
    action: "",
    notice: ""
  });

  const loadOrders = React.useCallback(() => {
    let alive = true;
    setState((current) => ({ ...current, loading: true, error: "" }));
    Promise.all([bbsApi.mallOrders({ limit: 50, offset: 0, status }, auth.accessToken), bbsApi.mallRefunds({ limit: 100, offset: 0 }, auth.accessToken)])
      .then(async ([data, refundData]) => {
        if (!alive) return;
        const filteredItems = listItems(data);
        let items = focusedOrderId
          ? [...filteredItems].sort((left, right) => {
              if (sameId(left.id, focusedOrderId)) return -1;
              if (sameId(right.id, focusedOrderId)) return 1;
              return 0;
            })
          : filteredItems;
        let detailError = "";
        if (focusedOrderId && !items.some((item) => sameId(item.id, focusedOrderId))) {
          try {
            const detailData = await bbsApi.mallOrder(focusedOrderId, auth.accessToken);
            const focusedOrder = detailData?.order;
            if (focusedOrder?.id && !items.some((item) => sameId(item.id, focusedOrder.id))) {
              items = [focusedOrder, ...items];
            }
          } catch (error) {
            detailError = error.message || "订单详情加载失败";
          }
        }
        if (!alive) return;
        const refundsByOrder = refundsByOrderId(listItems(refundData));
        setState((current) => ({
          ...current,
          items,
          total: Math.max(listTotal(data, items), items.length),
          refundsByOrder,
          loading: false,
          error: detailError,
          action: ""
        }));
      })
      .catch((error) => {
        if (!alive) return;
        setState({ items: [], total: 0, logsByOrder: {}, paymentsByOrder: {}, refundsByOrder: {}, loading: false, error: error.message || "订单加载失败", action: "", notice: "" });
      });
    return () => {
      alive = false;
    };
  }, [auth.accessToken, focusedOrderId, status]);

  React.useEffect(loadOrders, [loadOrders]);

  React.useEffect(() => {
    if (!focusedOrderId) return;
    setSelectedOrderId(focusedOrderId);
    setStatus(0);
  }, [focusedOrderId]);

  const selectedOrder = state.items.find((item) => sameId(item.id, selectedOrderId));
  const selectedOrderKey = String(toId(selectedOrder?.id) || "");
  const selectedOrderVersion = [selectedOrder?.status, selectedOrder?.updated_at, selectedOrder?.updatedAt].filter(Boolean).join(":");
  const selectedLogs = state.logsByOrder[selectedOrderKey] || [];
  const selectedPayments = state.paymentsByOrder[selectedOrderKey] || [];
  const selectedRefund = state.refundsByOrder[selectedOrderKey];

  React.useEffect(() => {
    if (!selectedOrderKey) return;
    let alive = true;
    Promise.allSettled([bbsApi.mallOrderLogs(selectedOrderKey, auth.accessToken), bbsApi.mallOrderPayments(selectedOrderKey, auth.accessToken)]).then(([logsResult, paymentsResult]) => {
      if (!alive) return;
      setState((current) => ({
        ...current,
        logsByOrder: {
          ...current.logsByOrder,
          [selectedOrderKey]: logsResult.status === "fulfilled" ? listItems(logsResult.value) : []
        },
        paymentsByOrder: {
          ...current.paymentsByOrder,
          [selectedOrderKey]: paymentsResult.status === "fulfilled" ? listItems(paymentsResult.value) : []
        }
      }));
    });
    return () => {
      alive = false;
    };
  }, [auth.accessToken, selectedOrderKey, selectedOrderVersion]);

  async function payOrder(order) {
    const id = toId(order.id);
    if (!id) return;
    setState((current) => ({ ...current, action: `pay-${id}`, error: "", notice: "" }));
    try {
      await bbsApi.payMallOrder(
        id,
        {
          payment_method: "credits",
          idempotency_key: paymentAttemptKey("dashboard-pay", id)
        },
        auth.accessToken
      );
      setState((current) => ({ ...current, action: "", error: "", notice: "订单已支付，积分流水已同步。" }));
      loadOrders();
    } catch (error) {
      setState((current) => ({ ...current, action: "", error: friendlyMallOrderActionError(error, "订单支付失败，请稍后重试。"), notice: "" }));
    }
  }

  async function cancelOrder(order) {
    const id = toId(order.id);
    if (!id) return;
    setState((current) => ({ ...current, action: `cancel-${id}`, error: "", notice: "" }));
    try {
      await bbsApi.cancelMallOrder(id, auth.accessToken);
      setState((current) => ({ ...current, action: "", error: "", notice: "订单已取消。" }));
      loadOrders();
    } catch (error) {
      setState((current) => ({ ...current, action: "", error: friendlyMallOrderActionError(error, "取消订单失败，请刷新订单后重试。"), notice: "" }));
    }
  }

  async function confirmOrder(order) {
    const id = toId(order.id);
    if (!id) return;
    setState((current) => ({ ...current, action: `confirm-${id}`, error: "", notice: "" }));
    try {
      await bbsApi.confirmMallOrder(id, auth.accessToken);
      setState((current) => ({ ...current, action: "", error: "", notice: "已确认收货，订单已完成。" }));
      loadOrders();
    } catch (error) {
      setState((current) => ({ ...current, action: "", error: friendlyMallOrderActionError(error, "确认收货失败，请刷新订单后重试。"), notice: "" }));
    }
  }

  function openRefundForm(order, refund) {
    const id = toId(order.id);
    if (!id || refund || !canApplyRefund(order)) return;
    setRefundForm({
      orderId: id,
      orderNo: order.order_no || order.orderNo || `订单 #${id}`,
      amountCredits: orderPaidCredits(order),
      reason: refundReasons[0].value,
      note: ""
    });
    setState((current) => ({ ...current, error: "", notice: "" }));
  }

  function openOrderDetail(order) {
    const id = toId(order.id);
    if (!id) return;
    setSelectedOrderId(id);
    setSearchParams({ order_id: id });
  }

  function closeOrderDetail() {
    setSelectedOrderId("");
    setSearchParams({}, { replace: true });
  }

  function openProductReview(productId, orderId) {
    const id = toId(productId);
    if (!id) return;
    const params = new URLSearchParams({ product_id: String(id) });
    const reviewOrderId = toId(orderId);
    if (reviewOrderId) {
      params.set("review_order_id", String(reviewOrderId));
    }
    navigate(`/shop?${params.toString()}`);
  }

  async function submitRefund(event) {
    event.preventDefault();
    if (!refundForm?.orderId) return;
    const note = refundForm.note.trim();
    if (note.length < 4) {
      setState((current) => ({ ...current, error: "请填写至少 4 个字的售后说明。", notice: "" }));
      return;
    }
    setState((current) => ({ ...current, action: `refund-${refundForm.orderId}`, error: "", notice: "" }));
    try {
      await bbsApi.createMallRefund(
        refundForm.orderId,
        {
          reason: refundForm.reason,
          note
        },
        auth.accessToken
      );
      setRefundForm(null);
      setState((current) => ({ ...current, action: "", error: "", notice: "售后申请已提交，运营审核后会同步更新积分流水。" }));
      loadOrders();
    } catch (error) {
      setState((current) => ({ ...current, action: "", error: friendlyMallOrderActionError(error, "售后申请失败，请稍后重试。"), notice: "" }));
    }
  }

  return (
    <ModerationSection
      actionError={state.error}
      emptyText="暂无商城订单"
      filters={orderStatusTabs}
      loading={state.loading}
      status={status}
      total={state.total}
      toolbar={
        <button className="route-link-button" type="button" onClick={() => navigate("/shop")}>
          去兑换
        </button>
      }
      onStatusChange={setStatus}
    >
      {state.notice && <p className="form-success order-dashboard-notice">{state.notice}</p>}
      {refundForm && (
        <form className="panel refund-request-form" onSubmit={submitRefund}>
          <div>
            <strong>申请售后</strong>
            <span>
              {refundForm.orderNo} · {refundForm.amountCredits} 积分
            </span>
          </div>
          <label>
            售后原因
            <select value={refundForm.reason} onChange={(event) => setRefundForm((current) => ({ ...current, reason: event.target.value }))}>
              {refundReasons.map((item) => (
                <option key={item.value} value={item.value}>
                  {item.label}
                </option>
              ))}
            </select>
          </label>
          <label>
            问题说明
            <textarea
              maxLength={300}
              placeholder="说明申请售后的原因，便于运营审核"
              value={refundForm.note}
              onChange={(event) => setRefundForm((current) => ({ ...current, note: event.target.value }))}
            />
          </label>
          <div className="refund-request-actions">
            <button type="button" onClick={() => setRefundForm(null)}>
              取消
            </button>
            <button type="submit" disabled={state.action === `refund-${refundForm.orderId}`}>
              {state.action === `refund-${refundForm.orderId}` ? "提交中" : "提交申请"}
            </button>
          </div>
        </form>
      )}
      {selectedOrder && (
        <OrderDetailPanel
          logs={selectedLogs}
          order={selectedOrder}
          payments={selectedPayments}
          refund={selectedRefund}
          confirming={state.action === `confirm-${selectedOrderKey}`}
          onClose={closeOrderDetail}
          onConfirm={() => confirmOrder(selectedOrder)}
          onReviewProduct={openProductReview}
          onRefund={() => openRefundForm(selectedOrder, selectedRefund)}
        />
      )}
      {state.items.map((order) => {
        const id = toId(order.id);
        const currentStatus = toNumber(order.status);
        const canPay = currentStatus === 1 || currentStatus === 2;
        const canCancel = currentStatus === 1 || currentStatus === 2;
        const logs = state.logsByOrder[String(id)] || [];
        const refund = state.refundsByOrder[String(id)];
        const canRefund = canApplyRefund(order) && !refund;
        const canConfirm = currentStatus === 5 && !refund;
        const reviewProductId = orderReviewProductId(order);
        const canReview = currentStatus === 6 && !refund && Boolean(reviewProductId);
        return (
          <WorkspaceRow
            key={id || order.order_no || order.orderNo}
            title={`${order.order_no || order.orderNo || `订单 #${id}`} · ${orderStatusLabel(currentStatus)}`}
            description={`${orderItemsSummary(order)} · ${orderFulfillmentSummary(order)}${orderLogisticsSummary(order) ? ` · ${orderLogisticsSummary(order)}` : ""}`}
            meta={`${orderAmountSummary(order)} · ${refundProgressMeta(refund) || orderProgressMeta(order, logs)}`}
            status={refund ? refundStatusLabel(refund.status) : orderStatusLabel(currentStatus)}
            tags={orderDisplayTags(order, logs, refund)}
            actions={
              <>
                <button type="button" onClick={() => openOrderDetail(order)}>
                  订单详情
                </button>
                {(canPay || canCancel || canConfirm || canRefund || canReview) && (
                  <>
                    {canPay && (
                      <button type="button" disabled={state.action === `pay-${id}`} onClick={() => payOrder(order)}>
                        {state.action === `pay-${id}` ? "支付中" : "继续支付"}
                      </button>
                    )}
                    {canCancel && (
                      <button type="button" disabled={state.action === `cancel-${id}`} onClick={() => cancelOrder(order)}>
                        {state.action === `cancel-${id}` ? "取消中" : "取消订单"}
                      </button>
                    )}
                    {canConfirm && (
                      <button type="button" disabled={state.action === `confirm-${id}`} onClick={() => confirmOrder(order)}>
                        {state.action === `confirm-${id}` ? "确认中" : "确认收货"}
                      </button>
                    )}
                    {canRefund && (
                      <button type="button" disabled={state.action === `refund-${id}`} onClick={() => openRefundForm(order, refund)}>
                        申请售后
                      </button>
                    )}
                    {canReview && (
                      <button type="button" onClick={() => openProductReview(reviewProductId, id)}>
                        评价商品
                      </button>
                    )}
                  </>
                )}
              </>
            }
          />
        );
      })}
    </ModerationSection>
  );
}

function CouponsPanel({ auth }) {
  const navigate = useNavigate();
  const [status, setStatus] = React.useState(4);
  const [state, setState] = React.useState({ items: [], total: 0, loading: false, error: "" });

  React.useEffect(() => {
    let alive = true;
    setState((current) => ({ ...current, loading: true, error: "" }));
    bbsApi
      .mallMyCoupons({ limit: 50, offset: 0, status }, auth.accessToken)
      .then((data) => {
        if (!alive) return;
        const items = listItems(data);
        setState({ items, total: listTotal(data, items), loading: false, error: "" });
      })
      .catch((error) => {
        if (!alive) return;
        setState({ items: [], total: 0, loading: false, error: error.message || "优惠券加载失败" });
      });
    return () => {
      alive = false;
    };
  }, [auth.accessToken, status]);

  return (
    <ModerationSection
      actionError={state.error}
      emptyText={status === 4 ? "暂无可使用优惠券" : "暂无优惠券记录"}
      filters={couponUsageStatusTabs}
      loading={state.loading}
      status={status}
      total={state.total}
      toolbar={
        <button className="route-link-button" type="button" onClick={() => navigate("/shop")}>
          去商城领券
        </button>
      }
      onStatusChange={setStatus}
    >
      {state.items.map((coupon) => {
        const code = couponCodeOf(coupon);
        const orderId = couponOrderIdOf(coupon);
        const canUse = couponUsageStatusValue(coupon?.status ?? coupon?.Status) === 4 && Boolean(code);
        return (
          <WorkspaceRow
            key={coupon.id || coupon.Id || `${code}-${coupon.created_at || coupon.createdAt}`}
            title={`${couponNameOf(coupon) || code || "优惠券"} · ${couponUsageStatusLabel(coupon?.status ?? coupon?.Status)}`}
            description={`${couponDescriptionOf(coupon) || couponTimeText(coupon)} · ${couponThresholdText(coupon)}`}
            meta={`${couponDiscountText(coupon)} · ${couponUsageTimeMeta(coupon)}`}
            status={couponUsageStatusLabel(coupon?.status ?? coupon?.Status)}
            tags={couponUsageTags(coupon)}
            actions={
              <>
                {canUse && (
                  <button type="button" onClick={() => navigate(`/shop?coupon_code=${encodeURIComponent(code)}`)}>
                    去使用
                  </button>
                )}
                {orderId && (
                  <button type="button" onClick={() => navigate(`/dashboard/orders?order_id=${encodeURIComponent(orderId)}`)}>
                    查看订单
                  </button>
                )}
              </>
            }
          />
        );
      })}
    </ModerationSection>
  );
}

function AddressesPanel({ auth }) {
  const navigate = useNavigate();
  const [state, setState] = React.useState({ items: [], total: 0, loading: false, error: "", action: "", notice: "" });
  const [editingId, setEditingId] = React.useState("");
  const [form, setForm] = React.useState(() => emptyAddressForm(auth?.user?.nickname || ""));

  const loadAddresses = React.useCallback(() => {
    let alive = true;
    setState((current) => ({ ...current, loading: true, error: "" }));
    bbsApi
      .mallAddresses({ limit: 50, offset: 0 }, auth.accessToken)
      .then((data) => {
        if (!alive) return;
        const items = sortAddresses(listItems(data));
        setState((current) => ({ ...current, items, total: listTotal(data, items), loading: false, error: "", action: "" }));
      })
      .catch((error) => {
        if (!alive) return;
        setState((current) => ({ ...current, items: [], total: 0, loading: false, error: error.message || "地址加载失败", action: "" }));
      });
    return () => {
      alive = false;
    };
  }, [auth.accessToken]);

  React.useEffect(loadAddresses, [loadAddresses]);

  function updateForm(field, value) {
    setForm((current) => ({ ...current, [field]: value }));
  }

  function startCreateAddress() {
    setEditingId("");
    setForm(emptyAddressForm(auth?.user?.nickname || ""));
    setState((current) => ({ ...current, error: "", notice: "" }));
  }

  function startEditAddress(address) {
    const id = addressIdOf(address);
    if (!id) return;
    setEditingId(id);
    setForm(addressToForm(address));
    setState((current) => ({ ...current, error: "", notice: "" }));
  }

  async function saveAddress(event) {
    event.preventDefault();
    const validation = validateAddressForm(form);
    if (validation) {
      setState((current) => ({ ...current, error: validation, notice: "" }));
      return;
    }
    setState((current) => ({ ...current, action: "save", error: "", notice: "" }));
    try {
      const currentAddress = state.items.find((address) => sameId(addressIdOf(address), editingId));
      const payload = addressFormPayload(form, {
        isDefault: editingId ? addressIsDefault(currentAddress) : state.items.length === 0
      });
      const data = editingId ? await bbsApi.updateMallAddress(editingId, payload, auth.accessToken) : await bbsApi.createMallAddress(payload, auth.accessToken);
      setState((current) => ({
        ...current,
        notice: editingId ? "收货地址已更新。" : "收货地址已新增。",
        action: "",
        error: ""
      }));
      setEditingId("");
      setForm(data?.address ? addressToForm(data.address) : emptyAddressForm(auth?.user?.nickname || ""));
      loadAddresses();
    } catch (error) {
      setState((current) => ({ ...current, action: "", error: error.message || "地址保存失败", notice: "" }));
    }
  }

  async function setDefaultAddress(address) {
    const id = addressIdOf(address);
    if (!id) return;
    setState((current) => ({ ...current, action: `default-${id}`, error: "", notice: "" }));
    try {
      await bbsApi.setDefaultMallAddress(id, auth.accessToken);
      setState((current) => ({ ...current, action: "", notice: "默认收货地址已更新。" }));
      loadAddresses();
    } catch (error) {
      setState((current) => ({ ...current, action: "", error: error.message || "默认地址设置失败", notice: "" }));
    }
  }

  async function deleteAddress(address) {
    const id = addressIdOf(address);
    if (!id) return;
    setState((current) => ({ ...current, action: `delete-${id}`, error: "", notice: "" }));
    try {
      await bbsApi.deleteMallAddress(id, auth.accessToken);
      if (sameId(id, editingId)) {
        startCreateAddress();
      }
      setState((current) => ({ ...current, action: "", notice: "收货地址已删除。" }));
      loadAddresses();
    } catch (error) {
      setState((current) => ({ ...current, action: "", error: error.message || "地址删除失败", notice: "" }));
    }
  }

  return (
    <section className="address-manager-panel">
      <form className="panel address-manager-form" onSubmit={saveAddress}>
        <header>
          <div>
            <strong>{editingId ? "编辑收货地址" : "新增收货地址"}</strong>
            <span>默认地址会在商城结算时自动选中，也会用于订单履约信息。</span>
          </div>
          {editingId && (
            <button type="button" onClick={startCreateAddress}>
              新增地址
            </button>
          )}
        </header>
        <div className="address-manager-grid">
          <label>
            <span>收件人</span>
            <input value={form.receiver} onChange={(event) => updateForm("receiver", event.target.value)} />
          </label>
          <label>
            <span>联系电话</span>
            <input value={form.phone} onChange={(event) => updateForm("phone", event.target.value)} />
          </label>
          <label>
            <span>省份</span>
            <input value={form.province} onChange={(event) => updateForm("province", event.target.value)} />
          </label>
          <label>
            <span>城市</span>
            <input value={form.city} onChange={(event) => updateForm("city", event.target.value)} />
          </label>
          <label>
            <span>区县</span>
            <input value={form.district} onChange={(event) => updateForm("district", event.target.value)} />
          </label>
          <label>
            <span>邮编</span>
            <input value={form.postalCode} onChange={(event) => updateForm("postalCode", event.target.value)} />
          </label>
          <label className="is-wide">
            <span>详细地址</span>
            <input value={form.detail} onChange={(event) => updateForm("detail", event.target.value)} />
          </label>
        </div>
        {state.error && <p className="form-error">{state.error}</p>}
        {state.notice && <p className="form-success">{state.notice}</p>}
        <div className="address-manager-actions">
          <button type="submit" disabled={state.action === "save"}>
            {state.action === "save" ? "保存中" : editingId ? "保存修改" : "保存地址"}
          </button>
          {editingId && (
            <button type="button" disabled={state.action === "save"} onClick={startCreateAddress}>
              取消编辑
            </button>
          )}
        </div>
      </form>

      <ModerationSection
        actionError=""
        emptyText="暂无收货地址"
        filters={[]}
        loading={state.loading}
        status={0}
        total={state.total}
        toolbar={
          <button className="route-link-button" type="button" onClick={() => navigate("/shop")}>
            去商城使用
          </button>
        }
        onStatusChange={() => {}}
      >
        {state.items.map((address) => {
          const id = addressIdOf(address);
          const isDefault = addressIsDefault(address);
          return (
            <WorkspaceRow
              focused={isDefault}
              key={id || `${address.receiver}-${address.phone}`}
              title={`${address.receiver || "未填写收件人"}${isDefault ? " · 默认地址" : ""}`}
              description={`${address.phone || "未填写电话"} · ${addressLine(address) || "未填写详细地址"}`}
              meta={addressTimeMeta(address)}
              status={isDefault ? "默认" : "地址"}
              tags={addressTags(address)}
              actions={
                <>
                  <button type="button" onClick={() => startEditAddress(address)}>
                    编辑
                  </button>
                  {!isDefault && (
                    <button type="button" disabled={state.action === `default-${id}`} onClick={() => setDefaultAddress(address)}>
                      {state.action === `default-${id}` ? "设置中" : "设默认"}
                    </button>
                  )}
                  <button type="button" disabled={state.action === `delete-${id}`} onClick={() => deleteAddress(address)}>
                    {state.action === `delete-${id}` ? "删除中" : "删除"}
                  </button>
                </>
              }
            />
          );
        })}
      </ModerationSection>
    </section>
  );
}

function RefundsPanel({ auth }) {
  const navigate = useNavigate();
  const [status, setStatus] = React.useState(0);
  const [state, setState] = React.useState({ items: [], total: 0, loading: false, error: "" });

  React.useEffect(() => {
    let alive = true;
    setState((current) => ({ ...current, loading: true, error: "" }));
    bbsApi
      .mallRefunds({ limit: 50, offset: 0, status }, auth.accessToken)
      .then((data) => {
        if (!alive) return;
        const items = listItems(data);
        setState({ items, total: listTotal(data, items), loading: false, error: "" });
      })
      .catch((error) => {
        if (!alive) return;
        setState({ items: [], total: 0, loading: false, error: error.message || "售后列表加载失败" });
      });
    return () => {
      alive = false;
    };
  }, [auth.accessToken, status]);

  return (
    <ModerationSection
      actionError={state.error}
      emptyText="暂无售后申请"
      filters={refundStatusTabs}
      loading={state.loading}
      status={status}
      total={state.total}
      toolbar={
        <button className="route-link-button" type="button" onClick={() => navigate("/dashboard/orders")}>
          查看订单
        </button>
      }
      onStatusChange={setStatus}
    >
      {state.items.map((refund) => {
        const orderId = toId(refund.order_id ?? refund.orderId);
        return (
          <WorkspaceRow
            key={refund.id || `${orderId}-${refund.reason}`}
            title={`${refund.order_no || refund.orderNo || `订单 #${orderId || "-"}`} · ${refundStatusLabel(refund.status)}`}
            description={`${refundReasonLabel(refund.reason)} · ${refund.user_note || refund.userNote || "未填写售后说明"}`}
            meta={`${refundAmountSummary(refund)} · ${refundTimeMeta(refund)}`}
            status={refundStatusLabel(refund.status)}
            tags={refundTags(refund)}
            actions={
              orderId && (
                <button type="button" onClick={() => navigate(`/dashboard/orders?order_id=${encodeURIComponent(orderId)}`)}>
                  查看订单
                </button>
              )
            }
          />
        );
      })}
    </ModerationSection>
  );
}

function ReviewsPanel({ auth }) {
  const navigate = useNavigate();
  const [searchParams, setSearchParams] = useSearchParams();
  const focusedReviewId = toId(searchParams.get("review_id"));
  const focusedProductId = toId(searchParams.get("product_id"));
  const hasReviewFocus = Boolean(focusedReviewId || focusedProductId);
  const [status, setStatus] = React.useState(0);
  const [state, setState] = React.useState({ items: [], total: 0, loading: false, error: "" });

  React.useEffect(() => {
    let alive = true;
    setState((current) => ({ ...current, loading: true, error: "" }));
    bbsApi
      .mallReviews({ limit: 50, offset: 0, status, product_id: focusedProductId || undefined }, auth.accessToken)
      .then((data) => {
        if (!alive) return;
        const items = sortFocusedReviews(listItems(data), focusedReviewId, focusedProductId);
        setState({ items, total: listTotal(data, items), loading: false, error: "" });
      })
      .catch((error) => {
        if (!alive) return;
        setState({ items: [], total: 0, loading: false, error: error.message || "评价加载失败" });
      });
    return () => {
      alive = false;
    };
  }, [auth.accessToken, focusedProductId, focusedReviewId, status]);

  React.useEffect(() => {
    if (hasReviewFocus) {
      setStatus(0);
    }
  }, [hasReviewFocus, focusedProductId, focusedReviewId]);

  function changeStatus(nextStatus) {
    setStatus(nextStatus);
    if (hasReviewFocus) {
      setSearchParams({}, { replace: true });
    }
  }

  function clearFocus() {
    setSearchParams({}, { replace: true });
  }

  return (
    <ModerationSection
      actionError={state.error}
      emptyText={hasReviewFocus ? "暂无匹配评价" : "暂无商品评价"}
      filters={reviewStatusTabs}
      loading={state.loading}
      status={status}
      total={state.total}
      toolbar={
        <>
          <button className="route-link-button" type="button" onClick={() => navigate("/shop")}>
            去商城
          </button>
          {hasReviewFocus && (
            <button className="route-link-button" type="button" onClick={clearFocus}>
              清除定位
            </button>
          )}
        </>
      }
      onStatusChange={changeStatus}
    >
      {state.items.map((review) => {
        const productId = toId(review.product_id ?? review.productId);
        const orderId = toId(review.order_id ?? review.orderId);
        const createdAt = review.created_at || review.createdAt;
        const reviewImages = markdownImageUrls(review.content);
        const focused = reviewMatchesFocus(review, focusedReviewId, focusedProductId);
        const tags = focused ? ["当前定位", ...reviewTags(review)] : reviewTags(review);
        return (
          <WorkspaceRow
            focused={focused}
            key={review.id}
            title={`${review.product_title || review.productTitle || `商品 #${productId || "-"}`} · ${reviewRatingText(review.rating)}`}
            description={textWithoutMarkdownImages(review.content) || "未填写评价内容"}
            meta={`${orderId ? `订单 #${orderId}` : "订单"} · ${createdAt ? timeAgoMillis(createdAt) : "刚刚"}`}
            media={reviewImages}
            status={reviewStatusLabel(review.status)}
            tags={tags}
            actions={
              <>
                <button type="button" onClick={() => navigate(productId ? `/shop?product_id=${encodeURIComponent(productId)}` : "/shop")}>
                  查看商品
                </button>
                {focused && (
                  <button type="button" onClick={clearFocus}>
                    清除定位
                  </button>
                )}
              </>
            }
          />
        );
      })}
    </ModerationSection>
  );
}

function OrderDetailPanel({ confirming = false, logs = [], order, payments = [], refund, onClose, onConfirm, onReviewProduct, onRefund }) {
  const items = Array.isArray(order?.items) ? order.items : [];
  const entitlements = digitalEntitlementsOf(order);
  const orderId = toId(order?.id);
  const status = toNumber(order?.status);
  const canRefund = canApplyRefund(order) && !refund;
  const canConfirm = status === 5 && !refund;
  const canReview = status === 6 && !refund;

  return (
    <section className="panel order-detail-panel">
      <header>
        <div>
          <span>订单详情</span>
          <strong>{order?.order_no || order?.orderNo || `订单 #${order?.id}`}</strong>
          <p>
            {orderStatusLabel(status)} · {orderAmountSummary(order)}
          </p>
        </div>
        <div className="order-detail-actions">
          {canConfirm && (
            <button type="button" disabled={confirming} onClick={onConfirm}>
              {confirming ? "确认中" : "确认收货"}
            </button>
          )}
          {canRefund && (
            <button type="button" onClick={onRefund}>
              申请售后
            </button>
          )}
          <button type="button" onClick={onClose}>
            关闭
          </button>
        </div>
      </header>
      <div className="order-detail-grid">
        <section>
          <h3>商品明细</h3>
          <div className="order-detail-items">
            {items.length === 0 && <p>暂无商品明细</p>}
            {items.map((item) => {
              const productId = itemProductId(item);
              return (
                <article key={`${productId || "product"}-${item.sku || item.title}`}>
                  <div>
                    <strong>{item.title || item.sku || `商品 #${productId || "-"}`}</strong>
                    <span>
                      {toNumber(item.quantity)} 件 · {toNumber(item.unit_price_credits ?? item.unitPriceCredits)} 积分/件 · 小计{" "}
                      {toNumber(item.subtotal_credits ?? item.subtotalCredits)} 积分
                    </span>
                  </div>
                  {canReview && productId && (
                    <button type="button" onClick={() => onReviewProduct(productId, orderId)}>
                      去评价
                    </button>
                  )}
                </article>
              );
            })}
          </div>
        </section>
        <section>
          <h3>履约信息</h3>
          <dl className="order-detail-fields">
            <div>
              <dt>收件人</dt>
              <dd>{order?.receiver || "未填写"}</dd>
            </div>
            <div>
              <dt>联系电话</dt>
              <dd>{order?.phone || "未填写"}</dd>
            </div>
            <div>
              <dt>收货地址</dt>
              <dd>{order?.address || "未填写"}</dd>
            </div>
            <div>
              <dt>物流</dt>
              <dd>{orderTrackingSummary(order) || "暂无物流"}</dd>
            </div>
          </dl>
          {entitlements.length > 0 && (
            <div className="order-entitlement-list">
              {entitlements.map((entitlement) => (
                <article key={`${entitlementProductId(entitlement)}-${entitlementCode(entitlement) || entitlement.sku || entitlement.title}`}>
                  <strong>{entitlement.title || entitlement.sku || `权益 #${entitlementProductId(entitlement)}`}</strong>
                  <span>
                    {entitlementCode(entitlement) || "已发放"} · {entitlementIssuedText(entitlement)}
                  </span>
                </article>
              ))}
            </div>
          )}
        </section>
        <section>
          <h3>金额信息</h3>
          <dl className="order-detail-fields">
            <div>
              <dt>商品原价</dt>
              <dd>{orderOriginalCredits(order)} 积分</dd>
            </div>
            <div>
              <dt>优惠金额</dt>
              <dd>{orderDiscountCredits(order)} 积分</dd>
            </div>
            <div>
              <dt>实付积分</dt>
              <dd>{orderPaidCredits(order)} 积分</dd>
            </div>
            <div>
              <dt>优惠券码</dt>
              <dd>{orderCouponCode(order) || "未使用"}</dd>
            </div>
          </dl>
        </section>
      </div>
      {refund && (
        <section className="order-detail-refund">
          <h3>售后进度</h3>
          <p>
            {refundStatusLabel(refund.status)} · {refundReasonLabel(refund.reason)}
          </p>
          {(refund.user_note || refund.userNote) && <span>申请说明：{refund.user_note || refund.userNote}</span>}
          {(refund.admin_note || refund.adminNote) && <span>审核备注：{refund.admin_note || refund.adminNote}</span>}
        </section>
      )}
      <section className="order-detail-payments">
        <h3>支付记录</h3>
        {payments.length === 0 && <p>暂无支付记录</p>}
        {payments.map((payment) => {
          const failureReason = payment.failure_reason || payment.failureReason;
          return (
            <article key={payment.id || `${payment.order_id || payment.orderId}-${payment.idempotency_key || payment.idempotencyKey}`}>
              <strong>
                {paymentStatusLabel(payment.status)} · {toNumber(payment.amount_credits ?? payment.amountCredits)} 积分
              </strong>
              <span>
                {payment.provider || "credits"} · {paymentTimeMeta(payment)}
                {failureReason ? ` · ${failureReason}` : ""}
              </span>
            </article>
          );
        })}
      </section>
      <section className="order-detail-timeline">
        <h3>状态时间线</h3>
        {logs.length === 0 && <p>暂无状态记录</p>}
        {logs.map((log) => (
          <article key={log.id || `${log.to_status}-${log.created_at}`}>
            <span>{timeAgoMillis(log.created_at || log.createdAt)}</span>
            <strong>
              {orderTimelineStatusLabel(log.from_status ?? log.fromStatus)} 到 {orderTimelineStatusLabel(log.to_status ?? log.toStatus)}
            </strong>
            <p>{log.note || orderLogReasonLabel(log.reason) || "系统状态更新"}</p>
          </article>
        ))}
      </section>
    </section>
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
    background_url: auth.user?.background_url || auth.user?.backgroundUrl || "",
    bio: auth.user?.bio || ""
  });
  const [state, setState] = React.useState({ saving: false, error: "", message: "" });
  const [avatarUpload, setAvatarUpload] = React.useState({ loading: false, error: "", message: "" });
  const [backgroundUpload, setBackgroundUpload] = React.useState({ loading: false, error: "", message: "" });
  const [verification, setVerification] = React.useState({ loading: false, error: "", message: "", verifyUrl: "" });
  const verified = isEmailVerified(auth.user);

  React.useEffect(() => {
    setForm({
      nickname: auth.user?.nickname || "",
      avatar_url: auth.user?.avatar_url || auth.user?.avatarUrl || "",
      background_url: auth.user?.background_url || auth.user?.backgroundUrl || "",
      bio: auth.user?.bio || ""
    });
  }, [auth]);

  function updateField(field, value) {
    setForm((current) => ({ ...current, [field]: value }));
  }

  async function submit(event) {
    event.preventDefault();
    setState({ saving: true, error: "", message: "" });
    setAvatarUpload((current) => ({ ...current, message: "" }));
    setBackgroundUpload((current) => ({ ...current, message: "" }));
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

  async function uploadAvatar(event) {
    const file = event.target.files?.[0];
    event.target.value = "";
    if (!file) return;
    setAvatarUpload({ loading: true, error: "", message: "" });
    setState((current) => ({ ...current, error: "", message: "" }));
    try {
      const data = await bbsApi.uploadAvatar(file, auth.accessToken);
      const avatarUrl = data?.avatar_url || data?.avatarUrl || data?.url || data?.path || "";
      if (!avatarUrl) {
        throw new Error("头像上传成功但未返回地址");
      }
      updateField("avatar_url", avatarUrl);
      setAvatarUpload({ loading: false, error: "", message: "头像已上传，保存资料后生效。" });
    } catch (error) {
      setAvatarUpload({ loading: false, error: error.message || "头像上传失败", message: "" });
    }
  }

  async function uploadBackground(event) {
    const file = event.target.files?.[0];
    event.target.value = "";
    if (!file) return;
    setBackgroundUpload({ loading: true, error: "", message: "" });
    setState((current) => ({ ...current, error: "", message: "" }));
    try {
      const data = await bbsApi.uploadImage(file, auth.accessToken);
      const backgroundUrl = data?.image_url || data?.imageUrl || data?.url || "";
      if (!backgroundUrl) {
        throw new Error("背景图上传成功但未返回地址");
      }
      updateField("background_url", backgroundUrl);
      setBackgroundUpload({ loading: false, error: "", message: "背景图已上传，保存资料后生效。" });
    } catch (error) {
      setBackgroundUpload({ loading: false, error: error.message || "背景图上传失败", message: "" });
    }
  }

  async function requestVerification() {
    setVerification({ loading: true, error: "", message: "", verifyUrl: "" });
    try {
      const data = await bbsApi.requestEmailVerification(auth.accessToken);
      if (data?.already_verified) {
        onAuthUserUpdate?.({
          ...auth.user,
          email_verified: true,
          email_verified_at: auth.user?.email_verified_at || Date.now()
        });
        setVerification({ loading: false, error: "", message: "邮箱已验证。", verifyUrl: "" });
        return;
      }
      setVerification({
        loading: false,
        error: "",
        message: "验证链接已生成，请在邮箱中完成验证。",
        verifyUrl: data?.verify_url || ""
      });
    } catch (error) {
      setVerification({ loading: false, error: error.message || "发送验证链接失败", message: "", verifyUrl: "" });
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
        <div className="profile-background-upload">
          <div className="profile-background-preview" style={form.background_url ? { backgroundImage: `url(${JSON.stringify(form.background_url)})` } : undefined} />
          <label>
            <input accept="image/jpeg,image/png,image/gif,image/webp" className="sr-only" disabled={backgroundUpload.loading} type="file" onChange={uploadBackground} />
            <ImagePlus size={17} aria-hidden="true" />
            <span>{backgroundUpload.loading ? "上传中..." : "上传背景图"}</span>
          </label>
        </div>
        <label>
          背景图 URL
          <input value={form.background_url} onChange={(event) => updateField("background_url", event.target.value)} />
        </label>
        <div className="profile-avatar-upload profile-avatar-upload--wide">
          <img src={form.avatar_url || userAvatar(auth.user)} alt="" />
          <label>
            <input accept="image/jpeg,image/png,image/gif,image/webp" className="sr-only" disabled={avatarUpload.loading} type="file" onChange={uploadAvatar} />
            <span>{avatarUpload.loading ? "上传中..." : "上传头像"}</span>
          </label>
        </div>
        <label>
          头像 URL
          <input value={form.avatar_url} onChange={(event) => updateField("avatar_url", event.target.value)} />
        </label>
        <label>
          个人简介
          <textarea value={form.bio} onChange={(event) => updateField("bio", event.target.value)} />
        </label>
        <div className="email-verify-box">
          <div>
            <MailCheck size={18} aria-hidden="true" />
            <span>{auth.user?.email || "未绑定邮箱"}</span>
            <strong>{verified ? "已验证" : "未验证"}</strong>
          </div>
          {!verified && (
            <button type="button" disabled={verification.loading} onClick={requestVerification}>
              {verification.loading ? "发送中..." : "发送验证链接"}
            </button>
          )}
        </div>
        {avatarUpload.error && <p className="form-error">{avatarUpload.error}</p>}
        {avatarUpload.message && <p className="form-success">{avatarUpload.message}</p>}
        {backgroundUpload.error && <p className="form-error">{backgroundUpload.error}</p>}
        {backgroundUpload.message && <p className="form-success">{backgroundUpload.message}</p>}
        {verification.error && <p className="form-error">{verification.error}</p>}
        {verification.message && <p className="form-success">{verification.message}</p>}
        {verification.verifyUrl && (
          <a className="route-link-button" href={verification.verifyUrl}>
            本地继续验证
          </a>
        )}
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

function WorkspaceRow({ actions, description, focused = false, media = [], meta, status, tags = [], title }) {
  return (
    <article className={`moderation-row panel ${focused ? "is-focused" : ""}`}>
      <div>
        <strong>{title}</strong>
        <p>{description}</p>
        {media.length > 0 && (
          <div className="moderation-row-media">
            {media.slice(0, 6).map((url, index) => (
              <img src={url} alt="" key={`${url}-${index}`} />
            ))}
          </div>
        )}
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

function couponSourceOf(coupon) {
  return coupon?.coupon || coupon?.Coupon || coupon || {};
}

function couponCodeOf(coupon) {
  const source = couponSourceOf(coupon);
  return String(coupon?.code || coupon?.Code || source?.code || source?.Code || "").trim().toUpperCase();
}

function couponNameOf(coupon) {
  const source = couponSourceOf(coupon);
  return source?.name || source?.Name || coupon?.name || coupon?.Name || "";
}

function couponDescriptionOf(coupon) {
  const source = couponSourceOf(coupon);
  return source?.description || source?.Description || coupon?.description || coupon?.Description || "";
}

function couponDiscountOf(coupon) {
  const source = couponSourceOf(coupon);
  return toNumber(coupon?.discount_credits ?? coupon?.discountCredits ?? source?.discount_credits ?? source?.discountCredits);
}

function couponMinOrderOf(coupon) {
  const source = couponSourceOf(coupon);
  return toNumber(coupon?.min_order_credits ?? coupon?.minOrderCredits ?? source?.min_order_credits ?? source?.minOrderCredits);
}

function couponStartsAtOf(coupon) {
  const source = couponSourceOf(coupon);
  return toNumber(coupon?.starts_at ?? coupon?.startsAt ?? source?.starts_at ?? source?.startsAt);
}

function couponEndsAtOf(coupon) {
  const source = couponSourceOf(coupon);
  return toNumber(coupon?.ends_at ?? coupon?.endsAt ?? source?.ends_at ?? source?.endsAt);
}

function couponOrderIdOf(coupon) {
  return toId(coupon?.order_id ?? coupon?.orderId);
}

function couponThresholdText(coupon) {
  const minOrder = couponMinOrderOf(coupon);
  return minOrder > 0 ? `满 ${minOrder} 积分可用` : "无门槛";
}

function couponDiscountText(coupon) {
  return `优惠 ${couponDiscountOf(coupon)} 积分`;
}

function couponTimeText(coupon) {
  const startsAt = couponStartsAtOf(coupon);
  const endsAt = couponEndsAtOf(coupon);
  if (!startsAt && !endsAt) return "长期有效";
  return `${formatCouponDate(startsAt) || "现在"} 至 ${formatCouponDate(endsAt) || "不限"}`;
}

function couponUsageTimeMeta(coupon) {
  const usedAt = coupon?.used_at || coupon?.usedAt;
  const releasedAt = coupon?.released_at || coupon?.releasedAt;
  const createdAt = coupon?.created_at || coupon?.createdAt;
  const status = couponUsageStatusValue(coupon?.status ?? coupon?.Status);
  if (status === 2 && usedAt) return `使用于 ${timeAgoMillis(usedAt)}`;
  if (status === 3 && releasedAt) return `释放于 ${timeAgoMillis(releasedAt)}`;
  return createdAt ? `领取于 ${timeAgoMillis(createdAt)}` : "刚刚领取";
}

function couponUsageStatusValue(status) {
  if (status === undefined || status === null || status === "") return 0;
  const numeric = Number(status);
  if (!Number.isNaN(numeric) && numeric > 0) return numeric;
  const text = String(status).trim().toUpperCase();
  const labels = {
    RESERVED: 1,
    COUPON_USAGE_STATUS_RESERVED: 1,
    USED: 2,
    COUPON_USAGE_STATUS_USED: 2,
    RELEASED: 3,
    COUPON_USAGE_STATUS_RELEASED: 3,
    CLAIMED: 4,
    COUPON_USAGE_STATUS_CLAIMED: 4
  };
  return labels[text] || 0;
}

function couponUsageStatusLabel(status) {
  const labels = {
    1: "已锁定",
    2: "已使用",
    3: "已释放",
    4: "可使用"
  };
  return labels[couponUsageStatusValue(status)] || "优惠券状态未知";
}

function couponUsageTags(coupon) {
  const tags = [];
  const code = couponCodeOf(coupon);
  const orderId = couponOrderIdOf(coupon);
  const endsAt = couponEndsAtOf(coupon);
  if (code) tags.push(`券码：${code}`);
  tags.push(couponThresholdText(coupon));
  if (endsAt) tags.push(`有效期至 ${formatCouponDate(endsAt)}`);
  if (orderId) tags.push(`订单 #${orderId}`);
  return tags;
}

function formatCouponDate(value) {
  const timestamp = toNumber(value);
  if (!timestamp) return "";
  const millis = timestamp > 9999999999 ? timestamp : timestamp * 1000;
  return new Date(millis).toLocaleDateString("zh-CN", { year: "numeric", month: "2-digit", day: "2-digit" });
}

function emptyAddressForm(receiver = "") {
  return {
    receiver,
    phone: "",
    province: "",
    city: "",
    district: "",
    postalCode: "",
    detail: ""
  };
}

function addressToForm(address) {
  return {
    receiver: address?.receiver || "",
    phone: address?.phone || "",
    province: address?.province || "",
    city: address?.city || "",
    district: address?.district || "",
    postalCode: address?.postal_code || address?.postalCode || "",
    detail: address?.detail || addressLine(address)
  };
}

function addressFormPayload(form, options = {}) {
  return {
    receiver: form.receiver.trim(),
    phone: form.phone.trim(),
    province: form.province.trim(),
    city: form.city.trim(),
    district: form.district.trim(),
    postal_code: form.postalCode.trim(),
    detail: form.detail.trim(),
    is_default: Boolean(options.isDefault)
  };
}

function validateAddressForm(form) {
  if (!form.receiver.trim()) return "请填写收件人。";
  if (!form.phone.trim()) return "请填写联系电话。";
  if (!form.detail.trim()) return "请填写详细地址。";
  return "";
}

function sortAddresses(items = []) {
  return [...items].sort((left, right) => {
    const leftDefault = addressIsDefault(left);
    const rightDefault = addressIsDefault(right);
    if (leftDefault !== rightDefault) return leftDefault ? -1 : 1;
    return toNumber(right.updated_at ?? right.updatedAt ?? right.created_at ?? right.createdAt) - toNumber(left.updated_at ?? left.updatedAt ?? left.created_at ?? left.createdAt);
  });
}

function addressIdOf(address) {
  return toId(address?.id ?? address?.Id);
}

function addressIsDefault(address) {
  return Boolean(address?.is_default || address?.isDefault);
}

function addressLine(address) {
  return [address?.province, address?.city, address?.district, address?.detail].filter(Boolean).join(" ").trim();
}

function addressTags(address) {
  const tags = [];
  const postalCode = address?.postal_code || address?.postalCode;
  if (address?.province || address?.city) tags.push([address.province, address.city].filter(Boolean).join(" / "));
  if (address?.district) tags.push(address.district);
  if (postalCode) tags.push(`邮编：${postalCode}`);
  return tags;
}

function addressTimeMeta(address) {
  const updatedAt = address?.updated_at || address?.updatedAt;
  const createdAt = address?.created_at || address?.createdAt;
  if (updatedAt) return `更新于 ${timeAgoMillis(updatedAt)}`;
  if (createdAt) return `创建于 ${timeAgoMillis(createdAt)}`;
  return "地址簿";
}

function orderStatusLabel(status) {
  const labels = {
    1: "待支付",
    2: "支付中",
    3: "已支付",
    4: "已取消",
    5: "已发货",
    6: "已完成",
    7: "已关闭",
    8: "已退款"
  };
  return labels[toNumber(status)] || "未知";
}

function reviewStatusLabel(status) {
  const normalized = typeof status === "string" ? status.toUpperCase() : String(toNumber(status));
  const labels = {
    "1": "待审核",
    "2": "已展示",
    "3": "已隐藏",
    PENDING: "待审核",
    PUBLISHED: "已展示",
    HIDDEN: "已隐藏",
    PRODUCT_REVIEW_STATUS_PENDING: "待审核",
    PRODUCT_REVIEW_STATUS_PUBLISHED: "已展示",
    PRODUCT_REVIEW_STATUS_HIDDEN: "已隐藏"
  };
  return labels[normalized] || "评价状态未知";
}

function reviewRatingText(value) {
  const rating = Math.max(1, Math.min(5, toNumber(value, 5)));
  return `${"★".repeat(rating)}${"☆".repeat(5 - rating)}`;
}

function reviewTags(review) {
  const tags = [];
  const sku = review.product_sku || review.productSku;
  const productId = toId(review.product_id ?? review.productId);
  const orderId = toId(review.order_id ?? review.orderId);
  if (sku) tags.push(`SKU：${sku}`);
  if (productId) tags.push(`商品 #${productId}`);
  if (orderId) tags.push(`订单 #${orderId}`);
  return tags;
}

function sortFocusedReviews(items = [], focusedReviewId, focusedProductId) {
  if (!focusedReviewId && !focusedProductId) return items;
  return [...items].sort((left, right) => {
    const leftFocused = reviewMatchesFocus(left, focusedReviewId, focusedProductId);
    const rightFocused = reviewMatchesFocus(right, focusedReviewId, focusedProductId);
    if (leftFocused !== rightFocused) return leftFocused ? -1 : 1;
    return 0;
  });
}

function reviewMatchesFocus(review, focusedReviewId, focusedProductId) {
  if (!review) return false;
  if (focusedReviewId) return sameId(review.id, focusedReviewId);
  if (focusedProductId && sameId(review.product_id ?? review.productId, focusedProductId)) return true;
  return false;
}

function refundStatusLabel(status) {
  const normalized = typeof status === "string" ? status.toUpperCase() : String(toNumber(status));
  const labels = {
    "1": "售后待审核",
    "2": "退款处理中",
    "3": "已退款",
    "4": "售后已拒绝",
    REQUESTED: "售后待审核",
    PROCESSING: "退款处理中",
    APPROVED: "已退款",
    REJECTED: "售后已拒绝",
    REFUND_STATUS_REQUESTED: "售后待审核",
    REFUND_STATUS_PROCESSING: "退款处理中",
    REFUND_STATUS_APPROVED: "已退款",
    REFUND_STATUS_REJECTED: "售后已拒绝"
  };
  return labels[normalized] || "售后状态未知";
}

function refundReasonLabel(reason) {
  return refundReasons.find((item) => item.value === reason)?.label || reason || "售后申请";
}

function refundAmountSummary(refund) {
  return `${toNumber(refund?.amount_credits ?? refund?.amountCredits)} 积分`;
}

function refundTimeMeta(refund) {
  const refundedAt = refund?.refunded_at || refund?.refundedAt;
  const reviewedAt = refund?.reviewed_at || refund?.reviewedAt;
  const requestedAt = refund?.requested_at || refund?.requestedAt || refund?.created_at || refund?.createdAt;
  if (refundedAt) return `退款于 ${timeAgoMillis(refundedAt)}`;
  if (reviewedAt) return `审核于 ${timeAgoMillis(reviewedAt)}`;
  return `申请于 ${timeAgoMillis(requestedAt)}`;
}

function refundTags(refund) {
  const tags = [refundReasonLabel(refund?.reason)];
  const adminNote = refund?.admin_note || refund?.adminNote;
  const operatorID = refund?.operator_id || refund?.operatorId;
  if (adminNote) tags.push(`审核：${adminNote}`);
  if (operatorID) tags.push(`审核人：${operatorID}`);
  if (refund?.restore_stock || refund?.restoreStock) tags.push("已恢复库存");
  return tags;
}

function refundsByOrderId(refunds = []) {
  return Object.fromEntries(
    refunds
      .map((refund) => {
        const orderId = toId(refund.order_id ?? refund.orderId);
        return orderId ? [String(orderId), refund] : null;
      })
      .filter(Boolean)
  );
}

function canApplyRefund(order) {
  return [3, 5, 6].includes(toNumber(order?.status));
}

function refundProgressMeta(refund) {
  if (!refund) return "";
  const refundedAt = refund.refunded_at || refund.refundedAt;
  const reviewedAt = refund.reviewed_at || refund.reviewedAt;
  const requestedAt = refund.requested_at || refund.requestedAt || refund.created_at || refund.createdAt;
  if (refundedAt) return `退款于 ${timeAgoMillis(refundedAt)}`;
  if (reviewedAt) return `审核于 ${timeAgoMillis(reviewedAt)}`;
  return `申请于 ${timeAgoMillis(requestedAt)}`;
}

function itemProductId(item) {
  return toId(item?.product_id ?? item?.productId ?? item?.product?.id);
}

function orderReviewProductId(order) {
  const items = Array.isArray(order?.items) ? order.items : [];
  const reviewableItem = items.find((item) => itemProductId(item));
  return itemProductId(reviewableItem);
}

function orderItemsSummary(order) {
  const items = Array.isArray(order?.items) ? order.items : [];
  if (items.length === 0) {
    return "暂无商品明细";
  }
  return items
    .map((item) => {
      const title = item.title || item.sku || `商品 #${itemProductId(item) || "-"}`;
      return `${title} x${toNumber(item.quantity)}`;
    })
    .join("，");
}

function orderItemsTags(order) {
  const items = Array.isArray(order?.items) ? order.items : [];
  return items.slice(0, 4).map((item) => item.sku || item.title || `商品 #${itemProductId(item) || "-"}`);
}

function orderPaidCredits(order) {
  return toNumber(order?.total_credits ?? order?.totalCredits);
}

function orderOriginalCredits(order) {
  const paid = orderPaidCredits(order);
  return toNumber(order?.original_credits ?? order?.originalCredits, paid) || paid;
}

function orderDiscountCredits(order) {
  return toNumber(order?.discount_credits ?? order?.discountCredits);
}

function orderCouponCode(order) {
  return order?.coupon_code || order?.couponCode || "";
}

function digitalEntitlementsOf(order) {
  const entitlements = order?.digital_entitlements ?? order?.digitalEntitlements ?? [];
  return Array.isArray(entitlements) ? entitlements : [];
}

function entitlementCode(entitlement) {
  return entitlement?.fulfillment_code || entitlement?.fulfillmentCode || "";
}

function entitlementIssuedAt(entitlement) {
  return entitlement?.issued_at || entitlement?.issuedAt;
}

function entitlementIssuedText(entitlement) {
  const issuedAt = entitlementIssuedAt(entitlement);
  return issuedAt ? timeAgoMillis(issuedAt) : "发放时间待同步";
}

function entitlementProductId(entitlement) {
  return entitlement?.product_id ?? entitlement?.productId ?? "";
}

function digitalEntitlementSummary(order) {
  const entitlements = digitalEntitlementsOf(order);
  if (entitlements.length === 0) return "";
  const first = entitlements[0];
  const title = first.title || first.sku || "数字权益";
  const code = entitlementCode(first);
  const suffix = entitlements.length > 1 ? ` 等 ${entitlements.length} 项` : "";
  return `${title}${suffix}${code ? ` · ${code}` : ""}`;
}

function isDigitalFulfillmentOrder(order) {
  return !order?.receiver && !order?.phone && !order?.address;
}

function orderAmountSummary(order) {
  const paid = orderPaidCredits(order);
  const discount = orderDiscountCredits(order);
  const original = orderOriginalCredits(order);
  const couponCode = orderCouponCode(order);
  if (discount > 0) {
    return `实付 ${paid} 积分 · 优惠 ${discount} 积分${couponCode ? ` · ${couponCode}` : ""} · 原价 ${original}`;
  }
  return `${paid} 积分`;
}

function orderDisplayTags(order, logs = [], refund) {
  const tags = orderItemsTags(order);
  const entitlement = digitalEntitlementSummary(order);
  if (entitlement) {
    tags.push(`权益：${entitlement}`);
  }
  if (orderDiscountCredits(order) > 0) {
    tags.push(`优惠 ${orderDiscountCredits(order)} 积分`);
  }
  if (refund) {
    tags.push(`${refundStatusLabel(refund.status)}：${refundReasonLabel(refund.reason)}`);
    const note = refund.admin_note || refund.adminNote || refund.user_note || refund.userNote;
    if (note) {
      tags.push(`售后：${note}`);
    }
  }
  const carrier = order.shipping_carrier || order.shippingCarrier;
  const trackingNo = order.tracking_no || order.trackingNo;
  if (carrier || trackingNo) {
    tags.push(`物流：${[carrier, trackingNo].filter(Boolean).join(" / ")}`);
  }
  const fulfillmentLog = latestOrderLog(logs.filter(isFulfillmentLog));
  const fulfillmentNote = fulfillmentLog?.note || fulfillmentLog?.remark || "";
  if (fulfillmentNote) {
    tags.push(`履约：${fulfillmentNote}`);
    return tags;
  }
  const failedPaymentLog = latestOrderLog(logs.filter(isPaymentFailedLog));
  const failedPaymentNote = failedPaymentLog?.note || failedPaymentLog?.remark || "";
  if (failedPaymentNote) {
    tags.push(`支付失败：${failedPaymentNote}`);
  }
  return tags;
}

function isFulfillmentLog(log) {
  const note = log?.note || log?.remark || "";
  const status = toNumber(log?.to_status ?? log?.toStatus);
  const operatorType = log?.operator_type || log?.operatorType;
  return Boolean(note) && operatorType === "admin" && (status === 5 || status === 6);
}

function isPaymentFailedLog(log) {
  const note = log?.note || log?.remark || "";
  const reason = log?.reason || "";
  return Boolean(note) && reason === "payment_failed";
}

function orderFulfillmentSummary(order) {
  const entitlement = digitalEntitlementSummary(order);
  if (entitlement) {
    return `数字权益已发放：${entitlement}`;
  }
  if (isDigitalFulfillmentOrder(order)) {
    return "数字权益在线发放，无需收货地址";
  }
  const receiver = order?.receiver || "未填写收件人";
  const phone = order?.phone || "未填写电话";
  const address = order?.address || "未填写地址";
  return `${receiver} / ${phone} / ${address}`;
}

function orderLogisticsSummary(order) {
  const entitlement = digitalEntitlementSummary(order);
  if (entitlement) {
    return `数字权益 ${entitlement}`;
  }
  const carrier = order?.shipping_carrier || order?.shippingCarrier;
  const trackingNo = order?.tracking_no || order?.trackingNo;
  if (carrier || trackingNo) {
    return `物流 ${[carrier, trackingNo].filter(Boolean).join(" / ")}`;
  }
  const shippedAt = order?.shipped_at || order?.shippedAt;
  if (shippedAt) {
    return `发货于 ${timeAgoMillis(shippedAt)}`;
  }
  const completedAt = order?.completed_at || order?.completedAt;
  if (completedAt) {
    return `完成于 ${timeAgoMillis(completedAt)}`;
  }
  return "";
}

function paymentStatusLabel(status) {
  const normalized = typeof status === "string" ? status.toUpperCase() : String(toNumber(status));
  const labels = {
    "1": "待支付",
    "2": "支付成功",
    "3": "支付失败",
    PENDING: "待支付",
    SUCCEEDED: "支付成功",
    FAILED: "支付失败",
    PAYMENT_STATUS_PENDING: "待支付",
    PAYMENT_STATUS_SUCCEEDED: "支付成功",
    PAYMENT_STATUS_FAILED: "支付失败"
  };
  return labels[normalized] || "支付状态未知";
}

function paymentTimeMeta(payment) {
  const paidAt = payment?.paid_at || payment?.paidAt;
  const updatedAt = payment?.updated_at || payment?.updatedAt;
  const createdAt = payment?.created_at || payment?.createdAt;
  if (paidAt) return `支付于 ${timeAgoMillis(paidAt)}`;
  if (updatedAt) return `更新于 ${timeAgoMillis(updatedAt)}`;
  return `创建于 ${timeAgoMillis(createdAt)}`;
}

function orderTrackingSummary(order) {
  const entitlement = digitalEntitlementSummary(order);
  if (entitlement) {
    return `数字权益 ${entitlement}`;
  }
  const carrier = order?.shipping_carrier || order?.shippingCarrier;
  const trackingNo = order?.tracking_no || order?.trackingNo;
  if (carrier || trackingNo) {
    return [carrier, trackingNo].filter(Boolean).join(" / ");
  }
  return "";
}

function orderProgressMeta(order, logs = []) {
  const latest = latestOrderLog(logs);
  if (latest) {
    const status = orderStatusLabel(latest.to_status ?? latest.toStatus);
    return `${status}于 ${timeAgoMillis(latest.created_at || latest.createdAt)}`;
  }
  return orderTimeMeta(order);
}

function orderTimelineStatusLabel(status) {
  if (toNumber(status) === 0) {
    return "初始";
  }
  return orderStatusLabel(status);
}

function orderLogReasonLabel(reason) {
  const labels = {
    created: "订单创建",
    paying: "进入支付",
    paid: "支付成功",
    payment_failed: "支付失败",
    canceled_by_user: "用户取消",
    expired: "订单超时关闭",
    shipped: "运营发货",
    completed: "订单完成",
    refunded: "售后退款"
  };
  return labels[reason] || reason || "";
}

function latestOrderLog(logs = []) {
  return [...logs]
    .filter(Boolean)
    .sort((left, right) => toNumber(right.created_at || right.createdAt) - toNumber(left.created_at || left.createdAt))[0];
}

function orderTimeMeta(order) {
  const paidAt = order?.paid_at || order?.paidAt;
  const updatedAt = order?.updated_at || order?.updatedAt;
  const createdAt = order?.created_at || order?.createdAt;
  if (paidAt) {
    return `支付于 ${timeAgoMillis(paidAt)}`;
  }
  if (updatedAt) {
    return `更新于 ${timeAgoMillis(updatedAt)}`;
  }
  return `创建于 ${timeAgoMillis(createdAt)}`;
}

function isEmailVerified(user) {
  return Boolean(user?.email_verified || user?.emailVerified || user?.email_verified_at || user?.emailVerifiedAt);
}
