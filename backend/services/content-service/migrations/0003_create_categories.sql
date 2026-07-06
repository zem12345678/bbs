CREATE TABLE IF NOT EXISTS categories (
  id BIGINT PRIMARY KEY,
  slug VARCHAR(128) NOT NULL UNIQUE,
  name VARCHAR(80) NOT NULL,
  description TEXT NOT NULL DEFAULT '',
  sort INTEGER NOT NULL DEFAULT 0,
  status SMALLINT NOT NULL DEFAULT 2,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

INSERT INTO categories (id, slug, name, description, sort, status)
VALUES
  (1, 'general', '综合讨论', '默认社区分类，用于承接未归类的话题和动态。', 0, 2),
  (2, 'engineering', '技术交流', '后端、前端、架构和工程效率相关讨论。', 10, 2),
  (3, 'product', '产品体验', '产品设计、用户体验和社区运营讨论。', 20, 2),
  (4, 'resources', '资源分享', '模板、工具、文档和学习资料分享。', 30, 2)
ON CONFLICT (id) DO NOTHING;

ALTER TABLE topics
  ADD COLUMN IF NOT EXISTS category_id BIGINT NOT NULL DEFAULT 1;

CREATE INDEX IF NOT EXISTS idx_topics_category_status_published
  ON topics(category_id, status, published_at DESC);
