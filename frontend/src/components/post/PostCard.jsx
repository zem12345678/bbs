import React from "react";
import { Link } from "react-router-dom";
import { Archive, Edit3, FileText, Heart, MessageSquare, Share2, ShieldCheck, Star, Zap } from "lucide-react";
import { bbsApi } from "../../api";
import { people } from "../../data/communityData";
import { listItems, listTotal } from "../../lib/apiShapes";
import { sameId, timeAgo, toNumber } from "../../lib/formatters";
import { articleToPost, topicToPost, userToPerson } from "../../lib/postMappers";
import Avatar from "../Avatar.jsx";
import { ArticleDetailModal, AuthorProfileModal } from "./PostModals.jsx";

export default function PostCard({
  post,
  index,
  auth,
  onPostArchived,
  onPostStatsChange
}) {
  const safePeople = people.length > 0 ? people : [post.author];
  const [liked, setLiked] = React.useState(Boolean(post.liked));
  const [favorited, setFavorited] = React.useState(Boolean(post.favorited));
  const [likes, setLikes] = React.useState(toNumber(post.likes));
  const [favorites, setFavorites] = React.useState(toNumber(post.favorites));
  const [commentCount, setCommentCount] = React.useState(toNumber(post.comments));
  const [commentsOpen, setCommentsOpen] = React.useState(false);
  const [comments, setComments] = React.useState([]);
  const [commentText, setCommentText] = React.useState("");
  const [commentsLoading, setCommentsLoading] = React.useState(false);
  const [deletingCommentId, setDeletingCommentId] = React.useState(0);
  const [actionError, setActionError] = React.useState("");
  const [detailOpen, setDetailOpen] = React.useState(false);
  const [detailPost, setDetailPost] = React.useState(null);
  const [detailLoading, setDetailLoading] = React.useState(false);
  const [detailError, setDetailError] = React.useState("");
  const [authorOpen, setAuthorOpen] = React.useState(false);
  const [authorProfile, setAuthorProfile] = React.useState(post.author);
  const [authorLoading, setAuthorLoading] = React.useState(false);
  const [authorError, setAuthorError] = React.useState("");
  const [followingAuthor, setFollowingAuthor] = React.useState(false);
  const [followBusy, setFollowBusy] = React.useState(false);
  const [reportBusy, setReportBusy] = React.useState(false);
  const [archiveBusy, setArchiveBusy] = React.useState(false);
  const topicPost = post.kind === "topic";
  const realPost = Boolean(post.id);
  const detailPath = topicPost ? `/topic/${post.id}` : `/article/${post.id}`;
  const editPath = topicPost ? `/topic/edit/${post.id}` : `/article/edit/${post.id}`;
  const ownerPost = realPost && sameId(auth?.user?.id, post.authorId);

  React.useEffect(() => {
    setLiked(Boolean(post.liked));
    setFavorited(Boolean(post.favorited));
    setLikes(toNumber(post.likes));
    setFavorites(toNumber(post.favorites));
    setCommentCount(toNumber(post.comments));
    setComments([]);
    setCommentsOpen(false);
    setDeletingCommentId(0);
    setActionError("");
    setDetailOpen(false);
    setDetailPost(null);
    setDetailError("");
    setAuthorOpen(false);
    setAuthorProfile(post.author);
    setAuthorError("");
    setFollowingAuthor(false);
    setReportBusy(false);
    setArchiveBusy(false);
  }, [post]);

  function ensureActionable() {
    if (!realPost) {
      setActionError("当前内容不可操作，请刷新后重试。");
      return false;
    }
    if (!auth?.accessToken) {
      setActionError("请先登录后再操作。");
      return false;
    }
    return true;
  }

  async function toggleLike() {
    if (!ensureActionable()) return;
    setActionError("");
    try {
      const data = liked
        ? topicPost
          ? await bbsApi.unlikeTopic(post.id, auth.accessToken)
          : await bbsApi.unlikeArticle(post.id, auth.accessToken)
        : topicPost
          ? await bbsApi.likeTopic(post.id, auth.accessToken)
          : await bbsApi.likeArticle(post.id, auth.accessToken);
      const nextLiked = !liked;
      const nextLikes = toNumber(data?.count, Math.max(0, likes + (liked ? -1 : 1)));
      setLiked(nextLiked);
      setLikes(nextLikes);
      onPostStatsChange?.(post.id, { liked: nextLiked, likes: nextLikes });
    } catch (error) {
      setActionError(error.message || "点赞失败");
    }
  }

  async function toggleFavorite() {
    if (!ensureActionable()) return;
    setActionError("");
    try {
      const data = favorited
        ? topicPost
          ? await bbsApi.unfavoriteTopic(post.id, auth.accessToken)
          : await bbsApi.unfavoriteArticle(post.id, auth.accessToken)
        : topicPost
          ? await bbsApi.favoriteTopic(post.id, auth.accessToken)
          : await bbsApi.favoriteArticle(post.id, auth.accessToken);
      const nextFavorited = !favorited;
      const nextFavorites = toNumber(data?.count, Math.max(0, favorites + (favorited ? -1 : 1)));
      setFavorited(nextFavorited);
      setFavorites(nextFavorites);
      onPostStatsChange?.(post.id, { favorited: nextFavorited, favorites: nextFavorites });
    } catch (error) {
      setActionError(error.message || "收藏失败");
    }
  }

  async function submitReport() {
    if (!ensureActionable()) return;
    setActionError("");
    setReportBusy(true);
    try {
      const payload = {
        reason: "content_violation",
        description: "用户从帖子卡片提交举报"
      };
      const data = topicPost
        ? await bbsApi.reportTopic(post.id, payload, auth.accessToken)
        : await bbsApi.reportArticle(post.id, payload, auth.accessToken);
      setActionError(data?.created ? "举报已提交，管理员会尽快处理。" : "你已经举报过该内容，管理员会尽快处理。");
    } catch (error) {
      setActionError(error.message || "举报失败");
    } finally {
      setReportBusy(false);
    }
  }

  async function archivePost() {
    if (!ensureActionable()) return;
    if (!ownerPost) {
      setActionError("只有作者本人可以归档内容。");
      return;
    }
    setArchiveBusy(true);
    setActionError("");
    try {
      topicPost ? await bbsApi.deleteTopic(post.id, auth.accessToken) : await bbsApi.deleteArticle(post.id, auth.accessToken);
      onPostArchived?.(post.id, post.kind);
      if (!onPostArchived) {
        setActionError("内容已归档。");
      }
    } catch (error) {
      setActionError(error.message || "归档失败");
    } finally {
      setArchiveBusy(false);
    }
  }

  async function loadComments() {
    if (!realPost) {
      setActionError("当前内容没有可加载的评论。");
      return;
    }
    const nextOpen = !commentsOpen;
    setCommentsOpen(nextOpen);
    if (!nextOpen || comments.length > 0) {
      return;
    }
    setCommentsLoading(true);
    setActionError("");
    try {
      const data = topicPost ? await bbsApi.listTopicComments(post.id) : await bbsApi.listComments(post.id);
      const items = listItems(data);
      const nextComments = listTotal(data, items);
      setComments(items);
      setCommentCount(nextComments);
      onPostStatsChange?.(post.id, { comments: nextComments });
    } catch (error) {
      setActionError(error.message || "评论加载失败");
    } finally {
      setCommentsLoading(false);
    }
  }

  async function openDetail() {
    setDetailOpen(true);
    setDetailError("");
    setDetailPost(post);
    if (!realPost) {
      setDetailError("当前内容没有可加载的详情。");
      return;
    }
    setDetailLoading(true);
    try {
      const [articleData, reactionData, commentsData] = await Promise.all([
        topicPost ? bbsApi.getTopic(post.id) : bbsApi.getArticle(post.id),
        (topicPost ? bbsApi.topicReactions(post.id) : bbsApi.articleReactions(post.id)).catch(() => null),
        (topicPost ? bbsApi.listTopicComments(post.id) : bbsApi.listComments(post.id)).catch(() => null)
      ]);
      if (articleData?.topic) {
        setDetailPost(topicToPost(articleData.topic, auth));
      }
      if (articleData?.article) {
        setDetailPost(articleToPost(articleData.article, auth));
      }
      if (reactionData) {
        const nextLikes = toNumber(reactionData.like_count ?? reactionData.likeCount, likes);
        const nextFavorites = toNumber(reactionData.favorite_count ?? reactionData.favoriteCount, favorites);
        setLikes(nextLikes);
        setFavorites(nextFavorites);
        onPostStatsChange?.(post.id, { likes: nextLikes, favorites: nextFavorites });
      }
      if (commentsData) {
        setComments(commentsData.items || []);
        if (typeof commentsData.total !== "undefined") {
          const nextComments = toNumber(commentsData.total);
          setCommentCount(nextComments);
          onPostStatsChange?.(post.id, { comments: nextComments });
        }
      }
    } catch (error) {
      setDetailError(error.message || "详情加载失败");
    } finally {
      setDetailLoading(false);
    }
  }

  async function openAuthorProfile() {
    setAuthorOpen(true);
    setAuthorProfile(post.author);
    setAuthorError("");
    if (!post.authorId) {
      return;
    }
    setAuthorLoading(true);
    try {
      const [userData, stateData] = await Promise.all([
        bbsApi.getUser(post.authorId),
        auth?.accessToken && !sameId(auth?.user?.id, post.authorId)
          ? bbsApi.followingState(post.authorId, auth.accessToken).catch(() => null)
          : null
      ]);
      if (userData?.user) {
        setAuthorProfile(userToPerson(userData.user, post.author));
      }
      setFollowingAuthor(Boolean(stateData?.following));
    } catch (error) {
      setAuthorError(error.message || "作者资料加载失败");
    } finally {
      setAuthorLoading(false);
    }
  }

  async function toggleFollowAuthor() {
    if (!post.authorId) {
      setAuthorError("当前作者资料不完整，暂不能关注。");
      return;
    }
    if (!auth?.accessToken) {
      setAuthorError("请先登录后再关注。");
      return;
    }
    if (sameId(auth.user?.id, post.authorId)) {
      setAuthorError("不能关注自己。");
      return;
    }
    setFollowBusy(true);
    setAuthorError("");
    try {
      if (followingAuthor) {
        await bbsApi.unfollowUser(post.authorId, auth.accessToken);
      } else {
        await bbsApi.followUser(post.authorId, auth.accessToken);
      }
      const nextFollowing = !followingAuthor;
      setFollowingAuthor(nextFollowing);
      setAuthorProfile((current) => ({
        ...current,
        followerCount: Math.max(0, toNumber(current?.followerCount) + (nextFollowing ? 1 : -1))
      }));
    } catch (error) {
      setAuthorError(error.message || "关注操作失败");
    } finally {
      setFollowBusy(false);
    }
  }

  async function submitComment(event) {
    event.preventDefault();
    if (!ensureActionable()) return;
    const content = commentText.trim();
    if (!content) return;
    setActionError("");
    try {
      const data = topicPost
        ? await bbsApi.createTopicComment(post.id, { content, parent_id: 0 }, auth.accessToken)
        : await bbsApi.createComment(post.id, { content, parent_id: 0 }, auth.accessToken);
      if (data?.comment) {
        setComments((current) => [data.comment, ...current]);
      }
      setCommentCount((count) => {
        const nextCount = count + 1;
        onPostStatsChange?.(post.id, { comments: nextCount });
        return nextCount;
      });
      setCommentText("");
      setCommentsOpen(true);
    } catch (error) {
      setActionError(error.message || "评论失败");
    }
  }

  async function deleteComment(commentId) {
    if (!auth?.accessToken) {
      setActionError("请先登录后再操作。");
      return;
    }
    if (!commentId) {
      return;
    }
    setDeletingCommentId(commentId);
    setActionError("");
    try {
      await bbsApi.deleteComment(commentId, auth.accessToken);
      setComments((current) => current.filter((comment) => !sameId(comment.id, commentId)));
      setCommentCount((count) => {
        const nextCount = Math.max(0, count - 1);
        onPostStatsChange?.(post.id, { comments: nextCount });
        return nextCount;
      });
    } catch (error) {
      setActionError(error.message || "删除评论失败");
    } finally {
      setDeletingCommentId(0);
    }
  }

  function canDeleteComment(comment) {
    return sameId(comment.author_id || comment.authorId, auth?.user?.id);
  }

  return (
    <article className="post panel">
      <header className="post-header">
        <button className="author-button" type="button" onClick={openAuthorProfile}>
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
        </button>
      </header>
      <div className="post-body">
        {post.title && (
          <h2 className="post-title">
            {realPost ? <Link to={detailPath}>{post.title}</Link> : post.title}
          </h2>
        )}
        <p className="post-text">{post.text}</p>
        {post.images && (
          <div className="image-strip" aria-label="帖子配图">
            {post.images.map((src) => (
              <img src={src} alt="桌面工作区照片" key={src} />
            ))}
          </div>
        )}
        <div className="tag-row">
          {post.tags.map((tag) => (
            <Link to={`${topicPost ? "/topics" : "/articles"}/tag/${encodeURIComponent(tag)}`} key={tag}>
              <Zap size={13} aria-hidden="true" />
              {tag}
            </Link>
          ))}
        </div>
        <footer className="action-row">
          <button type="button" className={liked ? "liked" : ""} onClick={toggleLike}>
            <Heart size={20} aria-hidden="true" />
            {likes}
          </button>
          <button type="button" onClick={loadComments}>
            <MessageSquare size={20} aria-hidden="true" />
            {commentCount || "评论"}
          </button>
          {realPost ? (
            <Link to={detailPath}>
              <FileText size={20} aria-hidden="true" />
              详情
            </Link>
          ) : (
            <button type="button" onClick={openDetail}>
              <FileText size={20} aria-hidden="true" />
              详情
            </button>
          )}
          <button type="button" className={favorited ? "liked" : ""} onClick={toggleFavorite}>
            <Star size={20} aria-hidden="true" />
            {favorites || "收藏"}
          </button>
          <button type="button">
            <Share2 size={20} aria-hidden="true" />
            分享
          </button>
          <button type="button" onClick={submitReport} disabled={reportBusy}>
            <ShieldCheck size={18} aria-hidden="true" />
            {reportBusy ? "提交中" : "举报"}
          </button>
          {ownerPost && (
            <>
              <Link to={editPath}>
                <Edit3 size={18} aria-hidden="true" />
                编辑
              </Link>
              <button type="button" onClick={archivePost} disabled={archiveBusy}>
                <Archive size={18} aria-hidden="true" />
                {archiveBusy ? "归档中" : "归档"}
              </button>
            </>
          )}
        </footer>
        {actionError && <p className="form-error post-error">{actionError}</p>}
        {commentsOpen && (
          <section className="comment-panel" aria-label="评论">
            {commentsLoading && <p className="comment-empty">正在加载评论...</p>}
            {!commentsLoading && comments.length === 0 && <p className="comment-empty">暂无评论，来发第一条。</p>}
            {comments.map((comment) => (
              <div className="comment-item" key={comment.id}>
                <Avatar
                  person={{
                    name: `用户 #${comment.author_id || comment.authorId || "?"}`,
                    handle: `u${comment.author_id || comment.authorId || "unknown"}`,
                    avatar: safePeople[toNumber(comment.author_id || comment.authorId) % safePeople.length].avatar
                  }}
                  small
                />
                <div>
                  <div className="comment-head">
                    <strong>用户 #{comment.author_id || comment.authorId || "?"}</strong>
                    {canDeleteComment(comment) && (
                      <button type="button" onClick={() => deleteComment(comment.id)} disabled={sameId(deletingCommentId, comment.id)}>
                        {sameId(deletingCommentId, comment.id) ? "删除中" : "删除"}
                      </button>
                    )}
                  </div>
                  <p>{comment.content}</p>
                  <span>{timeAgo(comment.created_at || comment.createdAt)}</span>
                </div>
              </div>
            ))}
            <form className="comment-form" onSubmit={submitComment}>
              <input
                placeholder={auth ? "写下你的评论" : "登录后参与评论"}
                value={commentText}
                onChange={(event) => setCommentText(event.target.value)}
                disabled={!auth}
              />
              <button type="submit" disabled={!auth || !commentText.trim()}>
                发送
              </button>
            </form>
          </section>
        )}
      </div>
      {detailOpen && (
        <ArticleDetailModal
          auth={auth}
          commentCount={commentCount}
          commentText={commentText}
          comments={comments}
          commentsLoading={commentsLoading || detailLoading}
          error={detailError || actionError}
          favorites={favorites}
          favorited={favorited}
          liked={liked}
          likes={likes}
          onClose={() => setDetailOpen(false)}
          onCommentTextChange={setCommentText}
          onDeleteComment={deleteComment}
          onFavorite={toggleFavorite}
          onLike={toggleLike}
          onSubmitComment={submitComment}
          people={safePeople}
          post={detailPost || post}
          canDeleteComment={canDeleteComment}
          deletingCommentId={deletingCommentId}
        />
      )}
      {authorOpen && (
        <AuthorProfileModal
          auth={auth}
          error={authorError}
          followBusy={followBusy}
          following={followingAuthor}
          loading={authorLoading}
          onClose={() => setAuthorOpen(false)}
          onToggleFollow={toggleFollowAuthor}
          person={authorProfile}
          self={sameId(auth?.user?.id, post.authorId)}
        />
      )}
    </article>
  );
}
