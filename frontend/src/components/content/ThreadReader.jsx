import React from "react";
import { Link } from "react-router-dom";
import {
  Archive,
  CheckCircle2,
  CornerDownRight,
  Edit3,
  Eye,
  Flag,
  Heart,
  ImagePlus,
  MessageSquare,
  Quote,
	RotateCcw,
  Share2,
  ShieldCheck,
  Star,
  Trash2,
  Zap
} from "lucide-react";
import { bbsApi } from "../../api";
import { listItems, listTotal } from "../../lib/apiShapes";
import { appendMarkdownImage, textWithoutMarkdownImages } from "../../lib/markdownMedia";
import { compactNumber, sameId, timeAgoMillis, toId, toNumber } from "../../lib/formatters";
import { fallbackPerson, userToPerson } from "../../lib/postMappers";
import Avatar from "../Avatar.jsx";
import { ReportModal } from "../post/PostModals.jsx";
import MarkdownPreview from "./MarkdownPreview.jsx";
import TopicAttachments from "./TopicAttachments.jsx";

const COMMENT_PAGE_SIZE = 50;

export default function ThreadReader({ auth, focusedCommentId, item, kind = "topic", onEdit, onPostArchived, onPostStatsChange, post }) {
  const topicPost = kind === "topic";
  const [liked, setLiked] = React.useState(Boolean(post?.liked));
  const [favorited, setFavorited] = React.useState(Boolean(post?.favorited));
  const [likes, setLikes] = React.useState(toNumber(post?.likes));
  const [favorites, setFavorites] = React.useState(toNumber(post?.favorites));
  const [qaStatus, setQaStatus] = React.useState(post?.qaStatus || item?.qa_status || item?.qaStatus || "");
  const [acceptedCommentId, setAcceptedCommentId] = React.useState(() => normalizeAcceptedCommentId(post?.acceptedCommentId ?? item?.accepted_comment_id ?? item?.acceptedCommentId));
  const [commentTotal, setCommentTotal] = React.useState(toNumber(post?.comments));
  const hasViews = post?.views !== undefined && post?.views !== null;
  const viewCount = hasViews ? toNumber(post.views) : 0;
  const [comments, setComments] = React.useState([]);
  const [replyState, setReplyState] = React.useState({});
  const [commentAuthorMap, setCommentAuthorMap] = React.useState({});
  const [commentText, setCommentText] = React.useState("");
  const [targetComment, setTargetComment] = React.useState(null);
  const [commentsLoading, setCommentsLoading] = React.useState(false);
  const [actionError, setActionError] = React.useState("");
  const [notice, setNotice] = React.useState("");
  const [submitting, setSubmitting] = React.useState(false);
  const [deletingCommentId, setDeletingCommentId] = React.useState("");
  const [acceptingCommentId, setAcceptingCommentId] = React.useState("");
	const [unacceptingCommentId, setUnacceptingCommentId] = React.useState("");
  const [uploadingTarget, setUploadingTarget] = React.useState("");
  const [reportOpen, setReportOpen] = React.useState(false);
  const [commentReportTarget, setCommentReportTarget] = React.useState(null);
  const [related, setRelated] = React.useState({ items: [], loading: false, error: "" });
  const [lastReadId, setLastReadId] = React.useState(() => readLastRead(post?.kind, post?.id));
  const contentBody = item?.body || item?.content || post?.text || "";
  const ownerPost = sameId(auth?.user?.id, post?.authorId);
  const questionPost = topicPost && (post?.topicType === "qa" || item?.type === "qa");
  const questionResolved = questionPost && (qaStatus === "resolved" || Boolean(acceptedCommentId));
  const latestCommentId = latestVisibleCommentId(comments, replyState);

  React.useEffect(() => {
    setLiked(Boolean(post?.liked));
    setFavorited(Boolean(post?.favorited));
    setLikes(toNumber(post?.likes));
    setFavorites(toNumber(post?.favorites));
    setCommentTotal(toNumber(post?.comments));
    setQaStatus(post?.qaStatus || item?.qa_status || item?.qaStatus || "");
    setAcceptedCommentId(normalizeAcceptedCommentId(post?.acceptedCommentId ?? item?.accepted_comment_id ?? item?.acceptedCommentId));
    setLastReadId(readLastRead(post?.kind, post?.id));
  }, [
    item?.acceptedCommentId,
    item?.accepted_comment_id,
    item?.qaStatus,
    item?.qa_status,
    post?.acceptedCommentId,
    post?.comments,
    post?.favorited,
    post?.favorites,
    post?.id,
    post?.kind,
    post?.liked,
    post?.likes,
    post?.qaStatus
  ]);

  const loadComments = React.useCallback(async () => {
    if (!post?.id) return;
    setCommentsLoading(true);
    setActionError("");
    try {
      const data = topicPost
        ? await bbsApi.listTopicComments(post.id, { page: 1, page_size: COMMENT_PAGE_SIZE })
        : await bbsApi.listComments(post.id, { page: 1, page_size: COMMENT_PAGE_SIZE });
      const items = sortComments(listItems(data));
      const total = listTotal(data, items);
      setComments(items);
      setCommentTotal(total);
      onPostStatsChange?.(post.id, { comments: total });
    } catch (error) {
      setActionError(error.message || "评论加载失败");
    } finally {
      setCommentsLoading(false);
    }
  }, [onPostStatsChange, post?.id, topicPost]);

  React.useEffect(() => {
    setComments([]);
    setReplyState({});
    setCommentAuthorMap({});
    setCommentText("");
    setTargetComment(null);
    setCommentReportTarget(null);
    setActionError("");
    setNotice("");
    loadComments();
  }, [loadComments, post?.id, post?.kind]);

  React.useEffect(() => {
    if (!focusedCommentId || !post?.id) return;
    let alive = true;
    async function revealFocusedComment() {
      try {
        const data = await bbsApi.getComment(focusedCommentId);
        if (!alive) return;
        const comment = data?.comment;
        const rootId = commentRootId(comment);
        if (rootId && !sameId(rootId, focusedCommentId)) {
          await loadReplies({ id: rootId }, true);
        }
      } catch {
        // The root list may still include the target; scroll attempt below is harmless.
      }
      if (!alive) return;
      scrollToComment(focusedCommentId);
    }
    revealFocusedComment();
    return () => {
      alive = false;
    };
  }, [focusedCommentId, post?.id]);

  React.useEffect(() => {
    const missingAuthorIds = new Set();
    const collectAuthor = (comment) => {
      const authorId = toId(comment?.author_id ?? comment?.authorId);
      if (!authorId || sameId(authorId, auth?.user?.id) || commentAuthorMap[String(authorId)]) return;
      missingAuthorIds.add(String(authorId));
    };
    comments.forEach(collectAuthor);
    Object.values(replyState).forEach((state) => {
      (state?.items || []).forEach(collectAuthor);
    });
    if (missingAuthorIds.size === 0) return undefined;

    let alive = true;
    Promise.all(
      Array.from(missingAuthorIds).map(async (authorId) => {
        const data = await bbsApi.getUser(authorId).catch(() => null);
        return data?.user ? [authorId, userToPerson(data.user)] : null;
      })
    ).then((entries) => {
      if (!alive) return;
      const found = entries.filter(Boolean);
      if (found.length === 0) return;
      setCommentAuthorMap((current) => {
        const next = { ...current };
        found.forEach(([authorId, person]) => {
          next[authorId] = person;
        });
        return next;
      });
    });
    return () => {
      alive = false;
    };
  }, [auth?.user?.id, commentAuthorMap, comments, replyState]);

  React.useEffect(() => {
    if (!post?.id) return;
    let alive = true;
    setRelated({ items: [], loading: true, error: "" });
    const tag = post.tags?.[0] || "";
    const loader = topicPost ? bbsApi.listTopics : bbsApi.listArticles;
    loader({ limit: 6, offset: 0, tag: tag || undefined })
      .then((data) => {
        if (!alive) return;
        const items = listItems(data)
          .filter((row) => !sameId(row.id, post.id))
          .slice(0, 5);
        setRelated({ items, loading: false, error: "" });
      })
      .catch((error) => {
        if (alive) setRelated({ items: [], loading: false, error: error.message || "相关内容加载失败" });
      });
    return () => {
      alive = false;
    };
  }, [post?.id, post?.tags?.join("|"), topicPost]);

  function ensureActionable() {
    if (!post?.id) {
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

  async function acceptAnswer(comment) {
    if (!ensureActionable()) return;
    if (!questionPost) {
      setActionError("只有问答内容可以采纳答案。");
      return;
    }
    if (!ownerPost) {
      setActionError("只有提问者可以采纳答案。");
      return;
    }
    if (acceptedCommentId) {
      setActionError("该问题已经采纳过答案。");
      return;
    }
    const commentId = toId(comment?.id);
    if (!commentId) return;
    setAcceptingCommentId(commentId);
    setActionError("");
    try {
      const data = await bbsApi.acceptTopicComment(post.id, commentId, auth.accessToken);
      const topic = data?.topic || {};
      const nextAcceptedId = normalizeAcceptedCommentId(topic.accepted_comment_id ?? topic.acceptedCommentId ?? commentId);
      const nextQaStatus = topic.qa_status || topic.qaStatus || "resolved";
      setAcceptedCommentId(nextAcceptedId);
      setQaStatus(nextQaStatus);
      setNotice("已采纳答案，问题状态已更新为已解决。");
      onPostStatsChange?.(post.id, { qaStatus: nextQaStatus, acceptedCommentId: nextAcceptedId });
    } catch (error) {
      setActionError(acceptAnswerErrorMessage(error));
    } finally {
      setAcceptingCommentId("");
    }
  }

  async function unacceptAnswer(comment) {
    if (!ensureActionable()) return;
    if (!questionPost) {
      setActionError("只有问答内容可以撤销采纳。");
      return;
    }
    if (!ownerPost) {
      setActionError("只有提问者可以撤销采纳。");
      return;
    }
    const commentId = toId(comment?.id);
    if (!commentId || !sameId(commentId, acceptedCommentId)) {
      setActionError("当前答案不是已采纳答案。");
      return;
    }
    setUnacceptingCommentId(commentId);
    setActionError("");
    try {
      const data = await bbsApi.unacceptTopicComment(post.id, commentId, auth.accessToken);
      const topic = data?.topic || {};
      const nextAcceptedId = normalizeAcceptedCommentId(topic.accepted_comment_id ?? topic.acceptedCommentId);
      const nextQaStatus = topic.qa_status || topic.qaStatus || "open";
      setAcceptedCommentId(nextAcceptedId);
      setQaStatus(nextQaStatus);
      setNotice("已撤销采纳，悬赏继续冻结，问题已重新开放。");
      onPostStatsChange?.(post.id, { qaStatus: nextQaStatus, acceptedCommentId: nextAcceptedId });
    } catch (error) {
      setActionError(unacceptAnswerErrorMessage(error));
    } finally {
      setUnacceptingCommentId("");
    }
  }

  function openReport() {
    if (!ensureActionable()) return;
    setActionError("");
    setReportOpen(true);
  }

  async function submitReport(payload) {
    if (!ensureActionable()) {
      throw new Error("请先登录后再操作。");
    }
    const reportPayload = { ...payload, description: payload.description || "用户从详情页提交举报" };
    try {
      const data = topicPost
        ? await bbsApi.reportTopic(post.id, reportPayload, auth.accessToken)
        : await bbsApi.reportArticle(post.id, reportPayload, auth.accessToken);
      setNotice(data?.created ? "举报已提交，管理员会尽快处理。" : "你已经举报过该内容，管理员会尽快处理。");
    } catch (error) {
      throw new Error(error.message || "举报失败");
    }
  }

  function openCommentReport(comment) {
    if (!ensureActionable()) return;
    setActionError("");
    setCommentReportTarget(comment);
  }

  function commentReportTitle(comment) {
    const text = textWithoutMarkdownImages(comment?.content || "").replace(/\s+/g, " ").trim();
    if (!text) return "这条评论";
    return `评论：${text.slice(0, 42)}${text.length > 42 ? "..." : ""}`;
  }

  async function submitCommentReport(payload) {
    if (!ensureActionable()) {
      throw new Error("请先登录后再操作。");
    }
    const commentId = commentReportTarget?.id;
    if (!commentId) {
      throw new Error("未找到要举报的评论。");
    }
    const reportPayload = { ...payload, description: payload.description || "用户从详情页提交评论举报" };
    try {
      const data = await bbsApi.reportComment(commentId, reportPayload, auth.accessToken);
      setNotice(data?.created ? "评论举报已提交，管理员会尽快处理。" : "你已经举报过这条评论，管理员会尽快处理。");
    } catch (error) {
      throw new Error(error.message || "举报失败");
    }
  }

  async function archivePost() {
    if (!ensureActionable()) return;
    if (!ownerPost) {
      setActionError("只有作者本人可以归档内容。");
      return;
    }
    setActionError("");
    try {
      topicPost ? await bbsApi.deleteTopic(post.id, auth.accessToken) : await bbsApi.deleteArticle(post.id, auth.accessToken);
      onPostArchived?.(post.id, post.kind);
    } catch (error) {
      setActionError(error.message || "归档失败");
    }
  }

  async function shareThread() {
    const url = window.location.href;
    try {
      await navigator.clipboard?.writeText(url);
      setNotice("链接已复制。");
    } catch {
      setNotice(url);
    }
  }

  function emptyReplyState() {
    return { items: [], total: 0, loading: false, open: false, error: "" };
  }

  function getReplyState(commentId) {
    return replyState[String(commentId)] || emptyReplyState();
  }

  async function loadReplies(comment, forceOpen = false) {
    const rootId = commentRootId(comment);
    if (!rootId) return;
    const key = String(rootId);
    const current = replyState[key];
    if (!forceOpen && current?.open) {
      setReplyState((items) => ({
        ...items,
        [key]: { ...emptyReplyState(), ...items[key], open: false, loading: false }
      }));
      return;
    }
    if (!forceOpen && current?.items?.length) {
      setReplyState((items) => ({
        ...items,
        [key]: { ...emptyReplyState(), ...items[key], open: true, error: "" }
      }));
      return;
    }
    setReplyState((items) => ({
      ...items,
      [key]: { ...emptyReplyState(), ...items[key], open: true, loading: true, error: "" }
    }));
    try {
      const data = await bbsApi.listReplies(rootId, { page: 1, page_size: COMMENT_PAGE_SIZE });
      const items = sortComments(listItems(data));
      setReplyState((currentItems) => ({
        ...currentItems,
        [key]: {
          ...emptyReplyState(),
          ...currentItems[key],
          items,
          total: listTotal(data, items),
          loading: false,
          open: true,
          error: ""
        }
      }));
    } catch (error) {
      setReplyState((items) => ({
        ...items,
        [key]: { ...emptyReplyState(), ...items[key], loading: false, open: true, error: error.message || "回复加载失败" }
      }));
    }
  }

  function quoteComment(comment) {
    const person = commentPerson(comment);
    const snippet = textWithoutMarkdownImages(comment?.content || "").replace(/\s+/g, " ").slice(0, 160);
    setTargetComment(comment);
    setCommentText((current) => `${current.trimEnd()}${current.trim() ? "\n\n" : ""}> ${person.name}：${snippet}\n\n`);
    focusCommentEditor();
  }

  function replyTo(comment) {
    setTargetComment(comment);
    setCommentText((current) => current || `@${commentPerson(comment).name} `);
    focusCommentEditor();
  }

  async function submitComment(event) {
    event.preventDefault();
    if (!ensureActionable()) return;
    const content = commentText.trim();
    if (!content) return;
    const parentId = targetComment ? commentRootId(targetComment) : 0;
    setSubmitting(true);
    setActionError("");
    try {
      const data = topicPost
        ? await bbsApi.createTopicComment(post.id, { content, parent_id: parentId }, auth.accessToken)
        : await bbsApi.createComment(post.id, { content, parent_id: parentId }, auth.accessToken);
      if (data?.comment) {
        if (parentId) {
          const key = String(parentId);
          setReplyState((items) => {
            const current = { ...emptyReplyState(), ...items[key] };
            return {
              ...items,
              [key]: {
                ...current,
                items: sortComments([...current.items, data.comment]),
                total: toNumber(current.total) + 1,
                open: true,
                error: ""
              }
            };
          });
          setComments((items) => incrementReplyCount(items, parentId, 1));
        } else {
          setComments((items) => sortComments([...items, data.comment]));
        }
      }
      setCommentText("");
      setTargetComment(null);
      setCommentTotal((count) => {
        const nextCount = count + 1;
        onPostStatsChange?.(post.id, { comments: nextCount });
        return nextCount;
      });
    } catch (error) {
      setActionError(error.message || "评论失败");
    } finally {
      setSubmitting(false);
    }
  }

  async function uploadCommentImage(event) {
    const file = event.target.files?.[0];
    event.target.value = "";
    if (!file) return;
    if (!ensureActionable()) return;
    setUploadingTarget("root");
    setActionError("");
    try {
      const data = await bbsApi.uploadImage(file, auth.accessToken);
      const imageUrl = data?.image_url || data?.imageUrl || data?.url || "";
      if (!imageUrl) {
        throw new Error("图片上传成功但未返回地址");
      }
      setCommentText((current) => appendMarkdownImage(current, imageUrl));
      setNotice("图片已插入评论。");
    } catch (error) {
      setActionError(error.message || "图片上传失败");
    } finally {
      setUploadingTarget("");
    }
  }

  async function deleteComment(comment, rootId = 0) {
    if (!auth?.accessToken) {
      setActionError("请先登录后再操作。");
      return;
    }
    const commentId = toId(comment?.id);
    if (!commentId) return;
    setDeletingCommentId(commentId);
    setActionError("");
    try {
      await bbsApi.deleteComment(commentId, auth.accessToken);
      if (rootId) {
        const key = String(rootId);
        setReplyState((items) => ({
          ...items,
          [key]: {
            ...emptyReplyState(),
            ...items[key],
            items: (items[key]?.items || []).filter((item) => !sameId(item.id, commentId)),
            total: Math.max(0, toNumber(items[key]?.total) - 1)
          }
        }));
        setComments((items) => incrementReplyCount(items, rootId, -1));
      } else {
        setComments((items) => items.filter((item) => !sameId(item.id, commentId)));
        setReplyState((items) => {
          const nextItems = { ...items };
          delete nextItems[String(commentId)];
          return nextItems;
        });
      }
      setCommentTotal((count) => {
        const nextCount = Math.max(0, count - 1);
        onPostStatsChange?.(post.id, { comments: nextCount });
        return nextCount;
      });
    } catch (error) {
      setActionError(error.message || "删除评论失败");
    } finally {
      setDeletingCommentId("");
    }
  }

  function markRead() {
    if (!post?.id || !latestCommentId) return;
    writeLastRead(post.kind, post.id, latestCommentId);
    setLastReadId(latestCommentId);
    setNotice("已记录当前阅读位置。");
  }

  function jumpLastRead() {
    if (lastReadId) {
      scrollToComment(lastReadId);
      return;
    }
    jumpLatest();
  }

  function jumpLatest() {
    if (latestCommentId) {
      scrollToComment(latestCommentId);
      return;
    }
    document.getElementById("thread-comments")?.scrollIntoView({ behavior: "smooth", block: "start" });
  }

  async function jumpAcceptedAnswer() {
    if (!acceptedCommentId) return;
    try {
      const data = await bbsApi.getComment(acceptedCommentId);
      const rootId = commentRootId(data?.comment);
      if (rootId && !sameId(rootId, acceptedCommentId)) {
        await loadReplies({ id: rootId }, true);
      }
    } catch {
      // If the accepted answer is already rendered, the scroll below is enough.
    }
    scrollToComment(acceptedCommentId);
  }

  function canDeleteComment(comment) {
    return sameId(comment?.author_id ?? comment?.authorId, auth?.user?.id);
  }

  function fallbackCommentPerson(comment) {
    const authorId = toId(comment?.author_id ?? comment?.authorId);
    return fallbackPerson(authorId);
  }

  function commentPerson(comment) {
    const authorId = toId(comment?.author_id ?? comment?.authorId);
    const fallback = fallbackCommentPerson(comment);
    if (sameId(authorId, auth?.user?.id) && auth?.user) {
      return userToPerson(auth.user, fallback);
    }
    return commentAuthorMap[String(authorId)] || fallback;
  }

  function renderComment(comment, floor, rootId = 0) {
    const root = !rootId;
    const person = commentPerson(comment);
    const replies = root ? getReplyState(comment.id) : emptyReplyState();
    const replyCount = Math.max(toNumber(comment?.reply_count ?? comment?.replyCount), toNumber(replies.total));
    const commentId = toId(comment?.id);
    const acceptedAnswer = questionPost && sameId(commentId, acceptedCommentId);
    const acceptingThisAnswer = sameId(acceptingCommentId, commentId);
	const unacceptingThisAnswer = sameId(unacceptingCommentId, commentId);
    const canAcceptAnswer = questionPost && ownerPost && !acceptedCommentId;
    const canUnacceptAnswer = questionPost && ownerPost && acceptedAnswer;
    return (
      <article className={`thread-comment ${root ? "is-root" : "is-reply"} ${acceptedAnswer ? "is-accepted" : ""}`} id={`comment-${commentId}`} key={commentId}>
        <aside className="thread-comment-index">{root ? `#${floor}` : <CornerDownRight size={16} aria-hidden="true" />}</aside>
        <div className="thread-comment-main">
          <header>
            <Avatar person={person} small />
            <div>
              <strong>{person.name}</strong>
              <span>{timeAgoMillis(comment?.created_at || comment?.createdAt)}</span>
            </div>
            {acceptedAnswer && (
              <span className="thread-answer-badge">
                <CheckCircle2 size={14} aria-hidden="true" />
                已采纳
              </span>
            )}
            {root && <a href={`#comment-${commentId}`}>#{floor}</a>}
          </header>
          <MarkdownPreview className="thread-comment-body" text={comment?.content || ""} />
          <footer>
            {canAcceptAnswer && (
              <button type="button" onClick={() => acceptAnswer(comment)} disabled={Boolean(acceptingCommentId)}>
                <CheckCircle2 size={16} aria-hidden="true" />
                {acceptingThisAnswer ? "采纳中" : "采纳答案"}
              </button>
            )}
            {canUnacceptAnswer && (
              <button type="button" onClick={() => unacceptAnswer(comment)} disabled={Boolean(unacceptingCommentId)}>
                <RotateCcw size={16} aria-hidden="true" />
                {unacceptingThisAnswer ? "撤销中" : "撤销采纳"}
              </button>
            )}
            <button type="button" onClick={() => quoteComment(comment)}>
              <Quote size={16} aria-hidden="true" />
              引用
            </button>
            <button type="button" onClick={() => replyTo(comment)}>
              <MessageSquare size={16} aria-hidden="true" />
              回复
            </button>
            {auth && (
              <button type="button" onClick={() => openCommentReport(comment)}>
                <Flag size={16} aria-hidden="true" />
                举报
              </button>
            )}
            {root && replyCount > 0 && (
              <button type="button" onClick={() => loadReplies(comment)}>
                {replies.open ? "收起回复" : `查看 ${replyCount} 条回复`}
              </button>
            )}
            {canDeleteComment(comment) && (
              <button type="button" onClick={() => deleteComment(comment, rootId)} disabled={sameId(deletingCommentId, commentId)}>
                <Trash2 size={16} aria-hidden="true" />
                {sameId(deletingCommentId, commentId) ? "删除中" : "删除"}
              </button>
            )}
          </footer>
          {root && replies.open && (
            <div className="thread-replies">
              {replies.loading && <p>正在加载回复...</p>}
              {replies.error && <p className="form-error">{replies.error}</p>}
              {replies.items.map((reply) => renderComment(reply, floor, comment.id))}
            </div>
          )}
        </div>
      </article>
    );
  }

  return (
    <article className="thread-reader">
      <section className="thread-hero panel">
        <div className="thread-hero-meta">
          <Avatar person={post.author} />
          <div>
            <strong>{post.author.name}</strong>
            <span>{post.author.role} · {post.time}</span>
          </div>
        </div>
        <h1>{post.title}</h1>
        {questionPost && (
          <div className={`thread-qa-summary ${questionResolved ? "is-resolved" : ""}`}>
            <span>
              <CheckCircle2 size={16} aria-hidden="true" />
              {questionResolved ? "已解决" : "等待采纳答案"}
            </span>
            {toNumber(post?.bountyScore) > 0 && <span>{post.bountyScore} 积分悬赏</span>}
            {acceptedCommentId && (
              <button type="button" onClick={jumpAcceptedAnswer}>
                查看采纳答案
              </button>
            )}
          </div>
        )}
        <MarkdownPreview className="thread-body" text={contentBody} />
        {topicPost && <TopicAttachments auth={auth} canManage={ownerPost} topicId={post.id} />}
        {post.tags?.length > 0 && (
          <div className="tag-row">
            {post.tags.map((tag) => (
              <Link to={`${topicPost ? "/topics" : "/articles"}/tag/${encodeURIComponent(tag)}`} key={tag}>
                <Zap size={13} aria-hidden="true" />
                {tag}
              </Link>
            ))}
          </div>
        )}
        <div className="thread-action-row">
          <button className={liked ? "liked" : ""} type="button" onClick={toggleLike}>
            <Heart size={18} aria-hidden="true" />
            {likes}
          </button>
          <button className={favorited ? "liked" : ""} type="button" onClick={toggleFavorite}>
            <Star size={18} aria-hidden="true" />
            {favorites || "收藏"}
          </button>
          <button type="button" onClick={jumpLatest}>
            <MessageSquare size={18} aria-hidden="true" />
            {commentTotal || "评论"}
          </button>
          {hasViews && (
            <span className="thread-view-count">
              <Eye size={18} aria-hidden="true" />
              {compactNumber(viewCount)} 浏览
            </span>
          )}
          <button type="button" onClick={shareThread}>
            <Share2 size={18} aria-hidden="true" />
            分享
          </button>
          <button type="button" onClick={openReport}>
            <ShieldCheck size={18} aria-hidden="true" />
            举报
          </button>
          {ownerPost && (
            <>
              <button type="button" onClick={onEdit}>
                <Edit3 size={18} aria-hidden="true" />
                编辑
              </button>
              <button type="button" onClick={archivePost}>
                <Archive size={18} aria-hidden="true" />
                归档
              </button>
            </>
          )}
        </div>
        {(actionError || notice) && <p className={actionError ? "form-error post-error" : "form-success post-error"}>{actionError || notice}</p>}
      </section>

      <section className="thread-toolbar panel" aria-label="话题阅读工具">
        <div>
          <strong>{commentTotal}</strong>
          <span>条讨论</span>
        </div>
        <button type="button" onClick={jumpLastRead}>
          {lastReadId ? "跳到上次阅读" : "跳到最新评论"}
        </button>
        <button type="button" disabled={!latestCommentId} onClick={markRead}>
          <CheckCircle2 size={16} aria-hidden="true" />
          标记已读
        </button>
      </section>

      <section className="thread-comments panel" id="thread-comments">
        <header>
          <div>
            <h2>讨论楼层</h2>
            <p>按时间顺序阅读回复，可引用任意楼层继续讨论。</p>
          </div>
          <button type="button" onClick={loadComments}>刷新</button>
        </header>
        {commentsLoading && <p className="thread-empty">正在加载评论...</p>}
        {!commentsLoading && comments.length === 0 && <p className="thread-empty">暂无评论，来发第一条。</p>}
        {comments.map((comment, index) => renderComment(comment, index + 1))}
        <form className="thread-reply-form" onSubmit={submitComment}>
          {targetComment && (
            <div className="thread-reply-target">
              <span>回复 {commentPerson(targetComment).name}</span>
              <button type="button" onClick={() => setTargetComment(null)}>取消</button>
            </div>
          )}
          <textarea
            id="thread-comment-editor"
            placeholder={auth ? "写下你的回复，支持引用和图片 Markdown" : "登录后参与讨论"}
            value={commentText}
            disabled={!auth}
            onChange={(event) => setCommentText(event.target.value)}
          />
          <div>
            <label className="comment-image-upload">
              <input accept="image/jpeg,image/png,image/gif,image/webp" className="sr-only" disabled={!auth || Boolean(uploadingTarget)} type="file" onChange={uploadCommentImage} />
              <ImagePlus size={16} aria-hidden="true" />
              <span>{uploadingTarget ? "上传中" : "图片"}</span>
            </label>
            <button type="submit" disabled={!auth || submitting || !commentText.trim()}>
              {submitting ? "发送中" : "发表回复"}
            </button>
          </div>
        </form>
      </section>

      <section className="thread-related panel">
        <header>
          <h2>相关内容</h2>
          <span>{related.loading ? "加载中" : `${related.items.length} 条`}</span>
        </header>
        {related.error && <p className="thread-empty">{related.error}</p>}
        {!related.error && related.items.length === 0 && !related.loading && <p className="thread-empty">暂无相关内容。</p>}
        {related.items.map((row) => (
          <Link key={row.id} to={`${topicPost ? "/topic" : "/article"}/${row.id}`}>
            <strong>{row.title || "未命名内容"}</strong>
            <span>{timeAgoMillis(row.published_at || row.publishedAt || row.created_at || row.createdAt)}</span>
          </Link>
        ))}
      </section>
      {reportOpen && <ReportModal targetTitle={post.title || "当前内容"} onClose={() => setReportOpen(false)} onSubmit={submitReport} />}
      {commentReportTarget && (
        <ReportModal targetTitle={commentReportTitle(commentReportTarget)} onClose={() => setCommentReportTarget(null)} onSubmit={submitCommentReport} />
      )}
    </article>
  );
}

function sortComments(items = []) {
  return [...items].sort((left, right) => toNumber(left?.created_at ?? left?.createdAt) - toNumber(right?.created_at ?? right?.createdAt));
}

function commentRootId(comment) {
  const rootId = comment?.root_id ?? comment?.rootId;
  return toNumber(rootId) > 0 ? toId(rootId) : toId(comment?.id);
}

function latestVisibleCommentId(comments, replyState) {
  const ids = [];
  comments.forEach((comment) => {
    if (comment?.id) ids.push(toId(comment.id));
    const replies = replyState[String(comment?.id)]?.items || [];
    replies.forEach((reply) => {
      if (reply?.id) ids.push(toId(reply.id));
    });
  });
  return ids[ids.length - 1] || "";
}

function incrementReplyCount(items, rootId, delta) {
  return items.map((item) => {
    if (!sameId(item.id, rootId)) return item;
    const nextCount = Math.max(0, toNumber(item.reply_count ?? item.replyCount) + delta);
    return { ...item, reply_count: nextCount, replyCount: nextCount };
  });
}

function normalizeAcceptedCommentId(value) {
  const id = toId(value);
  return id && id !== "0" ? id : "";
}

function acceptAnswerErrorMessage(error) {
  const message = error?.message || "";
  if (message.includes("TOPIC_COMMENT_ALREADY_ACCEPTED")) return "该问题已经采纳过答案。";
  if (message.includes("TOPIC_NOT_QUESTION")) return "只有问答内容可以采纳答案。";
  if (message.includes("TOPIC_ACCEPTED_COMMENT_INVALID")) return "这条评论暂时不能被采纳。";
  if (message.includes("COMMENT_NOT_IN_TOPIC")) return "这条评论不属于当前问题。";
  if (message.includes("COMMENT_NOT_FOUND")) return "没有找到要采纳的评论。";
  return message || "采纳答案失败";
}

function unacceptAnswerErrorMessage(error) {
  const message = error?.message || "";
  if (message.includes("TOPIC_QA_ACCEPTANCE_SETTLEMENT_PENDING")) return "悬赏正在结算，请稍后再撤销采纳。";
  if (message.includes("TOPIC_QA_ACCEPTANCE_REVERSAL_INSUFFICIENT_CREDIT")) return "答主当前积分不足，暂时无法撤销采纳。";
  if (message.includes("TOPIC_COMMENT_NOT_ACCEPTED")) return "当前答案未处于采纳状态。";
  return message || "撤销采纳失败";
}

function focusCommentEditor() {
  window.requestAnimationFrame(() => {
    document.getElementById("thread-comment-editor")?.focus();
  });
}

function scrollToComment(commentId) {
  window.requestAnimationFrame(() => {
    document.getElementById(`comment-${commentId}`)?.scrollIntoView({ behavior: "smooth", block: "center" });
  });
}

function readLastRead(kind, id) {
  if (!kind || !id || typeof localStorage === "undefined") return "";
  try {
    return localStorage.getItem(lastReadKey(kind, id)) || "";
  } catch {
    return "";
  }
}

function writeLastRead(kind, id, commentId) {
  if (!kind || !id || !commentId || typeof localStorage === "undefined") return;
  try {
    localStorage.setItem(lastReadKey(kind, id), String(commentId));
  } catch {
    // Reading position is a convenience hint only.
  }
}

function lastReadKey(kind, id) {
  return `bbs:thread:last-read:${kind}:${id}:v1`;
}
