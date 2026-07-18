import React from "react";
import { useNavigate } from "react-router-dom";
import { Activity, Crown, Download, Gift, Heart, Star } from "lucide-react";
import { bbsApi } from "../api";
import PostCard from "../components/post/PostCard.jsx";
import { creditBalance, listItems, listTotal } from "../lib/apiShapes";
import { digitalEntitlementLookupLimit, entitlementDashboardTarget, isActiveMembershipEntitlement as isUsableMembershipEntitlement } from "../lib/entitlements";
import { loadListForFocus } from "../lib/focusedLists";
import { creditEntryMeta, creditReasonLabel, timeAgoMillis, toNumber } from "../lib/formatters";
import { hydratePostsMeta, interactionToPost } from "../lib/postMappers";
import { BenefitCard, BlockHeader, ListRow, PageHero } from "./SectionBlocks.jsx";
import { memberBenefits, pageImages } from "./sectionData";

export default function MemberPage({ auth, categories = [] }) {
  const navigate = useNavigate();
  const [creditState, setCreditState] = React.useState({
    balance: null,
    items: [],
    loading: false,
    error: ""
  });
  const [levelsState, setLevelsState] = React.useState({
    items: [],
    loading: false,
    error: ""
  });
  const [entitlementState, setEntitlementState] = React.useState({
    items: [],
    loading: false,
    error: ""
  });
  const [attachmentDownloadState, setAttachmentDownloadState] = React.useState({
    items: [],
    loading: false,
    error: ""
  });

  React.useEffect(() => {
    let alive = true;
    setLevelsState((current) => ({ ...current, loading: true, error: "" }));
    bbsApi
      .levels({ limit: 20, offset: 0 })
      .then((data) => {
        if (!alive) return;
        setLevelsState({ items: normalizeLevels(data), loading: false, error: "" });
      })
      .catch((error) => {
        if (!alive) return;
        setLevelsState({ items: [], loading: false, error: error.message || "等级配置暂不可用" });
      });
    return () => {
      alive = false;
    };
  }, []);

  React.useEffect(() => {
    if (!auth?.accessToken) {
      setCreditState({ balance: null, items: [], loading: false, error: "" });
      return;
    }
    let alive = true;
    setCreditState((current) => ({ ...current, loading: true, error: "" }));
    Promise.all([bbsApi.creditBalance(auth.accessToken), bbsApi.creditLedger({ limit: 6, offset: 0 }, auth.accessToken)])
      .then(([balanceData, ledgerData]) => {
        if (!alive) return;
        const items = listItems(ledgerData);
        setCreditState({
          balance: creditBalance(balanceData) || creditBalance(ledgerData),
          items,
          loading: false,
          error: ""
        });
      })
      .catch((error) => {
        if (!alive) return;
        setCreditState({ balance: null, items: [], loading: false, error: error.message || "积分服务暂不可用" });
      });
    return () => {
      alive = false;
    };
  }, [auth?.accessToken]);

  React.useEffect(() => {
    if (!auth?.accessToken) {
      setAttachmentDownloadState({ items: [], loading: false, error: "" });
      return;
    }
    let alive = true;
    setAttachmentDownloadState((current) => ({ ...current, loading: true, error: "" }));
    bbsApi
      .attachmentDownloads({ limit: 6, offset: 0 }, auth.accessToken)
      .then((data) => {
        if (!alive) return;
        setAttachmentDownloadState({ items: listItems(data), loading: false, error: "" });
      })
      .catch((error) => {
        if (!alive) return;
        setAttachmentDownloadState({ items: [], loading: false, error: error.message || "附件下载记录加载失败" });
      });
    return () => {
      alive = false;
    };
  }, [auth?.accessToken]);

  React.useEffect(() => {
    if (!auth?.accessToken) {
      setEntitlementState({ items: [], loading: false, error: "" });
      return;
    }
    let alive = true;
    setEntitlementState((current) => ({ ...current, loading: true, error: "" }));
    loadListForFocus(
      bbsApi.mallDigitalEntitlements,
      { status: "ACTIVE", grant_type: "membership", limit: digitalEntitlementLookupLimit, offset: 0 },
      auth.accessToken,
      "membership",
      (entitlement) => isUsableMembershipEntitlement(entitlement),
      null,
      { focusLimit: digitalEntitlementLookupLimit }
    )
      .then((data) => {
        if (!alive) return;
        setEntitlementState({ items: listItems(data), loading: false, error: "" });
      })
      .catch((error) => {
        if (!alive) return;
        setEntitlementState({ items: [], loading: false, error: error.message || "会员权益加载失败" });
      });
    return () => {
      alive = false;
    };
  }, [auth?.accessToken]);

  const totalCredit = toNumber(creditState.balance?.total);
  const membership = buildMembership(totalCredit, levelsState.items);
  const membershipEntitlements = entitlementState.items.filter(isUsableMembershipEntitlement);
  const membershipText =
    creditState.error ||
    (creditState.loading
      ? "正在同步积分明细..."
      : levelsState.loading && levelsState.items.length === 0
        ? "正在同步等级配置..."
        : levelsState.error
          ? `等级配置暂不可用，已按默认规则估算：${membership.summary}`
          : membership.summary);

  return (
    <>
      <PageHero
        icon={Crown}
        eyebrow="会员"
        title="把贡献转成持续权益"
        description="通过发帖、回答、上传资源和圈子共建累积成长值，解锁更高优先级的社区服务。"
        image={pageImages.会员}
        stats={[
          [auth ? membership.label : "LV.-", "当前等级"],
          [auth ? `${membership.progress}%` : "--", "升级进度"],
          [auth ? String(totalCredit) : "--", "成长值"],
          [auth ? String(membershipEntitlements.length) : "--", "会员权益"]
        ]}
      />
      <section className="panel membership-summary">
        <div>
          <span>{auth ? "当前成长值" : "会员成长"}</span>
          <h2>{auth ? `${totalCredit} 积分` : "登录后查看"}</h2>
          <p>
            {auth
              ? membershipText
              : "注册、发帖、评论、点赞和收藏都会进入成长记录。"}
          </p>
        </div>
        <button type="button" disabled={!auth} onClick={() => navigate("/dashboard/scores")}>
          管理会员
        </button>
      </section>
      <section className="panel content-block">
        <BlockHeader
          icon={Gift}
          title="已购会员权益"
          action={auth ? "去兑换" : "登录查看"}
          onAction={() => navigate(auth ? "/shop?category=digital&keyword=vip" : "/user/signin")}
        />
        <div className="compact-list">
          {!auth && <ListRow title="登录后查看会员权益" meta="购买会员月卡后会同步到这里。" />}
          {auth && entitlementState.loading && <ListRow title="正在同步会员权益" meta="请稍候" />}
          {auth && entitlementState.error && <ListRow title="会员权益加载失败" meta={entitlementState.error} />}
          {auth && !entitlementState.loading && !entitlementState.error && membershipEntitlements.length === 0 && (
            <ListRow
              actionLabel="去兑换"
              title="暂无 active 会员权益"
              meta="兑换会员月卡后可使用悬赏问答。"
              onAction={() => navigate("/shop?category=digital&keyword=vip")}
            />
          )}
          {auth &&
            !entitlementState.loading &&
            !entitlementState.error &&
            membershipEntitlements.slice(0, 3).map((entitlement) => (
              <ListRow
                actionLabel="查看"
                key={entitlement.id || `${entitlementOrderId(entitlement)}-${entitlementGrantKey(entitlement)}`}
                title={membershipEntitlementTitle(entitlement)}
                meta={membershipEntitlementMeta(entitlement)}
                onAction={() =>
                  navigate(
                    entitlementDashboardTarget(entitlement, {
                      grantType: "membership",
                      status: "ACTIVE"
                    })
                  )
                }
              />
            ))}
        </div>
      </section>
      <section className="panel content-block">
        <BlockHeader icon={Download} title="附件下载记录" />
        <div className="compact-list">
          {!auth && <ListRow title="登录后查看附件下载记录" meta="已授权的附件会保留在这里。" />}
          {auth && attachmentDownloadState.loading && <ListRow title="正在同步附件下载记录" meta="请稍候" />}
          {auth && attachmentDownloadState.error && <ListRow title="附件下载记录加载失败" meta={attachmentDownloadState.error} />}
          {auth && !attachmentDownloadState.loading && !attachmentDownloadState.error && attachmentDownloadState.items.length === 0 && (
            <ListRow title="暂无附件下载记录" meta="下载付费或免费附件后会出现在这里。" />
          )}
          {auth &&
            !attachmentDownloadState.loading &&
            !attachmentDownloadState.error &&
            attachmentDownloadState.items.map((download) => {
              const topicID = attachmentDownloadTopicID(download);
              return (
                <ListRow
                  actionLabel="查看帖子"
                  key={attachmentDownloadKey(download)}
                  title={attachmentDownloadTitle(download)}
                  meta={attachmentDownloadMeta(download)}
                  onAction={topicID ? () => navigate(`/topic/${topicID}`) : undefined}
                />
              );
            })}
        </div>
      </section>
      <InteractionPanel
        auth={auth}
        categories={categories}
      />
      <section className="panel content-block">
        <BlockHeader
          icon={Activity}
          title="积分明细"
          action={auth ? "查看全部" : "登录查看"}
          onAction={() => navigate(auth ? "/dashboard/scores" : "/user/signin")}
        />
        <div className="compact-list">
          {!auth && <ListRow title="登录后同步积分明细" meta="发帖、互动和被互动都会累计成长值" />}
          {auth && creditState.loading && <ListRow title="正在加载积分明细" meta="请稍候" />}
          {auth && !creditState.loading && creditState.items.length === 0 && <ListRow title="暂无积分记录" meta="发布第一篇帖子开始累计" />}
          {auth &&
            !creditState.loading &&
            creditState.items.map((entry) => (
              <ListRow key={entry.id || `${entry.reason}-${entry.source_event_id}`} title={creditReasonLabel(entry.reason)} meta={creditEntryMeta(entry)} />
            ))}
        </div>
      </section>
      <div className="benefit-grid">
        {memberBenefits.map((benefit) => (
          <BenefitCard benefit={benefit} key={benefit.title} />
        ))}
      </div>
    </>
  );
}

function normalizeLevels(data) {
  return listItems(data)
    .map((item, index) => {
      const rawStatus = item?.status ?? item?.Status;
      if (rawStatus !== undefined && rawStatus !== null && toNumber(rawStatus) !== 2) {
        return null;
      }
      return {
        ...item,
        label: item?.name || item?.key || `LV.${index + 1}`,
        description: item?.description || "",
        minScore: Math.max(0, toNumber(item?.min_score ?? item?.minScore)),
        maxScore: Math.max(0, toNumber(item?.max_score ?? item?.maxScore)),
        sort: toNumber(item?.sort)
      };
    })
    .filter(Boolean)
    .sort((left, right) => left.minScore - right.minScore || left.sort - right.sort);
}

function buildMembership(totalCredit, levels) {
  const fallbackLevel = Math.max(1, Math.floor(totalCredit / 100) + 1);
  const fallbackProgress = Math.min(99, totalCredit % 100);
  const fallback = {
    label: `LV.${fallbackLevel}`,
    progress: fallbackProgress,
    summary: `距离下一级还差 ${100 - fallbackProgress} 成长值。`
  };
  if (!levels.length) {
    return fallback;
  }

  let currentLevel = levels[0];
  levels.forEach((level) => {
    if (totalCredit >= level.minScore) {
      currentLevel = level;
    }
  });
  const nextLevel = levels.find((level) => level.minScore > totalCredit);
  const description = currentLevel.description ? `${currentLevel.description} · ` : "";
  if (!nextLevel) {
    return {
      label: currentLevel.label,
      progress: 100,
      summary: `${description}已达到当前最高等级。`
    };
  }

  const baseScore = currentLevel.minScore;
  const scoreRange = Math.max(1, nextLevel.minScore - baseScore);
  const progress = Math.max(0, Math.min(99, Math.floor(((totalCredit - baseScore) / scoreRange) * 100)));
  return {
    label: currentLevel.label,
    progress,
    summary: `${description}距离 ${nextLevel.label} 还差 ${Math.max(0, nextLevel.minScore - totalCredit)} 成长值。`
  };
}

function entitlementExpiresAt(entitlement) {
  return toNumber(entitlement?.expires_at ?? entitlement?.expiresAt);
}

function entitlementExpired(entitlement) {
  const expiresAt = entitlementExpiresAt(entitlement);
  return expiresAt > 0 && expiresAt <= Date.now();
}

function entitlementGrantKey(entitlement) {
  return String(entitlement?.grant_key || entitlement?.grantKey || entitlement?.sku || "").trim();
}

function entitlementOrderId(entitlement) {
  return entitlement?.order_id || entitlement?.orderId || "";
}

function entitlementIssuedText(entitlement) {
  const issuedAt = toNumber(entitlement?.issued_at || entitlement?.issuedAt);
  return issuedAt ? timeAgoMillis(issuedAt) : "发放时间待同步";
}

function membershipEntitlementTitle(entitlement) {
  return entitlement?.title || entitlement?.sku || entitlementGrantKey(entitlement) || "会员权益";
}

function membershipEntitlementMeta(entitlement) {
  const code = entitlement?.fulfillment_code || entitlement?.fulfillmentCode;
  const orderId = entitlementOrderId(entitlement);
  return [`已发放 · ${entitlementIssuedText(entitlement)}`, membershipExpiryText(entitlement), orderId ? `订单 #${orderId}` : "", code ? `交付码 ${code}` : ""]
    .filter(Boolean)
    .join(" · ");
}

function membershipExpiryText(entitlement) {
  const expiresAt = entitlementExpiresAt(entitlement);
  if (!expiresAt) return "";
  const date = new Date(expiresAt);
  if (Number.isNaN(date.getTime())) return "";
  return `${entitlementExpired(entitlement) ? "已过期" : "有效至"} ${date.toLocaleDateString("zh-CN", { year: "numeric", month: "2-digit", day: "2-digit" })}`;
}

function attachmentDownloadAttachment(download) {
  return download?.attachment || download?.Attachment || {};
}

function attachmentDownloadTopicID(download) {
  const attachment = attachmentDownloadAttachment(download);
  return attachment?.topic_id || attachment?.topicId || "";
}

function attachmentDownloadKey(download) {
  const attachment = attachmentDownloadAttachment(download);
  return `${attachment?.id || attachment?.ID || "attachment"}-${download?.authorized_at || download?.authorizedAt || download?.created_at || download?.createdAt || "record"}`;
}

function attachmentDownloadTitle(download) {
  const attachment = attachmentDownloadAttachment(download);
  return attachment?.original_name || attachment?.originalName || "附件";
}

function attachmentDownloadMeta(download) {
  const attachment = attachmentDownloadAttachment(download);
  const attachmentStatus = String(attachment?.status || "").trim();
  const chargedCredits = toNumber(download?.charged_credits ?? download?.chargedCredits);
  const status = String(download?.status || "").trim();
  const timestamp = toNumber(download?.authorized_at ?? download?.authorizedAt ?? download?.created_at ?? download?.createdAt);
  return [
    chargedCredits > 0 ? `已扣 ${chargedCredits} 积分` : "免费获取",
    status === "AUTHORIZED" ? "已授权" : status,
    attachmentStatus === "ARCHIVED" ? "附件已归档" : "",
    timestamp ? timeAgoMillis(timestamp) : ""
  ]
    .filter(Boolean)
    .join(" · ");
}

function InteractionPanel({ auth, categories = [] }) {
  const [mode, setMode] = React.useState("likes");
  const [state, setState] = React.useState({
    posts: [],
    total: 0,
    loading: false,
    error: ""
  });
  const modes = [
    { value: "likes", label: "点赞", icon: Heart },
    { value: "favorites", label: "收藏", icon: Star }
  ];

  React.useEffect(() => {
    if (!auth?.accessToken) {
      setState({ posts: [], total: 0, loading: false, error: "" });
      return;
    }
    let alive = true;
    setState((current) => ({ ...current, loading: true, error: "" }));
    const loader = mode === "likes" ? bbsApi.likes : bbsApi.favorites;
    loader({ limit: 8, offset: 0 }, auth.accessToken)
      .then(async (data) => {
        const rawPosts = await Promise.all(listItems(data).map((item) => interactionToPost(item, auth, mode)));
        const hydrated = await hydratePostsMeta(rawPosts.filter(Boolean), auth);
        if (!alive) return;
        setState({
          posts: hydrated.map((post) => ({
            ...post,
            liked: mode === "likes" ? true : post.liked,
            favorited: mode === "favorites" ? true : post.favorited
          })),
          total: listTotal(data, hydrated),
          loading: false,
          error: ""
        });
      })
      .catch((error) => {
        if (!alive) return;
        setState({ posts: [], total: 0, loading: false, error: error.message || "互动记录加载失败" });
      });
    return () => {
      alive = false;
    };
  }, [auth?.accessToken, auth?.user?.id, hydratePostsMeta, interactionToPost, mode]);

  function handlePostStatsChange(postId, stats) {
    setState((current) => {
      const removed = (mode === "likes" && stats.liked === false) || (mode === "favorites" && stats.favorited === false);
      if (removed) {
        return {
          ...current,
          posts: current.posts.filter((item) => String(item.id) !== String(postId)),
          total: Math.max(0, current.total - 1)
        };
      }
      return {
        ...current,
        posts: current.posts.map((item) => (String(item.id) === String(postId) ? { ...item, ...stats } : item))
      };
    });
  }

  function handlePostArchived(postId, postKind) {
    setState((current) => {
      const nextPosts = current.posts.filter((item) => String(item.id) !== String(postId) || (postKind && item.kind !== postKind));
      return {
        ...current,
        posts: nextPosts,
        total: nextPosts.length === current.posts.length ? current.total : Math.max(0, current.total - 1)
      };
    });
  }

  return (
    <section className="interaction-section" aria-label="我的互动">
      <header className="interaction-toolbar panel">
        <div>
          <strong>我的互动</strong>
          <span>
            {!auth ? "登录后查看点赞和收藏" : state.loading ? "正在同步互动记录" : `${state.total} 条${mode === "likes" ? "点赞" : "收藏"}记录`}
          </span>
        </div>
        <div className="feed-switch" role="tablist" aria-label="互动类型">
          {modes.map(({ value, label, icon: Icon }) => (
            <button
              aria-pressed={mode === value}
              className={mode === value ? "is-active" : ""}
              key={value}
              type="button"
              onClick={() => setMode(value)}
            >
              <Icon size={17} aria-hidden="true" />
              {label}
            </button>
          ))}
        </div>
      </header>
      {!auth && <div className="interaction-status panel">请先登录后查看你的点赞和收藏。</div>}
      {auth && state.error && <div className="interaction-status panel">{state.error}</div>}
      {auth && state.loading && <div className="interaction-status panel">正在加载互动记录...</div>}
      {auth && !state.loading && !state.error && state.posts.length === 0 && (
        <div className="interaction-status panel">暂无{mode === "likes" ? "点赞" : "收藏"}记录。</div>
      )}
      {auth && !state.loading && !state.error && state.posts.length > 0 && (
        <div className="interaction-list">
          {state.posts.map((post, index) => (
            <PostCard
              auth={auth}
              categories={categories}
              index={index}
              key={`${mode}-${post.kind}-${post.id}`}
              post={post}
              onPostArchived={handlePostArchived}
              onPostStatsChange={handlePostStatsChange}
            />
          ))}
        </div>
      )}
    </section>
  );
}
