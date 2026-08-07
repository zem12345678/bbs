import React from "react";
import { Check, Copy, LoaderCircle, LogOut, Megaphone, ShieldCheck, ShieldOff, Volume2, VolumeX, X } from "lucide-react";
import {
  canManageChatMemberRole,
  canMuteChatMember,
  chatMemberRole,
  chatMemberRoleLabel,
  isChatMemberMuted,
  isPermanentChatMute
} from "../../lib/chat.js";

function DialogShell({ title, children, onClose, labelledBy, wide = false }) {
  return (
    <div className="chat-dialog-overlay" role="presentation" onMouseDown={onClose}>
      <section className={`chat-dialog panel ${wide ? "chat-dialog--wide" : ""}`} role="dialog" aria-modal="true" aria-labelledby={labelledBy} onMouseDown={(event) => event.stopPropagation()}>
        <header>
          <h2 id={labelledBy}>{title}</h2>
          <button type="button" title="关闭" aria-label="关闭" onClick={onClose}>
            <X size={18} aria-hidden="true" />
          </button>
        </header>
        {children}
      </section>
    </div>
  );
}

function formatMemberTime(value) {
  const raw = Number(value || 0);
  if (!Number.isFinite(raw) || raw <= 0) return "";
  const millis = raw < 1_000_000_000_000 ? raw * 1000 : raw;
  return new Intl.DateTimeFormat("zh-CN", {
    year: "numeric",
    month: "2-digit",
    day: "2-digit",
    hour: "2-digit",
    minute: "2-digit"
  }).format(new Date(millis));
}

function memberName(member) {
  return member?.user?.name || member?.user?.nickname || member?.user?.username || `用户 ${member?.user_id || ""}`;
}

function muteStatus(member, now) {
  if (!isChatMemberMuted(member, now)) return "";
  if (isPermanentChatMute(member)) return "永久禁言";
  return `禁言至 ${formatMemberTime(member.muted_until)}`;
}

export function ChatMembersDialog({
  roomNo,
  members,
  total,
  loading,
  loadingMore,
  action,
  error,
  actorMembership,
  now,
  onClose,
  onLoadMore,
  onRoleChange,
  onMute,
  onUnmute
}) {
  const [muteDurations, setMuteDurations] = React.useState({});

  React.useEffect(() => {
    setMuteDurations({});
  }, [roomNo]);

  return (
    <DialogShell title="房间成员" labelledBy="chat-members-dialog-title" onClose={onClose} wide>
      <div className="chat-members-dialog">
        <div className="chat-member-summary">
          <span>全部成员</span>
          <b>{total}</b>
        </div>

        {error && <p className="chat-form-error" role="status">{error}</p>}
        {loading && members.length === 0 ? (
          <div className="chat-member-state"><LoaderCircle className="chat-spin" size={22} aria-hidden="true" />正在加载成员</div>
        ) : members.length === 0 ? (
          <div className="chat-member-state">暂无成员</div>
        ) : (
          <div className="chat-member-list">
            {members.map((member) => {
              const userId = String(member.user_id || "");
              const role = chatMemberRole(member);
              const muted = isChatMemberMuted(member, now);
              const canChangeRole = canManageChatMemberRole(actorMembership, member);
              const canMute = canMuteChatMember(actorMembership, member);
              const busy = Boolean(action);
              const memberBusy = action?.userId === userId;
              const duration = muteDurations[userId] || "3600000";
              return (
                <article className="chat-member-row" key={userId}>
                  {member.user?.avatar_url ? (
                    <img className="chat-avatar" src={member.user.avatar_url} alt="" />
                  ) : (
                    <span className="chat-avatar" aria-hidden="true">{memberName(member).slice(0, 1).toUpperCase()}</span>
                  )}
                  <div className="chat-member-row__identity">
                    <div>
                      <strong>{memberName(member)}</strong>
                      <span className={`chat-member-role chat-member-role--${role}`}>{chatMemberRoleLabel(member)}</span>
                    </div>
                    <small>
                      {member.user?.username ? `@${member.user.username}` : `ID ${userId}`}
                      {member.joined_at ? ` · ${formatMemberTime(member.joined_at)} 加入` : ""}
                    </small>
                    {muted && <em><VolumeX size={13} aria-hidden="true" />{muteStatus(member, now)}</em>}
                  </div>
                  {(canChangeRole || canMute) && (
                    <div className="chat-member-row__actions">
                      {canChangeRole && (
                        <button
                          type="button"
                          disabled={busy}
                          title={role === "manager" ? "取消管理员" : "设为管理员"}
                          onClick={() => onRoleChange(member, role === "manager" ? "member" : "manager")}
                        >
                          {memberBusy && action?.kind === "role" ? <LoaderCircle className="chat-spin" size={15} aria-hidden="true" /> : role === "manager" ? <ShieldOff size={15} aria-hidden="true" /> : <ShieldCheck size={15} aria-hidden="true" />}
                          {role === "manager" ? "取消管理员" : "设为管理员"}
                        </button>
                      )}
                      {canMute && (muted ? (
                        <button type="button" disabled={busy} onClick={() => onUnmute(member)}>
                          {memberBusy && action?.kind === "unmute" ? <LoaderCircle className="chat-spin" size={15} aria-hidden="true" /> : <Volume2 size={15} aria-hidden="true" />}
                          解除禁言
                        </button>
                      ) : (
                        <div className="chat-member-mute-action">
                          <select
                            aria-label={`选择对${memberName(member)}的禁言时长`}
                            disabled={busy}
                            value={duration}
                            onChange={(event) => setMuteDurations((current) => ({ ...current, [userId]: event.target.value }))}
                          >
                            <option value="3600000">1 小时</option>
                            <option value="86400000">24 小时</option>
                            <option value="604800000">7 天</option>
                            <option value="permanent">永久</option>
                          </select>
                          <button
                            type="button"
                            disabled={busy}
                            onClick={() => onMute(member, duration === "permanent" ? null : Date.now() + Number(duration))}
                          >
                            {memberBusy && action?.kind === "mute" ? <LoaderCircle className="chat-spin" size={15} aria-hidden="true" /> : <VolumeX size={15} aria-hidden="true" />}
                            禁言
                          </button>
                        </div>
                      ))}
                    </div>
                  )}
                </article>
              );
            })}
          </div>
        )}

        {members.length < total && (
          <button className="chat-member-load-more" type="button" disabled={loadingMore || Boolean(action)} onClick={onLoadMore}>
            {loadingMore && <LoaderCircle className="chat-spin" size={15} aria-hidden="true" />}
            {loadingMore ? "加载中..." : "加载更多成员"}
          </button>
        )}
      </div>
    </DialogShell>
  );
}

export function ChatRoomDialog({ mode, preview, loading, error, onClose, onLookup, onJoin, onCreate }) {
  const [roomNo, setRoomNo] = React.useState("");
  const [name, setName] = React.useState("");
  React.useEffect(() => {
    setRoomNo("");
    setName("");
  }, [mode]);

  return (
    <DialogShell title={mode === "create" ? "创建房间" : "加入房间"} labelledBy="chat-room-dialog-title" onClose={onClose}>
      {mode === "create" ? (
        <form className="chat-dialog__form" onSubmit={(event) => { event.preventDefault(); onCreate(name); }}>
          <label>
            房间名称
            <input autoFocus maxLength={80} placeholder="例如：产品讨论组" value={name} onChange={(event) => setName(event.target.value)} />
          </label>
          {error && <p className="chat-form-error">{error}</p>}
          <button className="chat-primary-btn" type="submit" disabled={loading || !name.trim()}>{loading ? "创建中..." : "创建并进入"}</button>
        </form>
      ) : (
        <div className="chat-dialog__form">
          <form className="chat-lookup-form" onSubmit={(event) => { event.preventDefault(); onLookup(roomNo); }}>
            <label>
              房间号
              <input autoFocus maxLength={8} placeholder="输入完整房间号" value={roomNo} onChange={(event) => setRoomNo(event.target.value.toUpperCase())} />
            </label>
            <button className="chat-primary-btn" type="submit" disabled={loading || !roomNo.trim()}>查找</button>
          </form>
          {preview && (
            <div className="chat-room-preview">
              <div>
                <strong>{preview.name}</strong>
                <span>{preview.room_no} · {preview.member_count || 0} 位成员</span>
              </div>
              <button className="chat-primary-btn" type="button" disabled={loading || preview.joined} onClick={() => onJoin(preview.room_no)}>
                {preview.joined ? "已加入" : loading ? "加入中..." : "加入房间"}
              </button>
            </div>
          )}
          {error && <p className="chat-form-error">{error}</p>}
        </div>
      )}
    </DialogShell>
  );
}

export function ChatAnnouncementDialog({ room, canEdit, loading, error, onClose, onSave, onSeen }) {
  const [editing, setEditing] = React.useState(false);
  const [announcement, setAnnouncement] = React.useState(room?.announcement || "");
  React.useEffect(() => {
    setAnnouncement(room?.announcement || "");
    setEditing(false);
  }, [room?.announcement, room?.announcement_version]);

  function close() {
    if (!editing) onSeen?.();
    onClose();
  }

  return (
    <DialogShell title="房间公告" labelledBy="chat-announcement-dialog-title" onClose={close}>
      <div className="chat-announcement-dialog">
        <div className="chat-announcement-dialog__icon"><Megaphone size={22} aria-hidden="true" /></div>
        {editing ? (
          <textarea maxLength={4000} value={announcement} onChange={(event) => setAnnouncement(event.target.value)} />
        ) : (
          <p>{room?.announcement || "房主还没有发布公告。"}</p>
        )}
        {error && <p className="chat-form-error">{error}</p>}
        <footer>
          {canEdit && (
            editing ? (
              <button className="chat-primary-btn" type="button" disabled={loading} onClick={() => onSave(announcement)}>
                <Check size={16} aria-hidden="true" />
                {loading ? "保存中..." : "保存公告"}
              </button>
            ) : (
              <button type="button" onClick={() => setEditing(true)}>编辑公告</button>
            )
          )}
          <button type="button" onClick={close}>知道了</button>
        </footer>
      </div>
    </DialogShell>
  );
}

export function ChatLeaveDialog({ room, loading, error, onClose, onConfirm }) {
  const close = () => {
    if (!loading) onClose();
  };

  return (
    <DialogShell title="离开房间" labelledBy="chat-leave-dialog-title" onClose={close}>
      <div className="chat-leave-dialog">
        <div className="chat-leave-dialog__icon"><LogOut size={22} aria-hidden="true" /></div>
        <p>离开“{room?.name || room?.room_no || "当前房间"}”后，它会从你的房间列表移除；房间和聊天记录不会删除，仍可通过房间号重新加入。</p>
        {error && <p className="chat-form-error">{error}</p>}
        <footer>
          <button type="button" disabled={loading} onClick={close}>取消</button>
          <button className="chat-primary-btn chat-primary-btn--danger" type="button" disabled={loading} onClick={onConfirm}>
            {loading ? "离开中..." : "确认离开"}
          </button>
        </footer>
      </div>
    </DialogShell>
  );
}

export function ChatShareDialog({ roomNo, onClose }) {
  const [copied, setCopied] = React.useState(false);
  const link = typeof window === "undefined" ? `/room/${roomNo}` : `${window.location.origin}/room/${roomNo}`;
  async function copy() {
    try {
      await navigator.clipboard.writeText(link);
      setCopied(true);
    } catch {
      setCopied(false);
    }
  }
  return (
    <DialogShell title="分享房间" labelledBy="chat-share-dialog-title" onClose={onClose}>
      <div className="chat-share-dialog">
        <p>把这个链接发给成员即可加入房间。</p>
        <code>{link}</code>
        <button className="chat-primary-btn" type="button" onClick={copy}>
          {copied ? <Check size={16} aria-hidden="true" /> : <Copy size={16} aria-hidden="true" />}
          {copied ? "已复制" : "复制链接"}
        </button>
      </div>
    </DialogShell>
  );
}
