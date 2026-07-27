import React from "react";
import { ArrowDown, ChevronDown, LoaderCircle, Trash2 } from "lucide-react";
import { chatId, chatMessageSeq, chatUserName } from "../../lib/chat";
import { timeAgoMillis } from "../../lib/formatters";
import { ChatUserAvatar } from "./ChatSidebar.jsx";

function messageTime(message) {
  return message?.created_at ? timeAgoMillis(message.created_at) : "刚刚";
}

export default function ChatTimeline({
  messages,
  users,
  currentUserId,
  unreadIndex,
  hasOlder,
  loadingOlder,
  hasNewer,
  loadingNewer,
  loading,
  scrollRef,
  onScroll,
  onLoadOlder,
  onLoadNewer,
  onJumpLatest,
  deletingMessageId,
  onDeleteMessage
}) {
  const userMap = users instanceof Map ? users : new Map();
  return (
    <section className="chat-timeline panel" aria-label="消息记录">
      <div className="chat-timeline__tools">
        <button className="chat-timeline__history-action" type="button" disabled={!hasOlder || loadingOlder} onClick={onLoadOlder}>
          {loadingOlder ? <LoaderCircle className="chat-spin" size={15} aria-hidden="true" /> : <ChevronDown size={15} aria-hidden="true" />}
          {loadingOlder ? "加载中" : hasOlder ? "更早消息" : "已到最早"}
        </button>
        <button className="chat-timeline__history-action" type="button" disabled={!hasNewer || loadingNewer} onClick={onLoadNewer}>
          {loadingNewer ? <LoaderCircle className="chat-spin" size={15} aria-hidden="true" /> : <ArrowDown size={15} aria-hidden="true" />}
          {loadingNewer ? "加载中" : hasNewer ? "更新消息" : "已是最新"}
        </button>
        <button className="chat-timeline__jump-latest" type="button" title="跳到最新消息" aria-label="跳到最新消息" onClick={onJumpLatest}>
          <ArrowDown size={16} aria-hidden="true" />
        </button>
      </div>
      <div className="chat-message-list" ref={scrollRef} onScroll={onScroll}>
        {loading && <div className="chat-message-state">正在加载消息...</div>}
        {!loading && messages.length === 0 && <div className="chat-message-state">从这里开始一段新的讨论。</div>}
        {!loading &&
          messages.map((message, index) => {
            const sender = userMap.get(chatId(message.sender_id)) || message.sender;
            const mine = chatId(message.sender_id) === chatId(currentUserId);
            const deleted = Number(message.status) === 2;
            return (
              <React.Fragment key={message.id || `${message.client_message_id}-${message.seq}`}>
                {index === unreadIndex && <div className="chat-unread-divider" data-unread-separator="true">未读消息</div>}
                <article className={`chat-message ${mine ? "is-mine" : ""} ${message.pending ? "is-pending" : ""}`} data-seq={message.seq || undefined}>
                  {!mine && <ChatUserAvatar user={sender} size="normal" />}
                  <div className="chat-message__body">
                    <header>
                      <strong>{mine ? "我" : chatUserName(sender)}</strong>
                      <time>{messageTime(message)}</time>
                      {mine && !message.pending && !deleted && message.id && (
                        <button
                          className="chat-message__delete"
                          type="button"
                          title="删除消息"
                          aria-label="删除消息"
                          disabled={chatId(deletingMessageId) === chatId(message.id)}
                          onClick={() => onDeleteMessage(message)}
                        >
                          {chatId(deletingMessageId) === chatId(message.id)
                            ? <LoaderCircle className="chat-spin" size={13} aria-hidden="true" />
                            : <Trash2 size={13} aria-hidden="true" />}
                        </button>
                      )}
                    </header>
                    <p>{deleted ? "这条消息已删除" : message.body}</p>
                    {message.pending && <small className="chat-message__pending">发送中...</small>}
                  </div>
                </article>
              </React.Fragment>
            );
          })}
      </div>
    </section>
  );
}
