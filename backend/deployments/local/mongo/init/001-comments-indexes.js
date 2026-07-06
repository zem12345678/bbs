db = db.getSiblingDB("bbs_comment");

db.comments.createIndex(
  { entityType: 1, entityId: 1, parentId: 1, status: 1, createdAt: -1 },
  { name: "idx_comments_entity_parent_status_created" }
);

db.comments.createIndex(
  { rootId: 1, status: 1, createdAt: 1 },
  { name: "idx_comments_root_status_created" }
);

db.comments.createIndex(
  { authorId: 1, createdAt: -1 },
  { name: "idx_comments_author_created" }
);

db.comments.createIndex(
  { quoteId: 1 },
  { name: "idx_comments_quote" }
);

db.comment_audit_logs.createIndex(
  { commentId: 1, createdAt: -1 },
  { name: "idx_comment_audit_comment_created" }
);

db.comment_audit_logs.createIndex(
  { actorId: 1, createdAt: -1 },
  { name: "idx_comment_audit_actor_created" }
);
