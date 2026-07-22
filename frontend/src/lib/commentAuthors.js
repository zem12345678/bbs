import { listItems } from "./apiShapes.js";
import { toId } from "./formatters.js";

export const COMMENT_AUTHOR_BATCH_SIZE = 100;
const COMMENT_AUTHOR_BATCH_CONCURRENCY = 4;

export function collectMissingCommentAuthorIDs({
  comments = [],
  replyState = {},
  knownAuthors = {},
  currentUserID
} = {}) {
  const currentID = toId(currentUserID);
  const ids = new Set();
  const collect = (comment) => {
    const authorID = toId(comment?.author_id ?? comment?.authorId);
    if (!/^[1-9]\d*$/.test(authorID) || authorID === currentID || knownAuthors[authorID]) return;
    ids.add(authorID);
  };
  comments.forEach(collect);
  Object.values(replyState).forEach((state) => {
    (state?.items || []).forEach(collect);
  });
  return [...ids];
}

export async function loadCommentAuthors(ids, listUsers) {
  const batches = [];
  for (let start = 0; start < ids.length; start += COMMENT_AUTHOR_BATCH_SIZE) {
    batches.push(ids.slice(start, start + COMMENT_AUTHOR_BATCH_SIZE));
  }
  const results = new Array(batches.length);
  let nextBatchIndex = 0;
  const workers = Array.from(
    { length: Math.min(COMMENT_AUTHOR_BATCH_CONCURRENCY, batches.length) },
    async () => {
      for (;;) {
        const batchIndex = nextBatchIndex;
        nextBatchIndex += 1;
        const batch = batches[batchIndex];
        if (!batch) return;
        try {
          results[batchIndex] = listItems(await listUsers(batch));
        } catch {
          results[batchIndex] = [];
        }
      }
    }
  );
  await Promise.all(workers);
  return results.flat();
}
