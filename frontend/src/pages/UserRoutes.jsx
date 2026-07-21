import React from "react";
import { useNavigate, useParams } from "react-router-dom";
import { BadgeCheck, Bell, FileText, Heart, LockKeyhole, Star, Trophy, UserRound, Users } from "lucide-react";
import { bbsApi } from "../api";
import Avatar from "../components/Avatar.jsx";
import MessageFilterPanel from "../components/notifications/MessageFilterPanel.jsx";
import PostCard from "../components/post/PostCard.jsx";
import { creditBalance, listItems, listTotal, notificationRead, unreadCount } from "../lib/apiShapes";
import { userBadgeRows } from "../lib/badges";
import { creditEntryMeta, creditReasonLabel, sameId, timeAgoMillis, toId, toNumber } from "../lib/formatters";
import { emitNotificationsChanged } from "../lib/notificationEvents";
import {
  filterNotifications,
  isMallNotification,
  notificationGroupLabel,
  notificationTarget,
  notificationTargetLabel,
  summarizeNotifications
} from "../lib/notificationTargets";
import { articleToPost, authProfileAppearanceNeedsVerification, authToPerson, hydratePostsMeta, interactionToPost, profileThemeClass, userToPerson } from "../lib/postMappers";
import { DataRows, EmptyState, PillTabs, RouteHeader } from "./RouteBlocks.jsx";

const currentUserTabs = [
  { value: "profile", label: "资料", icon: UserRound, path: "/user/profile" },
  { value: "account", label: "账号", icon: LockKeyhole, path: "/user/profile/account" },
  { value: "favorites", label: "收藏", icon: Star, path: "/user/favorites" },
  { value: "messages", label: "消息", icon: Bell, path: "/user/messages" },
  { value: "scores", label: "积分", icon: Trophy, path: "/user/scores" }
];

const publicUserTabs = [
  { value: "profile", label: "主页", icon: UserRound },
  { value: "articles", label: "文章", icon: FileText },
  { value: "badges", label: "徽章", icon: BadgeCheck },
  { value: "fans", label: "粉丝", icon: Users },
  { value: "followed", label: "关注", icon: Heart }
];

const FOLLOW_LIST_PAGE_SIZE = 30;
const BADGE_PAGE_SIZE = 30;
const MESSAGE_PAGE_SIZE = 30;

export function UserRoutePage({ auth, view = "profile" }) {
  const params = useParams();
  const navigate = useNavigate();
  const userId = params.userId ? toId(params.userId) : toId(auth?.user?.id);
  const publicSpace = Boolean(params.userId);
  const [profileState, setProfileState] = React.useState({
    person: auth?.user ? authToPerson(auth) : null,
    loading: false,
    error: ""
  });

  React.useEffect(() => {
    if (!publicSpace) {
      if (!auth?.user) {
        setProfileState({ person: null, loading: false, error: "" });
        return undefined;
      }
      setProfileState({
        person: authToPerson(auth),
        loading: false,
        error: ""
      });
      if (!authProfileAppearanceNeedsVerification(auth) || !userId) {
        return undefined;
      }
      let alive = true;
      bbsApi
        .getUser(userId)
        .then((data) => {
          if (!alive || !data?.user) return;
          setProfileState({ person: userToPerson(data.user), loading: false, error: "" });
        })
        .catch(() => {
          if (!alive) return;
          setProfileState((current) => ({ ...current, loading: false }));
        });
      return () => {
        alive = false;
      };
    }
    let alive = true;
    setProfileState((current) => ({ ...current, loading: true, error: "" }));
    bbsApi
      .getUser(userId)
      .then((data) => {
        if (!alive) return;
        setProfileState({ person: data?.user ? userToPerson(data.user) : null, loading: false, error: "" });
      })
      .catch((error) => {
        if (!alive) return;
        setProfileState({ person: null, loading: false, error: error.message || "用户资料加载失败" });
      });
    return () => {
      alive = false;
    };
  }, [auth?.user, publicSpace, userId]);

  const tabs = publicSpace ? publicUserTabs : currentUserTabs;
  const activeValue = tabs.some((item) => item.value === view) ? view : "profile";

  function changeTab(value) {
    if (publicSpace) {
      const suffix = value === "profile" ? "" : `/${value}`;
      navigate(`/user/${userId}${suffix}`);
      return;
    }
    const tab = currentUserTabs.find((item) => item.value === value);
    if (tab) {
      navigate(tab.path);
    }
  }

  return (
    <>
      <RouteHeader
        icon={UserRound}
        eyebrow={publicSpace ? "用户空间" : "个人中心"}
        title={profileState.person?.name || (auth ? "我的社区主页" : "登录后查看个人中心")}
        description="集中展示用户资料、收藏、消息、积分、粉丝和关注。"
      />
      <PillTabs items={tabs} label="用户中心导航" value={activeValue} onChange={changeTab} />
      {profileState.loading && <EmptyState title="正在加载用户资料..." />}
      {profileState.error && <EmptyState title={profileState.error} />}
      {activeValue === "profile" && <UserProfilePanel auth={auth} person={profileState.person} publicSpace={publicSpace} />}
      {activeValue === "account" && <AccountSecurityPanel auth={auth} />}
      {activeValue === "favorites" && <UserInteractionPanel auth={auth} mode="favorites" />}
      {activeValue === "messages" && <UserMessagesPanel auth={auth} />}
      {activeValue === "scores" && <UserScoresPanel auth={auth} />}
      {activeValue === "articles" && <UserArticlesPanel auth={auth} userId={userId} />}
      {activeValue === "badges" && <UserBadgesPanel userId={userId} />}
      {activeValue === "fans" && <UserFollowPanel direction="followers" userId={userId} />}
      {activeValue === "followed" && <UserFollowPanel direction="following" userId={userId} />}
    </>
  );
}

function UserProfilePanel({ auth, person, publicSpace }) {
  const [following, setFollowing] = React.useState(false);
  const [followBusy, setFollowBusy] = React.useState(false);
  const [followError, setFollowError] = React.useState("");
  const [followerCount, setFollowerCount] = React.useState(toNumber(person?.followerCount));
  const profileUserId = toId(person?.id);
  const self = sameId(auth?.user?.id, profileUserId);

  React.useEffect(() => {
    setFollowerCount(toNumber(person?.followerCount));
  }, [person?.followerCount]);

  React.useEffect(() => {
    if (!publicSpace || !auth?.accessToken || !profileUserId || self) {
      setFollowing(false);
      setFollowError("");
      return;
    }
    let alive = true;
    setFollowError("");
    bbsApi
      .followingState(profileUserId, auth.accessToken)
      .then((data) => {
        if (!alive) return;
        setFollowing(Boolean(data?.following));
      })
      .catch((error) => {
        if (!alive) return;
        setFollowError(error.message || "关注状态加载失败");
      });
    return () => {
      alive = false;
    };
  }, [auth?.accessToken, profileUserId, publicSpace, self]);

  async function toggleFollow() {
    if (!profileUserId) {
      setFollowError("用户资料不完整，暂不能关注。");
      return;
    }
    if (!auth?.accessToken) {
      setFollowError("请先登录后再关注。");
      return;
    }
    if (self) {
      setFollowError("不能关注自己。");
      return;
    }
    setFollowBusy(true);
    setFollowError("");
    try {
      if (following) {
        await bbsApi.unfollowUser(profileUserId, auth.accessToken);
      } else {
        await bbsApi.followUser(profileUserId, auth.accessToken);
      }
      const nextFollowing = !following;
      setFollowing(nextFollowing);
      setFollowerCount((count) => Math.max(0, toNumber(count) + (nextFollowing ? 1 : -1)));
    } catch (error) {
      setFollowError(error.message || "关注操作失败");
    } finally {
      setFollowBusy(false);
    }
  }

  if (!person) {
    return <EmptyState title={auth ? "暂无用户资料" : "请先登录"} description="登录后可以查看并维护个人资料。" />;
  }

  return (
    <section className={`user-profile-card panel ${profileThemeClass(person.profileTheme)}`}>
      <div className="user-profile-cover" style={person.background ? { backgroundImage: `url(${JSON.stringify(person.background)})` } : undefined} />
      <div className="user-profile-main">
        <Avatar person={person} />
        <div>
          <h2>{person.name}</h2>
          <p>@{person.handle}</p>
          <span>{person.bio || "正在参与社区讨论"}</span>
        </div>
        {publicSpace && !self && (
          <button className={`follow-action user-profile-follow ${following ? "is-following" : ""}`} type="button" onClick={toggleFollow} disabled={followBusy}>
            {followBusy ? "处理中..." : following ? "取消关注" : auth ? "关注用户" : "登录后关注"}
          </button>
        )}
      </div>
      {followError && <p className="form-error user-profile-error">{followError}</p>}
      <div className="user-stats">
        <span>
          <strong>{followerCount}</strong>
          粉丝
        </span>
        <span>
          <strong>{toNumber(person.followingCount)}</strong>
          关注
        </span>
        <span>
          <strong>{publicSpace ? "公开" : "本人"}</strong>
          空间
        </span>
      </div>
    </section>
  );
}

function AccountSecurityPanel({ auth }) {
  const [form, setForm] = React.useState({
    old_password: "",
    new_password: "",
    confirm_password: ""
  });
  const [saving, setSaving] = React.useState(false);
  const [error, setError] = React.useState("");
  const [message, setMessage] = React.useState("");

  function updateField(field, value) {
    setForm((current) => ({ ...current, [field]: value }));
  }

  async function submit(event) {
    event.preventDefault();
    if (!auth?.accessToken) {
      setError("请先登录后再修改密码。");
      return;
    }
    if (!form.old_password || !form.new_password) {
      setError("请输入当前密码和新密码。");
      return;
    }
    if (form.new_password.length < 8) {
      setError("新密码至少需要 8 位。");
      return;
    }
    if (form.new_password !== form.confirm_password) {
      setError("两次输入的新密码不一致。");
      return;
    }
    setSaving(true);
    setError("");
    setMessage("");
    try {
      await bbsApi.changePassword(
        {
          old_password: form.old_password,
          new_password: form.new_password
        },
        auth.accessToken
      );
      setForm({ old_password: "", new_password: "", confirm_password: "" });
      setMessage("密码已更新，下次登录请使用新密码。");
    } catch (submitError) {
      setError(submitError.message || "密码修改失败");
    } finally {
      setSaving(false);
    }
  }

  if (!auth) {
    return <EmptyState title="请先登录" description="登录后可以管理账号安全设置。" />;
  }

  return (
    <section className="account-security panel">
      <header>
        <strong>账号安全</strong>
        <p>定期更新密码可以降低账号被盗风险。</p>
      </header>
      <form onSubmit={submit}>
        <label>
          当前密码
          <input
            autoComplete="current-password"
            type="password"
            value={form.old_password}
            onChange={(event) => updateField("old_password", event.target.value)}
          />
        </label>
        <label>
          新密码
          <input
            autoComplete="new-password"
            type="password"
            value={form.new_password}
            onChange={(event) => updateField("new_password", event.target.value)}
          />
        </label>
        <label>
          确认新密码
          <input
            autoComplete="new-password"
            type="password"
            value={form.confirm_password}
            onChange={(event) => updateField("confirm_password", event.target.value)}
          />
        </label>
        {error && <p className="form-error">{error}</p>}
        {message && <p className="form-success">{message}</p>}
        <button type="submit" disabled={saving}>
          {saving ? "保存中..." : "更新密码"}
        </button>
      </form>
    </section>
  );
}

function UserInteractionPanel({ auth, mode }) {
  const [state, setState] = React.useState({
    posts: [],
    loading: false,
    error: ""
  });

  React.useEffect(() => {
    if (!auth?.accessToken) {
      setState({ posts: [], loading: false, error: "" });
      return;
    }
    let alive = true;
    setState({ posts: [], loading: true, error: "" });
    const loader = mode === "favorites" ? bbsApi.favorites : bbsApi.likes;
    loader({ limit: 20, offset: 0 }, auth.accessToken)
      .then(async (data) => {
        const rawPosts = await Promise.all(listItems(data).map((item) => interactionToPost(item, auth, mode)));
        const posts = await hydratePostsMeta(rawPosts.filter(Boolean), auth);
        if (!alive) return;
        setState({ posts, loading: false, error: "" });
      })
      .catch((error) => {
        if (!alive) return;
        setState({ posts: [], loading: false, error: error.message || "互动记录加载失败" });
      });
    return () => {
      alive = false;
    };
  }, [auth, mode]);

  function handlePostArchived(postId, postKind) {
    setState((current) => ({
      ...current,
      posts: current.posts.filter((post) => String(post.id) !== String(postId) || (postKind && post.kind !== postKind))
    }));
  }

  if (!auth) {
    return <EmptyState title="请先登录" description="登录后可以查看收藏和点赞记录。" />;
  }
  if (state.loading) {
    return <EmptyState title="正在加载收藏..." />;
  }
  if (state.error) {
    return <EmptyState title={state.error} />;
  }
  if (state.posts.length === 0) {
    return <EmptyState title="暂无收藏内容" description="在帖子或文章里点击收藏后会出现在这里。" />;
  }

  return state.posts.map((post, index) => (
    <PostCard auth={auth} index={index} key={`${post.kind}-${post.id}`} post={post} onPostArchived={handlePostArchived} />
  ));
}

function UserMessagesPanel({ auth }) {
  const navigate = useNavigate();
  const [state, setState] = React.useState({
    items: [],
    total: 0,
    unread: 0,
    offset: 0,
    loading: false,
    loadingMore: false,
    error: "",
    action: ""
  });
  const [filter, setFilter] = React.useState("all");

  const loadMessages = React.useCallback((offset = 0, appending = false) => {
    if (!auth?.accessToken) {
      setState({ items: [], total: 0, unread: 0, offset: 0, loading: false, loadingMore: false, error: "", action: "" });
      return;
    }
    let alive = true;
    setState((current) => ({
      ...current,
      items: appending ? current.items : [],
      total: appending ? current.total : 0,
      unread: appending ? current.unread : 0,
      offset: appending ? current.offset : 0,
      loading: !appending,
      loadingMore: appending,
      error: "",
      action: appending ? current.action : ""
    }));
    bbsApi
      .notifications({ limit: MESSAGE_PAGE_SIZE, offset }, auth.accessToken)
      .then((data) => {
        if (!alive) return;
        const pageItems = listItems(data);
        if (appending) {
          setState((current) => {
            const items = appendUniqueMessageItems(current.items, pageItems);
            const nextOffset = current.offset + pageItems.length;
            return {
              ...current,
              items,
              total: pageItems.length > 0 ? Math.max(nextOffset, listTotal(data, pageItems)) : current.offset,
              unread: unreadCount(data),
              offset: nextOffset,
              loadingMore: false,
              error: ""
            };
          });
          return;
        }
        setState({
          items: pageItems,
          total: Math.max(listTotal(data, pageItems), pageItems.length),
          unread: unreadCount(data),
          offset: pageItems.length,
          loading: false,
          loadingMore: false,
          error: "",
          action: ""
        });
      })
      .catch((error) => {
        if (!alive) return;
        if (appending) {
          setState((current) => ({ ...current, loadingMore: false, error: error.message || "更多消息加载失败" }));
          return;
        }
        setState({ items: [], total: 0, unread: 0, offset: 0, loading: false, loadingMore: false, error: error.message || "消息加载失败", action: "" });
      });
    return () => {
      alive = false;
    };
  }, [auth?.accessToken]);

  React.useEffect(loadMessages, [loadMessages]);

  function loadMoreMessages() {
    if (state.loading || state.loadingMore || state.offset >= state.total) return;
    loadMessages(state.offset, true);
  }

  async function markRead(id) {
    if (!id) return;
    setState((current) => ({ ...current, action: `read-${id}`, error: "" }));
    try {
      await bbsApi.markNotificationRead(id, auth.accessToken);
      emitNotificationsChanged();
      loadMessages();
    } catch (error) {
      setState((current) => ({ ...current, action: "", error: error.message || "消息操作失败" }));
    }
  }

  async function markAllRead() {
    setState((current) => ({ ...current, action: "read-all", error: "" }));
    try {
      await bbsApi.markAllNotificationsRead(auth.accessToken);
      emitNotificationsChanged();
      loadMessages();
    } catch (error) {
      setState((current) => ({ ...current, action: "", error: error.message || "消息操作失败" }));
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
      setState((current) => ({ ...current, action: "", error: error.message || "消息操作失败" }));
    }
  }

  const summary = React.useMemo(() => summarizeNotifications(state.items), [state.items]);
  const visibleItems = React.useMemo(() => filterNotifications(state.items, filter), [state.items, filter]);

  if (!auth) return <EmptyState title="请先登录" description="登录后可以查看站内消息。" />;
  if (state.loading) return <EmptyState title="正在加载消息..." />;
  if (state.error && state.items.length === 0) return <EmptyState title={state.error} />;
  if (state.items.length === 0) return <EmptyState title="暂无消息" description="评论、点赞、收藏、关注和商城通知会出现在这里。" />;
  return (
    <section className="messages-panel">
      <div className="message-toolbar panel">
        <div>
          <strong>站内消息</strong>
          <span>
            {state.total} 条消息 · {state.unread} 条未读
            {summary.mall.total > 0 ? ` · 商城 ${summary.mall.total} 条` : ""}
          </span>
        </div>
        <button type="button" disabled={state.unread === 0 || state.action === "read-all"} onClick={markAllRead}>
          {state.action === "read-all" ? "处理中..." : "全部已读"}
        </button>
      </div>
      <MessageFilterPanel filter={filter} summary={summary} onFilterChange={setFilter} />
      {visibleItems.length === 0 && <EmptyState title="暂无商城消息" description="订单、售后和商品评价通知会归到这里。" />}
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
                    {item.title || "站内消息"}
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
      {state.offset < state.total && (
        <div className="dashboard-history-more">
          <span>{state.loadingMore ? "正在加载更多消息..." : state.error || "继续查看更多消息。"}</span>
          <button aria-label="加载更多站内消息" type="button" disabled={state.loadingMore} onClick={loadMoreMessages}>
            {state.loadingMore ? "加载中" : "加载更多"}
          </button>
        </div>
      )}
    </section>
  );
}

function appendUniqueMessageItems(currentItems, pageItems) {
  const ids = new Set(currentItems.map((item) => String(item.id)));
  return [
    ...currentItems,
    ...pageItems.filter((item) => {
      const id = String(item.id);
      if (ids.has(id)) return false;
      ids.add(id);
      return true;
    })
  ];
}

function UserScoresPanel({ auth }) {
  const [state, setState] = React.useState({
    balance: null,
    rows: [],
    loading: false,
    error: ""
  });

  React.useEffect(() => {
    if (!auth?.accessToken) {
      setState({ balance: null, rows: [], loading: false, error: "" });
      return;
    }
    let alive = true;
    setState({ balance: null, rows: [], loading: true, error: "" });
    Promise.all([bbsApi.creditBalance(auth.accessToken), bbsApi.creditLedger({ limit: 30, offset: 0 }, auth.accessToken)])
      .then(([balanceData, ledgerData]) => {
        if (!alive) return;
        const balance = creditBalance(balanceData) || creditBalance(ledgerData);
        setState({
          balance,
          rows: listItems(ledgerData).map((entry) => ({
            key: entry.id || entry.source_event_id,
            title: creditReasonLabel(entry.reason),
            description: creditEntryMeta(entry),
            meta: `${toNumber(entry.delta)}`
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
  }, [auth?.accessToken]);

  if (!auth) return <EmptyState title="请先登录" description="登录后可以查看积分和成长记录。" />;
  if (state.loading) return <EmptyState title="正在加载积分..." />;
  if (state.error) return <EmptyState title={state.error} />;

  return (
    <>
      <section className="score-summary panel">
        <span>当前积分</span>
        <strong>{toNumber(state.balance?.total)}</strong>
        <p>积分由发帖、评论、点赞、收藏和任务事件驱动，后端由积分服务统一结算。</p>
      </section>
      {state.rows.length > 0 ? <DataRows rows={state.rows} /> : <EmptyState title="暂无积分明细" />}
    </>
  );
}

function UserArticlesPanel({ auth, userId }) {
  const [state, setState] = React.useState({
    posts: [],
    loading: false,
    error: ""
  });

  React.useEffect(() => {
    if (!userId) {
      setState({ posts: [], loading: false, error: "" });
      return;
    }
    let alive = true;
    setState({ posts: [], loading: true, error: "" });
    bbsApi
      .listArticles({ author_id: userId, limit: 20, offset: 0 })
      .then(async (data) => {
        const posts = await hydratePostsMeta(listItems(data).map((item) => articleToPost(item, auth)), auth);
        if (!alive) return;
        setState({ posts, loading: false, error: "" });
      })
      .catch((error) => {
        if (!alive) return;
        setState({ posts: [], loading: false, error: error.message || "文章加载失败" });
      });
    return () => {
      alive = false;
    };
  }, [auth, userId]);

  function handlePostArchived(postId, postKind) {
    setState((current) => ({
      ...current,
      posts: current.posts.filter((post) => String(post.id) !== String(postId) || (postKind && post.kind !== postKind))
    }));
  }

  if (state.loading) return <EmptyState title="正在加载文章..." />;
  if (state.error) return <EmptyState title={state.error} />;
  if (state.posts.length === 0) return <EmptyState title="暂无公开文章" />;
  return state.posts.map((post, index) => (
    <PostCard auth={auth} index={index} key={`${post.kind}-${post.id}`} post={post} onPostArchived={handlePostArchived} />
  ));
}

function UserBadgesPanel({ userId }) {
  const [state, setState] = React.useState({
    rows: [],
    total: 0,
    offset: 0,
    loading: false,
    loadingMore: false,
    error: ""
  });

  React.useEffect(() => {
    if (!userId) {
      setState({ rows: [], total: 0, offset: 0, loading: false, loadingMore: false, error: "" });
      return;
    }
    let alive = true;
    setState({ rows: [], total: 0, offset: 0, loading: true, loadingMore: false, error: "" });
    bbsApi
      .userBadges(userId, { limit: BADGE_PAGE_SIZE, offset: 0 })
      .then((data) => {
        if (!alive) return;
        const items = listItems(data);
        const rows = userBadgeRows(items);
        setState({
          rows,
          total: Math.max(listTotal(data, items), items.length),
          offset: items.length,
          loading: false,
          loadingMore: false,
          error: ""
        });
      })
      .catch((error) => {
        if (!alive) return;
        setState({
          rows: [],
          total: 0,
          offset: 0,
          loading: false,
          loadingMore: false,
          error: error.message || "徽章加载失败"
        });
      });
    return () => {
      alive = false;
    };
  }, [userId]);

  async function loadMoreBadges() {
    if (state.loading || state.loadingMore || state.offset >= state.total) return;
    const offset = state.offset;
    setState((current) => ({ ...current, loadingMore: true, error: "" }));
    try {
      const data = await bbsApi.userBadges(userId, { limit: BADGE_PAGE_SIZE, offset });
      const items = listItems(data);
      const pageRows = userBadgeRows(items);
      setState((current) => {
        const rows = appendUniqueBadgeRows(current.rows, pageRows);
        const nextOffset = current.offset + items.length;
        return {
          ...current,
          rows,
          total: items.length > 0 ? Math.max(nextOffset, listTotal(data, items)) : current.offset,
          offset: nextOffset,
          loadingMore: false,
          error: ""
        };
      });
    } catch (error) {
      setState((current) => ({ ...current, loadingMore: false, error: error.message || "更多徽章加载失败" }));
    }
  }

  if (state.loading) return <EmptyState title="正在加载用户徽章..." />;
  if (state.error && state.rows.length === 0) {
    return <EmptyState title="徽章加载失败" description={state.error} />;
  }
  if (state.rows.length === 0) return <EmptyState title="暂无公开徽章" />;
  return (
    <>
      <DataRows rows={state.rows} />
      {state.offset < state.total && (
        <div className="dashboard-history-more">
          <span>{state.loadingMore ? "正在加载更多徽章..." : state.error || "继续查看更多徽章。"}</span>
          <button aria-label="加载更多用户徽章" type="button" disabled={state.loadingMore} onClick={loadMoreBadges}>
            {state.loadingMore ? "加载中" : "加载更多"}
          </button>
        </div>
      )}
    </>
  );
}

function appendUniqueBadgeRows(currentRows, pageRows) {
  const keys = new Set(currentRows.map((row) => String(row.key)));
  return [
    ...currentRows,
    ...pageRows.filter((row) => {
      const key = String(row.key);
      if (keys.has(key)) return false;
      keys.add(key);
      return true;
    })
  ];
}

function UserFollowPanel({ direction, userId }) {
  const [state, setState] = React.useState({
    rows: [],
    total: 0,
    page: 0,
    loading: false,
    loadingMore: false,
    error: ""
  });

  const loadFollows = React.useCallback((page = 1, appending = false) => {
    if (!userId) {
      setState({ rows: [], total: 0, page: 0, loading: false, loadingMore: false, error: "" });
      return undefined;
    }
    let alive = true;
    setState((current) => ({
      ...current,
      rows: appending ? current.rows : [],
      total: appending ? current.total : 0,
      page: appending ? current.page : 0,
      loading: !appending,
      loadingMore: appending,
      error: ""
    }));
    const loader = direction === "followers" ? bbsApi.followers : bbsApi.following;
    loader(userId, { page, page_size: FOLLOW_LIST_PAGE_SIZE })
      .then((data) => {
        if (!alive) return;
        const pageRows = followRows(listItems(data));
        if (appending) {
          setState((current) => {
            const rows = appendUniqueFollowRows(current.rows, pageRows);
            return {
              ...current,
              rows,
              total: pageRows.length > 0 ? Math.max(listTotal(data, pageRows), rows.length) : rows.length,
              page: pageRows.length > 0 ? page : current.page,
              loadingMore: false,
              error: ""
            };
          });
          return;
        }
        setState({
          rows: pageRows,
          total: Math.max(listTotal(data, pageRows), pageRows.length),
          page: pageRows.length > 0 ? page : 0,
          loading: false,
          loadingMore: false,
          error: ""
        });
      })
      .catch((error) => {
        if (!alive) return;
        if (appending) {
          setState((current) => ({ ...current, loadingMore: false, error: error.message || "更多用户加载失败" }));
          return;
        }
        setState({ rows: [], total: 0, page: 0, loading: false, loadingMore: false, error: error.message || "关系链加载失败" });
      });
    return () => {
      alive = false;
    };
  }, [direction, userId]);

  React.useEffect(loadFollows, [loadFollows]);

  function loadMoreFollows() {
    if (state.loading || state.loadingMore || state.rows.length >= state.total) return;
    loadFollows(state.page + 1, true);
  }

  if (state.loading) return <EmptyState title="正在加载用户列表..." />;
  if (state.error && state.rows.length === 0) return <EmptyState title={state.error} />;
  if (state.rows.length === 0) return <EmptyState title={direction === "followers" ? "暂无粉丝" : "暂无关注"} />;
  const label = direction === "followers" ? "粉丝" : "关注";
  return (
    <>
      <DataRows rows={state.rows} />
      {state.rows.length < state.total && (
        <div className="dashboard-history-more">
          <span>{state.loadingMore ? `正在加载更多${label}...` : state.error || `继续查看更多${label}。`}</span>
          <button aria-label={`加载更多${label}`} type="button" disabled={state.loadingMore} onClick={loadMoreFollows}>
            {state.loadingMore ? "加载中" : "加载更多"}
          </button>
        </div>
      )}
    </>
  );
}

function followRows(items) {
  return items.map((item) => {
    const user = item.user || item;
    return {
      key: user.id,
      title: user.nickname || user.username || `用户 #${user.id}`,
      description: user.bio || "社区成员",
      meta: `@${user.username || user.id}`
    };
  });
}

function appendUniqueFollowRows(currentRows, pageRows) {
  const keys = new Set(currentRows.map((row) => String(row.key)));
  return [...currentRows, ...pageRows.filter((row) => !keys.has(String(row.key)))];
}
