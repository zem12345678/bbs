import React from "react";
import { Download, FileText, LoaderCircle, Paperclip, Save, Trash2 } from "lucide-react";

import { bbsApi } from "../../api";
import { listItems } from "../../lib/apiShapes";
import {
  isMembershipPaidAttachmentError,
  isPaidAttachmentSalesMembershipInactiveError
} from "../../lib/contentErrors";

const MAX_ATTACHMENT_SIZE = 50 * 1024 * 1024;

function emptyState() {
  return { items: [], priceDrafts: {}, loading: false, uploading: false, downloadingId: "", deletingId: "", savingPriceId: "", error: "", notice: "" };
}

export default function TopicAttachments({ auth, canManage = false, topicId }) {
  const [state, setState] = React.useState(emptyState);
  const [priceCredits, setPriceCredits] = React.useState("0");

  React.useEffect(() => {
    if (!topicId) {
      setState(emptyState());
      return undefined;
    }
    let alive = true;
    setState((current) => ({ ...current, loading: true, error: "", notice: "" }));
    bbsApi
      .listTopicAttachments(topicId)
      .then((data) => {
        if (!alive) return;
        const items = listItems(data);
        setState((current) => ({ ...current, items, priceDrafts: attachmentPriceDrafts(items), loading: false }));
      })
      .catch((error) => {
        if (!alive) return;
        setState((current) => ({ ...current, items: [], loading: false, error: error.message || "附件加载失败" }));
      });
    return () => {
      alive = false;
    };
  }, [topicId]);

  async function uploadAttachment(event) {
    const file = event.target.files?.[0];
    event.target.value = "";
    if (!file || !topicId || state.uploading) return;
    if (!auth?.accessToken) {
      setState((current) => ({ ...current, error: "请先登录后再上传附件。", notice: "" }));
      return;
    }
    if (file.size <= 0 || file.size > MAX_ATTACHMENT_SIZE) {
      setState((current) => ({ ...current, error: "附件大小需在 1 字节至 50 MiB 之间。", notice: "" }));
      return;
    }
    const price = parsePrice(priceCredits);
    if (price === null) {
      setState((current) => ({ ...current, error: "附件积分价格必须是非负整数。", notice: "" }));
      return;
    }
    setState((current) => ({ ...current, uploading: true, error: "", notice: "" }));
    try {
      const attachment = await bbsApi.uploadTopicAttachment(topicId, file, price, auth.accessToken);
      if (!attachment?.id) {
        throw new Error("附件上传成功但未返回附件信息");
      }
      setState((current) => ({
        ...current,
        items: [...current.items, attachment],
        uploading: false,
        notice: "附件已添加。"
      }));
    } catch (error) {
      setState((current) => ({ ...current, uploading: false, error: attachmentActionError(error, "附件上传失败") }));
    }
  }

  async function downloadAttachment(attachment) {
    if (!auth?.accessToken) {
      setState((current) => ({ ...current, error: "请先登录后再下载附件。", notice: "" }));
      return;
    }
    const attachmentId = String(attachment?.id || "");
    if (!attachmentId || state.downloadingId) return;
    setState((current) => ({ ...current, downloadingId: attachmentId, error: "", notice: "" }));
    try {
      const result = await bbsApi.downloadTopicAttachment(attachmentId, auth.accessToken);
      saveAttachment(result.blob, result.filename || attachment.original_name || `attachment-${attachmentId}`);
      setState((current) => ({ ...current, downloadingId: "", notice: "附件下载已开始。" }));
    } catch (error) {
      setState((current) => ({ ...current, downloadingId: "", error: attachmentActionError(error, "附件下载失败") }));
    }
  }

  async function archiveAttachment(attachment) {
    if (!auth?.accessToken || !canManage) return;
    const attachmentId = String(attachment?.id || "");
    if (!attachmentId || state.deletingId) return;
    setState((current) => ({ ...current, deletingId: attachmentId, error: "", notice: "" }));
    try {
      await bbsApi.archiveTopicAttachment(attachmentId, auth.accessToken);
      setState((current) => ({
        ...current,
        deletingId: "",
        items: current.items.filter((item) => String(item?.id) !== attachmentId),
        notice: "附件已归档。"
      }));
    } catch (error) {
      setState((current) => ({ ...current, deletingId: "", error: error.message || "附件归档失败" }));
    }
  }

  async function updateAttachmentPrice(attachment) {
    if (!auth?.accessToken || !canManage) return;
    const attachmentId = String(attachment?.id || "");
    if (!attachmentId || state.savingPriceId) return;
    const currentPrice = Number(attachment?.price_credits ?? attachment?.priceCredits) || 0;
    const price = parsePrice(state.priceDrafts[attachmentId] ?? String(currentPrice));
    if (price === null) {
      setState((current) => ({ ...current, error: "附件积分价格必须是非负整数。", notice: "" }));
      return;
    }
    setState((current) => ({ ...current, savingPriceId: attachmentId, error: "", notice: "" }));
    try {
      const updated = await bbsApi.updateTopicAttachmentPrice(attachmentId, price, auth.accessToken);
      if (!updated?.id) {
        throw new Error("附件价格已保存但未返回附件信息");
      }
      const updatedPrice = Number(updated.price_credits ?? updated.priceCredits) || 0;
      setState((current) => ({
        ...current,
        savingPriceId: "",
        items: current.items.map((item) => (String(item?.id) === attachmentId ? { ...item, ...updated } : item)),
        priceDrafts: { ...current.priceDrafts, [attachmentId]: String(updatedPrice) },
        notice: "附件积分价格已更新。"
      }));
    } catch (error) {
      setState((current) => ({ ...current, savingPriceId: "", error: attachmentActionError(error, "附件积分价格更新失败") }));
    }
  }

  if (!topicId) return null;

  return (
    <section className="topic-attachments" aria-label="主题附件">
      <header className="topic-attachments-head">
        <div>
          <Paperclip size={18} aria-hidden="true" />
          <h2>附件</h2>
          {!state.loading && <span>{state.items.length}</span>}
        </div>
        {canManage && (
          <div className="topic-attachment-upload">
            <label>
              <span>积分</span>
              <input
                aria-label="附件积分价格"
                min="0"
                step="1"
                type="number"
                value={priceCredits}
                onChange={(event) => setPriceCredits(event.target.value)}
              />
            </label>
            <label className="topic-attachment-upload-button">
              <input className="sr-only" disabled={state.uploading} type="file" onChange={uploadAttachment} />
              {state.uploading ? <LoaderCircle className="is-spinning" size={16} aria-hidden="true" /> : <Paperclip size={16} aria-hidden="true" />}
              <span>{state.uploading ? "上传中" : "上传附件"}</span>
            </label>
          </div>
        )}
      </header>
      {state.loading && <p className="topic-attachments-empty">正在加载附件...</p>}
      {!state.loading && state.items.length === 0 && <p className="topic-attachments-empty">暂无附件</p>}
      {!state.loading && state.items.length > 0 && (
        <div className="topic-attachment-list">
          {state.items.map((attachment) => {
            const attachmentId = String(attachment?.id || "");
            const price = Number(attachment?.price_credits ?? attachment?.priceCredits) || 0;
            const downloading = state.downloadingId === attachmentId;
            const deleting = state.deletingId === attachmentId;
            const savingPrice = state.savingPriceId === attachmentId;
            const priceDraft = state.priceDrafts[attachmentId] ?? String(price);
            return (
              <article className="topic-attachment-row" key={attachmentId}>
                <FileText size={20} aria-hidden="true" />
                <div className="topic-attachment-main">
                  <strong title={attachment?.original_name}>{attachment?.original_name || "未命名附件"}</strong>
                  <span>
                    {formatBytes(attachment?.size_bytes ?? attachment?.sizeBytes)}
                    {price > 0 ? ` · ${price} 积分` : " · 免费"}
                  </span>
                </div>
                <div className="topic-attachment-actions">
                  {canManage && (
                    <div className="topic-attachment-price">
                      <input
                        aria-label={`${attachment?.original_name || attachmentId} 的积分价格`}
                        disabled={Boolean(state.savingPriceId) || deleting}
                        min="0"
                        step="1"
                        type="number"
                        value={priceDraft}
                        onChange={(event) =>
                          setState((current) => ({
                            ...current,
                            priceDrafts: { ...current.priceDrafts, [attachmentId]: event.target.value }
                          }))
                        }
                      />
                      <button
                        aria-label={`保存附件 ${attachment?.original_name || attachmentId} 的积分价格`}
                        disabled={Boolean(state.savingPriceId) || deleting}
                        title="保存积分价格"
                        type="button"
                        onClick={() => updateAttachmentPrice(attachment)}
                      >
                        {savingPrice ? <LoaderCircle className="is-spinning" size={16} aria-hidden="true" /> : <Save size={16} aria-hidden="true" />}
                      </button>
                    </div>
                  )}
                  <button disabled={Boolean(state.downloadingId) || deleting || savingPrice} type="button" onClick={() => downloadAttachment(attachment)}>
                    {downloading ? <LoaderCircle className="is-spinning" size={16} aria-hidden="true" /> : <Download size={16} aria-hidden="true" />}
                    {downloading ? "下载中" : "下载"}
                  </button>
                  {canManage && (
                    <button
                      aria-label={`归档附件 ${attachment?.original_name || attachmentId}`}
                      className="topic-attachment-remove"
                      disabled={Boolean(state.deletingId) || downloading || savingPrice}
                      title="归档附件"
                      type="button"
                      onClick={() => archiveAttachment(attachment)}
                    >
                      {deleting ? <LoaderCircle className="is-spinning" size={16} aria-hidden="true" /> : <Trash2 size={16} aria-hidden="true" />}
                    </button>
                  )}
                </div>
              </article>
            );
          })}
        </div>
      )}
      {state.error && <p className="form-error topic-attachments-message">{state.error}</p>}
      {state.notice && <p className="form-success topic-attachments-message">{state.notice}</p>}
    </section>
  );
}

function parsePrice(value) {
  const text = String(value ?? "").trim();
  if (!/^\d+$/.test(text)) return null;
  const price = Number(text);
  return Number.isSafeInteger(price) ? price : null;
}

function attachmentActionError(error, fallback) {
  if (isPaidAttachmentSalesMembershipInactiveError(error)) {
    return "该付费附件作者的会员权益已失效，暂时无法购买。";
  }
  if (isMembershipPaidAttachmentError(error)) {
    return "付费附件需要有效会员权益，请先兑换会员后再设置价格。";
  }
  return error?.message || fallback;
}

function attachmentPriceDrafts(items) {
  return Object.fromEntries(
    items.map((attachment) => {
      const price = Number(attachment?.price_credits ?? attachment?.priceCredits) || 0;
      return [String(attachment?.id || ""), String(price)];
    })
  );
}

function formatBytes(value) {
  const bytes = Number(value);
  if (!Number.isFinite(bytes) || bytes < 0) return "大小未知";
  if (bytes < 1024) return `${bytes} B`;
  if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)} KB`;
  return `${(bytes / (1024 * 1024)).toFixed(1)} MB`;
}

function saveAttachment(blob, filename) {
  const url = URL.createObjectURL(blob);
  const link = document.createElement("a");
  link.href = url;
  link.download = filename;
  document.body.appendChild(link);
  link.click();
  link.remove();
  window.setTimeout(() => URL.revokeObjectURL(url), 0);
}
