import React from "react";
import { ChevronDown, ChevronUp, FolderPlus, Link2, MoreHorizontal, Pencil, Plus, Search, Trash2, Users } from "lucide-react";
import { chatId, chatInteger, chatUserName, compareChatIntegers, groupedChatRooms, orderedChatGroups } from "../../lib/chat";
import { timeAgoMillis } from "../../lib/formatters";

function displayRoomName(room) {
  return room?.room?.name || room?.room_no || "未命名房间";
}

function displayLastMessage(room, users) {
  const message = room?.last_message;
  if (Number(message?.status) === 2) return "这条消息已删除";
  if (!message?.body) return "还没有消息";
  const sender = users.get(chatId(message.sender_id)) || null;
  const prefix = sender ? `${chatUserName(sender)}：` : "";
  return `${prefix}${message.body}`;
}

function displayLastMessageTime(room) {
  const createdAt = room?.last_message?.created_at;
  return createdAt ? timeAgoMillis(createdAt) : "";
}

function ChatUserAvatar({ user, size = "normal" }) {
  const name = chatUserName(user);
  return user?.avatar_url ? (
    <img className={`chat-avatar chat-avatar--${size}`} src={user.avatar_url} alt="" />
  ) : (
    <span className={`chat-avatar chat-avatar--${size} chat-avatar--fallback`} aria-hidden="true">
      {name.slice(0, 1)}
    </span>
  );
}

export { ChatUserAvatar };

export default function ChatSidebar({
  groups,
  rooms,
  users,
  activeRoomNo,
  manageMode,
  onSelectRoom,
  onToggleManage,
  onCreateGroup,
  groupEditor,
  groupName,
  groupSaving,
  placementSaving,
  onGroupNameChange,
  onSubmitGroup,
  onCancelGroup,
  editingGroupId,
  editingGroupName,
  deletingGroupId,
  onStartEditGroup,
  onEditGroupNameChange,
  onSubmitGroupEdit,
  onCancelGroupEdit,
  onMoveGroup,
  onRequestDeleteGroup,
  onConfirmDeleteGroup,
  onCancelDeleteGroup,
  onPlaceRoom,
  onOpenRoomDialog,
  loading
}) {
  const [, refreshRelativeTime] = React.useState(0);
  const sections = groupedChatRooms(groups, rooms);
  const sortedGroups = orderedChatGroups(groups);
  const userMap = users instanceof Map ? users : new Map();

  React.useEffect(() => {
    const intervalId = window.setInterval(() => refreshRelativeTime((value) => value + 1), 60_000);
    return () => window.clearInterval(intervalId);
  }, []);

  return (
    <aside className="chat-sidebar panel" aria-label="聊天房间">
      <header className="chat-sidebar__head">
        <div>
          <span className="chat-eyebrow">实时通讯</span>
          <h1>我的房间</h1>
          <p>{rooms.length ? `${rooms.length} 个房间` : "从一个房间开始"}</p>
        </div>
        <div className="chat-sidebar__actions">
          <button type="button" title="查找或创建房间" aria-label="查找或创建房间" onClick={() => onOpenRoomDialog("join")}>
            <Search size={17} aria-hidden="true" />
          </button>
          <button type="button" title="创建房间" aria-label="创建房间" onClick={() => onOpenRoomDialog("create")}>
            <Plus size={17} aria-hidden="true" />
          </button>
          <button
            className={manageMode ? "is-active" : ""}
            type="button"
            title={manageMode ? "完成整理" : "整理房间"}
            aria-label={manageMode ? "完成整理" : "整理房间"}
            onClick={onToggleManage}
          >
            <MoreHorizontal size={17} aria-hidden="true" />
          </button>
        </div>
      </header>

      {loading && <div className="chat-sidebar__loading">正在同步房间...</div>}
      {groupEditor && (
        <form className="chat-group-form" onSubmit={onSubmitGroup}>
          <input autoFocus maxLength={40} placeholder="分组名称" value={groupName} onChange={(event) => onGroupNameChange(event.target.value)} />
          <button type="submit" disabled={groupSaving || !groupName.trim()}>保存</button>
          <button type="button" onClick={onCancelGroup}>取消</button>
        </form>
      )}
      {!loading && sections.length === 0 && (
        <div className="chat-sidebar__empty">
          <Users size={26} aria-hidden="true" />
          <strong>还没有加入房间</strong>
          <span>通过房间号加入一个讨论空间。</span>
          <button type="button" onClick={() => onOpenRoomDialog("join")}>
            <Link2 size={16} aria-hidden="true" />
            输入房间号
          </button>
        </div>
      )}

      <div className="chat-sidebar__sections">
        {sections.map(({ group, rooms: sectionRooms }) => (
          <section className="chat-room-section" key={group ? chatId(group.id) : "ungrouped"}>
            <header>
              {group && chatId(group.id) === editingGroupId ? (
                <form className="chat-group-inline-form" onSubmit={(event) => onSubmitGroupEdit(event, group)}>
                  <input autoFocus maxLength={40} aria-label="分组名称" value={editingGroupName} onChange={(event) => onEditGroupNameChange(event.target.value)} />
                  <button type="submit" disabled={groupSaving || !editingGroupName.trim()}>保存</button>
                  <button type="button" disabled={groupSaving} onClick={onCancelGroupEdit}>取消</button>
                </form>
              ) : (
                <>
                  <span>{group?.name || "未分组"}</span>
                  <em>{sectionRooms.length}</em>
                </>
              )}
              {!group && !editingGroupId && (
                <button type="button" title="创建分组" aria-label="创建分组" onClick={onCreateGroup}>
                  <FolderPlus size={14} aria-hidden="true" />
                </button>
              )}
              {group && manageMode && chatId(group.id) !== editingGroupId && (
                <div className="chat-group-actions">
                  {chatId(group.id) === deletingGroupId ? (
                    <>
                      <span>删除分组？</span>
                      <button type="button" title="确认删除分组" aria-label={`确认删除分组 ${group.name}`} disabled={groupSaving} onClick={() => onConfirmDeleteGroup(group)}>
                        删除
                      </button>
                      <button type="button" title="取消删除分组" aria-label={`取消删除分组 ${group.name}`} disabled={groupSaving} onClick={onCancelDeleteGroup}>
                        取消
                      </button>
                    </>
                  ) : (
                    <>
                      <button type="button" title="上移分组" aria-label={`上移分组 ${group.name}`} disabled={groupSaving || sortedGroups[0] === group} onClick={() => onMoveGroup(group, -1)}>
                        <ChevronUp size={14} aria-hidden="true" />
                      </button>
                      <button type="button" title="下移分组" aria-label={`下移分组 ${group.name}`} disabled={groupSaving || sortedGroups.at(-1) === group} onClick={() => onMoveGroup(group, 1)}>
                        <ChevronDown size={14} aria-hidden="true" />
                      </button>
                      <button type="button" title="重命名分组" aria-label={`重命名分组 ${group.name}`} disabled={groupSaving} onClick={() => onStartEditGroup(group)}>
                        <Pencil size={14} aria-hidden="true" />
                      </button>
                      <button type="button" title="删除分组" aria-label={`删除分组 ${group.name}`} disabled={groupSaving} onClick={() => onRequestDeleteGroup(group)}>
                        <Trash2 size={14} aria-hidden="true" />
                      </button>
                    </>
                  )}
                </div>
              )}
            </header>
            <div className="chat-room-list">
              {sectionRooms.map((item, roomIndex) => {
                const itemRoomNo = item.room_no || item.room?.room_no;
                const active = itemRoomNo === activeRoomNo;
                const unread = chatInteger(item.unread_count);
                const lastSender = userMap.get(chatId(item.last_message?.sender_id));
                const lastMessageTime = displayLastMessageTime(item);
                const groupId = chatId(item.membership?.group_id || "0");
                return (
                  <div className={`chat-room-row ${active ? "is-active" : ""}`} key={itemRoomNo || chatId(item.room?.id)}>
                    <button className="chat-room-row__main" type="button" title={`房间号：${itemRoomNo}`} onClick={() => onSelectRoom(itemRoomNo)}>
                      <ChatUserAvatar user={lastSender} size="small" />
                      <span className="chat-room-row__copy">
                        <strong>{displayRoomName(item)}</strong>
                        <small>{displayLastMessage(item, userMap)}</small>
                      </span>
                      <span className="chat-room-row__meta">
                        {lastMessageTime && <time>{lastMessageTime}</time>}
                        {compareChatIntegers(unread, "0") > 0 && <b>{compareChatIntegers(unread, "99") > 0 ? "99+" : unread}</b>}
                      </span>
                    </button>
                    {manageMode && (
                      <div className="chat-room-actions">
                        <button
                          type="button"
                          title="上移房间"
                          aria-label={`上移房间 ${displayRoomName(item)}`}
                          disabled={placementSaving || roomIndex === 0}
                          onClick={() => onPlaceRoom(itemRoomNo, groupId, roomIndex - 1)}
                        >
                          <ChevronUp size={14} aria-hidden="true" />
                        </button>
                        <button
                          type="button"
                          title="下移房间"
                          aria-label={`下移房间 ${displayRoomName(item)}`}
                          disabled={placementSaving || roomIndex === sectionRooms.length - 1}
                          onClick={() => onPlaceRoom(itemRoomNo, groupId, roomIndex + 1)}
                        >
                          <ChevronDown size={14} aria-hidden="true" />
                        </button>
                        <select
                          aria-label={`移动${displayRoomName(item)}`}
                          disabled={placementSaving}
                          value={groupId}
                          onChange={(event) => onPlaceRoom(itemRoomNo, event.target.value)}
                        >
                          <option value="0">未分组</option>
                          {groups.map((availableGroup) => (
                            <option key={chatId(availableGroup.id)} value={chatId(availableGroup.id)}>
                              {availableGroup.name}
                            </option>
                          ))}
                        </select>
                      </div>
                    )}
                  </div>
                );
              })}
            </div>
          </section>
        ))}
      </div>
    </aside>
  );
}
