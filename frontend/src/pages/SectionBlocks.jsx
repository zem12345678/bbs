import React from "react";
import { Link } from "react-router-dom";
import { ChevronDown, Hash, Heart, Zap } from "lucide-react";

export function PageHero({ icon: Icon, eyebrow, title, description, image, stats }) {
  return (
    <section className="panel section-hero">
      <div className="section-hero-copy">
        <span className="eyebrow">
          <Icon size={18} aria-hidden="true" />
          {eyebrow}
        </span>
        <h1>{title}</h1>
        <p>{description}</p>
        <div className="hero-stats">
          {stats.map(([value, label]) => (
            <span key={label}>
              <strong>{value}</strong>
              {label}
            </span>
          ))}
        </div>
      </div>
      <img src={image} alt="" />
    </section>
  );
}

export function MetricCard({ item }) {
  const Icon = item.icon;

  return (
    <article className="metric-card panel">
      <span>
        <Icon size={22} aria-hidden="true" />
      </span>
      <div>
        <strong>{item.value}</strong>
        <p>{item.label}</p>
        <em>{item.meta}</em>
      </div>
    </article>
  );
}

export function BlockHeader({ icon: Icon, title, action, onAction }) {
  return (
    <header className="block-head">
      <h2>
        <Icon size={20} aria-hidden="true" />
        {title}
      </h2>
      {action && (onAction ? <button type="button" onClick={onAction}>{action}</button> : <span className="block-head-status">{action}</span>)}
    </header>
  );
}

export function CircleCard({ category }) {
  const categoryId = category?.id;
  const topicCount = category?.topicCountKnown ? category.topicCount : null;
  const topicPath = categoryId ? `/topics/category/${categoryId}` : "/topics";
  return (
    <article className="circle-card panel">
      <div className="circle-body">
        <div className="circle-card-heading">
          <Hash size={19} aria-hidden="true" />
          <h2><Link to={topicPath}>{category?.name || "未命名分类"}</Link></h2>
        </div>
        <p>{category?.description || "暂无分类说明"}</p>
        <footer>
          <span>{topicCount === null ? "话题数统计中" : `${topicCount} 条话题`}</span>
          <Link className="circle-card-link" to={topicPath}>查看话题</Link>
        </footer>
      </div>
    </article>
  );
}

export function QuestionCard({ question }) {
  const detailPath = question.id ? `/topic/${question.id}` : "/help";
  return (
    <article className="question-card panel">
      <div>
        <span className={`status-badge ${question.status === "已解决" ? "is-done" : ""}`}>{question.status}</span>
        <h2>{question.title}</h2>
        <p>{question.desc}</p>
        <div className="tag-row">
          {question.tags.map((tag) => (
            <Link to={`/topics/tag/${encodeURIComponent(tag)}`} key={tag}>
              <Hash size={13} aria-hidden="true" />
              {tag}
            </Link>
          ))}
        </div>
      </div>
      <aside>
        <strong>{question.bounty}</strong>
        <span>{question.answers} 个回答</span>
        <Link className="question-card-link" to={detailPath}>查看</Link>
      </aside>
    </article>
  );
}

export function ResourceCard({ resource }) {
  const Icon = resource.icon;

  return (
    <article className="resource-card panel">
      <span className="resource-icon">
        <Icon size={24} aria-hidden="true" />
      </span>
      <div>
        <small>{resource.type}</small>
        <h2>{resource.title}</h2>
        <p>{resource.desc}</p>
        <em>{resource.meta}</em>
      </div>
      <div className="tag-row">
        {resource.tags.map((tag) => (
          <a href="#" key={tag}>
            <Zap size={13} aria-hidden="true" />
            {tag}
          </a>
        ))}
      </div>
    </article>
  );
}

export function ProductCard({
  product,
  actionLabel = "查看",
  actionDisabled = false,
  detailLabel = "",
  favoriteActive = false,
  favoriteDisabled = false,
  onAction,
  onDetail,
  onFavorite
}) {
  return (
    <article className="product-card panel">
      <img src={product.image} alt="" />
      <div>
        <div className="product-card-topline">
          <span>{product.badge}</span>
          {onFavorite && (
            <button
              type="button"
              className={`product-favorite-button ${favoriteActive ? "is-active" : ""}`.trim()}
              aria-label={favoriteActive ? "取消收藏" : "收藏商品"}
              title={favoriteActive ? "取消收藏" : "收藏商品"}
              disabled={favoriteDisabled}
              onClick={() => onFavorite?.(product)}
            >
              <Heart size={16} fill={favoriteActive ? "currentColor" : "none"} aria-hidden="true" />
            </button>
          )}
        </div>
        <h2>{product.title}</h2>
        <p>{product.desc}</p>
        <footer>
          <strong>{product.price}</strong>
          <div className="product-card-actions">
            {detailLabel && (
              <button type="button" onClick={() => onDetail?.(product)}>
                {detailLabel}
              </button>
            )}
            <button type="button" disabled={actionDisabled} onClick={() => onAction?.(product)}>
              {actionLabel}
            </button>
          </div>
        </footer>
      </div>
    </article>
  );
}

export function BenefitCard({ benefit }) {
  const Icon = benefit.icon;

  return (
    <article className="benefit-card panel">
      <span>
        <Icon size={24} aria-hidden="true" />
      </span>
      <h2>{benefit.title}</h2>
      <p>{benefit.desc}</p>
    </article>
  );
}

export function MoreCard({ item }) {
  const Icon = item.icon;

  return (
    <article className="more-card panel">
      <span>
        <Icon size={24} aria-hidden="true" />
      </span>
      <div>
        <strong>{item.value}</strong>
        <h2>{item.title}</h2>
        <p>{item.desc}</p>
      </div>
    </article>
  );
}

export function ListRow({ actionDisabled = false, actionIcon: ActionIcon, actionLabel = "查看", onAction, title, meta }) {
  return (
    <div className="list-row">
      <span />
      <div>
        <strong>{title}</strong>
        <p>{meta}</p>
      </div>
      {onAction ? (
        <button aria-label={actionLabel} disabled={actionDisabled} title={actionLabel} type="button" onClick={onAction}>
          {ActionIcon ? <ActionIcon aria-hidden="true" size={16} /> : actionLabel}
        </button>
      ) : (
        <ChevronDown size={18} aria-hidden="true" />
      )}
    </div>
  );
}

export function TrendBar({ label, value }) {
  return (
    <div className="trend-bar">
      <div>
        <span>{label}</span>
        <strong>{value}%</strong>
      </div>
      <i>
        <b style={{ width: `${value}%` }} />
      </i>
    </div>
  );
}

export function StepItem({ number, title, desc }) {
  return (
    <div className="step-item">
      <span>{number}</span>
      <strong>{title}</strong>
      <p>{desc}</p>
    </div>
  );
}
