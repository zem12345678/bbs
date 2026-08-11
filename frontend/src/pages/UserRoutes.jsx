import React from "react";
import { Link, useNavigate, useParams } from "react-router-dom";
import { BadgeCheck, Bell, Check, Copy, Download, FileText, Fingerprint, Folder, FolderPlus, Globe2, Heart, History, KeyRound, ListFilter, LockKeyhole, LogOut, MessageCircle, MonitorSmartphone, Pencil, Power, PowerOff, RefreshCw, Send, ShieldCheck, ShieldOff, Share2, Star, Trash2, Trophy, UserPlus, UserRound, Users, VolumeX, Webhook, X } from "lucide-react";
import { bbsApi } from "../api";
import Avatar from "../components/Avatar.jsx";
import MessageFilterPanel from "../components/notifications/MessageFilterPanel.jsx";
import PostCard from "../components/post/PostCard.jsx";
import { creditBalance, listItems, listTotal, notificationRead, unreadCount } from "../lib/apiShapes";
import { userBadgeRows } from "../lib/badges";
import { collectionPostKey } from "../lib/collections";
import { creditEntryMeta, creditReasonLabel, sameId, timeAgo, timeAgoMillis, toId, toNumber } from "../lib/formatters";
import { describeUserAgent, ipAddressLabel, loginFailureLabel, loginMethodLabel, normalizeLoginEventList, normalizeSessionList, sessionStatus, sessionStatusLabel } from "../lib/sessions";
import { loadAllListPages } from "../lib/focusedLists";
import { normalizeMFAStatus, recoveryCodesFromResponse, recoveryCodesText } from "../lib/mfa";
import { apiTokenScopeLabel, apiTokenStatus, apiTokenStatusLabel, apiTokenTime, normalizeAPITokenCreation, normalizeAPITokenList } from "../lib/apiTokens";
import { normalizeWebhook, normalizeWebhookList, validWebhookURL, WEBHOOK_EVENT_OPTIONS, webhookEventLabel, webhookTime } from "../lib/webhooks";
import { createPasskey, friendlyPasskeyError, normalizePasskeyList, passkeysSupported } from "../lib/passkeys";
import { emitNotificationsChanged } from "../lib/notificationEvents";
import {
  filterNotifications,
  isMallNotification,
  notificationGroupLabel,
  notificationTarget,
  notificationTargetLabel,
  summarizeNotifications
} from "../lib/notificationTargets";
import { articleToPost, authProfileAppearanceNeedsVerification, authToPerson, feedItemToPost, hydratePostsMeta, interactionToPost, profileThemeClass, uniquePosts, userToPerson } from "../lib/postMappers";
import { shareLink } from "../lib/share";
import { normalizeUserList, normalizeUserLists, userListOwnedBy, validateUserListName } from "../lib/userLists";
import { DataRows, EmptyState, PillTabs, RouteHeader } from "./RouteBlocks.jsx";

const currentUserTabs = [
  { value: "profile", label: "资料", icon: UserRound, path: "/user/profile" },
  { value: "account", label: "账号", icon: LockKeyhole, path: "/user/profile/account" },
  { value: "favorites", label: "收藏", icon: Star, path: "/user/favorites" },
  { value: "likes", label: "点赞", icon: Heart, path: "/user/likes" },
  { value: "lists", label: "用户列表", icon: ListFilter, path: "/user/lists" },
  { value: "messages", label: "消息", icon: Bell, path: "/user/messages" },
  { value: "safety", label: "屏蔽与静音", icon: ShieldOff, path: "/user/safety" },
  { value: "scores", label: "积分", icon: Trophy, path: "/user/scores" }
];

const publicUserTabs = [
  { value: "profile", label: "主页", icon: UserRound },
  { value: "articles", label: "文章", icon: FileText },
  { value: "badges", label: "徽章", icon: BadgeCheck },
  { value: "fans", label: "粉丝", icon: Users },
  { value: "followed", label: "关注", icon: Heart },
  { value: "lists", label: "列表", icon: ListFilter }
];

const FOLLOW_LIST_PAGE_SIZE = 30;
const BADGE_PAGE_SIZE = 30;
const MESSAGE_PAGE_SIZE = 30;
const USER_INTERACTION_PAGE_SIZE = 20;
const USER_SCORE_PAGE_SIZE = 30;
const USER_ARTICLE_PAGE_SIZE = 20;
const USER_LIST_FEED_PAGE_SIZE = 20;

export function UserRoutePage({ auth, view = "profile", onAuthInvalidated }) {
  const params = useParams();
  const navigate = useNavigate();
  const routeUserId = toId(params.userId);
  const routeUsername = String(params.username || "").trim();
  const publicSpace = Boolean(routeUserId || routeUsername);
  const publicProfileKey = routeUsername ? "username:" + routeUsername : "id:" + routeUserId;
  const ownUserId = toId(auth?.user?.id);
  const [profileState, setProfileState] = React.useState({
    person: auth?.user ? authToPerson(auth) : null,
    loading: false,
    error: "",
    source: auth?.user ? "self" : ""
  });
  const profileScopeKey = publicSpace ? publicProfileKey : "self";
  const resolvedPublicUserId = publicSpace && profileState.source === profileScopeKey ? toId(profileState.person?.id) : "";
  const userId = publicSpace ? resolvedPublicUserId : ownUserId;
  const person = profileState.source === profileScopeKey ? profileState.person : null;
  const shareUsername = routeUsername || String(person?.username || (!publicSpace ? auth?.user?.username : "") || "").trim();

  React.useEffect(() => {
    if (!publicSpace) {
      if (!auth?.user) {
        setProfileState({ person: null, loading: false, error: "", source: "" });
        return undefined;
      }
      setProfileState({
        person: authToPerson(auth),
        loading: false,
        error: "",
        source: "self"
      });
      if (!authProfileAppearanceNeedsVerification(auth) || !ownUserId) {
        return undefined;
      }
      let alive = true;
      bbsApi
        .getUser(ownUserId)
        .then((data) => {
          if (!alive || !data?.user) return;
          setProfileState({ person: userToPerson(data.user), loading: false, error: "", source: "self" });
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
    setProfileState({ person: null, loading: true, error: "", source: "" });
    const loadProfile = routeUsername ? bbsApi.getUserByUsername(routeUsername) : bbsApi.getUser(routeUserId);
    loadProfile
      .then((data) => {
        if (!alive) return;
        setProfileState({
          person: data?.user ? userToPerson(data.user) : null,
          loading: false,
          error: "",
          source: publicProfileKey
        });
      })
      .catch((error) => {
        if (!alive) return;
        setProfileState({ person: null, loading: false, error: error.message || "用户资料加载失败", source: publicProfileKey });
      });
    return () => {
      alive = false;
    };
  }, [auth, ownUserId, publicProfileKey, publicSpace, routeUserId, routeUsername]);

  const tabs = publicSpace ? publicUserTabs : currentUserTabs;
  const activeValue = tabs.some((item) => item.value === view) ? view : "profile";

  function changeTab(value) {
    if (publicSpace) {
      const suffix = value === "profile" ? "" : "/" + value;
      const publicProfileBase = routeUsername ? "/u/" + encodeURIComponent(routeUsername) : "/user/" + routeUserId;
      navigate(publicProfileBase + suffix);
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
        title={person?.name || (auth ? "我的社区主页" : "登录后查看个人中心")}
        description="集中展示用户资料、收藏、消息、积分、粉丝和关注。"
      />
      <PillTabs items={tabs} label="用户中心导航" value={activeValue} onChange={changeTab} />
      {profileState.loading && <EmptyState title="正在加载用户资料..." />}
      {profileState.error && <EmptyState title={profileState.error} />}
      {activeValue === "profile" && <UserProfilePanel auth={auth} person={person} publicSpace={publicSpace} publicUsername={shareUsername} />}
      {activeValue === "account" && <AccountSecurityPanel auth={auth} onAuthInvalidated={onAuthInvalidated} />}
      {activeValue === "favorites" && <UserFavoritesPanel auth={auth} />}
      {activeValue === "likes" && <UserInteractionPanel auth={auth} mode="likes" />}
      {activeValue === "lists" && <UserListsPanel auth={auth} editable={!publicSpace} ownerId={userId} />}
      {activeValue === "messages" && <UserMessagesPanel auth={auth} />}
      {activeValue === "safety" && <UserSafetyPanel auth={auth} />}
      {activeValue === "scores" && <UserScoresPanel auth={auth} />}
      {activeValue === "articles" && <UserArticlesPanel auth={auth} userId={userId} />}
      {activeValue === "badges" && <UserBadgesPanel userId={userId} />}
      {activeValue === "fans" && <UserFollowPanel direction="followers" userId={userId} />}
      {activeValue === "followed" && <UserFollowPanel direction="following" userId={userId} />}
    </>
  );
}

function UserProfilePanel({ auth, person, publicSpace, publicUsername }) {
  const [following, setFollowing] = React.useState(false);
  const [followPending, setFollowPending] = React.useState(false);
  const [safety, setSafety] = React.useState({ blocked: false, blockedBy: false, muted: false });
  const [relationLoading, setRelationLoading] = React.useState(false);
  const [followReady, setFollowReady] = React.useState(false);
  const [safetyReady, setSafetyReady] = React.useState(false);
  const [relationAction, setRelationAction] = React.useState("");
  const [followError, setFollowError] = React.useState("");
  const [shareNotice, setShareNotice] = React.useState("");
  const [followerCount, setFollowerCount] = React.useState(toNumber(person?.followerCount));
  const relationSessionRef = React.useRef(0);
  const relationActionRef = React.useRef(0);
  const nextRelationActionRef = React.useRef(0);
  const relationScopeRef = React.useRef({ targetUserId: "", accessToken: "" });
  const profileUserId = toId(person?.id);
  const self = sameId(auth?.user?.id, profileUserId);
  relationScopeRef.current = { targetUserId: profileUserId, accessToken: auth?.accessToken || "" };

  React.useEffect(() => {
    setFollowerCount(toNumber(person?.followerCount));
  }, [person?.followerCount]);

  React.useEffect(() => {
    setShareNotice("");
  }, [profileUserId, publicUsername]);

  React.useEffect(() => {
    const session = relationSessionRef.current + 1;
    relationSessionRef.current = session;
    relationActionRef.current = 0;
    setRelationAction("");
    setFollowing(false);
    setFollowPending(false);
    setSafety({ blocked: false, blockedBy: false, muted: false });
    setFollowReady(false);
    setSafetyReady(false);
    setRelationLoading(false);
    setFollowError("");
    if (!publicSpace || !auth?.accessToken || !profileUserId || self) {
      return;
    }
    let alive = true;
    const requestTargetUserId = profileUserId;
    const requestAccessToken = auth.accessToken;
    setRelationLoading(true);
    Promise.allSettled([bbsApi.followingState(profileUserId, auth.accessToken), bbsApi.userSafetyState(profileUserId, auth.accessToken)])
      .then(([followResult, safetyResult]) => {
        if (!alive || relationSessionRef.current !== session || !matchesRelationScope(relationScopeRef.current, requestTargetUserId, requestAccessToken)) return;
        const errors = [];
        if (followResult.status === "fulfilled") {
          setFollowing(Boolean(followResult.value?.following));
          setFollowPending(Boolean(followResult.value?.pending));
          setFollowReady(true);
        } else {
          errors.push(followResult.reason?.message || "关注状态加载失败");
        }
        if (safetyResult.status === "fulfilled") {
          const safetyData = safetyResult.value;
          setSafety({
            blocked: Boolean(safetyData?.blocked),
            blockedBy: Boolean(safetyData?.blocked_by ?? safetyData?.blockedBy),
            muted: Boolean(safetyData?.muted)
          });
          setSafetyReady(true);
        } else {
          errors.push(safetyResult.reason?.message || "屏蔽与静音状态加载失败");
        }
        setRelationLoading(false);
        setFollowError(errors.join("；"));
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
    if (!followReady || !safetyReady) {
      setFollowError("关注与屏蔽状态尚未加载完成，请稍后重试。");
      return;
    }
    if (self) {
      setFollowError("不能关注自己。");
      return;
    }
    if (safety.blocked || safety.blockedBy) {
      setFollowError(safety.blocked ? "请先解除屏蔽后再关注。" : "对方已屏蔽你，当前不能关注。");
      return;
    }
    if (relationActionRef.current) return;
    const targetUserId = profileUserId;
    const requestAccessToken = auth.accessToken;
    const requestSession = relationSessionRef.current;
    const wasFollowing = following;
    const actionID = nextRelationActionRef.current + 1;
    nextRelationActionRef.current = actionID;
    relationActionRef.current = actionID;
    setRelationAction("follow");
    setFollowError("");
    try {
      if (wasFollowing) {
        await bbsApi.unfollowUser(profileUserId, requestAccessToken);
        if (relationSessionRef.current !== requestSession || !matchesRelationScope(relationScopeRef.current, targetUserId, requestAccessToken)) return;
        setFollowing(false);
        setFollowPending(false);
        setFollowerCount((count) => Math.max(0, toNumber(count) - 1));
      } else if (followPending) {
        await bbsApi.cancelFollowRequest(profileUserId, requestAccessToken);
        if (relationSessionRef.current !== requestSession || !matchesRelationScope(relationScopeRef.current, targetUserId, requestAccessToken)) return;
        setFollowPending(false);
      } else {
        const response = await bbsApi.followUser(profileUserId, requestAccessToken);
        if (relationSessionRef.current !== requestSession || !matchesRelationScope(relationScopeRef.current, targetUserId, requestAccessToken)) return;
        if (response?.pending) {
          setFollowPending(true);
          return;
        }
        setFollowing(true);
        setFollowerCount((count) => Math.max(0, toNumber(count) + 1));
      }
      if (relationSessionRef.current !== requestSession || !matchesRelationScope(relationScopeRef.current, targetUserId, requestAccessToken)) return;
    } catch (error) {
      if (relationSessionRef.current === requestSession && matchesRelationScope(relationScopeRef.current, targetUserId, requestAccessToken)) {
        setFollowError(error.message || "关注操作失败");
      }
    } finally {
      if (relationActionRef.current === actionID) {
        relationActionRef.current = 0;
        if (relationSessionRef.current === requestSession && matchesRelationScope(relationScopeRef.current, targetUserId, requestAccessToken)) {
          setRelationAction("");
        }
      }
    }
  }

  async function toggleSafety(kind) {
    if (!profileUserId || !auth?.accessToken || self) return;
    if (!safetyReady || (kind === "block" && !followReady)) {
      setFollowError("屏蔽与静音状态尚未加载完成，请稍后重试。");
      return;
    }
    if (relationActionRef.current) return;
    const targetUserId = profileUserId;
    const requestAccessToken = auth.accessToken;
    const requestSession = relationSessionRef.current;
    const active = kind === "block" ? safety.blocked : safety.muted;
    const wasFollowing = following;
    if (kind === "block" && typeof window !== "undefined" && typeof window.confirm === "function") {
      const message = active ? "确定解除对该用户的屏蔽吗？" : "屏蔽后双方关注关系会解除，并同时静音该用户。确定继续吗？";
      if (!window.confirm(message)) return;
    }
    const actionID = nextRelationActionRef.current + 1;
    nextRelationActionRef.current = actionID;
    relationActionRef.current = actionID;
    setRelationAction(kind);
    setFollowError("");
    try {
      if (kind === "block") {
        await (active ? bbsApi.unblockUser(profileUserId, auth.accessToken) : bbsApi.blockUser(profileUserId, auth.accessToken));
        if (relationSessionRef.current !== requestSession || !matchesRelationScope(relationScopeRef.current, targetUserId, requestAccessToken)) return;
        setSafety((current) => ({ ...current, blocked: !active, muted: !active }));
        if (!active && wasFollowing) {
          setFollowing(false);
          setFollowerCount((count) => Math.max(0, toNumber(count) - 1));
        }
        if (!active) {
          setFollowPending(false);
        }
      } else {
        await (active ? bbsApi.unmuteUser(profileUserId, auth.accessToken) : bbsApi.muteUser(profileUserId, auth.accessToken));
        if (relationSessionRef.current !== requestSession || !matchesRelationScope(relationScopeRef.current, targetUserId, requestAccessToken)) return;
        setSafety((current) => ({ ...current, muted: !active }));
      }
    } catch (error) {
      if (relationSessionRef.current === requestSession && matchesRelationScope(relationScopeRef.current, targetUserId, requestAccessToken)) {
        setFollowError(error.message || (kind === "block" ? "屏蔽操作失败" : "静音操作失败"));
      }
    } finally {
      if (relationActionRef.current === actionID) {
        relationActionRef.current = 0;
        if (relationSessionRef.current === requestSession && matchesRelationScope(relationScopeRef.current, targetUserId, requestAccessToken)) {
          setRelationAction("");
        }
      }
    }
  }

  async function shareProfile() {
    if (typeof window === "undefined" || !publicUsername) {
      setShareNotice("当前用户暂无可分享的用户名链接。");
      return;
    }
    const url = new URL("/u/" + encodeURIComponent(publicUsername), window.location.origin).href;
    const result = await shareLink(url, { title: person.name + " 的主页" });
    setShareNotice(result.message);
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
        {(publicUsername || (publicSpace && !self)) && (
          <div className="user-profile-actions">
            {publicUsername && (
              <button className="author-home-link user-profile-share" type="button" onClick={shareProfile}>
                <Share2 size={16} aria-hidden="true" />
                分享主页
              </button>
            )}
            {publicSpace && !self && (
              <button
                className={`follow-action user-profile-follow ${following ? "is-following" : ""} ${followPending ? "is-pending" : ""}`}
                type="button"
                onClick={toggleFollow}
                disabled={Boolean(relationAction) || Boolean(auth?.accessToken && (relationLoading || !followReady || !safetyReady)) || safety.blocked || safety.blockedBy}
                title={safety.blocked ? "请先解除屏蔽后再关注" : safety.blockedBy ? "对方已屏蔽你，当前不能关注" : undefined}
              >
                {relationAction === "follow" ? "处理中..." : following ? "取消关注" : followPending ? "取消关注申请" : auth ? "关注用户" : "登录后关注"}
              </button>
            )}
            {publicSpace && !self && auth && (
              <button aria-pressed={safety.muted} className={`user-safety-action ${safety.muted ? "is-active" : ""}`} type="button" disabled={Boolean(relationAction) || relationLoading || !safetyReady} onClick={() => toggleSafety("mute")}>
                <VolumeX size={16} aria-hidden="true" />
                {relationAction === "mute" ? "处理中..." : safety.muted ? "解除静音" : "静音"}
              </button>
            )}
            {publicSpace && !self && auth && (
              <button aria-pressed={safety.blocked} className={`user-safety-action is-danger ${safety.blocked ? "is-active" : ""}`} type="button" disabled={Boolean(relationAction) || relationLoading || !followReady || !safetyReady} onClick={() => toggleSafety("block")}>
                <ShieldOff size={16} aria-hidden="true" />
                {relationAction === "block" ? "处理中..." : safety.blocked ? "解除屏蔽" : "屏蔽"}
              </button>
            )}
          </div>
        )}
      </div>
      {publicSpace && !self && auth && (safety.blocked || safety.blockedBy) && (
        <p className="user-safety-note">{safety.blocked ? "你已屏蔽该用户；解除屏蔽后才能重新关注。" : "该用户已屏蔽你，当前无法关注。"}</p>
      )}
      {followError && <p className="form-error user-profile-error" role="alert">{followError}</p>}
      {shareNotice && <p className="form-success user-profile-error" role="status">{shareNotice}</p>}
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

function matchesRelationScope(scope, targetUserId, accessToken) {
  return sameId(scope.targetUserId, targetUserId) && scope.accessToken === accessToken;
}

function UserSafetyPanel({ auth }) {
  const [mode, setMode] = React.useState("blocked");
  const [reloadKey, setReloadKey] = React.useState(0);
  const requestSessionRef = React.useRef(0);
  const scopeRef = React.useRef({ mode, accessToken: auth?.accessToken || "" });
  const [state, setState] = React.useState({ items: [], total: 0, page: 0, loading: false, loadingMore: false, error: "", action: "" });
  scopeRef.current = { mode, accessToken: auth?.accessToken || "" };

  React.useEffect(() => {
    const session = requestSessionRef.current + 1;
    requestSessionRef.current = session;
    if (!auth?.accessToken) {
      setState({ items: [], total: 0, page: 0, loading: false, loadingMore: false, error: "", action: "" });
      return undefined;
    }
    let alive = true;
    setState({ items: [], total: 0, page: 0, loading: true, loadingMore: false, error: "", action: "" });
    const loader = mode === "blocked" ? bbsApi.blockedUsers : bbsApi.mutedUsers;
    loader({ page: 1, page_size: FOLLOW_LIST_PAGE_SIZE }, auth.accessToken)
      .then((data) => {
        if (!alive || requestSessionRef.current !== session || !matchesSafetyScope(scopeRef.current, mode, auth.accessToken)) return;
        const items = listItems(data);
        setState({
          items,
          total: Math.max(listTotal(data, items), items.length),
          page: items.length > 0 ? 1 : 0,
          loading: false,
          loadingMore: false,
          error: "",
          action: ""
        });
      })
      .catch((error) => {
        if (alive && requestSessionRef.current === session && matchesSafetyScope(scopeRef.current, mode, auth.accessToken)) {
          setState({ items: [], total: 0, page: 0, loading: false, loadingMore: false, error: error.message || "安全关系加载失败", action: "" });
        }
      });
    return () => {
      alive = false;
    };
  }, [auth?.accessToken, mode, reloadKey]);

  async function loadMoreRelations() {
    if (!auth?.accessToken || state.loading || state.loadingMore || state.action || state.items.length >= state.total) return;
    const requestMode = mode;
    const requestAccessToken = auth.accessToken;
    const requestSession = requestSessionRef.current;
    const page = state.page + 1;
    const loader = requestMode === "blocked" ? bbsApi.blockedUsers : bbsApi.mutedUsers;
    setState((current) => ({ ...current, loadingMore: true, error: "" }));
    try {
      const data = await loader({ page, page_size: FOLLOW_LIST_PAGE_SIZE }, auth.accessToken);
      if (requestSessionRef.current !== requestSession || !matchesSafetyScope(scopeRef.current, requestMode, requestAccessToken)) return;
      const pageItems = listItems(data);
      setState((current) => {
        const items = appendUniqueSafetyUsers(current.items, pageItems);
        return {
          ...current,
          items,
          total: pageItems.length > 0 ? Math.max(listTotal(data, pageItems), items.length) : items.length,
          page: pageItems.length > 0 ? page : current.page,
          loadingMore: false,
          error: ""
        };
      });
    } catch (error) {
      if (requestSessionRef.current === requestSession && matchesSafetyScope(scopeRef.current, requestMode, requestAccessToken)) {
        setState((current) => ({ ...current, loadingMore: false, error: error.message || "更多安全关系列表加载失败" }));
      }
    }
  }

  async function removeRelation(userId) {
    if (!userId || !auth?.accessToken || state.action || state.loadingMore) return;
    const requestMode = mode;
    const requestAccessToken = auth.accessToken;
    const requestSession = requestSessionRef.current;
    setState((current) => ({ ...current, action: String(userId), error: "" }));
    try {
      if (requestMode === "blocked") await bbsApi.unblockUser(userId, auth.accessToken);
      else await bbsApi.unmuteUser(userId, auth.accessToken);
      if (requestSessionRef.current === requestSession && matchesSafetyScope(scopeRef.current, requestMode, requestAccessToken)) {
        requestSessionRef.current += 1;
        setReloadKey((value) => value + 1);
      }
    } catch (error) {
      if (requestSessionRef.current === requestSession && matchesSafetyScope(scopeRef.current, requestMode, requestAccessToken)) {
        setState((current) => ({ ...current, action: "", error: error.message || "解除关系失败" }));
      }
    }
  }

  if (!auth?.accessToken) return <EmptyState title="请先登录" description="登录后可以管理屏蔽和静音用户。" />;
  return (
    <section className="user-safety-panel">
      <PillTabs
        items={[
          { value: "blocked", label: "已屏蔽", icon: ShieldOff },
          { value: "muted", label: "已静音", icon: VolumeX }
        ]}
        label="用户安全关系"
        value={mode}
        onChange={setMode}
      />
      {state.loading && <EmptyState title={mode === "blocked" ? "正在加载屏蔽列表..." : "正在加载静音列表..."} />}
      {!state.loading && state.error && state.items.length === 0 && <EmptyState title={state.error} />}
      {!state.loading && state.items.length === 0 && !state.error && <EmptyState title={mode === "blocked" ? "暂无屏蔽用户" : "暂无静音用户"} />}
      {state.items.length > 0 && (
        <div className="data-rows">
          {state.items.map((item) => {
            const user = item.user || item;
            const displayName = user.nickname || user.username || `用户 #${user.id}`;
            const profilePath = user.username ? `/u/${encodeURIComponent(user.username)}` : `/user/${user.id}`;
            return (
              <article className="data-row user-safety-row panel" key={user.id}>
                <Link className="user-safety-user-link" to={profilePath}>
                  <strong>{displayName}</strong>
                  <p>{user.bio || `@${user.username || user.id}`}</p>
                </Link>
                <button aria-label={`${mode === "blocked" ? "解除屏蔽" : "解除静音"} ${displayName}`} type="button" disabled={Boolean(state.action) || state.loadingMore} onClick={() => removeRelation(user.id)}>
                  {state.action === String(user.id) ? "处理中..." : mode === "blocked" ? "解除屏蔽" : "解除静音"}
                </button>
              </article>
            );
          })}
        </div>
      )}
      {state.error && state.items.length > 0 && <p className="form-error" role="alert">{state.error}</p>}
      {state.items.length < state.total && (
        <div className="dashboard-history-more">
          <span>{state.loadingMore ? "正在加载更多用户..." : state.error || "继续查看更多用户。"}</span>
          <button aria-label="加载更多屏蔽或静音用户" type="button" disabled={state.loadingMore || Boolean(state.action)} onClick={loadMoreRelations}>
            {state.loadingMore ? "加载中" : "加载更多"}
          </button>
        </div>
      )}
    </section>
  );
}

function matchesSafetyScope(scope, mode, accessToken) {
  return scope.mode === mode && scope.accessToken === accessToken;
}

function appendUniqueSafetyUsers(currentItems, pageItems) {
  const ids = new Set(currentItems.map((item) => String((item.user || item).id)));
  return [
    ...currentItems,
    ...pageItems.filter((item) => {
      const id = String((item.user || item).id);
      if (ids.has(id)) return false;
      ids.add(id);
      return true;
    })
  ];
}

const EMPTY_MFA_STATUS = { enabled: false, recoveryCodesRemaining: 0, enabledAt: 0 };
const EMPTY_PASSKEY_STATE = { items: [], passwordlessEnabled: false, limit: 20 };
const ACCOUNT_STATE_LABELS = {
  active: "正常使用",
  suspended: "已停用",
  deletion_pending: "注销处理中",
  anonymized: "已注销"
};

function PasskeySecuritySection({ token, mfaEnabled }) {
  const [state, setState] = React.useState({ data: EMPTY_PASSKEY_STATE, loading: false, error: "" });
  const [form, setForm] = React.useState({ name: "", password: "", code: "" });
  const [drafts, setDrafts] = React.useState({});
  const [action, setAction] = React.useState("");
  const [notice, setNotice] = React.useState("");
  const [error, setError] = React.useState("");
  const requestRef = React.useRef(0);

  const loadPasskeys = React.useCallback(async () => {
    const requestID = requestRef.current + 1;
    requestRef.current = requestID;
    if (!token || !mfaEnabled) {
      setState({ data: EMPTY_PASSKEY_STATE, loading: false, error: "" });
      setDrafts({});
      return;
    }
    setState((current) => ({ ...current, loading: true, error: "" }));
    try {
      const data = normalizePasskeyList(await bbsApi.passkeys(token));
      if (requestRef.current !== requestID) return;
      setState({ data, loading: false, error: "" });
      setDrafts(Object.fromEntries(data.items.map((item) => [item.credentialId, item.name])));
    } catch (loadError) {
      if (requestRef.current !== requestID) return;
      setState((current) => ({ ...current, loading: false, error: loadError.message || "Passkey 状态加载失败" }));
    }
  }, [mfaEnabled, token]);

  React.useEffect(() => {
    setForm({ name: "", password: "", code: "" });
    setAction("");
    setNotice("");
    setError("");
    loadPasskeys();
    return () => {
      requestRef.current += 1;
    };
  }, [loadPasskeys]);

  function updateForm(field, value) {
    setForm((current) => ({ ...current, [field]: value }));
  }

  function requireReauthentication() {
    if (!form.password || !form.code.trim()) {
      setError("请输入当前密码及验证码或恢复码。");
      return false;
    }
    return true;
  }

  async function registerPasskey(event) {
    event.preventDefault();
    const name = form.name.trim();
    if (!name || [...name].length > 30) {
      setError("Passkey 名称需要 1–30 个字符。");
      return;
    }
    if (!requireReauthentication()) return;
    if (!passkeysSupported()) {
      setError("当前浏览器不支持 Passkey。");
      return;
    }
    setAction("register");
    setError("");
    setNotice("");
    try {
      const options = await bbsApi.beginPasskeyRegistration({ name, password: form.password, code: form.code.trim() }, token);
      setForm((current) => ({ ...current, password: "", code: "" }));
      const credential = await createPasskey(options);
      await bbsApi.finishPasskeyRegistration({ challenge: options.challenge, credential }, token);
      setForm((current) => ({ ...current, name: "" }));
      setNotice("Passkey 已绑定。");
      await loadPasskeys();
    } catch (actionError) {
      setError(friendlyPasskeyError(actionError, "Passkey 绑定失败"));
    } finally {
      setAction("");
    }
  }

  async function renamePasskey(item) {
    const name = String(drafts[item.credentialId] || "").trim();
    if (!name || [...name].length > 30) {
      setError("Passkey 名称需要 1–30 个字符。");
      return;
    }
    setAction(`rename:${item.credentialId}`);
    setError("");
    setNotice("");
    try {
      await bbsApi.updatePasskey(item.credentialId, { name }, token);
      setState((current) => ({
        ...current,
        data: { ...current.data, items: current.data.items.map((entry) => entry.credentialId === item.credentialId ? { ...entry, name } : entry) }
      }));
      setNotice("Passkey 名称已更新。");
    } catch (actionError) {
      setError(actionError.message || "Passkey 名称更新失败");
    } finally {
      setAction("");
    }
  }

  async function deletePasskey(item) {
    if (!requireReauthentication()) return;
    if (typeof window !== "undefined" && typeof window.confirm === "function" && !window.confirm(`确定删除 Passkey“${item.name}”吗？`)) return;
    setAction(`delete:${item.credentialId}`);
    setError("");
    setNotice("");
    try {
      await bbsApi.deletePasskey(item.credentialId, { password: form.password, code: form.code.trim() }, token);
      setForm((current) => ({ ...current, password: "", code: "" }));
      setNotice("Passkey 已删除。");
      await loadPasskeys();
    } catch (actionError) {
      setError(actionError.message || "Passkey 删除失败");
    } finally {
      setAction("");
    }
  }

  async function togglePasswordless() {
    if (!requireReauthentication()) return;
    const enabled = !state.data.passwordlessEnabled;
    setAction("passwordless");
    setError("");
    setNotice("");
    try {
      await bbsApi.setPasskeyPasswordless({ enabled, password: form.password, code: form.code.trim() }, token);
      setForm((current) => ({ ...current, password: "", code: "" }));
      setState((current) => ({ ...current, data: { ...current.data, passwordlessEnabled: enabled } }));
      setNotice(enabled ? "无密码 Passkey 登录已启用。" : "无密码 Passkey 登录已关闭。");
    } catch (actionError) {
      setError(actionError.message || "无密码登录设置失败");
    } finally {
      setAction("");
    }
  }

  return (
    <div className="account-security-section passkey-security-section">
      <div className="account-security-section-heading">
        <Fingerprint size={20} aria-hidden="true" />
        <div>
          <strong>Passkey 与安全密钥</strong>
          <p>使用设备解锁、指纹或安全密钥完成二次验证和无密码登录。</p>
        </div>
      </div>
      {!mfaEnabled ? (
        <p className="form-muted">请先启用双重验证，再绑定 Passkey。</p>
      ) : (
        <>
          {state.loading && <p className="form-muted">正在读取 Passkey...</p>}
          {state.error && (
            <div className="mfa-inline-feedback">
              <p className="form-error" role="alert">{state.error}</p>
              <button className="account-security-secondary" type="button" onClick={loadPasskeys}>重新加载</button>
            </div>
          )}
          {!state.loading && !state.error && (
            <>
              <div className={`mfa-status-row ${state.data.passwordlessEnabled ? "is-enabled" : ""}`}>
                <Fingerprint size={19} aria-hidden="true" />
                <div>
                  <strong>{state.data.items.length} / {state.data.limit} 枚 Passkey</strong>
                  <span>{state.data.passwordlessEnabled ? "无密码登录已启用" : "可作为登录二次验证"}</span>
                </div>
              </div>
              {state.data.items.length > 0 && (
                <div className="passkey-list">
                  {state.data.items.map((item) => (
                    <div className="passkey-row" key={item.credentialId}>
                      <div className="passkey-row-meta">
                        <Fingerprint size={18} aria-hidden="true" />
                        <div>
                          <strong>{item.name}</strong>
                          <span>{item.lastUsedAt ? `最近使用 ${timeAgoMillis(item.lastUsedAt)}` : `绑定于 ${timeAgoMillis(item.createdAt)}`}{item.backupEligible ? " · 可同步" : ""}</span>
                        </div>
                      </div>
                      <div className="passkey-row-actions">
                        <input
                          aria-label={`重命名 ${item.name}`}
                          maxLength={30}
                          value={drafts[item.credentialId] ?? item.name}
                          onChange={(event) => setDrafts((current) => ({ ...current, [item.credentialId]: event.target.value }))}
                        />
                        <button className="account-security-secondary" type="button" disabled={Boolean(action)} onClick={() => renamePasskey(item)}>
                          <Pencil size={16} aria-hidden="true" />
                          {action === `rename:${item.credentialId}` ? "保存中" : "保存"}
                        </button>
                        <button className="account-security-danger" type="button" disabled={Boolean(action)} onClick={() => deletePasskey(item)}>
                          <Trash2 size={16} aria-hidden="true" />
                          {action === `delete:${item.credentialId}` ? "删除中" : "删除"}
                        </button>
                      </div>
                    </div>
                  ))}
                </div>
              )}
              <form className="passkey-registration-form" onSubmit={registerPasskey}>
                <label>
                  Passkey 名称
                  <input maxLength={30} placeholder="例如：工作电脑" value={form.name} onChange={(event) => updateForm("name", event.target.value)} />
                </label>
                <label>
                  当前密码
                  <input autoComplete="current-password" type="password" value={form.password} onChange={(event) => updateForm("password", event.target.value)} />
                </label>
                <label>
                  当前验证码或恢复码
                  <input autoComplete="one-time-code" value={form.code} onChange={(event) => updateForm("code", event.target.value)} />
                </label>
                <p className="form-muted">绑定、删除或切换无密码登录前需要重新验证。恢复码使用后会立即失效。</p>
                <div className="account-security-actions">
                  <button type="submit" disabled={Boolean(action) || !passkeysSupported()}>
                    <Fingerprint size={17} aria-hidden="true" />
                    {action === "register" ? "绑定中..." : "绑定新 Passkey"}
                  </button>
                  {state.data.items.length > 0 && (
                    <button className="account-security-secondary" type="button" disabled={Boolean(action)} onClick={togglePasswordless}>
                      <KeyRound size={17} aria-hidden="true" />
                      {action === "passwordless" ? "保存中..." : state.data.passwordlessEnabled ? "关闭无密码登录" : "启用无密码登录"}
                    </button>
                  )}
                </div>
              </form>
              {!passkeysSupported() && <p className="form-muted">当前浏览器不能创建或使用 Passkey，但仍可管理已绑定凭据。</p>}
            </>
          )}
          {error && <p className="form-error" role="alert">{error}</p>}
          {notice && <p className="form-success" role="status">{notice}</p>}
        </>
      )}
    </div>
  );
}

const SESSION_PAGE_LIMIT = 20;
const EMPTY_SESSION_STATE = { sessions: [], loginEvents: [] };

function SessionSecuritySection({ token }) {
  const [state, setState] = React.useState({ data: EMPTY_SESSION_STATE, loading: false, error: "" });
  const [showHistory, setShowHistory] = React.useState(false);
  const [action, setAction] = React.useState("");
  const [notice, setNotice] = React.useState("");
  const [error, setError] = React.useState("");
  const requestRef = React.useRef(0);

  const loadSessions = React.useCallback(async () => {
    const requestID = requestRef.current + 1;
    requestRef.current = requestID;
    if (!token) {
      setState({ data: EMPTY_SESSION_STATE, loading: false, error: "" });
      return;
    }
    setState((current) => ({ ...current, loading: true, error: "" }));
    try {
      const [sessions, loginEvents] = await Promise.all([
        bbsApi.userSessions({ limit: SESSION_PAGE_LIMIT }, token),
        bbsApi.userLoginEvents({ limit: SESSION_PAGE_LIMIT }, token)
      ]);
      if (requestRef.current !== requestID) return;
      setState({
        data: {
          sessions: normalizeSessionList(sessions).items,
          loginEvents: normalizeLoginEventList(loginEvents).items
        },
        loading: false,
        error: ""
      });
    } catch (loadError) {
      if (requestRef.current !== requestID) return;
      setState((current) => ({ ...current, loading: false, error: loadError.message || "登录会话加载失败" }));
    }
  }, [token]);

  React.useEffect(() => {
    setAction("");
    setNotice("");
    setError("");
    loadSessions();
    return () => {
      requestRef.current += 1;
    };
  }, [loadSessions]);

  async function revokeSession(session) {
    const label = describeUserAgent(session.userAgent);
    if (typeof window !== "undefined" && typeof window.confirm === "function" && !window.confirm(`确定退出“${label}”的这次登录吗？`)) return;
    setAction(`revoke:${session.sessionId}`);
    setError("");
    setNotice("");
    try {
      await bbsApi.revokeUserSession(session.sessionId, token);
      setNotice("该登录已标记为退出。");
      await loadSessions();
    } catch (actionError) {
      setError(actionError.message || "退出登录失败");
    } finally {
      setAction("");
    }
  }

  const activeCount = state.data.sessions.filter((session) => sessionStatus(session) === "active").length;

  return (
    <div className="account-security-section">
      <div className="account-security-section-heading">
        <MonitorSmartphone size={20} aria-hidden="true" />
        <div>
          <strong>登录设备</strong>
          <p>查看最近的登录记录，并退出不再使用的设备。</p>
        </div>
      </div>
      {state.loading && <p className="form-muted">正在读取登录会话...</p>}
      {state.error && (
        <div className="mfa-inline-feedback">
          <p className="form-error" role="alert">{state.error}</p>
          <button className="account-security-secondary" type="button" onClick={loadSessions}>重新加载</button>
        </div>
      )}
      {!state.loading && !state.error && (
        <>
          <div className={`mfa-status-row ${activeCount > 0 ? "is-enabled" : ""}`}>
            <MonitorSmartphone size={19} aria-hidden="true" />
            <div>
              <strong>{activeCount} 个活跃登录</strong>
              <span>共 {state.data.sessions.length} 条会话记录</span>
            </div>
          </div>
          {state.data.sessions.length === 0 && <p className="form-muted">暂无登录会话记录。</p>}
          {state.data.sessions.length > 0 && (
            <div className="passkey-list">
              {state.data.sessions.map((session) => {
                const status = sessionStatus(session);
                return (
                  <div className="passkey-row" key={session.sessionId}>
                    <div className="passkey-row-meta">
                      <MonitorSmartphone size={18} aria-hidden="true" />
                      <div>
                        <strong>{describeUserAgent(session.userAgent)}</strong>
                        <span>
                          {sessionStatusLabel(status)} · {loginMethodLabel(session.loginMethod)} · {ipAddressLabel(session.ipAddress)} · 登录于 {timeAgo(session.createdAt)}
                        </span>
                      </div>
                    </div>
                    <div className="passkey-row-actions">
                      {status === "active" ? (
                        <button
                          className="account-security-danger"
                          type="button"
                          disabled={Boolean(action)}
                          onClick={() => revokeSession(session)}
                        >
                          <LogOut size={16} aria-hidden="true" />
                          {action === `revoke:${session.sessionId}` ? "退出中" : "退出登录"}
                        </button>
                      ) : (
                        <span className="form-muted">{sessionStatusLabel(status)}</span>
                      )}
                    </div>
                  </div>
                );
              })}
            </div>
          )}
          <p className="form-muted">
            退出后该会话会被标记为已失效，并记录在登录历史中。该设备已持有的访问令牌会在有效期结束后失效，如需立即断开请同时修改密码。
          </p>
          <div className="account-security-actions">
            <button
              className="account-security-secondary"
              type="button"
              aria-expanded={showHistory}
              onClick={() => setShowHistory((current) => !current)}
            >
              <History size={17} aria-hidden="true" />
              {showHistory ? "隐藏登录历史" : "查看登录历史"}
            </button>
          </div>
          {showHistory && (
            <>
              {state.data.loginEvents.length === 0 && <p className="form-muted">暂无登录历史。</p>}
              {state.data.loginEvents.length > 0 && (
                <div className="passkey-list">
                  {state.data.loginEvents.map((event) => (
                    <div className="passkey-row" key={event.id}>
                      <div className="passkey-row-meta">
                        <History size={18} aria-hidden="true" />
                        <div>
                          <strong>{event.success ? "登录成功" : loginFailureLabel(event.failureReason)}</strong>
                          <span>
                            {describeUserAgent(event.userAgent)} · {ipAddressLabel(event.ipAddress)} · {timeAgo(event.createdAt)}
                          </span>
                        </div>
                      </div>
                    </div>
                  ))}
                </div>
              )}
            </>
          )}
          {error && <p className="form-error" role="alert">{error}</p>}
          {notice && <p className="form-success" role="status">{notice}</p>}
        </>
      )}
    </div>
  );
}

function APITokenSecuritySection({ token }) {
  const [state, setState] = React.useState({ data: { items: [], total: 0 }, loading: false, error: "" });
  const [form, setForm] = React.useState({ name: "", read: true, write: false, expiresInDays: "90" });
  const [secret, setSecret] = React.useState("");
  const [copied, setCopied] = React.useState(false);
  const [action, setAction] = React.useState("");
  const [notice, setNotice] = React.useState("");
  const [error, setError] = React.useState("");
  const requestRef = React.useRef(0);

  const loadAPITokens = React.useCallback(async () => {
    const requestID = requestRef.current + 1;
    requestRef.current = requestID;
    if (!token) {
      setState({ data: { items: [], total: 0 }, loading: false, error: "" });
      return;
    }
    setState((current) => ({ ...current, loading: true, error: "" }));
    try {
      const data = normalizeAPITokenList(await bbsApi.listAPITokens(token));
      if (requestRef.current !== requestID) return;
      setState({ data, loading: false, error: "" });
    } catch (loadError) {
      if (requestRef.current !== requestID) return;
      setState((current) => ({ ...current, loading: false, error: loadError.message || "API 访问令牌加载失败" }));
    }
  }, [token]);

  React.useEffect(() => {
    setForm({ name: "", read: true, write: false, expiresInDays: "90" });
    setSecret("");
    setCopied(false);
    setAction("");
    setNotice("");
    setError("");
    loadAPITokens();
    return () => {
      requestRef.current += 1;
    };
  }, [loadAPITokens]);

  function updateField(field, value) {
    setForm((current) => ({ ...current, [field]: value }));
  }

  async function createToken(event) {
    event.preventDefault();
    const name = form.name.trim();
    const scopes = [form.read && "read", form.write && "write"].filter(Boolean);
    const requestID = requestRef.current;
    if (!name || name.length > 128) {
      setError("令牌名称需要 1–128 个字符。");
      return;
    }
    if (scopes.length === 0) {
      setError("至少选择一项权限。");
      return;
    }
    setAction("create");
    setSecret("");
    setCopied(false);
    setError("");
    setNotice("");
    try {
      const result = normalizeAPITokenCreation(await bbsApi.createAPIToken({ name, scopes, expires_in_days: Number(form.expiresInDays) }, token));
      if (requestRef.current !== requestID) return;
      setSecret(result.token);
      setCopied(false);
      setForm((current) => ({ ...current, name: "" }));
      setNotice("令牌已创建。明文只会显示这一次，请立即复制并妥善保存。");
      setAction("");
      await loadAPITokens();
    } catch (actionError) {
      if (requestRef.current !== requestID) return;
      setError(actionError.message || "API 访问令牌创建失败");
    } finally {
      if (requestRef.current === requestID) setAction("");
    }
  }

  async function revokeToken(item) {
    if (typeof window !== "undefined" && typeof window.confirm === "function" && !window.confirm(`确定撤销 API 访问令牌“${item.name}”吗？`)) return;
    const requestID = requestRef.current;
    setAction(`revoke:${item.id}`);
    setError("");
    setNotice("");
    try {
      await bbsApi.revokeAPIToken(item.id, token);
      if (requestRef.current !== requestID) return;
      setNotice("API 访问令牌已撤销。");
      setAction("");
      await loadAPITokens();
    } catch (actionError) {
      if (requestRef.current !== requestID) return;
      setError(actionError.message || "API 访问令牌撤销失败");
    } finally {
      if (requestRef.current === requestID) setAction("");
    }
  }

  async function copySecret() {
    if (!secret || typeof navigator === "undefined" || !navigator.clipboard?.writeText) {
      setError("当前浏览器无法访问剪贴板，请手动复制令牌。");
      return;
    }
    try {
      await navigator.clipboard.writeText(secret);
      setCopied(true);
      setError("");
      setNotice("令牌已复制。");
    } catch {
      setError("令牌复制失败，请手动复制。");
    }
  }

  return (
    <div className="account-security-section api-token-security-section">
      <div className="account-security-section-heading">
        <KeyRound size={20} aria-hidden="true" />
        <div>
          <strong>API 访问令牌</strong>
          <p>为脚本或第三方工具创建可撤销的账号访问凭据。</p>
        </div>
      </div>
      {secret && (
        <div className="mfa-recovery-codes api-token-secret" role="status">
          <div className="mfa-recovery-heading">
            <div><strong>新令牌明文</strong><p>离开此页面后将无法再次查看，请立即复制。</p></div>
            <div className="mfa-recovery-actions">
              <button type="button" title="复制访问令牌" aria-label="复制访问令牌" onClick={copySecret}>{copied ? <Check size={17} aria-hidden="true" /> : <Copy size={17} aria-hidden="true" />}</button>
              <button type="button" title="清除屏幕上的访问令牌" aria-label="清除屏幕上的访问令牌" onClick={() => { setSecret(""); setCopied(false); setNotice(""); }}><X size={17} aria-hidden="true" /></button>
            </div>
          </div>
          <code>{secret}</code>
        </div>
      )}
      {error && <p className="form-error" role="alert">{error}</p>}
      {notice && <p className="form-success" role="status">{notice}</p>}
      {state.loading && <p className="form-muted">正在读取 API 访问令牌...</p>}
      {state.error && (
        <div className="mfa-inline-feedback">
          <p className="form-error" role="alert">{state.error}</p>
          <button className="account-security-secondary" type="button" onClick={loadAPITokens}>重新加载</button>
        </div>
      )}
      {!state.loading && !state.error && (
        <>
          {state.data.items.length === 0 ? <p className="form-muted">暂无 API 访问令牌。</p> : (
            <div className="passkey-list">
              {state.data.items.map((item) => {
                const status = apiTokenStatus(item);
                return (
                  <div className="passkey-row" key={item.id}>
                    <div className="passkey-row-meta">
                      <KeyRound size={18} aria-hidden="true" />
                      <div>
                        <strong>{item.name || "未命名令牌"}</strong>
                        <span>{apiTokenStatusLabel(status)} · {item.scopes.map(apiTokenScopeLabel).join("、") || "无权限"} · 创建于 {apiTokenTime(item.createdAt)} · 到期 {item.expiresAt ? apiTokenTime(item.expiresAt) : "永不过期"}</span>
                      </div>
                    </div>
                    <div className="passkey-row-actions">
                      {status === "active" ? (
                        <button className="account-security-danger" type="button" disabled={Boolean(action)} onClick={() => revokeToken(item)}>
                          <Trash2 size={16} aria-hidden="true" />
                          {action === `revoke:${item.id}` ? "撤销中" : "撤销"}
                        </button>
                      ) : <span className="form-muted">{apiTokenStatusLabel(status)}</span>}
                    </div>
                  </div>
                );
              })}
            </div>
          )}
          <form className="api-token-form" onSubmit={createToken}>
            <label>
              令牌名称
              <input maxLength={128} required value={form.name} onChange={(event) => updateField("name", event.target.value)} placeholder="例如：部署脚本" />
            </label>
            <fieldset className="api-token-scope-options">
              <legend>权限</legend>
              <label><input type="checkbox" checked={form.read} onChange={(event) => updateField("read", event.target.checked)} />读取</label>
              <label><input type="checkbox" checked={form.write} onChange={(event) => updateField("write", event.target.checked)} />写入</label>
            </fieldset>
            <label>
              有效期
              <select value={form.expiresInDays} onChange={(event) => updateField("expiresInDays", event.target.value)}>
                <option value="30">30 天</option>
                <option value="90">90 天</option>
                <option value="180">180 天</option>
                <option value="365">365 天</option>
              </select>
            </label>
            <div className="account-security-actions">
              <button type="submit" disabled={Boolean(action)}><KeyRound size={17} aria-hidden="true" />{action === "create" ? "创建中..." : "创建访问令牌"}</button>
            </div>
          </form>
        </>
      )}
    </div>
  );
}

const EMPTY_WEBHOOK_FORM = { id: "", name: "", url: "", secret: "", events: ["note"] };

function WebhookSecuritySection({ token }) {
  const [state, setState] = React.useState({ data: { items: [], total: 0 }, loading: false, error: "" });
  const [form, setForm] = React.useState(EMPTY_WEBHOOK_FORM);
  const [testTypes, setTestTypes] = React.useState({});
  const [action, setAction] = React.useState("");
  const [notice, setNotice] = React.useState("");
  const [error, setError] = React.useState("");
  const requestRef = React.useRef(0);

  const loadWebhooks = React.useCallback(async () => {
    const requestID = requestRef.current + 1;
    requestRef.current = requestID;
    if (!token) {
      setState({ data: { items: [], total: 0 }, loading: false, error: "" });
      return;
    }
    setState((current) => ({ ...current, loading: true, error: "" }));
    try {
      const data = normalizeWebhookList(await bbsApi.listWebhooks(token));
      if (requestRef.current !== requestID) return;
      setState({ data, loading: false, error: "" });
      setTestTypes((current) => Object.fromEntries(data.items.map((item) => [item.id, current[item.id] || item.events[0] || "note"])));
    } catch (loadError) {
      if (requestRef.current !== requestID) return;
      setState((current) => ({ ...current, loading: false, error: loadError.message || "Webhook 加载失败" }));
    }
  }, [token]);

  React.useEffect(() => {
    setForm(EMPTY_WEBHOOK_FORM);
    setTestTypes({});
    setAction("");
    setNotice("");
    setError("");
    loadWebhooks();
    return () => {
      requestRef.current += 1;
    };
  }, [loadWebhooks]);

  function resetForm() {
    setForm(EMPTY_WEBHOOK_FORM);
  }

  function updateField(field, value) {
    setForm((current) => ({ ...current, [field]: value }));
  }

  function toggleEvent(eventType, checked) {
    setForm((current) => ({
      ...current,
      events: checked ? [...new Set([...current.events, eventType])] : current.events.filter((item) => item !== eventType)
    }));
  }

  async function saveWebhook(event) {
    event.preventDefault();
    const name = form.name.trim();
    const url = form.url.trim();
    const secret = form.secret.trim();
    if (!name || [...name].length > 100) {
      setError("Webhook 名称需要 1–100 个字符。");
      return;
    }
    if (!validWebhookURL(url)) {
      setError("请输入 HTTPS 接收地址；本地测试可使用 localhost HTTP 地址。");
      return;
    }
    if (form.events.length === 0) {
      setError("至少选择一种事件。");
      return;
    }
    if (secret.length > 1024) {
      setError("签名密钥不能超过 1024 个字符。");
      return;
    }

    const editing = Boolean(form.id);
    const requestID = requestRef.current;
    const payload = { name, url, on: form.events };
    if (!editing || secret) payload.secret = secret;
    setAction(editing ? `save:${form.id}` : "create");
    setError("");
    setNotice("");
    try {
      if (editing) {
        await bbsApi.updateWebhook(form.id, payload, token);
      } else {
        await bbsApi.createWebhook(payload, token);
      }
      if (requestRef.current !== requestID) return;
      setNotice(editing ? "Webhook 已更新。" : "Webhook 已创建。");
      resetForm();
      setAction("");
      await loadWebhooks();
    } catch (actionError) {
      if (requestRef.current !== requestID) return;
      setError(actionError.message || (editing ? "Webhook 更新失败" : "Webhook 创建失败"));
    } finally {
      if (requestRef.current === requestID) setAction("");
    }
  }

  async function editWebhook(item) {
    const requestID = requestRef.current;
    setAction(`show:${item.id}`);
    setError("");
    setNotice("");
    try {
      const fresh = normalizeWebhook(await bbsApi.showWebhook(item.id, token));
      if (requestRef.current !== requestID) return;
      setForm({ id: fresh.id, name: fresh.name, url: fresh.url, secret: "", events: fresh.events });
    } catch (actionError) {
      if (requestRef.current !== requestID) return;
      setError(actionError.message || "Webhook 详情加载失败");
    } finally {
      if (requestRef.current === requestID) setAction("");
    }
  }

  async function setWebhookActive(item) {
    const requestID = requestRef.current;
    setAction(`active:${item.id}`);
    setError("");
    setNotice("");
    try {
      await bbsApi.updateWebhook(item.id, { active: !item.active }, token);
      if (requestRef.current !== requestID) return;
      setNotice(item.active ? "Webhook 已停用。" : "Webhook 已启用。");
      setAction("");
      await loadWebhooks();
    } catch (actionError) {
      if (requestRef.current !== requestID) return;
      setError(actionError.message || "Webhook 状态更新失败");
    } finally {
      if (requestRef.current === requestID) setAction("");
    }
  }

  async function sendWebhookTest(item) {
    const eventType = testTypes[item.id] || item.events[0] || "note";
    const requestID = requestRef.current;
    setAction(`test:${item.id}`);
    setError("");
    setNotice("");
    try {
      await bbsApi.testWebhook(item.id, eventType, token);
      if (requestRef.current !== requestID) return;
      setNotice(`测试事件“${webhookEventLabel(eventType)}”已进入投递队列。`);
    } catch (actionError) {
      if (requestRef.current !== requestID) return;
      setError(actionError.message || "Webhook 测试失败");
    } finally {
      if (requestRef.current === requestID) setAction("");
    }
  }

  async function deleteWebhook(item) {
    if (typeof window !== "undefined" && typeof window.confirm === "function" && !window.confirm(`确定删除 Webhook“${item.name}”吗？`)) return;
    const requestID = requestRef.current;
    setAction(`delete:${item.id}`);
    setError("");
    setNotice("");
    try {
      await bbsApi.deleteWebhook(item.id, token);
      if (requestRef.current !== requestID) return;
      if (form.id === item.id) resetForm();
      setNotice("Webhook 已删除。");
      setAction("");
      await loadWebhooks();
    } catch (actionError) {
      if (requestRef.current !== requestID) return;
      setError(actionError.message || "Webhook 删除失败");
    } finally {
      if (requestRef.current === requestID) setAction("");
    }
  }

  return (
    <div className="account-security-section webhook-security-section">
      <div className="account-security-section-heading">
        <Webhook size={20} aria-hidden="true" />
        <div>
          <strong>Webhook</strong>
          <p>将账号事件投递到你管理的外部服务。</p>
        </div>
      </div>
      {error && <p className="form-error" role="alert">{error}</p>}
      {notice && <p className="form-success" role="status">{notice}</p>}
      {state.loading && <p className="form-muted">正在读取 Webhook...</p>}
      {state.error && (
        <div className="mfa-inline-feedback">
          <p className="form-error" role="alert">{state.error}</p>
          <button className="account-security-secondary" type="button" onClick={loadWebhooks}>重新加载</button>
        </div>
      )}
      {!state.loading && !state.error && (
        <>
          {state.data.items.length === 0 ? <p className="form-muted">暂无 Webhook。</p> : (
            <div className="passkey-list webhook-list">
              {state.data.items.map((item) => (
                <div className="passkey-row webhook-row" key={item.id}>
                  <div className="passkey-row-meta webhook-row-meta">
                    <Webhook size={18} aria-hidden="true" />
                    <div>
                      <strong>{item.name || "未命名 Webhook"}{item.active ? "" : " · 已停用"}</strong>
                      <span className="webhook-url">{item.url}</span>
                      <span>{item.events.map(webhookEventLabel).join("、") || "未订阅事件"} · {item.latestSentAt ? `最近投递 ${webhookTime(item.latestSentAt)}${item.latestStatus ? ` · HTTP ${item.latestStatus}` : ""}` : "尚无投递记录"}</span>
                    </div>
                  </div>
                  <div className="passkey-row-actions webhook-row-actions">
                    <select
                      className="webhook-test-select"
                      aria-label={`${item.name || "Webhook"}的测试事件`}
                      value={testTypes[item.id] || item.events[0] || "note"}
                      onChange={(event) => setTestTypes((current) => ({ ...current, [item.id]: event.target.value }))}
                      disabled={Boolean(action)}
                    >
                      {WEBHOOK_EVENT_OPTIONS.map((option) => <option key={option.value} value={option.value}>{option.label}</option>)}
                    </select>
                    <button className="account-security-secondary" type="button" disabled={Boolean(action)} onClick={() => sendWebhookTest(item)}>
                      <Send size={16} aria-hidden="true" />{action === `test:${item.id}` ? "测试中" : "测试"}
                    </button>
                    <button className="account-security-secondary" type="button" disabled={Boolean(action)} onClick={() => setWebhookActive(item)}>
                      {item.active ? <PowerOff size={16} aria-hidden="true" /> : <Power size={16} aria-hidden="true" />}{action === `active:${item.id}` ? "处理中" : item.active ? "停用" : "启用"}
                    </button>
                    <button className="account-security-secondary" type="button" disabled={Boolean(action)} onClick={() => editWebhook(item)}>
                      <Pencil size={16} aria-hidden="true" />{action === `show:${item.id}` ? "读取中" : "编辑"}
                    </button>
                    <button className="account-security-danger" type="button" disabled={Boolean(action)} onClick={() => deleteWebhook(item)}>
                      <Trash2 size={16} aria-hidden="true" />{action === `delete:${item.id}` ? "删除中" : "删除"}
                    </button>
                  </div>
                </div>
              ))}
            </div>
          )}
          <form className="api-token-form webhook-form" onSubmit={saveWebhook}>
            <label>
              Webhook 名称
              <input maxLength={100} required value={form.name} onChange={(event) => updateField("name", event.target.value)} placeholder="例如：发布通知" />
            </label>
            <label>
              接收地址
              <input type="url" maxLength={1024} required value={form.url} onChange={(event) => updateField("url", event.target.value)} placeholder="https://hooks.example.com/bbs" />
            </label>
            <label>
              签名密钥{form.id ? "（留空则保持不变）" : "（可选）"}
              <input type="password" autoComplete="new-password" maxLength={1024} value={form.secret} onChange={(event) => updateField("secret", event.target.value)} />
            </label>
            <fieldset className="api-token-scope-options webhook-event-options">
              <legend>事件</legend>
              {WEBHOOK_EVENT_OPTIONS.map((option) => (
                <label key={option.value}>
                  <input type="checkbox" checked={form.events.includes(option.value)} onChange={(event) => toggleEvent(option.value, event.target.checked)} />
                  {option.label}
                </label>
              ))}
            </fieldset>
            <div className="account-security-actions">
              <button type="submit" disabled={Boolean(action)}>
                <Webhook size={17} aria-hidden="true" />{action === "create" || action === `save:${form.id}` ? "保存中..." : form.id ? "保存 Webhook" : "创建 Webhook"}
              </button>
              {form.id && <button className="account-security-secondary" type="button" disabled={Boolean(action)} onClick={resetForm}><X size={17} aria-hidden="true" />取消编辑</button>}
            </div>
          </form>
        </>
      )}
    </div>
  );
}

function AccountDeletionSection({ token, username, mfaEnabled, verificationReady, onAuthInvalidated }) {
  const navigate = useNavigate();
  const [lifecycle, setLifecycle] = React.useState({ data: null, loading: Boolean(token), error: "" });
  const [form, setForm] = React.useState({ confirmation: "", password: "", code: "" });
  const [submitting, setSubmitting] = React.useState(false);
  const [error, setError] = React.useState("");
  const requestRef = React.useRef(0);

  const loadLifecycle = React.useCallback(async () => {
    const requestID = requestRef.current + 1;
    requestRef.current = requestID;
    if (!token) {
      setLifecycle({ data: null, loading: false, error: "" });
      return;
    }
    setLifecycle((current) => ({ ...current, loading: true, error: "" }));
    try {
      const data = await bbsApi.accountLifecycle(token);
      if (requestRef.current !== requestID) return;
      setLifecycle({ data, loading: false, error: "" });
    } catch (loadError) {
      if (requestRef.current !== requestID) return;
      setLifecycle({ data: null, loading: false, error: loadError.message || "账号状态加载失败" });
    }
  }, [token]);

  React.useEffect(() => {
    setForm({ confirmation: "", password: "", code: "" });
    setSubmitting(false);
    setError("");
    loadLifecycle();
    return () => {
      requestRef.current += 1;
    };
  }, [loadLifecycle]);

  function updateField(field, value) {
    setForm((current) => ({ ...current, [field]: value }));
  }

  const expectedUsername = String(username || "").trim();

  async function requestDeletion(event) {
    event.preventDefault();
    if (!token || !expectedUsername) {
      setError("当前账号信息不完整，无法确认注销请求。");
      return;
    }
    if (form.confirmation.trim() !== expectedUsername) {
      setError(`请输入完整用户名 ${expectedUsername} 以确认注销。`);
      return;
    }
    if (!form.password) {
      setError("请输入当前密码。");
      return;
    }
    if (mfaEnabled && !form.code.trim()) {
      setError("请输入当前验证码或恢复码。");
      return;
    }
    setSubmitting(true);
    setError("");
    try {
      await bbsApi.requestAccountDeletion({ password: form.password, code: form.code.trim() }, token);
      setSubmitting(false);
      onAuthInvalidated?.();
      navigate("/user/signin?account_deleted=pending", { replace: true });
    } catch (submitError) {
      setSubmitting(false);
      setError(submitError.message || "账号注销申请失败");
    }
  }

  const accountState = String(lifecycle.data?.state || "").trim();
  const activeJob = lifecycle.data?.active_deletion_job;
  const canRequestDeletion = accountState === "active" && lifecycle.data?.protected !== true && Boolean(expectedUsername);

  return (
    <div className="account-security-section account-deletion-section">
      <div className="account-security-section-heading">
        <Trash2 size={20} aria-hidden="true" />
        <div>
          <strong>数据与账号</strong>
          <p>查看账号状态并提交永久注销申请。</p>
        </div>
      </div>
      {lifecycle.loading && <p className="form-muted">正在读取账号状态...</p>}
      {lifecycle.error && (
        <div className="mfa-inline-feedback">
          <p className="form-error" role="alert">{lifecycle.error}</p>
          <button className="account-security-secondary" type="button" onClick={loadLifecycle}>重新加载</button>
        </div>
      )}
      {!lifecycle.loading && !lifecycle.error && lifecycle.data && (
        <>
          <div className={`account-lifecycle-status ${accountState === "deletion_pending" ? "is-pending" : ""}`}>
            <ShieldCheck size={19} aria-hidden="true" />
            <div>
              <strong>账号状态：{ACCOUNT_STATE_LABELS[accountState] || "未知"}</strong>
              {activeJob && <span>清理进度 {activeJob.completed_steps || 0}/{activeJob.total_steps || 0}</span>}
            </div>
          </div>
          {lifecycle.data.protected === true && <p className="form-muted">此账号受系统保护，不能通过自助入口注销。</p>}
          {canRequestDeletion && (
            <>
              <div className="account-deletion-warning" role="note">
                <strong>注销后无法撤销</strong>
                <p>提交后会立即退出登录，账号进入注销处理，相关数据将按系统策略清理。</p>
              </div>
              <form className="account-deletion-form" onSubmit={requestDeletion} aria-busy={submitting}>
                <label htmlFor="account-deletion-confirmation">
                  输入用户名 {expectedUsername} 确认
                  <input
                    id="account-deletion-confirmation"
                    autoComplete="off"
                    required
                    spellCheck={false}
                    value={form.confirmation}
                    onChange={(event) => updateField("confirmation", event.target.value)}
                  />
                </label>
                <label htmlFor="account-deletion-password">
                  当前密码
                  <input
                    id="account-deletion-password"
                    autoComplete="current-password"
                    required
                    type="password"
                    value={form.password}
                    onChange={(event) => updateField("password", event.target.value)}
                  />
                </label>
                {mfaEnabled && (
                  <label htmlFor="account-deletion-code">
                    当前验证码或恢复码
                    <input
                      id="account-deletion-code"
                      autoComplete="one-time-code"
                      required
                      value={form.code}
                      onChange={(event) => updateField("code", event.target.value)}
                    />
                  </label>
                )}
                {!verificationReady && <p className="form-muted">确认双重验证状态后才能提交注销申请。</p>}
                {error && <p className="form-error" role="alert">{error}</p>}
                <div className="account-security-actions">
                  <button className="account-security-danger" type="submit" disabled={submitting || !verificationReady}>
                    <Trash2 size={17} aria-hidden="true" />
                    {submitting ? "提交中..." : "永久注销账号"}
                  </button>
                </div>
              </form>
            </>
          )}
        </>
      )}
    </div>
  );
}

function AccountSecurityPanel({ auth, onAuthInvalidated }) {
  const navigate = useNavigate();
  const token = auth?.accessToken || "";
  const [form, setForm] = React.useState({
    old_password: "",
    new_password: "",
    confirm_password: ""
  });
  const [saving, setSaving] = React.useState(false);
  const [error, setError] = React.useState("");
  const [mfaState, setMFAState] = React.useState({ status: EMPTY_MFA_STATUS, loading: Boolean(token), error: "" });
  const [mfaForm, setMFAForm] = React.useState({ password: "", code: "", confirmationCode: "" });
  const [mfaAction, setMFAAction] = React.useState("");
  const [mfaError, setMFAError] = React.useState("");
  const [mfaNotice, setMFANotice] = React.useState("");
  const [enrollment, setEnrollment] = React.useState(null);
  const [recoveryCodes, setRecoveryCodes] = React.useState([]);
  const [codesCopied, setCodesCopied] = React.useState(false);
  const mfaRequestRef = React.useRef(0);

  const loadMFAStatus = React.useCallback(async () => {
    const requestID = mfaRequestRef.current + 1;
    mfaRequestRef.current = requestID;
    if (!token) {
      setMFAState({ status: EMPTY_MFA_STATUS, loading: false, error: "" });
      return;
    }
    setMFAState((current) => ({ ...current, loading: true, error: "" }));
    try {
      const data = await bbsApi.mfaStatus(token);
      if (mfaRequestRef.current !== requestID) return;
      setMFAState({ status: normalizeMFAStatus(data), loading: false, error: "" });
    } catch (statusError) {
      if (mfaRequestRef.current !== requestID) return;
      setMFAState((current) => ({ ...current, loading: false, error: statusError.message || "双重验证状态加载失败" }));
    }
  }, [token]);

  React.useEffect(() => {
    setEnrollment(null);
    setRecoveryCodes([]);
    setCodesCopied(false);
    setMFAForm({ password: "", code: "", confirmationCode: "" });
    setMFAError("");
    setMFANotice("");
    loadMFAStatus();
    return () => {
      mfaRequestRef.current += 1;
    };
  }, [loadMFAStatus]);

  function updateField(field, value) {
    setForm((current) => ({ ...current, [field]: value }));
  }

  function updateMFAField(field, value) {
    setMFAForm((current) => ({ ...current, [field]: value }));
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
    try {
      await bbsApi.changePassword(
        {
          old_password: form.old_password,
          new_password: form.new_password
        },
        auth.accessToken
      );
      onAuthInvalidated?.();
      navigate(`/user/signin?redirect=${encodeURIComponent("/user/profile/account")}`, { replace: true });
    } catch (submitError) {
      setError(submitError.message || "密码修改失败");
    } finally {
      setSaving(false);
    }
  }

  async function beginEnrollment(event) {
    event.preventDefault();
    if (!token) return;
    if (!mfaForm.password) {
      setMFAError("请输入当前密码。");
      return;
    }
    if (mfaState.status.enabled && !mfaForm.code.trim()) {
      setMFAError("重新绑定前请输入当前验证码或恢复码。");
      return;
    }
    setMFAAction("begin");
    setMFAError("");
    setMFANotice("");
    setRecoveryCodes([]);
    setCodesCopied(false);
    try {
      const data = await bbsApi.beginTotpEnrollment(
        { password: mfaForm.password, current_code: mfaForm.code.trim() },
        token
      );
      const nextEnrollment = {
        secret: String(data?.secret || "").trim(),
        otpauthUrl: String(data?.otpauth_url || "").trim(),
        qrDataUrl: String(data?.qr_data_url || "").trim(),
        issuer: String(data?.issuer || "").trim(),
        account: String(data?.account || "").trim()
      };
      if (!nextEnrollment.secret || !nextEnrollment.qrDataUrl) {
        throw new Error("验证器绑定信息不完整，请重试。");
      }
      setEnrollment(nextEnrollment);
      setMFAForm((current) => ({ ...current, password: "", code: "", confirmationCode: "" }));
    } catch (actionError) {
      setMFAError(actionError.message || "验证器绑定启动失败");
    } finally {
      setMFAAction("");
    }
  }

  async function confirmEnrollment(event) {
    event.preventDefault();
    if (!token || !enrollment) return;
    if (!mfaForm.confirmationCode.trim()) {
      setMFAError("请输入验证器中的 6 位验证码。");
      return;
    }
    setMFAAction("confirm");
    setMFAError("");
    setMFANotice("");
    try {
      const data = await bbsApi.confirmTotpEnrollment({ code: mfaForm.confirmationCode.trim() }, token);
      const codes = recoveryCodesFromResponse(data);
      if (!codes.length) {
        throw new Error("双重验证已启用，但恢复码未返回，请立即重新生成恢复码。");
      }
      setRecoveryCodes(codes);
      setCodesCopied(false);
      setEnrollment(null);
      setMFAForm({ password: "", code: "", confirmationCode: "" });
      setMFAState({
        status: { enabled: true, recoveryCodesRemaining: codes.length, enabledAt: Date.now() },
        loading: false,
        error: ""
      });
      setMFANotice("双重验证已启用。请保存恢复码。");
    } catch (actionError) {
      setMFAError(actionError.message || "验证码确认失败");
    } finally {
      setMFAAction("");
    }
  }

  async function regenerateRecoveryCodes() {
    if (!token || !mfaState.status.enabled) return;
    if (!mfaForm.password || !mfaForm.code.trim()) {
      setMFAError("请输入当前密码及验证码或恢复码。");
      return;
    }
    setMFAAction("regenerate");
    setMFAError("");
    setMFANotice("");
    try {
      const data = await bbsApi.regenerateMfaRecoveryCodes(
        { password: mfaForm.password, code: mfaForm.code.trim() },
        token
      );
      const codes = recoveryCodesFromResponse(data);
      if (!codes.length) throw new Error("恢复码生成失败，请重试。");
      setRecoveryCodes(codes);
      setCodesCopied(false);
      setMFAForm((current) => ({ ...current, password: "", code: "" }));
      setMFAState((current) => ({
        ...current,
        status: { ...current.status, recoveryCodesRemaining: codes.length }
      }));
      setMFANotice("旧恢复码已失效。请保存新的恢复码。");
    } catch (actionError) {
      setMFAError(actionError.message || "恢复码重新生成失败");
    } finally {
      setMFAAction("");
    }
  }

  async function disableMFA() {
    if (!token || !mfaState.status.enabled) return;
    if (!mfaForm.password || !mfaForm.code.trim()) {
      setMFAError("请输入当前密码及验证码或恢复码。");
      return;
    }
    if (typeof window !== "undefined" && typeof window.confirm === "function" && !window.confirm("确定关闭双重验证吗？现有恢复码将立即失效。")) {
      return;
    }
    setMFAAction("disable");
    setMFAError("");
    setMFANotice("");
    try {
      await bbsApi.disableTotp({ password: mfaForm.password, code: mfaForm.code.trim() }, token);
      setMFAState({ status: EMPTY_MFA_STATUS, loading: false, error: "" });
      setMFAForm({ password: "", code: "", confirmationCode: "" });
      setEnrollment(null);
      setRecoveryCodes([]);
      setCodesCopied(false);
      setMFANotice("双重验证已关闭。");
    } catch (actionError) {
      setMFAError(actionError.message || "双重验证关闭失败");
    } finally {
      setMFAAction("");
    }
  }

  async function copyRecoveryCodes() {
    const text = recoveryCodesText(recoveryCodes);
    if (!text || typeof navigator === "undefined" || typeof navigator.clipboard?.writeText !== "function") {
      setMFAError("当前浏览器无法访问剪贴板，请手动保存恢复码。");
      return;
    }
    try {
      await navigator.clipboard.writeText(text);
      setCodesCopied(true);
      setMFAError("");
      setMFANotice("恢复码已复制。");
    } catch {
      setMFAError("恢复码复制失败，请手动保存。");
    }
  }

  function downloadRecoveryCodes() {
    const text = recoveryCodesText(recoveryCodes);
    if (!text || typeof document === "undefined") return;
    const url = URL.createObjectURL(new Blob([`${text}\n`], { type: "text/plain;charset=utf-8" }));
    const link = document.createElement("a");
    link.href = url;
    link.download = "bbs-recovery-codes.txt";
    link.click();
    URL.revokeObjectURL(url);
    setMFANotice("恢复码文件已下载。");
  }

  if (!token) {
    return <EmptyState title="请先登录" description="登录后可以管理账号安全设置。" />;
  }

  return (
    <section className="account-security panel">
      <header>
        <strong>账号安全</strong>
        <p>管理登录验证、密码和账号生命周期。</p>
      </header>
      <div className="account-security-section mfa-security-section">
        <div className="account-security-section-heading">
          <ShieldCheck size={20} aria-hidden="true" />
          <div>
            <strong>双重验证</strong>
            <p>使用标准 TOTP 验证器保护密码登录。</p>
          </div>
        </div>
        {mfaState.loading && <p className="form-muted">正在读取双重验证状态...</p>}
        {mfaState.error && (
          <div className="mfa-inline-feedback">
            <p className="form-error" role="alert">{mfaState.error}</p>
            <button className="account-security-secondary" type="button" onClick={loadMFAStatus}>重新加载</button>
          </div>
        )}
        {!mfaState.loading && !mfaState.error && (
          <>
            <div className={`mfa-status-row ${mfaState.status.enabled ? "is-enabled" : ""}`}>
              {mfaState.status.enabled ? <ShieldCheck size={19} aria-hidden="true" /> : <KeyRound size={19} aria-hidden="true" />}
              <div>
                <strong>{mfaState.status.enabled ? "验证器已启用" : "验证器未启用"}</strong>
                <span>{mfaState.status.enabled ? `剩余 ${mfaState.status.recoveryCodesRemaining} 枚恢复码` : "登录当前仅验证账号密码"}</span>
              </div>
            </div>

            {recoveryCodes.length > 0 && (
              <div className="mfa-recovery-codes" role="status">
                <div className="mfa-recovery-heading">
                  <div>
                    <strong>恢复码</strong>
                    <p>每枚恢复码只能使用一次，离开此页面后不会再次显示。</p>
                  </div>
                  <div className="mfa-recovery-actions">
                    <button type="button" title="复制恢复码" aria-label="复制恢复码" onClick={copyRecoveryCodes}>
                      {codesCopied ? <Check size={17} aria-hidden="true" /> : <Copy size={17} aria-hidden="true" />}
                    </button>
                    <button type="button" title="下载恢复码" aria-label="下载恢复码" onClick={downloadRecoveryCodes}>
                      <Download size={17} aria-hidden="true" />
                    </button>
                    <button type="button" title="清除屏幕上的恢复码" aria-label="清除屏幕上的恢复码" onClick={() => setRecoveryCodes([])}>
                      <X size={17} aria-hidden="true" />
                    </button>
                  </div>
                </div>
                <ol>
                  {recoveryCodes.map((code) => <li key={code}><code>{code}</code></li>)}
                </ol>
              </div>
            )}

            {enrollment ? (
              <div className="mfa-enrollment">
                <div className="mfa-enrollment-content">
                  <img src={enrollment.qrDataUrl} alt="双重验证绑定二维码" width="220" height="220" />
                  <div>
                    <strong>绑定验证器</strong>
                    <p>{enrollment.issuer}{enrollment.account ? ` · ${enrollment.account}` : ""}</p>
                    <span>无法扫描时输入密钥</span>
                    <code>{enrollment.secret}</code>
                    {enrollment.otpauthUrl && <a href={enrollment.otpauthUrl}>在验证器中打开</a>}
                  </div>
                </div>
                <form onSubmit={confirmEnrollment}>
                  <label>
                    验证器验证码
                    <input
                      autoComplete="one-time-code"
                      inputMode="numeric"
                      maxLength={6}
                      value={mfaForm.confirmationCode}
                      onChange={(event) => updateMFAField("confirmationCode", event.target.value)}
                    />
                  </label>
                  <div className="account-security-actions">
                    <button type="submit" disabled={Boolean(mfaAction)}>
                      <ShieldCheck size={17} aria-hidden="true" />
                      {mfaAction === "confirm" ? "确认中..." : "确认并启用"}
                    </button>
                    <button className="account-security-secondary" type="button" disabled={Boolean(mfaAction)} onClick={() => setEnrollment(null)}>
                      取消
                    </button>
                  </div>
                </form>
              </div>
            ) : (
              <form onSubmit={beginEnrollment}>
                <label>
                  当前密码
                  <input
                    autoComplete="current-password"
                    type="password"
                    value={mfaForm.password}
                    onChange={(event) => updateMFAField("password", event.target.value)}
                  />
                </label>
                {mfaState.status.enabled && (
                  <label>
                    当前验证码或恢复码
                    <input
                      autoComplete="one-time-code"
                      value={mfaForm.code}
                      onChange={(event) => updateMFAField("code", event.target.value)}
                    />
                  </label>
                )}
                <div className="account-security-actions">
                  <button type="submit" disabled={Boolean(mfaAction)}>
                    {mfaState.status.enabled ? <RefreshCw size={17} aria-hidden="true" /> : <ShieldCheck size={17} aria-hidden="true" />}
                    {mfaAction === "begin" ? "处理中..." : mfaState.status.enabled ? "重新绑定" : "启用双重验证"}
                  </button>
                  {mfaState.status.enabled && (
                    <button className="account-security-secondary" type="button" disabled={Boolean(mfaAction)} onClick={regenerateRecoveryCodes}>
                      <KeyRound size={17} aria-hidden="true" />
                      {mfaAction === "regenerate" ? "生成中..." : "重新生成恢复码"}
                    </button>
                  )}
                  {mfaState.status.enabled && (
                    <button className="account-security-danger" type="button" disabled={Boolean(mfaAction)} onClick={disableMFA}>
                      <ShieldOff size={17} aria-hidden="true" />
                      {mfaAction === "disable" ? "关闭中..." : "关闭双重验证"}
                    </button>
                  )}
                </div>
              </form>
            )}
          </>
        )}
        {mfaError && <p className="form-error" role="alert">{mfaError}</p>}
        {mfaNotice && <p className="form-success" role="status">{mfaNotice}</p>}
      </div>

      <PasskeySecuritySection token={token} mfaEnabled={mfaState.status.enabled} />

      <APITokenSecuritySection token={token} />

      <WebhookSecuritySection token={token} />

      <SessionSecuritySection token={token} />

      <div className="account-security-section">
        <div className="account-security-section-heading">
          <LockKeyhole size={20} aria-hidden="true" />
          <div>
            <strong>登录密码</strong>
            <p>更新密码后需要重新登录。</p>
          </div>
        </div>
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
          {error && <p className="form-error" role="alert">{error}</p>}
          <button type="submit" disabled={saving}>
            {saving ? "保存中..." : "更新密码"}
          </button>
        </form>
      </div>

      <AccountDeletionSection
        token={token}
        username={auth?.user?.username || ""}
        mfaEnabled={mfaState.status.enabled}
        verificationReady={!mfaState.loading && !mfaState.error}
        onAuthInvalidated={onAuthInvalidated}
      />
    </section>
  );
}

const ALL_FAVORITES_ID = "all";
const EMPTY_COLLECTION_FORM = { name: "", description: "", is_public: false };

function UserFavoritesPanel({ auth }) {
  const token = auth?.accessToken || "";
  const authTokenRef = React.useRef(token);
  const collectionsRequestRef = React.useRef(0);
  const postsRequestRef = React.useRef(0);
  const actionBusyRef = React.useRef("");
  const [activeCollectionId, setActiveCollectionId] = React.useState(ALL_FAVORITES_ID);
  const [collectionsState, setCollectionsState] = React.useState({ items: [], loading: false, error: "" });
  const [editor, setEditor] = React.useState(null);
  const [choices, setChoices] = React.useState({});
  const [action, setAction] = React.useState({ busy: "", error: "", notice: "" });
  const [state, setState] = React.useState({
    posts: [],
    total: 0,
    offset: 0,
    loading: false,
    loadingMore: false,
    error: ""
  });
  authTokenRef.current = token;

  const activeCollection = collectionsState.items.find((item) => sameId(item.id, activeCollectionId)) || null;

  const loadCollections = React.useCallback(async () => {
    const requestId = collectionsRequestRef.current + 1;
    collectionsRequestRef.current = requestId;
    if (!token) {
      setCollectionsState({ items: [], loading: false, error: "" });
      return [];
    }
    setCollectionsState((current) => ({ ...current, loading: true, error: "" }));
    try {
      const data = await loadAllListPages(bbsApi.collections, { limit: 100, offset: 0 }, token, { pageLimit: 100 });
      if (collectionsRequestRef.current !== requestId || authTokenRef.current !== token) return [];
      const items = data.items.filter((item) => toId(item?.id));
      setCollectionsState({ items, loading: false, error: "" });
      setActiveCollectionId((current) => (
        current === ALL_FAVORITES_ID || items.some((item) => sameId(item.id, current)) ? current : ALL_FAVORITES_ID
      ));
      return items;
    } catch (error) {
      if (collectionsRequestRef.current !== requestId || authTokenRef.current !== token) return [];
      setCollectionsState((current) => ({ ...current, loading: false, error: error.message || "收藏夹加载失败" }));
      return [];
    }
  }, [token]);

  React.useEffect(() => {
    actionBusyRef.current = "";
    setAction({ busy: "", error: "", notice: "" });
    setEditor(null);
    setChoices({});
    setActiveCollectionId(ALL_FAVORITES_ID);
    loadCollections();
    return () => {
      collectionsRequestRef.current += 1;
    };
  }, [loadCollections]);

  const loadFavoritePosts = React.useCallback(async (offset = 0, appending = false) => {
    const requestId = postsRequestRef.current + 1;
    postsRequestRef.current = requestId;
    if (!token) {
      setState({ posts: [], total: 0, offset: 0, loading: false, loadingMore: false, error: "" });
      return;
    }
    setState((current) => ({
      ...current,
      posts: appending ? current.posts : [],
      total: appending ? current.total : 0,
      offset: appending ? current.offset : 0,
      loading: !appending,
      loadingMore: appending,
      error: ""
    }));
    try {
      const params = { limit: USER_INTERACTION_PAGE_SIZE, offset };
      const data = activeCollectionId === ALL_FAVORITES_ID
        ? await bbsApi.favorites(params, token)
        : await bbsApi.collectionItems(activeCollectionId, params, token);
      const pageItems = listItems(data);
      const rawPosts = await Promise.all(pageItems.map((item) => interactionToPost(item, auth, "favorites")));
      const pagePosts = await hydratePostsMeta(rawPosts.filter(Boolean), auth);
      if (postsRequestRef.current !== requestId || authTokenRef.current !== token) return;
      if (appending) {
        setState((current) => {
          const posts = appendUniqueInteractionPosts(current.posts, pagePosts);
          const nextOffset = current.offset + pageItems.length;
          return {
            ...current,
            posts,
            total: pageItems.length > 0 ? Math.max(nextOffset, listTotal(data, pageItems)) : current.offset,
            offset: nextOffset,
            loadingMore: false,
            error: ""
          };
        });
        return;
      }
      setState({
        posts: pagePosts,
        total: Math.max(listTotal(data, pageItems), pagePosts.length),
        offset: pageItems.length,
        loading: false,
        loadingMore: false,
        error: ""
      });
    } catch (error) {
      if (postsRequestRef.current !== requestId || authTokenRef.current !== token) return;
      if (appending) {
        setState((current) => ({ ...current, loadingMore: false, error: error.message || "更多收藏内容加载失败" }));
        return;
      }
      setState({ posts: [], total: 0, offset: 0, loading: false, loadingMore: false, error: error.message || "收藏内容加载失败" });
    }
  }, [activeCollectionId, auth, token]);

  React.useEffect(() => {
    loadFavoritePosts();
    return () => {
      postsRequestRef.current += 1;
    };
  }, [loadFavoritePosts]);

  function selectCollection(collectionId) {
    setActiveCollectionId(collectionId);
    setEditor(null);
    setAction({ busy: actionBusyRef.current, error: "", notice: "" });
  }

  function startCreate() {
    setEditor({ mode: "create", ...EMPTY_COLLECTION_FORM });
    setAction((current) => ({ ...current, error: "", notice: "" }));
  }

  function startEdit() {
    if (!activeCollection) return;
    setEditor({
      mode: "edit",
      id: toId(activeCollection.id),
      name: activeCollection.name || "",
      description: activeCollection.description || "",
      is_public: Boolean(activeCollection.is_public ?? activeCollection.isPublic)
    });
    setAction((current) => ({ ...current, error: "", notice: "" }));
  }

  function updateEditor(field, value) {
    setEditor((current) => (current ? { ...current, [field]: value } : current));
  }

  function beginAction(key) {
    if (!token || actionBusyRef.current) return false;
    actionBusyRef.current = key;
    setAction({ busy: key, error: "", notice: "" });
    return true;
  }

  function finishAction(key, next = {}) {
    if (actionBusyRef.current !== key) return;
    actionBusyRef.current = "";
    setAction({ busy: "", error: next.error || "", notice: next.notice || "" });
  }

  async function submitEditor(event) {
    event.preventDefault();
    if (!editor) return;
    const name = editor.name.trim();
    const description = editor.description.trim();
    if (!name || Array.from(name).length > 80 || Array.from(description).length > 500) {
      setAction((current) => ({ ...current, error: "请检查收藏夹名称和描述长度", notice: "" }));
      return;
    }
    const key = editor.mode === "create" ? "create" : `edit:${editor.id}`;
    if (!beginAction(key)) return;
    const requestToken = token;
    try {
      const payload = { name, description, is_public: Boolean(editor.is_public) };
      const data = editor.mode === "create"
        ? await bbsApi.createCollection(payload, requestToken)
        : await bbsApi.updateCollection(editor.id, payload, requestToken);
      if (authTokenRef.current !== requestToken) return;
      const collectionId = toId(data?.collection?.id || editor.id);
      await loadCollections();
      if (authTokenRef.current !== requestToken) return;
      setEditor(null);
      if (collectionId) setActiveCollectionId(collectionId);
      finishAction(key, { notice: editor.mode === "create" ? "收藏夹已创建" : "收藏夹已更新" });
    } catch (error) {
      if (authTokenRef.current === requestToken) finishAction(key, { error: error.message || "收藏夹保存失败" });
    } finally {
      if (authTokenRef.current !== requestToken && actionBusyRef.current === key) actionBusyRef.current = "";
    }
  }

  async function deleteActiveCollection() {
    if (!activeCollection) return;
    const collectionId = toId(activeCollection.id);
    if (!globalThis.confirm?.(`删除收藏夹“${activeCollection.name}”？`)) return;
    const key = `delete:${collectionId}`;
    if (!beginAction(key)) return;
    const requestToken = token;
    try {
      await bbsApi.deleteCollection(collectionId, requestToken);
      if (authTokenRef.current !== requestToken) return;
      setActiveCollectionId(ALL_FAVORITES_ID);
      setEditor(null);
      await loadCollections();
      if (authTokenRef.current === requestToken) finishAction(key, { notice: "收藏夹已删除" });
    } catch (error) {
      if (authTokenRef.current === requestToken) finishAction(key, { error: error.message || "收藏夹删除失败" });
    } finally {
      if (authTokenRef.current !== requestToken && actionBusyRef.current === key) actionBusyRef.current = "";
    }
  }

  function updateCollectionCount(collectionId, delta) {
    setCollectionsState((current) => ({
      ...current,
      items: current.items.map((item) => sameId(item.id, collectionId)
        ? { ...item, item_count: Math.max(0, toNumber(item.item_count ?? item.itemCount) + delta) }
        : item)
    }));
  }

  async function addPostToCollection(post) {
    const postKey = collectionPostKey(post);
    const collectionId = toId(choices[postKey] || collectionsState.items[0]?.id);
    if (!collectionId) {
      setAction((current) => ({ ...current, error: "请先新建收藏夹", notice: "" }));
      return;
    }
    const key = `add:${postKey}`;
    if (!beginAction(key)) return;
    const requestToken = token;
    try {
      const data = await bbsApi.addCollectionItem(collectionId, { entity_type: post.kind, entity_id: post.id }, requestToken);
      if (authTokenRef.current !== requestToken) return;
      if (data?.changed) updateCollectionCount(collectionId, 1);
      const target = collectionsState.items.find((item) => sameId(item.id, collectionId));
      finishAction(key, { notice: data?.changed ? `已加入“${target?.name || "收藏夹"}”` : "该内容已在收藏夹中" });
    } catch (error) {
      if (authTokenRef.current === requestToken) finishAction(key, { error: error.message || "加入收藏夹失败" });
    } finally {
      if (authTokenRef.current !== requestToken && actionBusyRef.current === key) actionBusyRef.current = "";
    }
  }

  async function removePostFromCollection(post) {
    if (!activeCollection) return;
    const postKey = collectionPostKey(post);
    const collectionId = toId(activeCollection.id);
    const key = `remove:${postKey}`;
    if (!beginAction(key)) return;
    const requestToken = token;
    try {
      const data = await bbsApi.removeCollectionItem(collectionId, { entity_type: post.kind, entity_id: post.id }, requestToken);
      if (authTokenRef.current !== requestToken) return;
      setState((current) => ({
        ...current,
        posts: current.posts.filter((item) => collectionPostKey(item) !== postKey),
        total: Math.max(0, current.total - (data?.changed ? 1 : 0))
      }));
      if (data?.changed) updateCollectionCount(collectionId, -1);
      finishAction(key, { notice: "已移出收藏夹" });
    } catch (error) {
      if (authTokenRef.current === requestToken) finishAction(key, { error: error.message || "移出收藏夹失败" });
    } finally {
      if (authTokenRef.current !== requestToken && actionBusyRef.current === key) actionBusyRef.current = "";
    }
  }

  function loadMoreFavorites() {
    if (state.loading || state.loadingMore || state.offset >= state.total) return;
    loadFavoritePosts(state.offset, true);
  }

  function handlePostArchived(postId, postKind) {
    setState((current) => ({
      ...current,
      posts: current.posts.filter((post) => String(post.id) !== String(postId) || (postKind && post.kind !== postKind))
    }));
  }

  if (!auth) {
    return <EmptyState title="请先登录" description="登录后可以查看收藏记录。" />;
  }

  return (
    <>
      <section className="collection-manager panel">
        <header className="collection-manager__header">
          <div>
            <strong>收藏夹</strong>
            <span>{collectionsState.loading ? "正在加载..." : `${collectionsState.items.length} 个`}</span>
          </div>
          <button type="button" onClick={startCreate} disabled={Boolean(action.busy)}>
            <FolderPlus size={17} aria-hidden="true" />
            新建
          </button>
        </header>
        <div className="collection-tabs" role="tablist" aria-label="收藏夹">
          <button className={activeCollectionId === ALL_FAVORITES_ID ? "is-active" : ""} type="button" onClick={() => selectCollection(ALL_FAVORITES_ID)}>
            <Star size={16} aria-hidden="true" />
            全部收藏
          </button>
          {collectionsState.items.map((collection) => (
            <button
              className={sameId(collection.id, activeCollectionId) ? "is-active" : ""}
              key={toId(collection.id)}
              type="button"
              onClick={() => selectCollection(toId(collection.id))}
            >
              <Folder size={16} aria-hidden="true" />
              <span>{collection.name}</span>
              <small>{toNumber(collection.item_count ?? collection.itemCount)}</small>
            </button>
          ))}
        </div>
        {activeCollection && (
          <div className="collection-current">
            <div>
              <strong>{activeCollection.name}</strong>
              {activeCollection.description && <span>{activeCollection.description}</span>}
            </div>
            <div className="collection-current__actions">
              <span>{activeCollection.is_public ?? activeCollection.isPublic ? "公开" : "私密"}</span>
              <button aria-label="编辑收藏夹" title="编辑收藏夹" type="button" onClick={startEdit} disabled={Boolean(action.busy)}>
                <Pencil size={16} aria-hidden="true" />
              </button>
              <button className="is-danger" aria-label="删除收藏夹" title="删除收藏夹" type="button" onClick={deleteActiveCollection} disabled={Boolean(action.busy)}>
                <Trash2 size={16} aria-hidden="true" />
              </button>
            </div>
          </div>
        )}
        {editor && (
          <form className="collection-editor" onSubmit={submitEditor}>
            <label>
              名称
              <input maxLength={80} required value={editor.name} onChange={(event) => updateEditor("name", event.target.value)} />
            </label>
            <label className="is-wide">
              描述
              <input maxLength={500} value={editor.description} onChange={(event) => updateEditor("description", event.target.value)} />
            </label>
            <label className="collection-editor__visibility">
              <input type="checkbox" checked={editor.is_public} onChange={(event) => updateEditor("is_public", event.target.checked)} />
              公开
            </label>
            <div className="collection-editor__actions">
              <button type="submit" disabled={Boolean(action.busy)}>
                <Check size={16} aria-hidden="true" />
                {editor.mode === "create" ? "创建" : "保存"}
              </button>
              <button className="is-secondary" type="button" onClick={() => setEditor(null)} disabled={Boolean(action.busy)}>
                <X size={16} aria-hidden="true" />
                取消
              </button>
            </div>
          </form>
        )}
        {collectionsState.error && <p className="collection-feedback is-error">{collectionsState.error}</p>}
        {action.error && <p className="collection-feedback is-error">{action.error}</p>}
        {action.notice && <p className="collection-feedback">{action.notice}</p>}
      </section>

      {state.loading && <EmptyState title="正在加载收藏内容..." />}
      {!state.loading && state.error && state.posts.length === 0 && <EmptyState title={state.error} />}
      {!state.loading && !state.error && state.posts.length === 0 && (
        <EmptyState title={activeCollection ? `“${activeCollection.name}”暂无内容` : "暂无收藏内容"} />
      )}
      {!state.loading && state.posts.map((post, index) => {
        const postKey = collectionPostKey(post);
        const selectedCollectionId = toId(choices[postKey] || collectionsState.items[0]?.id);
        return (
          <React.Fragment key={`${post.kind}-${post.id}`}>
            <PostCard auth={auth} index={index} post={post} onPostArchived={handlePostArchived} />
            <div className="collection-post-action">
              {activeCollectionId === ALL_FAVORITES_ID ? (
                collectionsState.items.length > 0 ? (
                  <>
                    <select
                      aria-label={`选择“${post.title || post.id}”的收藏夹`}
                      value={selectedCollectionId}
                      onChange={(event) => setChoices((current) => ({ ...current, [postKey]: event.target.value }))}
                    >
                      {collectionsState.items.map((collection) => (
                        <option key={toId(collection.id)} value={toId(collection.id)}>{collection.name}</option>
                      ))}
                    </select>
                    <button type="button" disabled={Boolean(action.busy)} onClick={() => addPostToCollection(post)}>
                      <FolderPlus size={16} aria-hidden="true" />
                      {action.busy === `add:${postKey}` ? "加入中..." : "加入收藏夹"}
                    </button>
                  </>
                ) : <span>暂无收藏夹</span>
              ) : (
                <button className="is-danger" type="button" disabled={Boolean(action.busy)} onClick={() => removePostFromCollection(post)}>
                  <X size={16} aria-hidden="true" />
                  {action.busy === `remove:${postKey}` ? "移除中..." : "移出收藏夹"}
                </button>
              )}
            </div>
          </React.Fragment>
        );
      })}
      {state.offset < state.total && (
        <div className="dashboard-history-more">
          <span>{state.loadingMore ? "正在加载更多收藏内容..." : state.error || "继续查看更多收藏内容。"}</span>
          <button aria-label="加载更多收藏内容" type="button" disabled={state.loadingMore} onClick={loadMoreFavorites}>
            {state.loadingMore ? "加载中" : "加载更多"}
          </button>
        </div>
      )}
    </>
  );
}

function UserInteractionPanel({ auth, mode }) {
  const [state, setState] = React.useState({
    posts: [],
    total: 0,
    offset: 0,
    loading: false,
    loadingMore: false,
    error: ""
  });

  const loadInteractions = React.useCallback((offset = 0, appending = false) => {
    if (!auth?.accessToken) {
      setState({ posts: [], total: 0, offset: 0, loading: false, loadingMore: false, error: "" });
      return;
    }
    let alive = true;
    setState((current) => ({
      ...current,
      posts: appending ? current.posts : [],
      total: appending ? current.total : 0,
      offset: appending ? current.offset : 0,
      loading: !appending,
      loadingMore: appending,
      error: ""
    }));
    const loader = mode === "favorites" ? bbsApi.favorites : bbsApi.likes;
    loader({ limit: USER_INTERACTION_PAGE_SIZE, offset }, auth.accessToken)
      .then(async (data) => {
        const pageItems = listItems(data);
        const rawPosts = await Promise.all(pageItems.map((item) => interactionToPost(item, auth, mode)));
        const pagePosts = await hydratePostsMeta(rawPosts.filter(Boolean), auth);
        if (!alive) return;
        if (appending) {
          setState((current) => {
            const posts = appendUniqueInteractionPosts(current.posts, pagePosts);
            const nextOffset = current.offset + pageItems.length;
            return {
              ...current,
              posts,
              total: pageItems.length > 0 ? Math.max(nextOffset, listTotal(data, pageItems)) : current.offset,
              offset: nextOffset,
              loadingMore: false,
              error: ""
            };
          });
          return;
        }
        setState({
          posts: pagePosts,
          total: Math.max(listTotal(data, pageItems), pagePosts.length),
          offset: pageItems.length,
          loading: false,
          loadingMore: false,
          error: ""
        });
      })
      .catch((error) => {
        if (!alive) return;
        if (appending) {
          setState((current) => ({ ...current, loadingMore: false, error: error.message || "更多互动记录加载失败" }));
          return;
        }
        setState({ posts: [], total: 0, offset: 0, loading: false, loadingMore: false, error: error.message || "互动记录加载失败" });
      });
    return () => {
      alive = false;
    };
  }, [auth, mode]);

  React.useEffect(loadInteractions, [loadInteractions]);

  function loadMoreInteractions() {
    if (state.loading || state.loadingMore || state.offset >= state.total) return;
    loadInteractions(state.offset, true);
  }

  function handlePostArchived(postId, postKind) {
    setState((current) => ({
      ...current,
      posts: current.posts.filter((post) => String(post.id) !== String(postId) || (postKind && post.kind !== postKind))
    }));
  }

  const interactionLabel = mode === "favorites" ? "收藏内容" : "点赞内容";
  const interactionAction = mode === "favorites" ? "收藏" : "点赞";

  if (!auth) {
    return <EmptyState title="请先登录" description="登录后可以查看收藏和点赞记录。" />;
  }
  if (state.loading) {
    return <EmptyState title={`正在加载${interactionLabel}...`} />;
  }
  if (state.error && state.posts.length === 0) {
    return <EmptyState title={state.error} />;
  }
  if (state.posts.length === 0) {
    return <EmptyState title={`暂无${interactionLabel}`} description={`在帖子或文章里点击${interactionAction}后会出现在这里。`} />;
  }

  return (
    <>
      {state.posts.map((post, index) => (
        <PostCard auth={auth} index={index} key={`${post.kind}-${post.id}`} post={post} onPostArchived={handlePostArchived} />
      ))}
      {state.offset < state.total && (
        <div className="dashboard-history-more">
          <span>{state.loadingMore ? `正在加载更多${interactionLabel}...` : state.error || `继续查看更多${interactionLabel}。`}</span>
          <button aria-label={`加载更多${interactionLabel}`} type="button" disabled={state.loadingMore} onClick={loadMoreInteractions}>
            {state.loadingMore ? "加载中" : "加载更多"}
          </button>
        </div>
      )}
    </>
  );
}

function appendUniqueInteractionPosts(currentPosts, pagePosts) {
  const keys = new Set(currentPosts.map((post) => `${post.kind}-${post.id}`));
  return [
    ...currentPosts,
    ...pagePosts.filter((post) => {
      const key = `${post.kind}-${post.id}`;
      if (keys.has(key)) return false;
      keys.add(key);
      return true;
    })
  ];
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
  if (state.items.length === 0) {
    return (
      <EmptyState
        title="暂无消息"
        description="评论、点赞、收藏、关注、商城和系统通知会出现在这里。需要实时讨论时可以直接进入聊天室。"
        action={
          <button type="button" onClick={() => navigate("/chat")}>
            <MessageCircle size={16} aria-hidden="true" />
            进入聊天室
          </button>
        }
      />
    );
  }
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
        <div className="message-toolbar__actions">
          <button className="message-chat-entry" type="button" onClick={() => navigate("/chat")}>
            <MessageCircle size={16} aria-hidden="true" />
            聊天室
          </button>
          <button type="button" disabled={state.unread === 0 || state.action === "read-all"} onClick={markAllRead}>
            {state.action === "read-all" ? "处理中..." : "全部已读"}
          </button>
        </div>
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
    items: [],
    total: 0,
    offset: 0,
    loading: false,
    loadingMore: false,
    error: ""
  });

  const loadScores = React.useCallback((offset = 0, appending = false) => {
    if (!auth?.accessToken) {
      setState({ balance: null, items: [], total: 0, offset: 0, loading: false, loadingMore: false, error: "" });
      return undefined;
    }
    let alive = true;
    if (appending) {
      setState((current) => ({ ...current, loadingMore: true, error: "" }));
    } else {
      setState({ balance: null, items: [], total: 0, offset: 0, loading: true, loadingMore: false, error: "" });
    }
    const request = appending
      ? bbsApi.creditLedger({ limit: USER_SCORE_PAGE_SIZE, offset }, auth.accessToken)
      : Promise.all([bbsApi.creditBalance(auth.accessToken), bbsApi.creditLedger({ limit: USER_SCORE_PAGE_SIZE, offset: 0 }, auth.accessToken)]);
    Promise.resolve(request)
      .then((data) => {
        if (!alive) return;
        if (appending) {
          const pageItems = listItems(data);
          setState((current) => {
            const items = appendUniqueCreditEntries(current.items, pageItems);
            const nextOffset = current.offset + pageItems.length;
            return {
              ...current,
              items,
              total: pageItems.length > 0 ? Math.max(nextOffset, listTotal(data, pageItems)) : current.offset,
              offset: nextOffset,
              loadingMore: false,
              error: ""
            };
          });
          return;
        }
        const [balanceData, ledgerData] = data;
        const balance = creditBalance(balanceData) || creditBalance(ledgerData);
        const items = listItems(ledgerData);
        setState({
          balance,
          items,
          total: Math.max(listTotal(ledgerData, items), items.length),
          offset: items.length,
          loading: false,
          loadingMore: false,
          error: ""
        });
      })
      .catch((error) => {
        if (!alive) return;
        if (appending) {
          setState((current) => ({ ...current, loadingMore: false, error: error.message || "更多积分明细加载失败" }));
          return;
        }
        setState({ balance: null, items: [], total: 0, offset: 0, loading: false, loadingMore: false, error: error.message || "积分加载失败" });
      });
    return () => {
      alive = false;
    };
  }, [auth?.accessToken]);

  React.useEffect(loadScores, [loadScores]);

  function loadMoreScores() {
    if (state.loading || state.loadingMore || state.offset >= state.total) return;
    loadScores(state.offset, true);
  }

  if (!auth) return <EmptyState title="请先登录" description="登录后可以查看积分和成长记录。" />;
  if (state.loading) return <EmptyState title="正在加载积分..." />;
  if (state.error && state.items.length === 0) return <EmptyState title={state.error} />;

  return (
    <>
      <section className="score-summary panel">
        <span>当前积分</span>
        <strong>{toNumber(state.balance?.total)}</strong>
        <p>积分由发帖、评论、点赞、收藏和任务事件驱动，后端由积分服务统一结算。</p>
      </section>
      {state.items.length > 0 ? (
        <>
          <DataRows
            rows={state.items.map((entry) => ({
              key: entry.id || entry.source_event_id,
              title: creditReasonLabel(entry.reason),
              description: creditEntryMeta(entry),
              meta: `${toNumber(entry.delta)}`
            }))}
          />
          {state.offset < state.total && (
            <div className="dashboard-history-more">
              <span>{state.loadingMore ? "正在加载更多积分明细..." : state.error || "继续查看更早的积分明细。"}</span>
              <button aria-label="加载更多个人积分明细" type="button" disabled={state.loadingMore} onClick={loadMoreScores}>
                {state.loadingMore ? "加载中" : "加载更多"}
              </button>
            </div>
          )}
        </>
      ) : (
        <EmptyState title="暂无积分明细" />
      )}
    </>
  );
}

function appendUniqueCreditEntries(currentItems, pageItems) {
  const keys = new Set(currentItems.map(creditEntryKey));
  return [
    ...currentItems,
    ...pageItems.filter((item) => {
      const key = creditEntryKey(item);
      if (keys.has(key)) return false;
      keys.add(key);
      return true;
    })
  ];
}

function creditEntryKey(entry) {
  return String(entry?.id ?? entry?.source_event_id ?? entry?.sourceEventId ?? "");
}

function UserArticlesPanel({ auth, userId }) {
  const [state, setState] = React.useState({
    posts: [],
    total: 0,
    offset: 0,
    loading: false,
    loadingMore: false,
    error: ""
  });

  const loadArticles = React.useCallback((offset = 0, appending = false) => {
    if (!userId) {
      setState({ posts: [], total: 0, offset: 0, loading: false, loadingMore: false, error: "" });
      return undefined;
    }
    let alive = true;
    setState((current) => ({
      ...current,
      posts: appending ? current.posts : [],
      total: appending ? current.total : 0,
      offset: appending ? current.offset : 0,
      loading: !appending,
      loadingMore: appending,
      error: ""
    }));
    bbsApi
      .listArticles({ author_id: userId, limit: USER_ARTICLE_PAGE_SIZE, offset })
      .then(async (data) => {
        const pageItems = listItems(data);
        const pagePosts = await hydratePostsMeta(pageItems.map((item) => articleToPost(item, auth)), auth);
        if (!alive) return;
        if (appending) {
          setState((current) => {
            const posts = uniquePosts([...current.posts, ...pagePosts]);
            const nextOffset = current.offset + pageItems.length;
            return {
              ...current,
              posts,
              total: pageItems.length > 0 ? Math.max(nextOffset, listTotal(data, pageItems)) : current.offset,
              offset: nextOffset,
              loadingMore: false,
              error: ""
            };
          });
          return;
        }
        setState({
          posts: pagePosts,
          total: Math.max(listTotal(data, pageItems), pagePosts.length),
          offset: pageItems.length,
          loading: false,
          loadingMore: false,
          error: ""
        });
      })
      .catch((error) => {
        if (!alive) return;
        if (appending) {
          setState((current) => ({ ...current, loadingMore: false, error: error.message || "更多文章加载失败" }));
          return;
        }
        setState({ posts: [], total: 0, offset: 0, loading: false, loadingMore: false, error: error.message || "文章加载失败" });
      });
    return () => {
      alive = false;
    };
  }, [auth, userId]);

  React.useEffect(loadArticles, [loadArticles]);

  function loadMoreArticles() {
    if (state.loading || state.loadingMore || state.offset >= state.total) return;
    loadArticles(state.offset, true);
  }

  function handlePostArchived(postId, postKind) {
    setState((current) => ({
      ...current,
      posts: current.posts.filter((post) => String(post.id) !== String(postId) || (postKind && post.kind !== postKind))
    }));
  }

  if (state.loading) return <EmptyState title="正在加载文章..." />;
  if (state.error && state.posts.length === 0) return <EmptyState title={state.error} />;
  if (state.posts.length === 0) return <EmptyState title="暂无公开文章" />;
  return (
    <>
      {state.posts.map((post, index) => (
        <PostCard auth={auth} index={index} key={`${post.kind}-${post.id}`} post={post} onPostArchived={handlePostArchived} />
      ))}
      {state.offset < state.total && (
        <div className="dashboard-history-more">
          <span>{state.loadingMore ? "正在加载更多文章..." : state.error || "继续查看更多文章。"}</span>
          <button aria-label="加载更多用户文章" type="button" disabled={state.loadingMore} onClick={loadMoreArticles}>
            {state.loadingMore ? "加载中" : "加载更多"}
          </button>
        </div>
      )}
    </>
  );
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

function UserListsPanel({ auth, editable, ownerId }) {
  const token = auth?.accessToken || "";
  const [mode, setMode] = React.useState("owned");
  const [state, setState] = React.useState({ items: [], total: 0, loading: false, error: "" });
  const [editor, setEditor] = React.useState(null);
  const [action, setAction] = React.useState({ busy: false, error: "", notice: "" });
  const requestRef = React.useRef(0);

  const loadLists = React.useCallback(async () => {
    const requestId = requestRef.current + 1;
    requestRef.current = requestId;
    if (!ownerId || (editable && !token)) {
      setState({ items: [], total: 0, loading: false, error: "" });
      return;
    }
    setState((current) => ({ ...current, loading: true, error: "" }));
    try {
      const params = { page: 1, page_size: 100 };
      const data = mode === "favorites"
        ? await bbsApi.favoriteUserLists(params, token)
        : editable
          ? await bbsApi.myUserLists(params, token)
          : await bbsApi.userLists(ownerId, params, token || undefined);
      if (requestRef.current !== requestId) return;
      const items = normalizeUserLists(listItems(data));
      setState({ items, total: Math.max(listTotal(data, items), items.length), loading: false, error: "" });
    } catch (error) {
      if (requestRef.current !== requestId) return;
      setState({ items: [], total: 0, loading: false, error: error.message || "用户列表加载失败" });
    }
  }, [editable, mode, ownerId, token]);

  React.useEffect(() => {
    loadLists();
    return () => {
      requestRef.current += 1;
    };
  }, [loadLists]);

  React.useEffect(() => {
    if (!editable && mode !== "owned") setMode("owned");
  }, [editable, mode]);

  async function submitList(event) {
    event.preventDefault();
    if (!editor || !token || action.busy) return;
    const { name, error } = validateUserListName(editor.name);
    if (error) {
      setAction({ busy: false, error, notice: "" });
      return;
    }
    setAction({ busy: true, error: "", notice: "" });
    try {
      await bbsApi.createUserList({ name, is_public: Boolean(editor.isPublic) }, token);
      setEditor(null);
      if (mode === "owned") {
        await loadLists();
      } else {
        setMode("owned");
      }
      setAction({ busy: false, error: "", notice: "列表已创建" });
    } catch (error) {
      setAction({ busy: false, error: error.message || "列表创建失败", notice: "" });
    }
  }

  if (editable && !auth) {
    return <EmptyState title="请先登录" description="登录后可以创建和管理用户列表。" />;
  }

  return (
    <section className="user-list-manager panel">
      <header className="user-list-manager__header">
        <div>
          <strong>{editable ? "我的用户列表" : "公开用户列表"}</strong>
          <span>{state.loading ? "正在加载..." : `${state.total} 个`}</span>
        </div>
        {editable && (
          <button aria-label="新建用户列表" type="button" disabled={action.busy} onClick={() => setEditor({ name: "", isPublic: false })}>
            <FolderPlus size={17} aria-hidden="true" />
            新建
          </button>
        )}
      </header>
      {editable && (
        <div className="user-list-manager__switch" role="tablist" aria-label="用户列表视图">
          <button className={mode === "owned" ? "is-active" : ""} type="button" onClick={() => setMode("owned")}>我创建的</button>
          <button className={mode === "favorites" ? "is-active" : ""} type="button" onClick={() => setMode("favorites")}>我收藏的</button>
        </div>
      )}
      {editor && (
        <form className="user-list-editor" onSubmit={submitList}>
          <label>
            名称
            <input autoFocus maxLength={100} required value={editor.name} onChange={(event) => setEditor((current) => ({ ...current, name: event.target.value }))} />
          </label>
          <label className="user-list-editor__visibility">
            <input type="checkbox" checked={editor.isPublic} onChange={(event) => setEditor((current) => ({ ...current, isPublic: event.target.checked }))} />
            公开列表
          </label>
          <div>
            <button type="submit" disabled={action.busy}><Check size={16} aria-hidden="true" />创建</button>
            <button className="is-secondary" type="button" disabled={action.busy} onClick={() => setEditor(null)}><X size={16} aria-hidden="true" />取消</button>
          </div>
        </form>
      )}
      {state.loading && <p className="user-list-feedback">正在加载用户列表...</p>}
      {!state.loading && state.error && <p className="user-list-feedback is-error">{state.error}</p>}
      {!state.loading && !state.error && state.items.length === 0 && (
        <p className="user-list-feedback">{mode === "favorites" ? "暂无收藏的公开列表" : "暂无用户列表"}</p>
      )}
      {!state.loading && state.items.length > 0 && (
        <div className="user-list-rows">
          {state.items.map((list) => (
            <Link className="user-list-row" key={list.id} to={`/user-lists/${list.id}`}>
              <span className="user-list-row__icon">{list.isPublic ? <Globe2 size={18} aria-hidden="true" /> : <LockKeyhole size={18} aria-hidden="true" />}</span>
              <span>
                <strong>{list.name}</strong>
                <small>{list.memberCount} 位成员 · {list.favoriteCount} 次收藏</small>
              </span>
              <em>{list.isPublic ? "公开" : "私密"}</em>
            </Link>
          ))}
        </div>
      )}
      {action.error && <p className="user-list-feedback is-error">{action.error}</p>}
      {action.notice && <p className="user-list-feedback">{action.notice}</p>}
    </section>
  );
}

export function UserListDetailPage({ auth }) {
  const { listId } = useParams();
  const navigate = useNavigate();
  const token = auth?.accessToken || "";
  const [state, setState] = React.useState({ list: null, members: [], posts: [], loading: true, error: "", feedOffset: 0, hasMore: false });
  const [editor, setEditor] = React.useState(null);
  const [search, setSearch] = React.useState({ query: "", results: [], loading: false, error: "" });
  const [action, setAction] = React.useState({ busy: "", error: "", notice: "" });
  const requestRef = React.useRef(0);
  const owner = userListOwnedBy(state.list, auth?.user?.id);

  const loadDetail = React.useCallback(async () => {
    const requestId = requestRef.current + 1;
    requestRef.current = requestId;
    if (!toId(listId)) {
      setState({ list: null, members: [], posts: [], loading: false, error: "用户列表不存在", feedOffset: 0, hasMore: false });
      return;
    }
    setState((current) => ({ ...current, loading: true, error: "" }));
    try {
      const [listData, membersData, feedData] = await Promise.all([
        bbsApi.userList(listId, token || undefined),
        bbsApi.userListMembers(listId, { page: 1, page_size: 100 }, token || undefined),
        bbsApi.userListFeed(listId, { limit: USER_LIST_FEED_PAGE_SIZE, offset: 0 }, token || undefined)
      ]);
      const rawFeed = listItems(feedData);
      const posts = await hydratePostsMeta(rawFeed.map((item) => feedItemToPost(item, auth)), auth, { skipCounts: true });
      if (requestRef.current !== requestId) return;
      setState({
        list: normalizeUserList(listData),
        members: listItems(membersData),
        posts: uniquePosts(posts),
        loading: false,
        error: "",
        feedOffset: rawFeed.length,
        hasMore: rawFeed.length >= USER_LIST_FEED_PAGE_SIZE
      });
    } catch (error) {
      if (requestRef.current !== requestId) return;
      setState({ list: null, members: [], posts: [], loading: false, error: error.message || "用户列表加载失败", feedOffset: 0, hasMore: false });
    }
  }, [auth, listId, token]);

  React.useEffect(() => {
    loadDetail();
    return () => {
      requestRef.current += 1;
    };
  }, [loadDetail]);

  async function saveList(event) {
    event.preventDefault();
    if (!editor || !owner || action.busy) return;
    const { name, error } = validateUserListName(editor.name);
    if (error) {
      setAction({ busy: "", error, notice: "" });
      return;
    }
    setAction({ busy: "save", error: "", notice: "" });
    try {
      const data = await bbsApi.updateUserList(listId, { name, is_public: Boolean(editor.isPublic) }, token);
      setState((current) => ({ ...current, list: normalizeUserList(data) }));
      setEditor(null);
      setAction({ busy: "", error: "", notice: "列表设置已保存" });
    } catch (error) {
      setAction({ busy: "", error: error.message || "列表保存失败", notice: "" });
    }
  }

  async function deleteList() {
    if (!owner || action.busy || !globalThis.confirm?.(`删除用户列表“${state.list.name}”？`)) return;
    setAction({ busy: "delete", error: "", notice: "" });
    try {
      await bbsApi.deleteUserList(listId, token);
      navigate("/user/lists", { replace: true });
    } catch (error) {
      setAction({ busy: "", error: error.message || "列表删除失败", notice: "" });
    }
  }

  async function toggleFavorite() {
    if (!auth || owner || action.busy) return;
    setAction({ busy: "favorite", error: "", notice: "" });
    try {
      const data = state.list.isFavorited
        ? await bbsApi.unfavoriteUserList(listId, token)
        : await bbsApi.favoriteUserList(listId, token);
      const next = normalizeUserList(data);
      setState((current) => ({ ...current, list: next }));
      setAction({ busy: "", error: "", notice: next.isFavorited ? "已收藏列表" : "已取消收藏" });
    } catch (error) {
      setAction({ busy: "", error: error.message || "列表收藏操作失败", notice: "" });
    }
  }

  async function copyList() {
    if (!auth || owner || action.busy) return;
    const name = Array.from(`${state.list.name} 副本`).slice(0, 100).join("");
    setAction({ busy: "copy", error: "", notice: "" });
    try {
      const data = await bbsApi.copyUserList(listId, name, token);
      const copy = normalizeUserList(data);
      setAction({ busy: "", error: "", notice: "列表已复制" });
      if (copy.id) navigate(`/user-lists/${copy.id}`);
    } catch (error) {
      setAction({ busy: "", error: error.message || "列表复制失败", notice: "" });
    }
  }

  async function searchMembers(event) {
    event.preventDefault();
    const query = search.query.trim();
    if (!query || search.loading) return;
    setSearch((current) => ({ ...current, results: [], loading: true, error: "" }));
    try {
      const data = await bbsApi.searchUsers(query, { page: 1, page_size: 10 });
      const memberIDs = new Set(state.members.map((item) => toId((item.user || item).id)));
      const results = listItems(data).map((item) => item.user || item).filter((item) => toId(item.id) && !memberIDs.has(toId(item.id)));
      setSearch((current) => ({ ...current, results, loading: false, error: results.length ? "" : "未找到可添加的用户" }));
    } catch (error) {
      setSearch((current) => ({ ...current, results: [], loading: false, error: error.message || "用户搜索失败" }));
    }
  }

  async function addMember(user) {
    const userId = toId(user?.id);
    if (!userId || action.busy) return;
    setAction({ busy: `add:${userId}`, error: "", notice: "" });
    try {
      await bbsApi.addUserListMember(listId, userId, token);
      setState((current) => ({
        ...current,
        members: [...current.members, user],
        list: { ...current.list, memberCount: current.list.memberCount + 1 }
      }));
      setSearch((current) => ({ ...current, results: current.results.filter((item) => !sameId(item.id, userId)) }));
      setAction({ busy: "", error: "", notice: "成员已添加" });
    } catch (error) {
      setAction({ busy: "", error: error.message || "成员添加失败", notice: "" });
    }
  }

  async function removeMember(user) {
    const userId = toId((user.user || user).id);
    if (!userId || action.busy) return;
    setAction({ busy: `remove:${userId}`, error: "", notice: "" });
    try {
      await bbsApi.removeUserListMember(listId, userId, token);
      setState((current) => ({
        ...current,
        members: current.members.filter((item) => !sameId((item.user || item).id, userId)),
        list: { ...current.list, memberCount: Math.max(0, current.list.memberCount - 1) }
      }));
      setAction({ busy: "", error: "", notice: "成员已移除" });
    } catch (error) {
      setAction({ busy: "", error: error.message || "成员移除失败", notice: "" });
    }
  }

  async function loadMoreFeed() {
    if (!state.hasMore || action.busy) return;
    setAction({ busy: "feed", error: "", notice: "" });
    try {
      const data = await bbsApi.userListFeed(listId, { limit: USER_LIST_FEED_PAGE_SIZE, offset: state.feedOffset }, token || undefined);
      const rawFeed = listItems(data);
      const page = await hydratePostsMeta(rawFeed.map((item) => feedItemToPost(item, auth)), auth, { skipCounts: true });
      setState((current) => ({ ...current, posts: uniquePosts([...current.posts, ...page]), feedOffset: current.feedOffset + rawFeed.length, hasMore: rawFeed.length >= USER_LIST_FEED_PAGE_SIZE }));
      setAction({ busy: "", error: "", notice: "" });
    } catch (error) {
      setAction({ busy: "", error: error.message || "更多动态加载失败", notice: "" });
    }
  }

  if (state.loading) return <EmptyState title="正在加载用户列表..." />;
  if (state.error || !state.list) return <EmptyState title={state.error || "用户列表不存在"} />;

  return (
    <>
      <RouteHeader
        icon={ListFilter}
        eyebrow={state.list.isPublic ? "公开用户列表" : "私密用户列表"}
        title={state.list.name}
        description={`${state.list.memberCount} 位成员 · ${state.list.favoriteCount} 次收藏`}
      />
      <section className="user-list-detail panel">
        <header className="user-list-detail__actions">
          <div>
            {state.list.isPublic ? <Globe2 size={18} aria-hidden="true" /> : <LockKeyhole size={18} aria-hidden="true" />}
            <span>{state.list.isPublic ? "所有人可查看" : "仅列表所有者可查看"}</span>
          </div>
          <div>
            {owner ? (
              <>
                <button aria-label="编辑用户列表" title="编辑用户列表" type="button" disabled={Boolean(action.busy)} onClick={() => setEditor({ name: state.list.name, isPublic: state.list.isPublic })}><Pencil size={17} aria-hidden="true" /></button>
                <button className="is-danger" aria-label="删除用户列表" title="删除用户列表" type="button" disabled={Boolean(action.busy)} onClick={deleteList}><Trash2 size={17} aria-hidden="true" /></button>
              </>
            ) : auth ? (
              <>
                <button type="button" disabled={Boolean(action.busy)} onClick={toggleFavorite}><Star size={17} aria-hidden="true" />{state.list.isFavorited ? "取消收藏" : "收藏"}</button>
                <button type="button" disabled={Boolean(action.busy)} onClick={copyList}><Copy size={17} aria-hidden="true" />复制</button>
              </>
            ) : null}
          </div>
        </header>
        {editor && (
          <form className="user-list-editor" onSubmit={saveList}>
            <label>名称<input maxLength={100} required value={editor.name} onChange={(event) => setEditor((current) => ({ ...current, name: event.target.value }))} /></label>
            <label className="user-list-editor__visibility"><input type="checkbox" checked={editor.isPublic} onChange={(event) => setEditor((current) => ({ ...current, isPublic: event.target.checked }))} />公开列表</label>
            <div><button type="submit" disabled={Boolean(action.busy)}><Check size={16} aria-hidden="true" />保存</button><button className="is-secondary" type="button" onClick={() => setEditor(null)}><X size={16} aria-hidden="true" />取消</button></div>
          </form>
        )}
        <div className="user-list-members-header">
          <strong>列表成员</strong>
          <span>{state.members.length} / 100</span>
        </div>
        {owner && state.members.length < 100 && (
          <form className="user-list-member-search" onSubmit={searchMembers}>
            <input aria-label="搜索要添加的用户" value={search.query} placeholder="搜索昵称或用户名" onChange={(event) => setSearch((current) => ({ ...current, query: event.target.value }))} />
            <button type="submit" disabled={search.loading}>{search.loading ? "搜索中" : "搜索"}</button>
          </form>
        )}
        {search.error && <p className="user-list-feedback is-error">{search.error}</p>}
        {search.results.length > 0 && (
          <div className="user-list-search-results">
            {search.results.map((user) => <UserListMemberRow action="add" busy={action.busy === `add:${toId(user.id)}`} key={toId(user.id)} user={user} onAction={() => addMember(user)} />)}
          </div>
        )}
        <div className="user-list-members">
          {state.members.map((item) => {
            const user = item.user || item;
            return <UserListMemberRow action={owner ? "remove" : ""} busy={action.busy === `remove:${toId(user.id)}`} key={toId(user.id)} user={user} onAction={() => removeMember(item)} />;
          })}
          {state.members.length === 0 && <p className="user-list-feedback">暂无成员</p>}
        </div>
        {action.error && <p className="user-list-feedback is-error">{action.error}</p>}
        {action.notice && <p className="user-list-feedback">{action.notice}</p>}
      </section>
      <section className="user-list-timeline" aria-label="列表动态">
        <header><strong>列表动态</strong><span>成员发布的最新内容</span></header>
        {state.posts.map((post, index) => <PostCard auth={auth} index={index} key={`${post.kind}-${post.id}`} post={post} />)}
        {state.posts.length === 0 && <EmptyState title="暂无列表动态" />}
        {state.hasMore && <div className="dashboard-history-more"><span>{action.busy === "feed" ? "正在加载更多动态..." : "继续查看更多列表动态。"}</span><button type="button" disabled={Boolean(action.busy)} onClick={loadMoreFeed}>{action.busy === "feed" ? "加载中" : "加载更多"}</button></div>}
      </section>
    </>
  );
}

function UserListMemberRow({ action, busy, onAction, user }) {
  const person = userToPerson(user);
  return (
    <div className="user-list-member-row">
      <Link to={person.username ? `/u/${encodeURIComponent(person.username)}` : `/user/${toId(person.id)}`}>
        <Avatar person={person} small />
        <span><strong>{person.name}</strong><small>@{person.username || person.handle}</small></span>
      </Link>
      {action && (
        <button className={action === "remove" ? "is-danger" : ""} aria-label={action === "add" ? `添加 ${person.name}` : `移除 ${person.name}`} title={action === "add" ? "添加成员" : "移除成员"} type="button" disabled={busy} onClick={onAction}>
          {action === "add" ? <UserPlus size={17} aria-hidden="true" /> : <X size={17} aria-hidden="true" />}
        </button>
      )}
    </div>
  );
}
