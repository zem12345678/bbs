import React from "react";
import { Activity, Clock3, Flame, Heart, Sparkles } from "lucide-react";

export function FeedToolbar({ loading, sort, onSortChange }) {
  const options = [
    { label: "最新", value: "latest", icon: Clock3, meta: "按发布时间" },
    { label: "活跃", value: "active", icon: Activity, meta: "按最新回复" },
    { label: "热门", value: "hot", icon: Flame, meta: "按互动热度" },
    { label: "关注", value: "follow", icon: Heart, meta: "只看已关注作者" }
  ];
  const title = { active: "活跃讨论", hot: "热门动态", follow: "关注动态" }[sort] || "最新动态";
  const subtitle =
    { active: "按最新回复和讨论活跃度排序", hot: "按互动热度排序", follow: "只看已关注作者" }[sort] || "按发布时间排序";

  return (
    <header className="feed-toolbar panel">
      <div>
        <strong>{title}</strong>
        <span>{loading ? "同步中" : subtitle}</span>
      </div>
      <div className="feed-switch" role="tablist" aria-label="动态排序">
        {options.map(({ label, value, icon: Icon, meta }) => (
          <button
            aria-pressed={sort === value}
            className={sort === value ? "is-active" : ""}
            key={value}
            type="button"
            onClick={() => onSortChange(value)}
            title={meta}
          >
            <Icon size={17} aria-hidden="true" />
            {label}
          </button>
        ))}
      </div>
    </header>
  );
}

export function FeedStatus({ text, actionLabel, onAction }) {
  return (
    <div className="feed-status panel">
      <Sparkles size={18} aria-hidden="true" />
      <span>{text}</span>
      {actionLabel && onAction && (
        <button type="button" onClick={onAction}>
          {actionLabel}
        </button>
      )}
    </div>
  );
}

export function SearchResultBar({ query, total, loading, error, onClear }) {
  return (
    <div className="search-result-bar panel">
      <div>
        <strong>{loading ? "正在搜索" : `搜索：${query}`}</strong>
        <span>{error || (loading ? "正在查询搜索服务..." : `找到 ${total} 条内容`)}</span>
      </div>
      <button type="button" onClick={onClear}>
        返回广场
      </button>
    </div>
  );
}
