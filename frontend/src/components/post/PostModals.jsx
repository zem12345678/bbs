import React from "react";
import { Link } from "react-router-dom";
import { Heart, MessageSquare, Star, X, Zap } from "lucide-react";
import Avatar from "../Avatar.jsx";
import { sameId, timeAgo, toNumber } from "../../lib/formatters";

export function ArticleDetailModal({
  auth,
  canDeleteComment,
  commentCount,
  commentText,
  comments,
  commentsLoading,
  deletingCommentId,
  error,
  favorites,
  favorited,
  liked,
  likes,
  onClose,
  onCommentTextChange,
  onDeleteComment,
  onFavorite,
  onLike,
  onSubmitComment,
  people,
  post,
  renderComment
}) {
  return (
    <div className="detail-overlay" role="presentation" onClick={onClose}>
      <article className="detail-modal panel" role="dialog" aria-modal="true" aria-label="帖子详情" onClick={(event) => event.stopPropagation()}>
        <button className="detail-close" type="button" aria-label="关闭详情" onClick={onClose}>
          <X size={22} aria-hidden="true" />
        </button>
        <header className="detail-header">
          <Avatar person={post.author} />
          <div>
            <div className="name-row">
              <h3>{post.author.name}</h3>
              <span className="level">{post.level}</span>
            </div>
            <p>
              {post.author.role} <span>{post.time}</span>
            </p>
          </div>
        </header>
        {post.title && <h1>{post.title}</h1>}
        <div className="detail-body">
          {(post.text || "").split(/\n+/).map((paragraph, index) => (
            <p key={`${paragraph}-${index}`}>{paragraph}</p>
          ))}
        </div>
        {post.images && (
          <div className="detail-images" aria-label="帖子配图">
            {post.images.map((src) => (
              <img src={src} alt="" key={src} />
            ))}
          </div>
        )}
        <div className="tag-row">
          {post.tags.map((tag) => (
            <Link to={`${post.kind === "topic" ? "/topics" : "/articles"}/tag/${encodeURIComponent(tag)}`} key={tag} onClick={onClose}>
              <Zap size={13} aria-hidden="true" />
              {tag}
            </Link>
          ))}
        </div>
        <footer className="detail-actions action-row">
          <button type="button" className={liked ? "liked" : ""} onClick={onLike}>
            <Heart size={20} aria-hidden="true" />
            {likes}
          </button>
          <button type="button" className={favorited ? "liked" : ""} onClick={onFavorite}>
            <Star size={20} aria-hidden="true" />
            {favorites || "收藏"}
          </button>
          <span>
            <MessageSquare size={20} aria-hidden="true" />
            {commentCount || 0} 条评论
          </span>
        </footer>
        {error && <p className="form-error post-error">{error}</p>}
        <section className="comment-panel detail-comments" aria-label="详情评论">
          {commentsLoading && <p className="comment-empty">正在加载详情...</p>}
          {!commentsLoading && comments.length === 0 && <p className="comment-empty">暂无评论，来发第一条。</p>}
          {comments.map((comment) =>
            renderComment ? (
              renderComment(comment)
            ) : (
              <div className="comment-item" key={comment.id}>
                <Avatar
                  person={{
                    name: `用户 #${comment.author_id || comment.authorId || "?"}`,
                    handle: `u${comment.author_id || comment.authorId || "unknown"}`,
                    avatar: people[toNumber(comment.author_id || comment.authorId) % people.length].avatar
                  }}
                  small
                />
                <div>
                  <div className="comment-head">
                    <strong>用户 #{comment.author_id || comment.authorId || "?"}</strong>
                    {canDeleteComment?.(comment) && (
                      <button type="button" onClick={() => onDeleteComment?.(comment.id)} disabled={sameId(deletingCommentId, comment.id)}>
                        {sameId(deletingCommentId, comment.id) ? "删除中" : "删除"}
                      </button>
                    )}
                  </div>
                  <p>{comment.content}</p>
                  <span>{timeAgo(comment.created_at || comment.createdAt)}</span>
                </div>
              </div>
            )
          )}
          <form className="comment-form" onSubmit={onSubmitComment}>
            <input
              placeholder={auth ? "写下你的评论" : "登录后参与评论"}
              value={commentText}
              onChange={(event) => onCommentTextChange(event.target.value)}
              disabled={!auth}
            />
            <button type="submit" disabled={!auth || !commentText.trim()}>
              发送
            </button>
          </form>
        </section>
      </article>
    </div>
  );
}

export function AuthorProfileModal({ auth, error, followBusy, following, loading, onClose, onToggleFollow, person, self }) {
  return (
    <div className="detail-overlay" role="presentation" onClick={onClose}>
      <aside
        className="author-modal panel"
        role="dialog"
        aria-modal="true"
        aria-label={`${person.name} 的资料`}
        onClick={(event) => event.stopPropagation()}
      >
        <button className="detail-close" type="button" aria-label="关闭资料" onClick={onClose}>
          <X size={22} aria-hidden="true" />
        </button>
        <div className="author-cover" />
        <div className="author-profile-main">
          <Avatar person={person} />
          <div>
            <h2>{person.name}</h2>
            <p>@{person.handle}</p>
          </div>
        </div>
        <p className="author-bio">{loading ? "正在加载作者资料..." : person.bio}</p>
        <div className="author-stats">
          <span>
            粉丝
            <strong>{toNumber(person.followerCount)}</strong>
          </span>
          <span>
            关注
            <strong>{toNumber(person.followingCount)}</strong>
          </span>
          <span>
            状态
            <strong>{self ? "本人" : following ? "已关注" : "未关注"}</strong>
          </span>
        </div>
        {error && <p className="form-error">{error}</p>}
        <div className="author-actions">
          {person.id && (
            <Link className="author-home-link" to={`/user/${person.id}`} onClick={onClose}>
              查看主页
            </Link>
          )}
          {!self && (
            <button className={`follow-action ${following ? "is-following" : ""}`} type="button" onClick={onToggleFollow} disabled={followBusy}>
              {followBusy ? "处理中..." : following ? "取消关注" : auth ? "关注作者" : "登录后关注"}
            </button>
          )}
        </div>
      </aside>
    </div>
  );
}
