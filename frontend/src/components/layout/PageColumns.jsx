import React from "react";
import { Link, useLocation, useNavigate } from "react-router-dom";
import {
  Bell,
  Bird,
  BookOpen,
  Bookmark,
  CircleHelp,
  Crown,
  Hash,
  Heart,
  Home,
  LayoutGrid,
  LogIn,
  MessagesSquare,
  Moon,
  Search,
  Settings,
  ShoppingBag,
  Sun,
  UserRound,
  Users,
  Wallet
} from "lucide-react";
import { readStoredAuth } from "../../lib/authStorage";
import { compactNumber, toNumber } from "../../lib/formatters";
import { userAvatar, userDisplayName } from "../../lib/postMappers";
import { defaultSiteConfig } from "../../lib/siteConfig";
import { currentTheme, subscribeTheme, toggleTheme } from "../../lib/theme";

/**
 * 左侧竖向主导航（参考社区客户端布局）：
 * 内容板块 + 个人功能全部入口统一竖排。
 */
const mainNavItems = [
  { label: "广场", path: "/plaza", icon: Home, patterns: ["/", "/plaza", "/search"] },
  { label: "话题", path: "/topics", icon: Hash, patterns: ["/topics", "/topic", "/articles", "/article"] },
  { label: "圈子", path: "/circles", icon: Users, patterns: ["/circles"] },
  { label: "聊天室", path: "/chat", icon: MessagesSquare, patterns: ["/chat", "/room"] },
  { label: "求助", path: "/help", icon: CircleHelp, patterns: ["/help", "/question"] },
  { label: "资源", path: "/resources", icon: BookOpen, patterns: ["/resources"] },
  { label: "商城", path: "/shop", icon: ShoppingBag, patterns: ["/shop"] },
  { label: "会员", path: "/member", icon: Crown, patterns: ["/member"] },
  { divider: true, key: "divider-personal" },
  { label: "主页", path: "/user/profile", icon: UserRound, authRequired: true, patterns: ["/user/profile", "/u"] },
  { label: "提醒", path: "/user/messages", icon: Bell, authRequired: true, patterns: ["/user/messages"] },
  { label: "收藏", path: "/user/favorites", icon: Bookmark, authRequired: true, patterns: ["/user/favorites"] },
  { label: "点赞", path: "/user/likes", icon: Heart, authRequired: true, patterns: ["/user/likes"] },
  { label: "钱包", path: "/user/scores", icon: Wallet, authRequired: true, patterns: ["/user/scores"] },
  { label: "设置", path: "/dashboard", icon: Settings, authRequired: true, patterns: ["/dashboard"] },
  { label: "更多", path: "/more", icon: LayoutGrid, patterns: ["/more", "/links", "/tasks", "/about", "/install", "/redirect"] }
];

/** 右栏静态展示数据（不依赖后端接口） */
const hotChatChannels = [
  { name: "综合闲聊", online: 1286 },
  { name: "前端技术交流", online: 864 },
  { name: "后端架构师", online: 623 },
  { name: "UI 设计美学", online: 458 },
  { name: "同城线下聚会", online: 205 }
];

const hotResources = [
  { title: "Vue 3 + Vite 实战教程", type: "教程" },
  { title: "React 源码深度解析", type: "专栏" },
  { title: "Go 微服务开发指南", type: "文档" },
  { title: "社区 UI 设计规范", type: "规范" },
  { title: "Markdown 写作模板包", type: "模板" }
];

function isItemActive(item, pathname) {
  return item.patterns.some((pattern) => {
    if (pattern === "/") {
      return pathname === "/";
    }
    return pathname === pattern || pathname.startsWith(`${pattern}/`);
  });
}

function formatHotCount(value) {
  const count = toNumber(value);
  if (count >= 1000) {
    return `${(count / 1000).toFixed(1)}k`;
  }
  return String(count);
}

export function LeftColumn({ activeCategoryId = 0, activePage, categories = [], hotTags = [], onCategoryChange, siteConfig }) {
  const location = useLocation();
  const navigate = useNavigate();
  const auth = readStoredAuth();
  const user = auth?.user;
  const categoryMode = activePage === "广场" && categories.length > 0;
  const brand = siteConfig?.siteName ? siteConfig : defaultSiteConfig;
  const [theme, setTheme] = React.useState(currentTheme);

  React.useEffect(() => subscribeTheme(setTheme), []);

  function handleNavClick(item) {
    if (item.authRequired && !auth?.accessToken) {
      navigate("/user/signin");
      return;
    }
    navigate(item.path);
  }

  return (
    <aside className="left-column" aria-label="侧边导航">
      <div className="nav-brand-row">
        <Link className="nav-brand" aria-label={brand.siteName} to="/plaza">
          {brand.logoUrl ? <img alt="" src={brand.logoUrl} /> : <Bird size={34} aria-hidden="true" strokeWidth={1.8} />}
          <span>{brand.siteName}</span>
        </Link>
        <button
          aria-label={theme === "dark" ? "切换到日间模式" : "切换到夜间模式"}
          className="theme-toggle"
          title={theme === "dark" ? "切换到日间模式" : "切换到夜间模式"}
          type="button"
          onClick={() => setTheme(toggleTheme())}
        >
          {theme === "dark" ? <Sun size={19} aria-hidden="true" /> : <Moon size={19} aria-hidden="true" />}
        </button>
      </div>
      <nav className="panel main-nav-panel" aria-label="功能导航">
        {mainNavItems.map((item) => {
          if (item.divider) {
            return <span aria-hidden="true" className="main-nav-divider" key={item.key} />;
          }
          const Icon = item.icon;
          const active = isItemActive(item, location.pathname);
          return (
            <button
              aria-current={active ? "page" : undefined}
              className={`main-nav-link ${active ? "is-active" : ""}`}
              key={item.label}
              type="button"
              onClick={() => handleNavClick(item)}
            >
              <Icon size={22} aria-hidden="true" strokeWidth={active ? 2.4 : 2} />
              <span>{item.label}</span>
            </button>
          );
        })}
      </nav>
      {categoryMode && (
        <section className="panel topic-panel">
          <h2>社区分类</h2>
          <ul>
            <li>
              <button className={activeCategoryId ? "" : "is-active"} type="button" onClick={() => onCategoryChange?.(0)}>
                <Hash size={14} aria-hidden="true" />
                全部分类
              </button>
            </li>
            {categories.map((topic) => (
              <li key={topic.id || topic.name}>
                <button
                  className={topic.id && activeCategoryId === topic.id ? "is-active" : ""}
                  type="button"
                  onClick={() => onCategoryChange?.(topic.id)}
                >
                  <Hash size={14} aria-hidden="true" />
                  {topic.name}
                </button>
              </li>
            ))}
          </ul>
        </section>
      )}
      <div className="nav-user">
        {user ? (
          <Link className="nav-user-card" title={userDisplayName(user)} to="/user/profile">
            <img alt="" src={userAvatar(user)} />
            <strong className="nav-user-name">@{user.username || user.id}</strong>
          </Link>
        ) : (
          <Link className="nav-login-btn" to="/user/signin">
            <LogIn size={18} aria-hidden="true" />
            登录
          </Link>
        )}
      </div>    </aside>
  );
}

export function RightColumn({ categories = [], hotTags = [] }) {
  const navigate = useNavigate();
  const [query, setQuery] = React.useState("");
  const visibleCategories = categories.slice(0, 6);
  const visibleTags = hotTags.slice(0, 8);

  function submitSearch(event) {
    event.preventDefault();
    const keyword = query.trim();
    navigate(keyword ? `/search?q=${encodeURIComponent(keyword)}` : "/search");
  }

  return (
    <aside className="right-column" aria-label="侧边栏">
      <form className="side-search" role="search" onSubmit={submitSearch}>
        <Search size={17} aria-hidden="true" />
        <input aria-label="搜索社区内容" placeholder="搜一搜..." value={query} onChange={(event) => setQuery(event.target.value)} />
      </form>
      <section className="panel hot-topics-card">
        <h2>热门话题</h2>
        <ul>
          {visibleTags.length === 0 && <li className="side-empty">暂无话题数据</li>}
          {visibleTags.map((tag) => (
            <li key={tag.id || tag.name}>
              <Link to={`/topics/tag/${encodeURIComponent(tag.name)}`}>
                <span className="hot-topic-name">#{tag.name}</span>
                <span className="hot-topic-count">{formatHotCount(tag.count)}</span>
              </Link>
            </li>
          ))}
        </ul>
      </section>
      <section className="panel hot-topics-card">
        <h2>热门聊天频道</h2>
        <ul>
          {hotChatChannels.map((channel) => (
            <li key={channel.name}>
              <Link to="/chat">
                <span className="hot-topic-name">
                  <MessagesSquare size={14} aria-hidden="true" />
                  {channel.name}
                </span>
                <span className="hot-topic-count">{formatHotCount(channel.online)} 在线</span>
              </Link>
            </li>
          ))}
        </ul>
      </section>
      <section className="panel hot-topics-card">
        <h2>热门资源</h2>
        <ul>
          {hotResources.map((resource) => (
            <li key={resource.title}>
              <Link to="/resources">
                <span className="hot-resource-name">{resource.title}</span>
                <span className="hot-resource-type">{resource.type}</span>
              </Link>
            </li>
          ))}
        </ul>
      </section>
      <section className="panel side-list-card">
        <h2>社区分类</h2>
        <div className="side-data-list">
          {visibleCategories.length === 0 && <p className="side-empty">暂无分类数据</p>}
          {visibleCategories.map((category) => (
            <Link className="side-data-row" key={category.id || category.name} to={`/topics/category/${category.id}`}>
              <Hash size={15} aria-hidden="true" />
              <strong>{category.name}</strong>
              <span>{category.topicCountKnown ? `${compactNumber(category.topicCount)} 话题` : "已启用"}</span>
            </Link>
          ))}
        </div>
      </section>
    </aside>
  );
}
