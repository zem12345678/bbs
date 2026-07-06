import React from "react";
import { Clock3, Flame, Sparkles } from "lucide-react";

export function FeedToolbar({ loading, sort, onSortChange }) {
  const options = [
    { label: "最新", value: "latest", icon: Clock3, meta: "按发布时间" },
    { label: "热门", value: "hot", icon: Flame, meta: "按互动热度" }
  ];

  return (
    <header className="feed-toolbar panel">
      <div>
        <strong>{sort === "hot" ? "热门动态" : "最新动态"}</strong>
        <span>{loading ? "同步中" : sort === "hot" ? "按互动热度排序" : "按发布时间排序"}</span>
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
