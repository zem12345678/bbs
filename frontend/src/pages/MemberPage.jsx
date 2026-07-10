import React from "react";
import { useNavigate } from "react-router-dom";
import { Activity, Crown, Heart, Star } from "lucide-react";
import { bbsApi } from "../api";
import PostCard from "../components/post/PostCard.jsx";
import { creditBalance, listItems, listTotal } from "../lib/apiShapes";
import { creditEntryMeta, creditReasonLabel, toNumber } from "../lib/formatters";
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

  const totalCredit = toNumber(creditState.balance?.total);
  const level = auth ? Math.max(1, Math.floor(totalCredit / 100) + 1) : 0;
  const progress = auth ? Math.min(99, totalCredit % 100) : 0;

  return (
    <>
      <PageHero
        icon={Crown}
        eyebrow="会员"
        title="把贡献转成持续权益"
        description="通过发帖、回答、上传资源和圈子共建累积成长值，解锁更高优先级的社区服务。"
        image={pageImages.会员}
        stats={[
          [auth ? `LV.${level}` : "LV.-", "当前等级"],
          [auth ? `${progress}%` : "--", "升级进度"],
          [auth ? String(totalCredit) : "--", "成长值"]
        ]}
      />
      <section className="panel membership-summary">
        <div>
          <span>{auth ? "当前成长值" : "会员成长"}</span>
          <h2>{auth ? `${totalCredit} 积分` : "登录后查看"}</h2>
          <p>
            {auth
              ? creditState.error || (creditState.loading ? "正在同步积分明细..." : `距离下一级还差 ${100 - progress} 成长值。`)
              : "注册、发帖、评论、点赞和收藏都会进入成长记录。"}
          </p>
        </div>
        <button type="button" disabled={!auth} onClick={() => navigate("/dashboard/scores")}>
          管理会员
        </button>
      </section>
      <InteractionPanel
        auth={auth}
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

function InteractionPanel({ auth }) {
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
