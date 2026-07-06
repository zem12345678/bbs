import React from "react";
import { Bell, ChevronDown, Pencil, Search } from "lucide-react";
import { bbsApi } from "../../api";
import { creditBalance, listItems, unreadCount } from "../../lib/apiShapes";
import { timeAgoMillis, toNumber } from "../../lib/formatters";
import { NOTIFICATIONS_CHANGED_EVENT } from "../../lib/notificationEvents";
import { userAvatar, userDisplayName } from "../../lib/postMappers";
import { navItems } from "../../routes";

export default function Header({ activePage, auth, onAuthSuccess, onCreate, onDashboard, onLogout, onNavigate, onSearch }) {
  const [query, setQuery] = React.useState("");
  const [authOpen, setAuthOpen] = React.useState(false);
  const [notificationOpen, setNotificationOpen] = React.useState(false);
  const [notificationState, setNotificationState] = React.useState({
    items: [],
    unreadCount: 0,
    loading: false,
    error: ""
  });

  const refreshNotifications = React.useCallback(
    async (loadItems = false) => {
      if (!auth?.accessToken) {
        setNotificationState({ items: [], unreadCount: 0, loading: false, error: "" });
        return;
      }
      setNotificationState((current) => ({ ...current, loading: loadItems, error: "" }));
      try {
        const [countData, listData] = await Promise.all([
          bbsApi.notificationUnreadCount(auth.accessToken),
          loadItems ? bbsApi.notifications({ limit: 20, offset: 0 }, auth.accessToken) : Promise.resolve(null)
        ]);
        setNotificationState((current) => ({
          items: listData ? listItems(listData) : current.items,
          unreadCount: unreadCount(listData) || unreadCount(countData),
          loading: false,
          error: ""
        }));
      } catch (error) {
        setNotificationState((current) => ({
          ...current,
          loading: false,
          error: error.message || "通知服务暂不可用"
        }));
      }
    },
    [auth?.accessToken]
  );

  function submitSearch(event) {
    event.preventDefault();
    onSearch(query);
  }

  React.useEffect(() => {
    setNotificationOpen(false);
    refreshNotifications(false);
  }, [refreshNotifications]);

  React.useEffect(() => {
    function handleNotificationsChanged() {
      refreshNotifications(notificationOpen);
    }

    window.addEventListener(NOTIFICATIONS_CHANGED_EVENT, handleNotificationsChanged);
    return () => window.removeEventListener(NOTIFICATIONS_CHANGED_EVENT, handleNotificationsChanged);
  }, [notificationOpen, refreshNotifications]);

  async function toggleNotifications() {
    if (!auth?.accessToken) {
      setAuthOpen(true);
      return;
    }
    const nextOpen = !notificationOpen;
    setNotificationOpen(nextOpen);
    if (nextOpen) {
      await refreshNotifications(true);
    }
  }

  async function markNotificationRead(id) {
    if (!auth?.accessToken || !id) {
      return;
    }
    await bbsApi.markNotificationRead(id, auth.accessToken);
    await refreshNotifications(true);
  }

  async function markAllNotificationsRead() {
    if (!auth?.accessToken) {
      return;
    }
    await bbsApi.markAllNotificationsRead(auth.accessToken);
    await refreshNotifications(true);
  }

  return (
    <header className="topbar">
      <nav className="nav-left" aria-label="主导航">
        {navItems.map((item) => (
          <button
            className={`nav-item ${item === activePage ? "is-active" : ""}`}
            key={item}
            type="button"
            onClick={() => onNavigate(item)}
          >
            {item}
            {item === "更多" && <ChevronDown size={15} aria-hidden="true" />}
          </button>
        ))}
      </nav>
      <div className="nav-right">
        <form className="search-box" role="search" onSubmit={submitSearch}>
          <Search size={22} aria-hidden="true" />
          <span className="sr-only">搜索内容</span>
          <input placeholder="请输入搜索内容" value={query} onChange={(event) => setQuery(event.target.value)} />
          <kbd>⌘ K</kbd>
        </form>
        <button className="create-btn" type="button" onClick={onCreate}>
          <Pencil size={22} aria-hidden="true" />
          创作中心
          <span className="divider" />
          <ChevronDown size={16} aria-hidden="true" />
        </button>
        <div className="notification-menu">
          <button className="icon-btn notification" type="button" aria-label="通知" onClick={toggleNotifications}>
            <Bell size={26} />
            {auth && notificationState.unreadCount > 0 && <span>{notificationState.unreadCount > 99 ? "99+" : notificationState.unreadCount}</span>}
          </button>
          {notificationOpen && auth && (
            <NotificationPopover
              error={notificationState.error}
              items={notificationState.items}
              loading={notificationState.loading}
              unreadCount={notificationState.unreadCount}
              onMarkAllRead={markAllNotificationsRead}
              onMarkRead={markNotificationRead}
              onRefresh={() => refreshNotifications(true)}
            />
          )}
        </div>
        <div className="auth-menu">
          <button
            className={auth ? "avatar-btn" : "login-entry"}
            type="button"
            aria-label={auth ? "个人中心" : "登录"}
            onClick={() => setAuthOpen((open) => !open)}
          >
            {auth ? (
              <>
                <img src={userAvatar(auth.user)} alt="" />
                <strong>V</strong>
              </>
            ) : (
              "登录"
            )}
          </button>
          {authOpen && (
            <AuthPopover
              auth={auth}
              onNavigate={() => {
                onDashboard();
                setAuthOpen(false);
              }}
              onAuthSuccess={(data) => {
                onAuthSuccess(data);
                setAuthOpen(false);
              }}
              onLogout={() => {
                onLogout();
                setAuthOpen(false);
                setNotificationOpen(false);
              }}
            />
          )}
        </div>
      </div>
    </header>
  );
}

function NotificationPopover({ error, items, loading, onMarkAllRead, onMarkRead, onRefresh, unreadCount }) {
  return (
    <section className="notification-popover panel" aria-label="通知列表">
      <header className="notification-head">
        <div>
          <strong>通知中心</strong>
          <span>{unreadCount > 0 ? `${unreadCount} 条未读` : "暂无未读"}</span>
        </div>
        <button type="button" onClick={onMarkAllRead} disabled={unreadCount === 0}>
          全部已读
        </button>
      </header>
      {error && (
        <div className="notification-empty">
          <p>{error}</p>
          <button type="button" onClick={onRefresh}>
            重试
          </button>
        </div>
      )}
      {!error && loading && <p className="notification-empty">正在加载通知...</p>}
      {!error && !loading && items.length === 0 && <p className="notification-empty">暂无通知</p>}
      {!error && !loading && items.length > 0 && (
        <div className="notification-list">
          {items.map((item) => (
            <button
              className={`notification-item ${item.read ? "" : "is-unread"}`}
              key={item.id}
              type="button"
              onClick={() => !item.read && onMarkRead(item.id)}
            >
              <span className={`notification-dot type-${item.type}`} />
              <div>
                <strong>{item.title}</strong>
                <p>{item.content}</p>
                <time>{timeAgoMillis(item.created_at || item.createdAt)}</time>
              </div>
            </button>
          ))}
        </div>
      )}
    </section>
  );
}

function AuthPopover({ auth, onAuthSuccess, onLogout, onNavigate }) {
  const [mode, setMode] = React.useState("login");
  const [form, setForm] = React.useState({
    account: "",
    username: "",
    email: "",
    nickname: "",
    password: ""
  });
  const [profileForm, setProfileForm] = React.useState({
    nickname: "",
    avatar_url: "",
    bio: ""
  });
  const [creditSummary, setCreditSummary] = React.useState(null);
  const [creditLoading, setCreditLoading] = React.useState(false);
  const [loading, setLoading] = React.useState(false);
  const [error, setError] = React.useState("");

  React.useEffect(() => {
    setProfileForm({
      nickname: auth?.user?.nickname || "",
      avatar_url: auth?.user?.avatar_url || auth?.user?.avatarUrl || "",
      bio: auth?.user?.bio || ""
    });
  }, [auth]);

  React.useEffect(() => {
    if (!auth?.accessToken) {
      setCreditSummary(null);
      return;
    }
    let alive = true;
    setCreditLoading(true);
    bbsApi
      .creditBalance(auth.accessToken)
      .then((data) => {
        if (!alive) return;
        setCreditSummary(creditBalance(data));
      })
      .catch(() => {
        if (!alive) return;
        setCreditSummary(null);
      })
      .finally(() => {
        if (alive) setCreditLoading(false);
      });
    return () => {
      alive = false;
    };
  }, [auth?.accessToken]);

  function updateField(field, value) {
    setForm((current) => ({ ...current, [field]: value }));
  }

  function updateProfileField(field, value) {
    setProfileForm((current) => ({ ...current, [field]: value }));
  }

  async function submit(event) {
    event.preventDefault();
    setLoading(true);
    setError("");
    try {
      const data =
        mode === "login"
          ? await bbsApi.login({ account: form.account, password: form.password })
          : await bbsApi.register({
              username: form.username,
              email: form.email,
              nickname: form.nickname || form.username,
              password: form.password
            });
      onAuthSuccess(data);
    } catch (submitError) {
      setError(submitError.message || "认证失败");
    } finally {
      setLoading(false);
    }
  }

  async function submitProfile(event) {
    event.preventDefault();
    setLoading(true);
    setError("");
    try {
      const data = await bbsApi.updateMe(profileForm, auth.accessToken);
      onAuthSuccess({
        access_token: auth.accessToken,
        expires_at: auth.expiresAt,
        user: data?.user || auth.user
      });
    } catch (submitError) {
      setError(submitError.message || "资料保存失败");
    } finally {
      setLoading(false);
    }
  }

  if (auth) {
    return (
      <section className="auth-popover panel">
        <div className="auth-profile">
          <img src={userAvatar(auth.user)} alt="" />
          <div>
            <strong>{userDisplayName(auth.user)}</strong>
            <span>@{auth.user?.username || auth.user?.id}</span>
          </div>
        </div>
        <div className="auth-credit-strip">
          <span>成长值</span>
          <strong>{creditLoading ? "同步中" : toNumber(creditSummary?.total)}</strong>
        </div>
        <button className="auth-center-btn" type="button" onClick={onNavigate}>
          个人工作台
        </button>
        <form className="auth-form profile-form" onSubmit={submitProfile}>
          <input
            placeholder="昵称"
            value={profileForm.nickname}
            onChange={(event) => updateProfileField("nickname", event.target.value)}
          />
          <input
            placeholder="头像 URL"
            value={profileForm.avatar_url}
            onChange={(event) => updateProfileField("avatar_url", event.target.value)}
          />
          <textarea
            placeholder="个人简介"
            value={profileForm.bio}
            onChange={(event) => updateProfileField("bio", event.target.value)}
          />
          {error && <p className="form-error">{error}</p>}
          <button type="submit" disabled={loading}>
            {loading ? "保存中..." : "保存资料"}
          </button>
        </form>
        <button className="logout-btn" type="button" onClick={onLogout}>
          退出登录
        </button>
      </section>
    );
  }

  return (
    <section className="auth-popover panel">
      <div className="auth-tabs" role="tablist" aria-label="认证方式">
        <button className={mode === "login" ? "is-active" : ""} type="button" onClick={() => setMode("login")}>
          登录
        </button>
        <button className={mode === "register" ? "is-active" : ""} type="button" onClick={() => setMode("register")}>
          注册
        </button>
      </div>
      <form className="auth-form" onSubmit={submit}>
        {mode === "login" ? (
          <input
            autoComplete="username"
            placeholder="用户名或邮箱"
            value={form.account}
            onChange={(event) => updateField("account", event.target.value)}
          />
        ) : (
          <>
            <input
              autoComplete="username"
              placeholder="用户名"
              value={form.username}
              onChange={(event) => updateField("username", event.target.value)}
            />
            <input
              autoComplete="email"
              placeholder="邮箱"
              type="email"
              value={form.email}
              onChange={(event) => updateField("email", event.target.value)}
            />
            <input
              placeholder="昵称"
              value={form.nickname}
              onChange={(event) => updateField("nickname", event.target.value)}
            />
          </>
        )}
        <input
          autoComplete={mode === "login" ? "current-password" : "new-password"}
          placeholder="密码"
          type="password"
          value={form.password}
          onChange={(event) => updateField("password", event.target.value)}
        />
        {error && <p className="form-error">{error}</p>}
        <button type="submit" disabled={loading}>
          {loading ? "处理中..." : mode === "login" ? "登录" : "创建账号"}
        </button>
      </form>
    </section>
  );
}
