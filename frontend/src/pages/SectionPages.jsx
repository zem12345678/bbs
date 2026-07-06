import React from "react";
import {
  Activity,
  CalendarDays,
  CircleHelp,
  Compass,
  FileText,
  FolderOpen,
  Grid3X3,
  MessageCircle,
  Rocket,
  ShieldCheck,
  ShoppingBag,
  Star,
  Trophy,
  Users
} from "lucide-react";
import { bbsApi } from "../api";
import { listItems, listTotal } from "../lib/apiShapes";
import { timeAgoMillis, toNumber } from "../lib/formatters";
import { EmptyState } from "./RouteBlocks.jsx";
import {
  BlockHeader,
  CircleCard,
  ListRow,
  MetricCard,
  MoreCard,
  PageHero,
  ProductCard,
  QuestionCard,
  ResourceCard,
  StepItem,
  TrendBar
} from "./SectionBlocks.jsx";
import { pageImages, workspacePhotos } from "./sectionData";

export function HomePage({ categories = [], hotTags = [] }) {
  const [reloadKey, setReloadKey] = React.useState(0);
  const [state, setState] = React.useState({
    topics: [],
    articles: [],
    loading: true,
    error: ""
  });

  React.useEffect(() => {
    let alive = true;
    setState((current) => ({ ...current, loading: true, error: "" }));
    Promise.allSettled([
      bbsApi.listTopics({ limit: 8, offset: 0 }),
      bbsApi.listArticles({ limit: 8, offset: 0 })
    ]).then(([topicResult, articleResult]) => {
      if (!alive) return;
      const topics = topicResult.status === "fulfilled" ? listItems(topicResult.value) : [];
      const articles = articleResult.status === "fulfilled" ? listItems(articleResult.value) : [];
      const failed = topicResult.status === "rejected" && articleResult.status === "rejected";
      setState({
        topics,
        articles,
        loading: false,
        error: failed ? "首页数据加载失败，请稍后重试。" : ""
      });
    });
    return () => {
      alive = false;
    };
  }, [reloadKey]);

  const recentItems = [...state.topics.map((item) => homeContentItem(item, "话题")), ...state.articles.map((item) => homeContentItem(item, "文章"))]
    .sort((a, b) => b.sortAt - a.sortAt)
    .slice(0, 4);
  const recentCount = state.topics.length + state.articles.length;
  const metricCards = [
    {
      label: "近期话题",
      value: state.loading ? "..." : String(state.topics.length),
      meta: state.error ? "加载失败" : state.topics.length > 0 ? "实时更新" : "暂无新话题",
      icon: MessageCircle
    },
    {
      label: "近期文章",
      value: state.loading ? "..." : String(state.articles.length),
      meta: state.error ? "加载失败" : state.articles.length > 0 ? "实时更新" : "暂无新文章",
      icon: FileText
    },
    {
      label: "开放分类",
      value: String(categories.length),
      meta: categories.length > 0 ? "已开放" : "暂无分类",
      icon: Users
    },
    {
      label: "热门标签",
      value: String(hotTags.length),
      meta: hotTags.length > 0 ? "持续更新" : "暂无标签",
      icon: FolderOpen
    }
  ];

  return (
    <>
      <PageHero
        icon={Compass}
        eyebrow="社区首页"
        title="今天的社区协作都在这里"
        description="快速查看关注动态、待办求助、热门圈子和资源更新，把分散的信息收进一个工作台。"
        image={pageImages.首页}
        stats={[
          [String(categories.length), "开放分类"],
          [state.loading ? "..." : String(recentCount), "近期内容"],
          [String(hotTags.length), "热门标签"]
        ]}
      />
      <div className="metric-grid">
        {metricCards.map((item) => (
          <MetricCard key={item.label} item={item} />
        ))}
      </div>
      <section className="panel content-block">
        <BlockHeader icon={CalendarDays} title="最新社区内容" action="刷新" onAction={() => setReloadKey((value) => value + 1)} />
        <div className="timeline-list">
          {state.loading && <ListRow title="正在加载社区内容" meta="请稍候" />}
          {!state.loading && state.error && <ListRow title={state.error} meta="可以点击刷新重新加载" />}
          {!state.loading && !state.error && recentItems.length === 0 && <ListRow title="暂无社区内容" meta="发布第一条话题或文章后会显示在这里" />}
          {!state.loading && !state.error && recentItems.map((item) => (
            <div className="timeline-item" key={`${item.type}-${item.id}`}>
              <time>{item.time}</time>
              <div>
                <strong>{item.title}</strong>
                <p>{item.desc}</p>
              </div>
            </div>
          ))}
        </div>
      </section>
    </>
  );
}

export function CirclesPage({ categories = [], hotTags = [] }) {
  const visibleCircles = categories.length > 0 ? categories.slice(0, 6).map(categoryToCircle) : hotTags.slice(0, 6).map(tagToCircle);
  const weeklyPosts = categories.reduce((sum, item) => sum + toNumber(item.topicCount), 0);

  return (
    <>
      <PageHero
        icon={Users}
        eyebrow="圈子"
        title="按主题加入长期讨论"
        description="每个圈子都围绕稳定的技术主题沉淀帖子、资源和成员经验，适合持续共建。"
        image={pageImages.圈子}
        stats={[
          [String(visibleCircles.length), "开放圈子"],
          [String(hotTags.length), "热门标签"],
          [String(weeklyPosts), "分类内容"]
        ]}
      />
      {visibleCircles.length === 0 ? (
        <EmptyState title="暂无圈子数据" description="开放分类或热门标签后会自动生成圈子入口。" />
      ) : (
        <div className="circle-grid">
          {visibleCircles.map((circle) => (
            <CircleCard circle={circle} key={circle.name} />
          ))}
        </div>
      )}
      <section className="panel content-block">
        <BlockHeader icon={Rocket} title="圈子更新" action="查看分类" />
        <div className="compact-list">
          {visibleCircles.length === 0 && <ListRow title="暂无圈子更新" meta="发布内容并维护分类后会显示更新" />}
          {visibleCircles.slice(0, 4).map((circle) => (
            <ListRow key={circle.name} title={`${circle.name} 已开放讨论`} meta={`${circle.posts} 条内容 · ${circle.tags.slice(0, 2).join(" / ")}`} />
          ))}
        </div>
      </section>
    </>
  );
}

export function HelpPage() {
  const [state, setState] = React.useState({ items: [], total: 0, loading: true, error: "" });

  React.useEffect(() => {
    let alive = true;
    setState({ items: [], total: 0, loading: true, error: "" });
    bbsApi
      .listTopics({ limit: 8, offset: 0 })
      .then((data) => {
        if (!alive) return;
        const items = listItems(data);
        setState({ items, total: listTotal(data, items), loading: false, error: "" });
      })
      .catch((error) => {
        if (!alive) return;
        setState({ items: [], total: 0, loading: false, error: error.message || "求助内容加载失败" });
      });
    return () => {
      alive = false;
    };
  }, []);

  const questions = state.items.map(topicToQuestion);

  return (
    <>
      <PageHero
        icon={CircleHelp}
        eyebrow="求助"
        title="把问题说清楚，让答案更快到达"
        description="求助区按状态、悬赏和标签组织问题，适合追踪排障进展和沉淀可复用解法。"
        image={pageImages.求助}
        stats={[
          [state.loading ? "..." : String(state.total), "可参与讨论"],
          [state.loading ? "..." : String(questions.filter((item) => item.answers > 0).length), "已有回复"],
          [state.error ? "失败" : "实时", "数据状态"]
        ]}
      />
      {state.loading && <EmptyState title="正在加载求助内容..." />}
      {state.error && <EmptyState title="求助内容加载失败" description={state.error} />}
      {!state.loading && !state.error && questions.length === 0 && <EmptyState title="暂无求助内容" description="发布话题后会出现在这里。" />}
      {!state.loading && !state.error && questions.length > 0 && (
        <div className="question-stack">
          {questions.map((question) => (
            <QuestionCard question={question} key={question.id} />
          ))}
        </div>
      )}
      <section className="panel content-block">
        <BlockHeader icon={Users} title="最新回复线索" action="去广场" />
        <div className="compact-list">
          {questions.length === 0 && <ListRow title="暂无回复线索" meta="有评论或互动后会显示在这里" />}
          {questions.slice(0, 4).map((question) => (
            <ListRow key={question.id} title={question.title} meta={`${question.answers} 个回答 · ${question.tags.slice(0, 3).join(" / ") || "未设置标签"}`} />
          ))}
        </div>
      </section>
    </>
  );
}

export function ResourcesPage() {
  const [state, setState] = React.useState({ items: [], total: 0, loading: true, error: "" });

  React.useEffect(() => {
    let alive = true;
    setState({ items: [], total: 0, loading: true, error: "" });
    bbsApi
      .links({ limit: 12, offset: 0 })
      .then((data) => {
        if (!alive) return;
        const items = listItems(data);
        setState({ items, total: listTotal(data, items), loading: false, error: "" });
      })
      .catch((error) => {
        if (!alive) return;
        setState({ items: [], total: 0, loading: false, error: error.message || "资源加载失败" });
      });
    return () => {
      alive = false;
    };
  }, []);

  const resources = state.items.map(linkToResource);

  return (
    <>
      <PageHero
        icon={FolderOpen}
        eyebrow="资源"
        title="把可复用的经验整理成资产"
        description="模板、清单、工具脚本和学习路线集中沉淀，方便团队在下一次项目里直接复用。"
        image={pageImages.资源}
        stats={[
          [state.loading ? "..." : String(state.total), "资源入口"],
          [state.error ? "失败" : "实时", "同步状态"],
          [String(resources.filter((item) => item.url).length), "可访问"]
        ]}
      />
      {state.loading && <EmptyState title="正在加载资源..." />}
      {state.error && <EmptyState title="资源加载失败" description={state.error} />}
      {!state.loading && !state.error && resources.length === 0 && <EmptyState title="暂无资源" description="在管理端维护友情链接后会展示在这里。" />}
      {!state.loading && !state.error && resources.length > 0 && (
        <div className="resource-grid">
          {resources.map((resource) => (
            <ResourceCard resource={resource} key={resource.key} />
          ))}
        </div>
      )}
      <section className="panel content-block">
        <BlockHeader icon={Activity} title="资源活跃度" action="全部资源" />
        <div className="trend-bars">
          {resources.length === 0 && <ListRow title="暂无资源趋势" meta="资源上线后会显示活跃度" />}
          {resources.slice(0, 3).map((resource, index) => (
            <TrendBar key={resource.key} label={resource.title} value={Math.max(18, 76 - index * 14)} />
          ))}
        </div>
      </section>
    </>
  );
}

export function ShopPage() {
  const [state, setState] = React.useState({ items: [], total: 0, loading: true, error: "" });

  React.useEffect(() => {
    let alive = true;
    setState({ items: [], total: 0, loading: true, error: "" });
    bbsApi
      .tasks({ limit: 12, offset: 0 })
      .then((data) => {
        if (!alive) return;
        const items = listItems(data);
        setState({ items, total: listTotal(data, items), loading: false, error: "" });
      })
      .catch((error) => {
        if (!alive) return;
        setState({ items: [], total: 0, loading: false, error: error.message || "权益任务加载失败" });
      });
    return () => {
      alive = false;
    };
  }, []);

  const products = state.items.map(taskToProduct);
  const totalReward = state.items.reduce((sum, item) => sum + toNumber(item.reward_points ?? item.rewardPoints), 0);

  return (
    <>
      <PageHero
        icon={ShoppingBag}
        eyebrow="商城"
        title="社区服务和云资源集中采购"
        description="面向开发团队的云产品、课程、代码审查和模板服务，优先展示会员可用权益。"
        image={pageImages.商城}
        stats={[
          [state.loading ? "..." : String(state.total), "可用权益"],
          [String(totalReward), "可获积分"],
          [state.error ? "失败" : "实时", "同步状态"]
        ]}
      />
      {state.loading && <EmptyState title="正在加载权益..." />}
      {state.error && <EmptyState title="权益加载失败" description={state.error} />}
      {!state.loading && !state.error && products.length === 0 && <EmptyState title="暂无权益任务" description="在管理端维护任务后会展示在这里。" />}
      {!state.loading && !state.error && products.length > 0 && (
        <div className="shop-grid">
          {products.map((product) => (
            <ProductCard product={product} key={product.key} />
          ))}
        </div>
      )}
      <section className="panel content-block">
        <BlockHeader icon={ShieldCheck} title="权益领取流程" action="任务中心" />
        <div className="step-row">
          <StepItem number="01" title="确认需求" desc="选择云资源、课程或审查服务" />
          <StepItem number="02" title="完成任务" desc="按任务说明发布内容或完善资料" />
          <StepItem number="03" title="积分入账" desc="完成后在积分明细里查看记录" />
        </div>
      </section>
    </>
  );
}

export function MorePage({ categories = [], hotTags = [] }) {
  const [state, setState] = React.useState({ links: [], tasks: [], loading: true, error: "" });

  React.useEffect(() => {
    let alive = true;
    setState({ links: [], tasks: [], loading: true, error: "" });
    Promise.allSettled([bbsApi.links({ limit: 6, offset: 0 }), bbsApi.tasks({ limit: 6, offset: 0 })]).then(([linkResult, taskResult]) => {
      if (!alive) return;
      const links = linkResult.status === "fulfilled" ? listItems(linkResult.value) : [];
      const tasks = taskResult.status === "fulfilled" ? listItems(taskResult.value) : [];
      const failed = linkResult.status === "rejected" && taskResult.status === "rejected";
      setState({
        links,
        tasks,
        loading: false,
        error: failed ? "更多内容加载失败，请稍后重试。" : ""
      });
    });
    return () => {
      alive = false;
    };
  }, []);

  const moreItems = [
    { title: "平台分类", desc: "当前开放的内容分区", icon: Grid3X3, value: `${categories.length} 个分类` },
    { title: "热门标签", desc: "用于发现内容和圈子", icon: Star, value: `${hotTags.length} 个标签` },
    { title: "资源入口", desc: "管理端维护的公开链接", icon: FolderOpen, value: state.loading ? "同步中" : `${state.links.length} 个资源` },
    { title: "成长任务", desc: "可参与的积分任务", icon: Trophy, value: state.loading ? "同步中" : `${state.tasks.length} 个任务` }
  ];
  const rows = [
    ...state.links.map((item) => ({ key: `link-${item.id || item.key}`, title: item.title || item.name || "资源入口", meta: item.description || item.url || "资源" })),
    ...state.tasks.map((item) => ({ key: `task-${item.id || item.key}`, title: item.title || item.name || "成长任务", meta: `${toNumber(item.reward_points ?? item.rewardPoints)} 积分 · ${item.description || "完成后获得成长值"}` }))
  ];

  return (
    <>
      <PageHero
        icon={Grid3X3}
        eyebrow="更多"
        title="把低频但重要的入口收在一起"
        description="活动、公告、排行榜和工具箱集中展示，减少顶部导航负担，也保留扩展空间。"
        image={pageImages.更多}
        stats={[
          [String(categories.length), "开放分类"],
          [String(hotTags.length), "热门标签"],
          [state.loading ? "..." : String(state.links.length + state.tasks.length), "扩展入口"]
        ]}
      />
      <div className="more-grid">
        {moreItems.map((item) => (
          <MoreCard item={item} key={item.title} />
        ))}
      </div>
      <section className="panel content-block">
        <BlockHeader icon={MessageCircle} title="扩展入口" action="刷新" />
        <div className="compact-list">
          {state.loading && <ListRow title="正在加载扩展入口" meta="请稍候" />}
          {state.error && <ListRow title={state.error} meta="请稍后重试" />}
          {!state.loading && !state.error && rows.length === 0 && <ListRow title="暂无扩展入口" meta="资源或任务上线后会显示在这里" />}
          {!state.loading && !state.error && rows.map((item) => <ListRow key={item.key} title={item.title} meta={item.meta} />)}
        </div>
      </section>
    </>
  );
}

function tagToCircle(tag, index) {
  return {
    name: tag.name,
    desc: `围绕 ${tag.name} 的帖子、经验和资源沉淀`,
    members: `${Math.max(128, tag.count * 128).toLocaleString("zh-CN")}`,
    posts: String(tag.count),
    image: workspacePhotos[index % workspacePhotos.length],
    tags: [tag.name, "讨论", "精选"]
  };
}

function categoryToCircle(category, index) {
  return {
    name: category.name,
    desc: category.description || `围绕 ${category.name} 的长期讨论`,
    members: `${Math.max(128, category.topicCount * 96).toLocaleString("zh-CN")}`,
    posts: String(category.topicCount),
    image: workspacePhotos[index % workspacePhotos.length],
    tags: [category.slug || "category", "分类", "讨论"]
  };
}

function homeContentItem(item, type) {
  const timestamp = item?.published_at || item?.publishedAt || item?.created_at || item?.createdAt;
  const sortAt = toNumber(timestamp);
  const tags = item?.tags || item?.tag_names || item?.tagNames || [];
  return {
    id: item?.id,
    type,
    sortAt,
    time: timeAgoMillis(timestamp),
    title: item?.title || `未命名${type}`,
    desc: `${type} · ${tags.length > 0 ? tags.slice(0, 3).join(" / ") : "暂无标签"}`
  };
}

function topicToQuestion(topic) {
  const tags = topic?.tags || topic?.tag_names || topic?.tagNames || [];
  const answers = toNumber(topic?.comment_count ?? topic?.commentCount);
  return {
    id: topic?.id,
    title: topic?.title || "未命名话题",
    desc: topic?.body || topic?.summary || "暂无问题描述",
    status: answers > 0 ? "已回复" : "待回答",
    bounty: answers > 0 ? "已参与" : "待解答",
    answers,
    tags
  };
}

function linkToResource(link, index) {
  return {
    key: link.id || link.key || index,
    title: link.title || link.name || "资源入口",
    desc: link.description || link.url || "暂无说明",
    type: "链接",
    meta: link.url || "已启用",
    icon: FileText,
    tags: [link.key || "resource", "资源"].filter(Boolean),
    url: link.url
  };
}

function taskToProduct(task, index) {
  const reward = toNumber(task.reward_points ?? task.rewardPoints);
  return {
    key: task.id || task.key || index,
    title: task.title || task.name || "成长任务",
    desc: task.description || "完成任务后获得成长值",
    price: reward > 0 ? `+${reward} 积分` : "待配置",
    badge: "任务",
    image: workspacePhotos[index % workspacePhotos.length]
  };
}
