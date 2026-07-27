const MAX_USER_BATCH = 100;

export function chatId(value) {
  return value === undefined || value === null || value === "" ? "" : String(value);
}

export function chatInteger(value) {
  const text = chatId(value).trim();
  if (!/^\d+$/.test(text)) return "0";
  return text.replace(/^0+(?=\d)/, "");
}

export function compareChatIntegers(left, right) {
  const leftValue = BigInt(chatInteger(left));
  const rightValue = BigInt(chatInteger(right));
  return leftValue === rightValue ? 0 : leftValue > rightValue ? 1 : -1;
}

export function maxChatInteger(left, right) {
  return compareChatIntegers(left, right) >= 0 ? chatInteger(left) : chatInteger(right);
}

export function subtractChatIntegers(left, right) {
  const result = BigInt(chatInteger(left)) - BigInt(chatInteger(right));
  return result > 0n ? result.toString() : "0";
}

export function chatRoomNo(value) {
  return String(value || "").trim().toUpperCase();
}

export function isCurrentChatRoomRequest(requestRoomNo, activeRoomNo) {
  const requestRoom = chatRoomNo(requestRoomNo);
  return Boolean(requestRoom) && requestRoom === chatRoomNo(activeRoomNo);
}

export function isCurrentChatRoomSessionRequest(requestRoomNo, requestSession, activeRoomNo, activeSession) {
  return isCurrentChatRoomRequest(requestRoomNo, activeRoomNo) && requestSession === activeSession;
}

export function chatUserName(user) {
  return user?.nickname || user?.username || (user?.id ? `用户 ${chatId(user.id)}` : "社区成员");
}

export function normalizeChatUser(user) {
  if (!user || !chatId(user.id)) return null;
  return {
    id: chatId(user.id),
    username: user.username || "",
    nickname: user.nickname || "",
    avatar_url: user.avatar_url || user.avatarUrl || "",
    name: chatUserName(user)
  };
}

export function indexChatUsers(users = [], current = new Map()) {
  const result = new Map(current);
  for (const user of users) {
    const normalized = normalizeChatUser(user);
    if (normalized) result.set(normalized.id, normalized);
  }
  return result;
}

export function normalizeChatRoom(room) {
  if (!room) return null;
  return {
    ...room,
    id: chatId(room.id),
    room_no: chatRoomNo(room.room_no ?? room.roomNo),
    creator_id: chatId(room.creator_id ?? room.creatorId),
    announcement_version: chatInteger(room.announcement_version ?? room.announcementVersion),
    last_message_seq: chatInteger(room.last_message_seq ?? room.lastMessageSeq),
    created_at: chatId(room.created_at ?? room.createdAt),
    updated_at: chatId(room.updated_at ?? room.updatedAt)
  };
}

export function normalizeChatMembership(membership) {
  if (!membership) return null;
  return {
    ...membership,
    room_id: chatId(membership.room_id ?? membership.roomId),
    user_id: chatId(membership.user_id ?? membership.userId),
    joined_at_seq: chatInteger(membership.joined_at_seq ?? membership.joinedAtSeq),
    last_read_seq: chatInteger(membership.last_read_seq ?? membership.lastReadSeq),
    last_seen_announcement_version: chatInteger(
      membership.last_seen_announcement_version ?? membership.lastSeenAnnouncementVersion
    ),
    group_id: chatId(membership.group_id ?? membership.groupId),
    joined_at: chatId(membership.joined_at ?? membership.joinedAt),
    left_at: chatId(membership.left_at ?? membership.leftAt),
    created_at: chatId(membership.created_at ?? membership.createdAt),
    updated_at: chatId(membership.updated_at ?? membership.updatedAt)
  };
}

export function normalizeChatGroup(group) {
  if (!group) return null;
  return {
    ...group,
    id: chatId(group.id),
    user_id: chatId(group.user_id ?? group.userId),
    created_at: chatId(group.created_at ?? group.createdAt),
    updated_at: chatId(group.updated_at ?? group.updatedAt)
  };
}

export function normalizeChatSidebar(data = {}) {
  const groups = Array.isArray(data.groups) ? data.groups.map(normalizeChatGroup).filter(Boolean) : [];
  const rooms = Array.isArray(data.rooms)
    ? data.rooms
        .filter(Boolean)
        .map((item) => ({
          ...item,
          room: normalizeChatRoom(item.room) || {},
          membership: normalizeChatMembership(item.membership) || {},
          last_message: item.last_message ? normalizeChatMessage(item.last_message) : null,
          unread_count: chatInteger(item.unread_count),
          room_no: chatRoomNo(item.room?.room_no ?? item.room_no)
        }))
    : [];
  return { groups, rooms, users: Array.isArray(data.users) ? data.users : [] };
}

export function normalizeChatDetails(data = {}) {
  const details = data.details || data;
  return {
    room: normalizeChatRoom(details.room),
    membership: normalizeChatMembership(details.membership),
    member_count: chatInteger(details.member_count),
    users: Array.isArray(data.users) ? data.users : []
  };
}

export function normalizeChatMessage(raw = {}, userMap = new Map()) {
  const message = raw?.message || raw || {};
  const senderId = chatId(message.sender_id ?? message.senderId);
  const id = chatId(message.id ?? message.message_id);
  const seq = message.seq === undefined || message.seq === null || message.seq === "" ? "" : chatInteger(message.seq);
  const clientMessageId = chatId(message.client_message_id ?? message.clientMessageId);
  const sender = userMap.get(senderId);
  return {
    ...message,
    id,
    room_id: chatId(message.room_id ?? message.roomId),
    room_no: chatRoomNo(message.room_no ?? message.roomNo),
    seq,
    sender_id: senderId,
    client_message_id: clientMessageId,
    body: String(message.body || ""),
    created_at: chatId(message.created_at ?? message.createdAt),
    updated_at: chatId(message.updated_at ?? message.updatedAt),
    deleted_at: chatId(message.deleted_at ?? message.deletedAt),
    sender
  };
}

export function mergeChatMessages(existing = [], incoming = [], userMap = new Map()) {
  const byKey = new Map();
  for (const message of [...existing, ...incoming]) {
    const normalized = normalizeChatMessage(message, userMap);
    const key = normalized.client_message_id
      ? `client:${normalized.client_message_id}`
      : normalized.id
        ? `id:${normalized.id}`
        : `${normalized.room_id}:${normalized.seq}`;
    if (!key || key === ":0") continue;
    const previous = byKey.get(key);
    byKey.set(key, previous ? { ...previous, ...normalized, pending: previous.pending && normalized.pending } : normalized);
  }
  return [...byKey.values()].sort((left, right) => {
    if (left.pending && !left.seq) return right.pending && !right.seq ? 0 : 1;
    if (right.pending && !right.seq) return -1;
    return compareChatIntegers(left.seq, right.seq);
  });
}

export function chatMessageSeq(message) {
  return chatInteger(message?.seq);
}

export function latestChatSeq(messages = []) {
  return messages.reduce((latest, message) => maxChatInteger(latest, chatMessageSeq(message)), "0");
}

// A directional page only answers whether there are more records in the
// direction it was requested. Keep the opposite boundary from the already
// rendered window so loading earlier history cannot falsely claim that newer
// messages are missing (and vice versa).
export function mergeChatMessagePage(current = {}, page = {}, direction = "both", fallbackLatestSeq = "0") {
  const base = {
    hasOlder: Boolean(current.hasOlder),
    hasNewer: Boolean(current.hasNewer),
    latestSeq: chatInteger(current.latestSeq)
  };
  const updateOlder = direction !== "newer";
  const updateNewer = direction !== "older";

  return {
    hasOlder: updateOlder && page.has_older !== undefined ? Boolean(page.has_older) : base.hasOlder,
    hasNewer: updateNewer && page.has_newer !== undefined ? Boolean(page.has_newer) : base.hasNewer,
    latestSeq: maxChatInteger(base.latestSeq, page.latest_seq ?? fallbackLatestSeq)
  };
}

// React state updates do not synchronously disable a just-clicked history
// button. Keep a per-room-session, per-direction claim so rapid clicks cannot
// issue duplicate page requests while a newly selected instance of that room
// can still load.
export function createChatHistoryRequestTracker() {
  const active = new Map();

  function keyFor(roomNo, direction, session = "") {
    const room = chatRoomNo(roomNo);
    const kind = String(direction || "").trim();
    return room && kind ? `${room}:${String(session)}:${kind}` : "";
  }

  return {
    claim(roomNo, direction, session) {
      const key = keyFor(roomNo, direction, session);
      if (!key || active.has(key)) return null;
      let finish;
      const pending = new Promise((resolve) => {
        finish = resolve;
      });
      const entry = { key, pending, finish };
      active.set(key, entry);
      return entry;
    },
    pending(roomNo, direction, session) {
      return active.get(keyFor(roomNo, direction, session))?.pending || null;
    },
    release(entry) {
      if (!entry || active.get(entry.key) !== entry) return false;
      active.delete(entry.key);
      entry.finish();
      return true;
    }
  };
}

export function pendingChatMessagesForRoom(messages = [], roomNo) {
  const targetRoomNo = chatRoomNo(roomNo);
  const seen = new Set();
  return messages.filter((message) => {
    const clientMessageId = chatId(message?.client_message_id);
    if (
      !message?.pending ||
      !clientMessageId ||
      !message?.body ||
      chatRoomNo(message.room_no) !== targetRoomNo ||
      seen.has(clientMessageId)
    ) return false;
    seen.add(clientMessageId);
    return true;
  });
}

// A React state update does not immediately replace the submit handler's
// captured composer value. Keep a tiny synchronous guard so a rapid Enter +
// submit or double click cannot mint two distinct idempotency keys for the
// same user intent. Editing the composer resets the guard; an error only
// releases its own request, so an old response cannot unlock a newer attempt.
export function createChatComposerSubmissionGuard() {
  let active = null;

  return {
    claim(body, requestToken) {
      const normalizedBody = String(body || "").trim();
      const token = String(requestToken || "");
      if (!normalizedBody || !token || active?.body === normalizedBody) return false;
      active = { body: normalizedBody, token };
      return true;
    },
    release(requestToken) {
      if (!active || active.token !== String(requestToken || "")) return false;
      active = null;
      return true;
    },
    reset() {
      active = null;
    }
  };
}

// Tracks request IDs that have been superseded by a replay or confirmed by a
// durable message event. A late error for one of these IDs is stale UI state,
// not a failure of the current message attempt.
export function createChatSupersededRequestTracker(maximumSize = 512) {
  const ids = new Set();
  const order = [];
  const limit = Math.max(1, Number(maximumSize) || 512);

  return {
    remember(requestId) {
      const id = String(requestId || "");
      if (!id || ids.has(id)) return;
      ids.add(id);
      order.push(id);
      while (order.length > limit) ids.delete(order.shift());
    },
    consume(requestId) {
      const id = String(requestId || "");
      if (!id || !ids.delete(id)) return false;
      return true;
    }
  };
}

export function unreadChatIndex(messages = [], lastReadSeq = 0) {
  const index = messages.findIndex((message) => compareChatIntegers(chatMessageSeq(message), lastReadSeq) > 0);
  return index < 0 ? -1 : index;
}

export function realtimePayload(event = {}) {
  const payload = event.payload || {};
  if (payload.payload && (payload.event_id || payload.event_type)) {
    return { ...payload.payload, event_id: payload.event_id, event_type: payload.event_type };
  }
  return payload;
}

export function realtimeMessage(event = {}, userMap = new Map()) {
  const payload = realtimePayload(event);
  const message = payload.message || payload;
  if (event.type === "message.ack" || payload.message) return normalizeChatMessage(message, userMap);
  return normalizeChatMessage(
    {
      id: message.id ?? message.message_id,
      room_id: message.room_id ?? message.roomId,
      room_no: message.room_no ?? message.roomNo,
      seq: message.seq,
      sender_id: message.sender_id ?? message.senderId,
      client_message_id: message.client_message_id ?? message.clientMessageId,
      body: message.body,
      status: message.status,
      created_at: message.created_at ?? message.createdAt,
      updated_at: message.updated_at ?? message.updatedAt,
      deleted_at: message.deleted_at ?? message.deletedAt
    },
    userMap
  );
}

export function needsChatRepair(messages, latestSeq) {
  const latest = latestChatSeq(messages);
  const nextExpected = BigInt(latest) + 1n;
  return BigInt(chatInteger(latestSeq)) > nextExpected ? latest : null;
}

export function groupedChatRooms(groups = [], rooms = []) {
  const sortedGroups = orderedChatGroups(groups);
  const byGroup = new Map(sortedGroups.map((group) => [chatId(group.id), []]));
  const ungrouped = [];
  for (const room of rooms) {
    const groupId = chatId(room.membership?.group_id || room.group_id);
    const target = groupId && byGroup.has(groupId) ? byGroup.get(groupId) : ungrouped;
    target.push(room);
  }
  const sortRooms = (items) => items.sort((a, b) => Number(a.membership?.sort_order || 0) - Number(b.membership?.sort_order || 0));
  return [
    ...sortedGroups.map((group) => ({ group, rooms: sortRooms(byGroup.get(chatId(group.id)) || []) })),
    { group: null, rooms: sortRooms(ungrouped) }
  ].filter((section) => section.rooms.length > 0 || section.group);
}

export function orderedChatGroups(groups = []) {
  return [...groups].sort(
    (left, right) =>
      Number(left.sort_order || 0) - Number(right.sort_order || 0) ||
      chatId(left.id).localeCompare(chatId(right.id))
  );
}

export function moveChatGroup(groups = [], groupId, direction) {
  const ordered = orderedChatGroups(groups);
  const index = ordered.findIndex((group) => chatId(group.id) === chatId(groupId));
  const target = index + Number(direction || 0);
  if (index < 0 || target < 0 || target >= ordered.length) return ordered;
  [ordered[index], ordered[target]] = [ordered[target], ordered[index]];
  return ordered;
}

export class CoalescedUserLoader {
  constructor(fetchUsers, { delay = 16, maxBatch = MAX_USER_BATCH } = {}) {
    this.fetchUsers = fetchUsers;
    this.delay = delay;
    this.maxBatch = Math.max(1, maxBatch);
    this.cache = new Map();
    this.pending = new Map();
    this.queued = new Set();
    this.timer = null;
  }

  prime(users = []) {
    for (const user of users) {
      const normalized = normalizeChatUser(user);
      if (normalized) this.cache.set(normalized.id, normalized);
    }
  }

  get(userId) {
    return this.cache.get(chatId(userId)) || null;
  }

  load(userId) {
    const id = chatId(userId);
    if (!id) return Promise.resolve(null);
    const cached = this.cache.get(id);
    if (cached) return Promise.resolve(cached);
    return new Promise((resolve, reject) => {
      const waiters = this.pending.get(id) || [];
      waiters.push({ resolve, reject });
      this.pending.set(id, waiters);
      this.queued.add(id);
      this.schedule();
    });
  }

  schedule() {
    if (this.timer !== null) return;
    this.timer = setTimeout(() => {
      this.timer = null;
      this.flush();
    }, this.delay);
  }

  async flush() {
    const ids = [...this.queued].slice(0, this.maxBatch);
    ids.forEach((id) => this.queued.delete(id));
    if (this.queued.size > 0) this.schedule();
    if (ids.length === 0) return;
    try {
      const data = await this.fetchUsers(ids);
      const users = Array.isArray(data) ? data : data?.items || data?.users || [];
      this.prime(users);
      ids.forEach((id) => this.resolve(id, this.cache.get(id) || null));
    } catch (error) {
      ids.forEach((id) => this.reject(id, error));
    }
  }

  resolve(id, value) {
    const waiters = this.pending.get(id) || [];
    this.pending.delete(id);
    waiters.forEach(({ resolve }) => resolve(value));
  }

  reject(id, error) {
    const waiters = this.pending.get(id) || [];
    this.pending.delete(id);
    waiters.forEach(({ reject }) => reject(error));
  }
}
