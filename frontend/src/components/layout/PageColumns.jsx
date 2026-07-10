import React from "react";
import {
  Activity,
  BadgeCheck,
  BookOpen,
  CalendarDays,
  CheckCircle2,
  CircleHelp,
  Clock3,
  Code2,
  Crown,
  FileText,
  Flame,
  Gift,
  Grid3X3,
  Hash,
  Heart,
  Home,
  Layers3,
  MessageCircle,
  Package,
  ShieldCheck,
  ShoppingBag,
  Sparkles,
  Star,
  Trophy,
  Users,
  Wrench
} from "lucide-react";

const sideNav = [
  { label: "推荐", icon: Sparkles },
  { label: "最新", icon: Clock3, value: "latest" },
  { label: "活跃", icon: Activity, value: "active" },
  { label: "热门", icon: Flame, value: "hot" },
  { label: "关注", icon: Heart, value: "follow" }
];

const pageSideNav = {
  首页: [
    { label: "概览", icon: Home, active: true },
    { label: "日程", icon: CalendarDays },
    { label: "任务", icon: CheckCircle2 },
    { label: "关注", icon: Heart }
  ],
  圈子: [
    { label: "我的圈", icon: Users, active: true },
    { label: "官方", icon: BadgeCheck },
    { label: "技术栈", icon: Code2 },
    { label: "产品", icon: Layers3 }
  ],
  求助: [
    { label: "待回答", icon: CircleHelp, active: true },
    { label: "高悬赏", icon: Trophy },
    { label: "已解决", icon: CheckCircle2 },
    { label: "关注", icon: Heart }
  ],
  资源: [
    { label: "精选", icon: Sparkles, active: true },
    { label: "模板", icon: FileText },
    { label: "工具", icon: Wrench },
    { label: "文档", icon: BookOpen }
  ],
  商城: [
    { label: "云产品", icon: ShoppingBag, active: true },
    { label: "插件", icon: Package },
    { label: "课程", icon: BookOpen },
    { label: "服务", icon: ShieldCheck }
  ],
  会员: [
    { label: "权益", icon: Crown, active: true },
    { label: "成长值", icon: Activity },
    { label: "勋章", icon: Star },
    { label: "礼包", icon: Gift }
  ],
  更多: [
    { label: "活动", icon: CalendarDays, active: true },
    { label: "公告", icon: MessageCircle },
    { label: "排行", icon: Trophy },
    { label: "工具", icon: Grid3X3 }
  ]
};

export function LeftColumn({ activeCategoryId = 0, activePage, categories = [], feedSort, hotTags = [], onCategoryChange, onFeedSortChange }) {
  const links = activePage === "广场" ? sideNav : pageSideNav[activePage];
  const visibleTopics = categories.length > 0 ? categories : hotTags.length > 0 ? hotTags.map((tag) => ({ name: tag.name })) : [];
  const categoryMode = categories.length > 0;

  return (
    <aside className="left-column" aria-label="侧边导航">
      <section className="panel nav-panel">
        {links.map(({ label, icon: Icon, active, value }) => (
          <button
            className={`side-link ${active || value === feedSort ? "is-active" : ""}`}
            key={label}
            type="button"
            onClick={value ? () => onFeedSortChange?.(value) : undefined}
          >
            <Icon size={23} aria-hidden="true" />
            {label}
          </button>
        ))}
      </section>
      <section className="panel topic-panel">
        <h2>{categoryMode ? "社区分类" : activePage === "广场" ? "热门话题" : "相关话题"}</h2>
        <ul>
          {activePage === "广场" && categoryMode && (
            <li>
              <button className={activeCategoryId ? "" : "is-active"} type="button" onClick={() => onCategoryChange?.(0)}>
                <Hash size={14} aria-hidden="true" />
                全部分类
              </button>
            </li>
          )}
          {visibleTopics.length === 0 && (
            <li>
              <span className="topic-empty">暂无可用分类</span>
            </li>
          )}
          {visibleTopics.map((topic) => (
            <li key={topic.id || topic.name}>
              <button
                className={topic.id && activeCategoryId === topic.id ? "is-active" : ""}
                type="button"
                onClick={topic.id ? () => onCategoryChange?.(topic.id) : undefined}
              >
                <Hash size={14} aria-hidden="true" />
                {topic.name}
              </button>
            </li>
          ))}
        </ul>
      </section>
    </aside>
  );
}

export function RightColumn({ activePage, categories = [], hotTags = [] }) {
  const visibleCategories = categories.slice(0, 6);
  const visibleTags = hotTags.slice(0, 8);

  return (
    <aside className="right-column" aria-label="社区数据">
      <section className="panel side-summary-card">
        <h2>{activePage || "社区"}数据</h2>
        <p>浏览当前开放的内容分类和热门标签，快速进入相关讨论。</p>
        <div className="side-metrics">
          <span>
            <strong>{categories.length}</strong>
            分类
          </span>
          <span>
            <strong>{hotTags.length}</strong>
            标签
          </span>
        </div>
      </section>
      <section className="panel side-list-card">
        <h2>社区分类</h2>
        <div className="side-data-list">
          {visibleCategories.length === 0 && <p className="side-empty">暂无分类数据</p>}
          {visibleCategories.map((category) => (
            <div className="side-data-row" key={category.id || category.name}>
              <Hash size={15} aria-hidden="true" />
              <strong>{category.name}</strong>
              <span>{category.topicCountKnown ? `${category.topicCount} 话题` : "已启用"}</span>
            </div>
          ))}
        </div>
      </section>
      <section className="panel side-list-card">
        <h2>热门标签</h2>
        <div className="side-tag-list">
          {visibleTags.length === 0 && <p className="side-empty">暂无标签数据</p>}
          {visibleTags.map((tag) => (
            <span key={tag.id || tag.name}>
              <Hash size={13} aria-hidden="true" />
              {tag.name}
              <em>{tag.count || 0}</em>
            </span>
          ))}
        </div>
      </section>
    </aside>
  );
}
