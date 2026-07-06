import React from "react";

export function RouteHeader({ icon: Icon, eyebrow, title, description, actions }) {
  return (
    <header className="route-head panel">
      <div>
        {eyebrow && (
          <span className="eyebrow">
            {Icon && <Icon size={18} aria-hidden="true" />}
            {eyebrow}
          </span>
        )}
        <h1>{title}</h1>
        {description && <p>{description}</p>}
      </div>
      {actions && <div className="route-actions">{actions}</div>}
    </header>
  );
}

export function EmptyState({ title, description, action }) {
  return (
    <section className="empty-state panel">
      <strong>{title}</strong>
      {description && <p>{description}</p>}
      {action}
    </section>
  );
}

export function PillTabs({ items, value, onChange, label }) {
  return (
    <div className="pill-tabs panel" role="tablist" aria-label={label}>
      {items.map((item) => {
        const Icon = item.icon;
        return (
          <button
            aria-pressed={value === item.value}
            className={value === item.value ? "is-active" : ""}
            key={item.value}
            type="button"
            onClick={() => onChange(item.value)}
          >
            {Icon && <Icon size={17} aria-hidden="true" />}
            {item.label}
          </button>
        );
      })}
    </div>
  );
}

export function DataRows({ rows }) {
  return (
    <div className="data-rows">
      {rows.map((row) => (
        <article className="data-row panel" key={row.key || row.title}>
          <div>
            <strong>{row.title}</strong>
            {row.description && <p>{row.description}</p>}
          </div>
          {row.meta && <span>{row.meta}</span>}
        </article>
      ))}
    </div>
  );
}
