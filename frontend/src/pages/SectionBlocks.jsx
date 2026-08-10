import React from "react";
import { Link } from "react-router-dom";
import { Archive, ChevronDown, ExternalLink, Hash, Heart, MessageCircle, Sparkles, Star, UserCheck, UserPlus, Users, Zap } from "lucide-react";
import { safeExternalURL } from "../lib/externalLinks.js";

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

export function CircleCard({ channel, pendingAction = "", onFavorite, onFollow }) {
  const detailPath = `/circles/${encodeURIComponent(channel.id)}`;
  const canFollow = !channel.is_archived || channel.is_following;
  const canFavorite = !channel.is_archived || channel.is_favorited;
  return (
    <article className="circle-card channel-card panel" style={{ "--channel-color": channel.color }}>
      <span className="channel-card-color" aria-hidden="true" />
      <div className="circle-body">
        <div className="circle-card-heading">
          <Hash size={19} aria-hidden="true" />
          <h2><Link to={detailPath}>{channel.name || "未命名圈子"}</Link></h2>
          {channel.is_archived ? (
            <span className="channel-state-badge is-archived"><Archive size={13} aria-hidden="true" />已归档</span>
          ) : channel.is_featured ? (
            <span className="channel-state-badge"><Sparkles size={13} aria-hidden="true" />精选</span>
          ) : channel.is_following && <span className="channel-state-badge"><UserCheck size={13} aria-hidden="true" />已关注</span>}
        </div>
        <p>{channel.description || "这个圈子暂时还没有简介。"}</p>
        <div className="channel-card-stats" aria-label="圈子数据">
          <span><MessageCircle size={14} aria-hidden="true" />{channel.topics_count} 个主题</span>
          <span><Users size={14} aria-hidden="true" />{channel.followers_count} 人关注</span>
        </div>
        <footer>
          <div className="channel-card-actions">
            {canFollow && (
              <button
                aria-pressed={channel.is_following}
                disabled={pendingAction === "follow"}
                title={channel.is_following ? "取消关注" : "关注圈子"}
                type="button"
                onClick={() => onFollow?.(channel)}
              >
                {channel.is_following ? <UserCheck size={15} aria-hidden="true" /> : <UserPlus size={15} aria-hidden="true" />}
                {pendingAction === "follow" ? "处理中" : channel.is_following ? "已关注" : "关注"}
              </button>
            )}
            {canFavorite && (
              <button
                aria-pressed={channel.is_favorited}
                className={channel.is_favorited ? "is-active" : ""}
                disabled={pendingAction === "favorite"}
                title={channel.is_favorited ? "取消收藏" : "收藏圈子"}
                type="button"
                onClick={() => onFavorite?.(channel)}
              >
                <Star fill={channel.is_favorited ? "currentColor" : "none"} size={15} aria-hidden="true" />
                {pendingAction === "favorite" ? "处理中" : channel.is_favorited ? "已收藏" : "收藏"}
              </button>
            )}
          </div>
          <Link className="circle-card-link" to={detailPath}>进入圈子</Link>
        </footer>
      </div>
    </article>
  );
}

export function QuestionCard({ question }) {
  const detailPath = question.path || (question.id ? `/topic/${question.id}` : "/help");
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

export function ResourceCard({ resource, onVisit }) {
  const Icon = resource.icon;
  const href = safeExternalURL(resource.url);

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
          <span className="resource-tag" key={tag}>
            <Zap size={13} aria-hidden="true" />
            {tag}
          </span>
        ))}
      </div>
      {href && (
        <a className="resource-card-link" href={href} target="_blank" rel="noreferrer noopener" onClick={onVisit}>
          访问资源
          <ExternalLink size={16} aria-hidden="true" />
        </a>
      )}
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

export function ListRow({ actionDisabled = false, actionHref, actionIcon: ActionIcon, actionLabel = "查看", onAction, title, meta }) {
  const href = safeExternalURL(actionHref);

  return (
    <div className="list-row">
      <span />
      <div>
        <strong>{title}</strong>
        <p>{meta}</p>
      </div>
      {href && !actionDisabled ? (
        <a aria-label={actionLabel} className="list-row-action" href={href} target="_blank" rel="noreferrer noopener" title={actionLabel}>
          {ActionIcon ? <ActionIcon aria-hidden="true" size={16} /> : actionLabel}
        </a>
      ) : onAction ? (
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
