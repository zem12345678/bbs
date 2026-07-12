import React from "react";

import { MALL_NOTIFICATION_GROUPS, NOTIFICATION_FILTERS } from "../../lib/notificationTargets";

export default function MessageFilterPanel({ filter = "all", noun = "消息", summary, onFilterChange }) {
  const mallSummary = summary?.mall || { total: 0, unread: 0, groups: {} };
  const total = summary?.total || 0;

  return (
    <div className="message-filter-panel panel">
      <div className="message-filter-tabs" role="tablist" aria-label={`${noun}类型筛选`}>
        {NOTIFICATION_FILTERS.map((item) => {
          const count = item.value === "mall" ? mallSummary.total : total;
          const unread = item.value === "mall" ? mallSummary.unread : summary?.unread || 0;
          return (
            <button
              aria-selected={filter === item.value}
              className={filter === item.value ? "is-active" : ""}
              disabled={count === 0}
              key={item.value}
              role="tab"
              type="button"
              onClick={() => onFilterChange(item.value)}
            >
              {item.label}
              <span>{count}</span>
              {unread > 0 && <small>{unread} 未读</small>}
            </button>
          );
        })}
      </div>
      {mallSummary.total > 0 && (
        <div className="message-mall-summary" aria-label="商城通知分组">
          {MALL_NOTIFICATION_GROUPS.map((group) => {
            const groupSummary = mallSummary.groups?.[group.value] || { total: 0, unread: 0 };
            return (
              <span key={group.value}>
                <strong>{groupSummary.total}</strong>
                {group.label}
                <small>{groupSummary.unread > 0 ? `${groupSummary.unread} 未读` : group.description}</small>
              </span>
            );
          })}
        </div>
      )}
    </div>
  );
}
