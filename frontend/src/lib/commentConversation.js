import { listItems } from "./apiShapes.js";
import { sameId, toId, toNumber } from "./formatters.js";

export function commentRootId(comment) {
  const rootId = comment?.root_id ?? comment?.rootId;
  return toNumber(rootId) > 0 ? toId(rootId) : toId(comment?.id);
}

export function commentReplyTargets(comment) {
  if (!comment) return { parentId: 0, rootId: 0 };
  return {
    parentId: toId(comment.id) || 0,
    rootId: commentRootId(comment) || 0
  };
}

export function isNestedReply(comment) {
  const parentId = toId(comment?.parent_id ?? comment?.parentId);
  const rootId = commentRootId(comment);
  return toNumber(parentId) > 0 && Boolean(rootId) && !sameId(parentId, rootId);
}

export function conversationItems(data) {
  const items = Array.isArray(data) ? data : listItems(data);
  return [...items].reverse();
}
