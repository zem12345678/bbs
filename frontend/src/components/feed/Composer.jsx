import React from "react";
import { ChevronDown, Hash, Image, Link2, Smile, Vote, Zap } from "lucide-react";
import { bbsApi } from "../../api";
import { toNumber } from "../../lib/formatters";
import { makeSlug } from "../../lib/slugs";

export default function Composer({ auth, categories = [], onPublished }) {
  const tools = [
    { label: "表情", icon: Smile },
    { label: "图片", icon: Image },
    { label: "链接", icon: Link2 },
    { label: "话题", icon: Hash },
    { label: "投票", icon: Vote }
  ];
  const [title, setTitle] = React.useState("");
  const [body, setBody] = React.useState("");
  const [tagText, setTagText] = React.useState("");
  const [selectedCategoryId, setSelectedCategoryId] = React.useState(0);
  const [submitting, setSubmitting] = React.useState(false);
  const [error, setError] = React.useState("");

  React.useEffect(() => {
    if (categories.length === 0) return;
    if (!selectedCategoryId || !categories.some((category) => category.id === selectedCategoryId)) {
      setSelectedCategoryId(categories[0].id);
    }
  }, [categories, selectedCategoryId]);

  async function submit(event) {
    event.preventDefault();
    if (!auth?.accessToken) {
      setError("请先登录后再发布。");
      return;
    }
    const content = body.trim();
    const finalTitle = title.trim() || content.slice(0, 28) || "未命名帖子";
    if (!content) {
      setError("请输入正文内容。");
      return;
    }
    setSubmitting(true);
    setError("");
    try {
      const tags = tagText
        .split(/[,，\s#]+/)
        .map((tag) => tag.trim())
        .filter(Boolean)
        .slice(0, 6);
      const data = await bbsApi.createTopic(
        {
          slug: makeSlug(finalTitle),
          type: "topic",
          title: finalTitle,
          body: content,
          tags,
          category_id: selectedCategoryId || undefined,
          publish: true
        },
        auth.accessToken
      );
      if (data?.topic) {
        onPublished(data.topic);
      }
      setTitle("");
      setBody("");
      setTagText("");
    } catch (submitError) {
      setError(submitError.message || "发布失败");
    } finally {
      setSubmitting(false);
    }
  }

  return (
    <form className="composer panel" onSubmit={submit}>
      <div className="compose-box">
        <input
          className="compose-title"
          placeholder="给帖子起个标题"
          value={title}
          onChange={(event) => setTitle(event.target.value)}
        />
        <textarea
          maxLength={1000}
          placeholder="聊聊新鲜事，分享图片、链接或发起投票..."
          value={body}
          onChange={(event) => setBody(event.target.value)}
        />
        <input
          className="compose-tags"
          placeholder="添加话题标签，用空格或逗号分隔"
          value={tagText}
          onChange={(event) => setTagText(event.target.value)}
        />
        <label className="circle-picker">
          <Zap size={15} aria-hidden="true" />
          <select
            aria-label="关联分类"
            disabled={categories.length === 0}
            value={selectedCategoryId}
            onChange={(event) => setSelectedCategoryId(toNumber(event.target.value))}
          >
            {categories.length === 0 ? (
              <option value={0}>默认分类</option>
            ) : (
              categories.map((category) => (
                <option key={category.id} value={category.id}>
                  {category.name}
                </option>
              ))
            )}
          </select>
          <ChevronDown size={14} aria-hidden="true" />
        </label>
      </div>
      {error && <p className="form-error compose-error">{error}</p>}
      <div className="compose-footer">
        <div className="compose-tools">
          {tools.map(({ label, icon: Icon }) => (
            <button type="button" key={label}>
              <Icon size={20} aria-hidden="true" />
              {label}
            </button>
          ))}
        </div>
        <div className="publish-group">
          <span>{body.length}/1000</span>
          <button type="submit" disabled={submitting}>
            {submitting ? "发布中" : "发布"}
          </button>
        </div>
      </div>
    </form>
  );
}
