import React from "react";
import { Link, useNavigate, useParams } from "react-router-dom";
import {
  AlertCircle,
  ArrowRight,
  CircleCheck,
  LoaderCircle,
  Megaphone,
  PanelLeft,
  Send,
  Share2,
  ShieldCheck,
  Wifi,
  WifiOff
} from "lucide-react";
import { bbsApi, chatWebSocketUrl } from "../api";
import { ChatAnnouncementDialog, ChatRoomDialog, ChatShareDialog } from "../components/chat/ChatDialogs.jsx";
import ChatSidebar from "../components/chat/ChatSidebar.jsx";
import ChatTimeline from "../components/chat/ChatTimeline.jsx";
import {
  chatId,
  chatInteger,
  createChatComposerSubmissionGuard,
  createChatHistoryRequestTracker,
  createChatSupersededRequestTracker,
  chatMessageSeq,
  chatRoomNo,
  compareChatIntegers,
  CoalescedUserLoader,
  indexChatUsers,
  isCurrentChatRoomSessionRequest,
  latestChatSeq,
  maxChatInteger,
  mergeChatMessagePage,
  mergeChatMessages,
  needsChatRepair,
  normalizeChatDetails,
  normalizeChatMembership,
  normalizeChatMessage,
  normalizeChatRoom,
  normalizeChatSidebar,
  pendingChatMessagesForRoom,
  realtimeMessage,
  realtimePayload,
  subtractChatIntegers,
  unreadChatIndex
} from "../lib/chat";
import { ChatRealtimeClient } from "../lib/chatRealtime";

const ROOM_NUMBER_PATTERN = /^[0-9ABCDEFGHJKMNPQRSTVWXYZ]{8}$/;
const INITIAL_BEFORE = 30;
const INITIAL_AFTER = 30;
const DIRECTIONAL_LIMIT = 100;

function randomUUID() {
  if (globalThis.crypto?.randomUUID) return globalThis.crypto.randomUUID();
  return "xxxxxxxx-xxxx-4xxx-yxxx-xxxxxxxxxxxx".replace(/[xy]/g, (character) => {
    const value = Math.floor(Math.random() * 16);
    return (character === "x" ? value : (value & 3) | 8).toString(16);
  });
}

function errorMessage(error, fallback) {
  return error?.message || fallback;
}

function membershipRequired(error) {
  return Number(error?.httpCode || error?.status || 0) === 403;
}

function roomMatches(payload, roomNo, room) {
  const eventRoomNo = chatRoomNo(payload?.room_no);
  if (eventRoomNo) return eventRoomNo === roomNo;
  return Boolean(chatId(payload?.room_id) && chatId(payload.room_id) === chatId(room?.id));
}

function connectionLabel(status) {
  if (status === "connected") return "实时连接正常";
  if (status === "rate_limited") return "实时连接受限，稍后自动重试";
  if (status === "reconnecting") return "正在恢复连接";
  if (status === "connecting" || status === "authenticating") return "正在连接";
  if (status === "protocol_error") return "实时协议异常";
  return "实时连接已断开";
}

function nearBottom(element) {
  return !element || element.scrollHeight - element.scrollTop - element.clientHeight < 96;
}

export function ChatPage({ auth }) {
  const params = useParams();
  const navigate = useNavigate();
  const activeRoomNo = chatRoomNo(params.roomNo);
  const chatPath = activeRoomNo ? `/room/${activeRoomNo}` : "/chat";
  const token = auth?.accessToken || "";
  const currentUserId = chatId(auth?.user?.id);

  const [sidebar, setSidebar] = React.useState({ groups: [], rooms: [] });
  const [sidebarLoading, setSidebarLoading] = React.useState(false);
  const [room, setRoom] = React.useState(null);
  const [membership, setMembership] = React.useState(null);
  const [memberCount, setMemberCount] = React.useState("0");
  const [preview, setPreview] = React.useState(null);
  const [phase, setPhase] = React.useState(token ? "loading" : "auth");
  const [pageError, setPageError] = React.useState("");
  const [messages, setMessages] = React.useState([]);
  const [messagePage, setMessagePage] = React.useState({ hasOlder: false, hasNewer: false, latestSeq: "0" });
  const [initialReadSeq, setInitialReadSeq] = React.useState("0");
  const [loadingOlder, setLoadingOlder] = React.useState(false);
  const [loadingNewer, setLoadingNewer] = React.useState(false);
  const [repairing, setRepairing] = React.useState(false);
  const [users, setUsers] = React.useState(new Map());
  const [connectionStatus, setConnectionStatus] = React.useState("disconnected");
  const [composer, setComposer] = React.useState("");
  const [composerError, setComposerError] = React.useState("");
  const [deletingMessageId, setDeletingMessageId] = React.useState("");
  const [roomDialogMode, setRoomDialogMode] = React.useState(null);
  const [roomDialogPreview, setRoomDialogPreview] = React.useState(null);
  const [dialogLoading, setDialogLoading] = React.useState(false);
  const [dialogError, setDialogError] = React.useState("");
  const [announcementOpen, setAnnouncementOpen] = React.useState(false);
  const [announcementSaving, setAnnouncementSaving] = React.useState(false);
  const [announcementError, setAnnouncementError] = React.useState("");
  const [shareOpen, setShareOpen] = React.useState(false);
  const [manageMode, setManageMode] = React.useState(false);
  const [groupEditor, setGroupEditor] = React.useState(false);
  const [groupName, setGroupName] = React.useState("");
  const [groupSaving, setGroupSaving] = React.useState(false);
  const [placementSaving, setPlacementSaving] = React.useState(false);
  const [editingGroupId, setEditingGroupId] = React.useState("");
  const [editingGroupName, setEditingGroupName] = React.useState("");
  const [deletingGroupId, setDeletingGroupId] = React.useState("");
  const [sidebarOpen, setSidebarOpen] = React.useState(false);
  const [reloadKey, setReloadKey] = React.useState(0);

  const scrollRef = React.useRef(null);
  const messagesRef = React.useRef([]);
  const usersRef = React.useRef(new Map());
  const roomRef = React.useRef(null);
  const membershipRef = React.useRef(null);
  const activeRoomNoRef = React.useRef(activeRoomNo);
  const activeTokenRef = React.useRef(token);
  const roomSessionRef = React.useRef(0);
  const phaseRef = React.useRef(phase);
  const realtimeRef = React.useRef(null);
  const eventHandlerRef = React.useRef(() => {});
  const stateHandlerRef = React.useRef(() => {});
  const repairRef = React.useRef(null);
  const repairActiveRef = React.useRef(() => Promise.resolve());
  const sidebarRefreshTimerRef = React.useRef(null);
  const sidebarRequestVersionRef = React.useRef(0);
  const sidebarLoadingRequestVersionRef = React.useRef(0);
  const readTimerRef = React.useRef(null);
  const pendingReadRef = React.useRef("0");
  const pendingRequestsRef = React.useRef(new Map());
  const historyRequestTrackerRef = React.useRef(null);
  const initialScrollPendingRef = React.useRef(false);
  const stickBottomRef = React.useRef(false);
  const scrollFrameRef = React.useRef(null);
  const connectedOnceRef = React.useRef(false);
  const subscribedRoomNumbersRef = React.useRef(new Set());
  const seenEventIDsRef = React.useRef({ set: new Set(), order: [] });
  const userLoaderRef = React.useRef(null);
  const composerSubmissionGuardRef = React.useRef(null);
  const supersededSendRequestTrackerRef = React.useRef(null);

  activeRoomNoRef.current = activeRoomNo;
  activeTokenRef.current = token;

  if (!userLoaderRef.current) {
    userLoaderRef.current = new CoalescedUserLoader((ids) => bbsApi.getUsers(ids));
  }
  if (!composerSubmissionGuardRef.current) {
    composerSubmissionGuardRef.current = createChatComposerSubmissionGuard();
  }
  if (!historyRequestTrackerRef.current) {
    historyRequestTrackerRef.current = createChatHistoryRequestTracker();
  }
  if (!supersededSendRequestTrackerRef.current) {
    supersededSendRequestTrackerRef.current = createChatSupersededRequestTracker();
  }

  React.useLayoutEffect(() => {
    roomSessionRef.current += 1;
    setAnnouncementSaving(false);
    setAnnouncementError("");
    setDeletingMessageId("");
    setComposerError("");
  }, [activeRoomNo, reloadKey, token]);

  React.useLayoutEffect(() => {
    sidebarRequestVersionRef.current += 1;
    sidebarLoadingRequestVersionRef.current = 0;
    if (sidebarRefreshTimerRef.current !== null) {
      clearTimeout(sidebarRefreshTimerRef.current);
      sidebarRefreshTimerRef.current = null;
    }
    setSidebarLoading(false);
    setSidebar({ groups: [], rooms: [] });
  }, [token]);

  const isCurrentRoomSession = React.useCallback((roomNo, session) => (
    isCurrentChatRoomSessionRequest(roomNo, session, activeRoomNoRef.current, roomSessionRef.current)
  ), []);

  const isCurrentRoomOperation = React.useCallback((roomNo, session, requestToken) => (
    requestToken === activeTokenRef.current && isCurrentRoomSession(roomNo, session)
  ), [isCurrentRoomSession]);

  React.useEffect(() => {
    phaseRef.current = phase;
  }, [phase]);

  const absorbUsers = React.useCallback((incoming = []) => {
    if (!incoming.length) return;
    userLoaderRef.current.prime(incoming);
    const next = indexChatUsers(incoming, usersRef.current);
    usersRef.current = next;
    setUsers(next);
  }, []);

  const ensureUser = React.useCallback((userId) => {
    const id = chatId(userId);
    if (!id || usersRef.current.has(id)) return;
    userLoaderRef.current
      .load(id)
      .then((user) => user && absorbUsers([user]))
      .catch(() => {});
  }, [absorbUsers]);

  const replaceMessages = React.useCallback((next) => {
    messagesRef.current = next;
    setMessages(next);
  }, []);

  const applyMessagePage = React.useCallback((data = {}, replace = false, direction = "both") => {
    absorbUsers(Array.isArray(data.users) ? data.users : []);
    const incoming = (Array.isArray(data.messages) ? data.messages : []).map((message) => normalizeChatMessage(message, usersRef.current));
    const base = replace ? [] : messagesRef.current;
    const next = mergeChatMessages(base, incoming, usersRef.current);
    replaceMessages(next);
    setMessagePage((current) => mergeChatMessagePage(
      replace ? { hasOlder: false, hasNewer: false, latestSeq: "0" } : current,
      data,
      direction,
      latestChatSeq(next)
    ));
    return next;
  }, [absorbUsers, replaceMessages]);

  const updateRoom = React.useCallback((nextRoom) => {
    const normalized = normalizeChatRoom(nextRoom);
    roomRef.current = normalized;
    setRoom(normalized);
  }, []);

  const updateMembership = React.useCallback((nextMembership) => {
    const normalized = normalizeChatMembership(nextMembership);
    membershipRef.current = normalized;
    setMembership(normalized);
  }, []);

  const loadSidebar = React.useCallback(async ({ quiet = false } = {}) => {
    const requestToken = token;
    if (requestToken !== activeTokenRef.current) return null;
    const requestVersion = ++sidebarRequestVersionRef.current;
    const isCurrentRequest = () => (
      requestToken === activeTokenRef.current && requestVersion === sidebarRequestVersionRef.current
    );
    if (sidebarLoadingRequestVersionRef.current) {
      sidebarLoadingRequestVersionRef.current = 0;
      setSidebarLoading(false);
    }
    if (!requestToken) return { groups: [], rooms: [] };
    if (!quiet) {
      sidebarLoadingRequestVersionRef.current = requestVersion;
      setSidebarLoading(true);
    }
    try {
      const data = normalizeChatSidebar(await bbsApi.chatSidebar(requestToken));
      if (!isCurrentRequest()) return null;
      absorbUsers(data.users);
      setSidebar({ groups: data.groups, rooms: data.rooms });
      return data;
    } catch (error) {
      if (!isCurrentRequest()) return null;
      throw error;
    } finally {
      if (
        !quiet &&
        requestToken === activeTokenRef.current &&
        sidebarLoadingRequestVersionRef.current === requestVersion
      ) {
        sidebarLoadingRequestVersionRef.current = 0;
        setSidebarLoading(false);
      }
    }
  }, [absorbUsers, token]);

  const scheduleSidebarRefresh = React.useCallback(() => {
    if (sidebarRefreshTimerRef.current !== null) clearTimeout(sidebarRefreshTimerRef.current);
    sidebarRefreshTimerRef.current = setTimeout(() => {
      sidebarRefreshTimerRef.current = null;
      loadSidebar({ quiet: true }).catch(() => {});
    }, 180);
  }, [loadSidebar]);

  const clearPendingSendRequests = React.useCallback((clientMessageId) => {
    for (const [requestId, pending] of pendingRequestsRef.current) {
      if (pending.kind === "send" && pending.clientMessageId === clientMessageId) {
        pendingRequestsRef.current.delete(requestId);
        supersededSendRequestTrackerRef.current.remember(requestId);
      }
    }
  }, []);

  const repairActiveRoom = React.useCallback(async () => {
    if (!token || !activeRoomNo || !membershipRef.current || phaseRef.current !== "ready") return;
    const requestedRoomNo = activeRoomNo;
    const requestedSession = roomSessionRef.current;
    if (repairRef.current?.roomNo === requestedRoomNo && repairRef.current?.session === requestedSession) return repairRef.current.operation;
    let entry;
    const operation = (async () => {
      setRepairing(true);
      const historyRequests = historyRequestTrackerRef.current;
      const pendingNewer = historyRequests.pending(requestedRoomNo, "newer", requestedSession);
      if (pendingNewer) await pendingNewer;
      if (!isCurrentRoomSession(requestedRoomNo, requestedSession)) return;
      const request = historyRequests.claim(requestedRoomNo, "newer", requestedSession);
      if (!request) return;
      try {
        let cursor = latestChatSeq(messagesRef.current);
        for (let page = 0; page < 6; page += 1) {
          const data = await bbsApi.chatMessages(requestedRoomNo, { after_seq: String(cursor), limit: DIRECTIONAL_LIMIT }, token);
          if (!isCurrentRoomSession(requestedRoomNo, requestedSession)) return;
          const next = applyMessagePage(data, false, "newer");
          const nextCursor = latestChatSeq(next);
          if (!data?.has_newer || compareChatIntegers(nextCursor, cursor) <= 0) break;
          cursor = nextCursor;
        }
        if (isCurrentRoomSession(requestedRoomNo, requestedSession)) scheduleSidebarRefresh();
      } finally {
        historyRequests.release(request);
      }
    })()
      .catch((error) => {
        if (isCurrentRoomSession(requestedRoomNo, requestedSession)) {
          setComposerError(errorMessage(error, "消息同步失败，请稍后重试。"));
        }
      })
      .finally(() => {
        if (repairRef.current !== entry) return;
        repairRef.current = null;
        if (isCurrentRoomSession(requestedRoomNo, requestedSession)) setRepairing(false);
      });
    entry = { roomNo: requestedRoomNo, session: requestedSession, operation };
    repairRef.current = entry;
    return operation;
  }, [activeRoomNo, applyMessagePage, isCurrentRoomSession, scheduleSidebarRefresh, token]);

  React.useEffect(() => {
    repairActiveRef.current = repairActiveRoom;
  }, [repairActiveRoom]);

  React.useEffect(() => {
    let alive = true;
    if (!token) {
      setPhase("auth");
      return undefined;
    }
    if (!activeRoomNo) {
      setPhase("loading");
      setPageError("");
      setPreview(null);
      setComposerError("");
      setAnnouncementOpen(false);
      setShareOpen(false);
      updateRoom(null);
      updateMembership(null);
      setMemberCount("0");
      replaceMessages([]);
      setMessagePage({ hasOlder: false, hasNewer: false, latestSeq: "0" });
      setLoadingOlder(false);
      setLoadingNewer(false);
      setRepairing(false);
      loadSidebar()
        .then(() => {
          if (alive) setPhase("lobby");
        })
        .catch((error) => {
          if (!alive) return;
          setPageError(errorMessage(error, "房间列表加载失败，请稍后重试。"));
          setPhase("error");
        });
      return () => {
        alive = false;
      };
    }
    if (!ROOM_NUMBER_PATTERN.test(activeRoomNo)) {
      updateRoom(null);
      updateMembership(null);
      setMemberCount("0");
      setPreview(null);
      setAnnouncementOpen(false);
      setShareOpen(false);
      setLoadingOlder(false);
      setLoadingNewer(false);
      setRepairing(false);
      setPageError("房间号格式无效。请检查分享链接后重试。");
      setPhase("error");
      return undefined;
    }

    async function loadRoom() {
      setPhase("loading");
      setPageError("");
      setPreview(null);
      setComposerError("");
      updateRoom(null);
      updateMembership(null);
      setMemberCount("0");
      setAnnouncementOpen(false);
      setShareOpen(false);
      replaceMessages([]);
      setMessagePage({ hasOlder: false, hasNewer: false, latestSeq: "0" });
      setLoadingOlder(false);
      setLoadingNewer(false);
      setRepairing(false);
      try {
        const [, detailsData] = await Promise.all([loadSidebar(), bbsApi.getChatRoom(activeRoomNo, token)]);
        if (!alive) return;
        const details = normalizeChatDetails(detailsData);
        absorbUsers(details.users);
        updateRoom(details.room);
        updateMembership(details.membership);
        setMemberCount(details.member_count);
        const readSeq = chatInteger(details.membership?.last_read_seq);
        setInitialReadSeq(readSeq);
        const page = await bbsApi.chatMessages(
          activeRoomNo,
          { anchor_seq: readSeq, before: INITIAL_BEFORE, after: INITIAL_AFTER },
          token
        );
        if (!alive) return;
        applyMessagePage(page, true);
        initialScrollPendingRef.current = true;
        setPhase("ready");
        setAnnouncementOpen(
          Boolean(
            details.room?.announcement &&
              compareChatIntegers(details.room?.announcement_version, details.membership?.last_seen_announcement_version) > 0
          )
        );
        if (page?.has_newer) setTimeout(() => repairActiveRef.current(), 0);
      } catch (error) {
        if (!alive) return;
        if (membershipRequired(error)) {
          try {
            const found = await bbsApi.lookupChatRoom(activeRoomNo, token);
            if (!alive) return;
            setPreview(found);
            setPhase("preview");
            loadSidebar().catch(() => {});
            return;
          } catch (lookupError) {
            error = lookupError;
          }
        }
        setPageError(errorMessage(error, "房间加载失败，请稍后重试。"));
        setPhase("error");
      }
    }

    loadRoom();
    return () => {
      alive = false;
    };
  }, [absorbUsers, activeRoomNo, applyMessagePage, loadSidebar, reloadKey, replaceMessages, token, updateMembership, updateRoom]);

  const rememberEvent = React.useCallback((event) => {
    const eventId = chatId(event?.payload?.event_id);
    if (!eventId) return true;
    const remembered = seenEventIDsRef.current;
    if (remembered.set.has(eventId)) return false;
    remembered.set.add(eventId);
    remembered.order.push(eventId);
    if (remembered.order.length > 512) remembered.set.delete(remembered.order.shift());
    return true;
  }, []);

  const mergeRealtimeMessage = React.useCallback(async (event) => {
    const message = realtimeMessage(event, usersRef.current);
    if (!message.id && !message.client_message_id) return;
    if (message.client_message_id) clearPendingSendRequests(message.client_message_id);
    ensureUser(message.sender_id);
    const belongsToActiveRoom = roomMatches(message, activeRoomNoRef.current, roomRef.current);
    if (!belongsToActiveRoom) {
      scheduleSidebarRefresh();
      return;
    }
    const scroll = scrollRef.current;
    stickBottomRef.current = nearBottom(scroll) || message.sender_id === currentUserId;
    if (needsChatRepair(messagesRef.current, message.seq) !== null) await repairActiveRoom();
    replaceMessages(mergeChatMessages(messagesRef.current, [message], usersRef.current));
    setMessagePage((current) => ({ ...current, latestSeq: maxChatInteger(current.latestSeq, chatMessageSeq(message)) }));
    scheduleSidebarRefresh();
  }, [activeRoomNo, clearPendingSendRequests, currentUserId, ensureUser, repairActiveRoom, replaceMessages, scheduleSidebarRefresh]);

  const applyDeletedMessage = React.useCallback((rawMessage) => {
    const deleted = normalizeChatMessage(rawMessage, usersRef.current);
    if (!deleted.id || !roomMatches(deleted, activeRoomNoRef.current, roomRef.current)) {
      scheduleSidebarRefresh();
      return;
    }
    let updated = false;
    const next = messagesRef.current.map((message) => {
      if (chatId(message.id) !== deleted.id) return message;
      updated = true;
      return {
        ...message,
        body: "",
        status: 2,
        pending: false,
        updated_at: deleted.updated_at || message.updated_at,
        deleted_at: deleted.deleted_at || message.deleted_at
      };
    });
    if (updated) replaceMessages(next);
    scheduleSidebarRefresh();
  }, [replaceMessages, scheduleSidebarRefresh]);

  const applyReadEvent = React.useCallback((event) => {
    const payload = realtimePayload(event);
    const currentRoomNo = activeRoomNoRef.current;
    if (!roomMatches(payload, currentRoomNo, roomRef.current)) return;
    if (chatId(payload.user_id) && chatId(payload.user_id) !== currentUserId) return;
    const lastRead = maxChatInteger(membershipRef.current?.last_read_seq, payload.last_read_seq);
    const nextMembership = { ...(membershipRef.current || {}), last_read_seq: lastRead };
    updateMembership(nextMembership);
    setSidebar((current) => ({
      ...current,
      rooms: current.rooms.map((item) => {
        if ((item.room_no || item.room?.room_no) !== currentRoomNo) return item;
        const latest = chatInteger(payload.latest_seq ?? item.room?.last_message_seq);
        return {
          ...item,
          membership: { ...item.membership, last_read_seq: lastRead },
          unread_count: subtractChatIntegers(latest, lastRead)
        };
      })
    }));
  }, [currentUserId, updateMembership]);

  const sendChatMessageFallback = React.useCallback(async (roomNo, roomSession, clientMessageId, body) => {
    const data = await bbsApi.sendChatMessage(roomNo, { client_message_id: clientMessageId, body }, token);
    if (!isCurrentRoomSession(roomNo, roomSession)) return false;
    absorbUsers(data?.users || []);
    await mergeRealtimeMessage({ type: "message.ack", payload: data });
    return true;
  }, [absorbUsers, isCurrentRoomSession, mergeRealtimeMessage, token]);

  const advanceChatReadFallback = React.useCallback(async (roomNo, roomSession, requestToken, readSeq) => {
    const data = await bbsApi.advanceChatRead(roomNo, readSeq, requestToken);
    if (!isCurrentRoomOperation(roomNo, roomSession, requestToken)) return false;
    if (data?.membership) updateMembership(data.membership);
    scheduleSidebarRefresh();
    return true;
  }, [isCurrentRoomOperation, scheduleSidebarRefresh, updateMembership]);

  const replayPendingMessages = React.useCallback(() => {
    if (phaseRef.current !== "ready" || !activeRoomNo || !realtimeRef.current?.isOpen()) return;
    const roomSession = roomSessionRef.current;
    for (const message of pendingChatMessagesForRoom(messagesRef.current, activeRoomNo)) {
      clearPendingSendRequests(message.client_message_id);
      const requestId = randomUUID();
      pendingRequestsRef.current.set(requestId, {
        kind: "send",
        clientMessageId: message.client_message_id,
        roomNo: activeRoomNo,
        roomSession,
        body: message.body
      });
      if (!realtimeRef.current.send("message.send", {
        room_no: activeRoomNo,
        client_message_id: message.client_message_id,
        body: message.body
      }, requestId)) {
        pendingRequestsRef.current.delete(requestId);
      }
    }
  }, [activeRoomNo, clearPendingSendRequests]);

  const handleRealtimeEvent = React.useCallback(async (event) => {
    if (!event?.type) return;
    if (!rememberEvent(event)) return;
    if (event.type === "message.ack" || event.type === "message.created") {
      const pending = pendingRequestsRef.current.get(event.request_id);
      if (pending?.kind === "send") clearPendingSendRequests(pending.clientMessageId);
      else if (pending) pendingRequestsRef.current.delete(event.request_id);
      await mergeRealtimeMessage(event);
      return;
    }
    if (event.type === "message.deleted") {
      applyDeletedMessage(realtimeMessage(event, usersRef.current));
      return;
    }
    if (event.type === "read.advanced") {
      applyReadEvent(event);
      return;
    }
    if (event.type === "room.subscribed") {
      const payload = realtimePayload(event);
      const subscribed = new Set(
        (Array.isArray(payload.subscriptions) ? payload.subscriptions : [])
          .map((subscription) => chatRoomNo(subscription?.room_no))
          .filter(Boolean)
      );
      subscribedRoomNumbersRef.current = subscribed;
      const activeSubscribed = Array.isArray(payload.subscriptions) && payload.subscriptions.some((subscription) =>
        chatRoomNo(subscription?.room_no) === activeRoomNo
      );
      if (activeSubscribed) {
        await repairActiveRoom();
        replayPendingMessages();
      }
      return;
    }
    if (event.type === "announcement.updated") {
      const payload = realtimePayload(event);
      if (roomMatches(payload, activeRoomNoRef.current, roomRef.current)) {
        updateRoom({
          ...(roomRef.current || {}),
          announcement: payload.announcement || "",
          announcement_version: chatInteger(payload.announcement_version)
        });
        setAnnouncementOpen(true);
      }
      scheduleSidebarRefresh();
      return;
    }
    if (event.type === "room.member.joined") {
      scheduleSidebarRefresh();
      return;
    }
    if (event.type === "resync.required") {
      await repairActiveRoom();
      scheduleSidebarRefresh();
      return;
    }
    if (event.type === "error") {
      if (event.request_id && supersededSendRequestTrackerRef.current.consume(event.request_id)) return;
      const pending = pendingRequestsRef.current.get(event.request_id);
      const payload = realtimePayload(event);
      if (event.request_id) pendingRequestsRef.current.delete(event.request_id);
      if (pending?.kind === "send" && payload.code === "not_subscribed") {
        try {
          const applied = await sendChatMessageFallback(pending.roomNo, pending.roomSession, pending.clientMessageId, pending.body);
          if (!applied) composerSubmissionGuardRef.current.release(pending.clientMessageId);
          return;
        } catch (error) {
          if (!isCurrentRoomSession(pending.roomNo, pending.roomSession)) {
            composerSubmissionGuardRef.current.release(pending.clientMessageId);
            return;
          }
          replaceMessages(messagesRef.current.filter((message) => message.client_message_id !== pending.clientMessageId || !message.pending));
          composerSubmissionGuardRef.current.release(pending.clientMessageId);
          setComposer(pending.body);
          setComposerError(errorMessage(error, "消息发送失败，请重试。"));
          return;
        }
      }
      if (pending?.kind === "read" && payload.code === "not_subscribed") {
        try {
          await advanceChatReadFallback(pending.roomNo, pending.roomSession, pending.requestToken, pending.readSeq);
          return;
        } catch (error) {
          if (isCurrentRoomOperation(pending.roomNo, pending.roomSession, pending.requestToken)) {
            setComposerError(errorMessage(error, "已读状态同步失败。"));
          }
          return;
        }
      }
      if (pending?.kind === "send") {
        if (!isCurrentRoomSession(pending.roomNo, pending.roomSession)) {
          composerSubmissionGuardRef.current.release(pending.clientMessageId);
          return;
        }
        replaceMessages(messagesRef.current.filter((message) => message.client_message_id !== pending.clientMessageId || !message.pending));
        composerSubmissionGuardRef.current.release(pending.clientMessageId);
        setComposer(pending.body);
      }
      if (pending?.kind === "read" && !isCurrentRoomOperation(pending.roomNo, pending.roomSession, pending.requestToken)) return;
      setComposerError(payload.message || "实时操作失败，请重试。");
    }
  }, [activeRoomNo, advanceChatReadFallback, applyDeletedMessage, applyReadEvent, isCurrentRoomOperation, isCurrentRoomSession, mergeRealtimeMessage, rememberEvent, repairActiveRoom, replayPendingMessages, replaceMessages, scheduleSidebarRefresh, sendChatMessageFallback, updateRoom]);

  eventHandlerRef.current = handleRealtimeEvent;
  stateHandlerRef.current = (status) => {
    setConnectionStatus(status);
    subscribedRoomNumbersRef.current = new Set();
    if (status === "connected") {
      if (connectedOnceRef.current) repairActiveRef.current();
      connectedOnceRef.current = true;
    }
  };

  React.useEffect(() => {
    if (!token) return undefined;
    connectedOnceRef.current = false;
    const client = new ChatRealtimeClient({
      issueTicket: () => bbsApi.createChatWebSocketTicket(token),
      websocketUrl: chatWebSocketUrl,
      onEvent: (event) => eventHandlerRef.current(event),
      onState: (status) => stateHandlerRef.current(status)
    });
    realtimeRef.current = client;
    client.start([]);
    return () => {
      client.stop({ notify: false });
      if (realtimeRef.current === client) realtimeRef.current = null;
    };
  }, [token]);

  const subscribedRooms = React.useMemo(() => {
    const values = sidebar.rooms.map((item) => item.room_no || item.room?.room_no);
    if (membership && activeRoomNo) values.push(activeRoomNo);
    return [...new Set(values.map(chatRoomNo).filter(Boolean))];
  }, [activeRoomNo, membership, sidebar.rooms]);

  React.useEffect(() => {
    subscribedRoomNumbersRef.current = new Set();
    realtimeRef.current?.setRooms(subscribedRooms);
  }, [subscribedRooms]);

  React.useLayoutEffect(() => {
    if (!initialScrollPendingRef.current || phase !== "ready" || !scrollRef.current) return;
    const container = scrollRef.current;
    const unread = container.querySelector("[data-unread-separator]");
    const anchor = container.querySelector(`[data-seq="${initialReadSeq}"]`);
    const target = unread || anchor;
    if (target) {
      container.scrollTop = Math.max(0, target.offsetTop - container.clientHeight * 0.42);
    } else {
      container.scrollTop = container.scrollHeight;
    }
    initialScrollPendingRef.current = false;
  }, [initialReadSeq, messages.length, phase]);

  React.useEffect(() => {
    if (!stickBottomRef.current || !scrollRef.current) return;
    scrollRef.current.scrollTop = scrollRef.current.scrollHeight;
    stickBottomRef.current = false;
  }, [messages.length]);

  const advanceRead = React.useCallback(async (sequence) => {
    const requestedRoomNo = activeRoomNo;
    const requestedSession = roomSessionRef.current;
    const requestToken = token;
    if (!isCurrentRoomOperation(requestedRoomNo, requestedSession, requestToken)) return;
    const target = chatInteger(sequence);
    if (
      compareChatIntegers(target, "0") <= 0 ||
      !membershipRef.current ||
      compareChatIntegers(target, membershipRef.current.last_read_seq) <= 0
    ) return;
    const requestId = randomUUID();
    const sent = subscribedRoomNumbersRef.current.has(requestedRoomNo) && realtimeRef.current?.send("read.advance", { room_no: requestedRoomNo, read_seq: target }, requestId);
    if (sent) {
      pendingRequestsRef.current.set(requestId, { kind: "read", roomNo: requestedRoomNo, roomSession: requestedSession, requestToken, readSeq: target });
      return;
    }
    try {
      await advanceChatReadFallback(requestedRoomNo, requestedSession, requestToken, target);
    } catch (error) {
      if (isCurrentRoomOperation(requestedRoomNo, requestedSession, requestToken)) {
        setComposerError(errorMessage(error, "已读状态同步失败。"));
      }
    }
  }, [activeRoomNo, advanceChatReadFallback, isCurrentRoomOperation, token]);

  const scheduleRead = React.useCallback((sequence) => {
    pendingReadRef.current = maxChatInteger(pendingReadRef.current, sequence);
    if (readTimerRef.current !== null) clearTimeout(readTimerRef.current);
    readTimerRef.current = setTimeout(() => {
      readTimerRef.current = null;
      const target = pendingReadRef.current;
      pendingReadRef.current = "0";
      advanceRead(target);
    }, 350);
  }, [advanceRead]);

  const handleTimelineScroll = React.useCallback(() => {
    if (scrollFrameRef.current !== null) return;
    scrollFrameRef.current = requestAnimationFrame(() => {
      scrollFrameRef.current = null;
      const container = scrollRef.current;
      if (!container) return;
      let visibleSeq = "0";
      const edge = container.scrollTop + container.clientHeight - 18;
      container.querySelectorAll("[data-seq]").forEach((element) => {
        if (element.offsetTop + Math.min(element.offsetHeight, 48) <= edge) {
          visibleSeq = maxChatInteger(visibleSeq, element.dataset.seq);
        }
      });
      if (compareChatIntegers(visibleSeq, "0") > 0) scheduleRead(visibleSeq);
    });
  }, [scheduleRead]);

  React.useEffect(() => () => {
    sidebarRequestVersionRef.current += 1;
    sidebarLoadingRequestVersionRef.current = 0;
    if (sidebarRefreshTimerRef.current !== null) clearTimeout(sidebarRefreshTimerRef.current);
    if (readTimerRef.current !== null) clearTimeout(readTimerRef.current);
    if (scrollFrameRef.current !== null) cancelAnimationFrame(scrollFrameRef.current);
  }, []);

  async function loadOlder() {
    const first = messagesRef.current.find((message) => compareChatIntegers(chatMessageSeq(message), "0") > 0);
    if (!first || loadingOlder) return;
    const requestedRoomNo = activeRoomNo;
    const requestedSession = roomSessionRef.current;
    const historyRequests = historyRequestTrackerRef.current;
    const pending = historyRequests.pending(requestedRoomNo, "older", requestedSession);
    if (pending) return pending;
    const request = historyRequests.claim(requestedRoomNo, "older", requestedSession);
    if (!request) return;
    const container = scrollRef.current;
    const previousHeight = container?.scrollHeight || 0;
    const previousTop = container?.scrollTop || 0;
    setLoadingOlder(true);
    try {
      const data = await bbsApi.chatMessages(requestedRoomNo, { before_seq: first.seq, limit: DIRECTIONAL_LIMIT }, token);
      if (!isCurrentRoomSession(requestedRoomNo, requestedSession)) return;
      applyMessagePage(data, false, "older");
      requestAnimationFrame(() => {
        if (container && isCurrentRoomSession(requestedRoomNo, requestedSession)) {
          container.scrollTop = previousTop + container.scrollHeight - previousHeight;
        }
      });
    } catch (error) {
      if (isCurrentRoomSession(requestedRoomNo, requestedSession)) {
        setComposerError(errorMessage(error, "更早消息加载失败。"));
      }
    } finally {
      historyRequests.release(request);
      if (isCurrentRoomSession(requestedRoomNo, requestedSession)) setLoadingOlder(false);
    }
  }

  async function loadNewer({ scrollToLatest = false } = {}) {
    if (loadingNewer || !messagePage.hasNewer) return;
    const requestedRoomNo = activeRoomNo;
    const requestedSession = roomSessionRef.current;
    if (repairRef.current?.roomNo === requestedRoomNo && repairRef.current?.session === requestedSession) return repairRef.current.operation;
    const historyRequests = historyRequestTrackerRef.current;
    const pending = historyRequests.pending(requestedRoomNo, "newer", requestedSession);
    if (pending) return pending;
    const request = historyRequests.claim(requestedRoomNo, "newer", requestedSession);
    if (!request) return;
    const container = scrollRef.current;
    setLoadingNewer(true);
    try {
      const cursor = latestChatSeq(messagesRef.current);
      const data = await bbsApi.chatMessages(requestedRoomNo, { after_seq: cursor, limit: DIRECTIONAL_LIMIT }, token);
      if (!isCurrentRoomSession(requestedRoomNo, requestedSession)) return;
      const next = applyMessagePage(data, false, "newer");
      if (scrollToLatest) {
        requestAnimationFrame(() => {
          if (!container || !isCurrentRoomSession(requestedRoomNo, requestedSession)) return;
          container.scrollTop = container.scrollHeight;
          scheduleRead(latestChatSeq(next));
        });
      }
    } catch (error) {
      if (isCurrentRoomSession(requestedRoomNo, requestedSession)) {
        setComposerError(errorMessage(error, "更新消息加载失败。"));
      }
    } finally {
      historyRequests.release(request);
      if (isCurrentRoomSession(requestedRoomNo, requestedSession)) setLoadingNewer(false);
    }
  }

  async function sendMessage(event) {
    event?.preventDefault?.();
    const body = composer.trim();
    if (!body || !membershipRef.current || roomRef.current?.status !== 1) return;
    const requestedRoomNo = activeRoomNo;
    const requestedSession = roomSessionRef.current;
    if (!isCurrentRoomSession(requestedRoomNo, requestedSession)) return;
    const clientMessageId = randomUUID();
    if (!composerSubmissionGuardRef.current.claim(body, clientMessageId)) return;
    const requestId = randomUUID();
    const optimistic = normalizeChatMessage({
      id: "",
      room_id: roomRef.current?.id,
      room_no: requestedRoomNo,
      seq: "",
      sender_id: currentUserId,
      client_message_id: clientMessageId,
      body,
      status: 1,
      created_at: String(Date.now()),
      pending: true
    }, usersRef.current);
    optimistic.pending = true;
    stickBottomRef.current = true;
    replaceMessages(mergeChatMessages(messagesRef.current, [optimistic], usersRef.current));
    setComposer("");
    setComposerError("");
    clearPendingSendRequests(clientMessageId);
    const realtimeReady = subscribedRoomNumbersRef.current.has(requestedRoomNo);
    const payload = { room_no: requestedRoomNo, client_message_id: clientMessageId, body };
    if (realtimeReady && realtimeRef.current?.send("message.send", payload, requestId)) {
      pendingRequestsRef.current.set(requestId, { kind: "send", clientMessageId, roomNo: requestedRoomNo, roomSession: requestedSession, body });
      return;
    }
    try {
      const applied = await sendChatMessageFallback(requestedRoomNo, requestedSession, clientMessageId, body);
      if (!applied) composerSubmissionGuardRef.current.release(clientMessageId);
    } catch (error) {
      if (!isCurrentRoomSession(requestedRoomNo, requestedSession)) {
        composerSubmissionGuardRef.current.release(clientMessageId);
        return;
      }
      replaceMessages(messagesRef.current.filter((message) => message.client_message_id !== clientMessageId || !message.pending));
      composerSubmissionGuardRef.current.release(clientMessageId);
      setComposer(body);
      setComposerError(errorMessage(error, "消息发送失败，请重试。"));
    }
  }

  async function deleteMessage(message) {
    const messageId = chatId(message?.id);
    if (!messageId || chatId(message?.sender_id) !== currentUserId || Number(message?.status) === 2) return;
    const requestedRoomNo = activeRoomNo;
    const requestedSession = roomSessionRef.current;
    const requestToken = token;
    if (!isCurrentRoomOperation(requestedRoomNo, requestedSession, requestToken)) return;
    setDeletingMessageId(messageId);
    setComposerError("");
    try {
      const data = await bbsApi.deleteChatMessage(requestedRoomNo, messageId, requestToken);
      if (!isCurrentRoomOperation(requestedRoomNo, requestedSession, requestToken)) return;
      applyDeletedMessage(data?.message || data);
    } catch (error) {
      if (isCurrentRoomOperation(requestedRoomNo, requestedSession, requestToken)) {
        setComposerError(errorMessage(error, "消息删除失败，请重试。"));
      }
    } finally {
      if (isCurrentRoomOperation(requestedRoomNo, requestedSession, requestToken)) {
        setDeletingMessageId((current) => current === messageId ? "" : current);
      }
    }
  }

  function handleComposerKeyDown(event) {
    if (event.key === "Enter" && !event.shiftKey && !event.nativeEvent?.isComposing) {
      event.preventDefault();
      sendMessage();
    }
  }

  async function joinRoom(roomNumber = activeRoomNo) {
    const normalized = chatRoomNo(roomNumber);
    if (!ROOM_NUMBER_PATTERN.test(normalized)) {
      setDialogError("请输入 8 位有效房间号。");
      return;
    }
    setDialogLoading(true);
    setDialogError("");
    try {
      await bbsApi.joinChatRoom(normalized, token);
      setRoomDialogMode(null);
      setRoomDialogPreview(null);
      if (normalized !== activeRoomNo) navigate(`/room/${normalized}`);
      else setReloadKey((value) => value + 1);
    } catch (error) {
      setDialogError(errorMessage(error, "加入房间失败。"));
    } finally {
      setDialogLoading(false);
    }
  }

  async function lookupRoom(roomNumber) {
    const normalized = chatRoomNo(roomNumber);
    if (!ROOM_NUMBER_PATTERN.test(normalized)) {
      setDialogError("请输入 8 位有效房间号。");
      return;
    }
    setDialogLoading(true);
    setDialogError("");
    try {
      setRoomDialogPreview(await bbsApi.lookupChatRoom(normalized, token));
    } catch (error) {
      setRoomDialogPreview(null);
      setDialogError(errorMessage(error, "没有找到这个房间。"));
    } finally {
      setDialogLoading(false);
    }
  }

  async function createRoom(name) {
    setDialogLoading(true);
    setDialogError("");
    try {
      const data = normalizeChatDetails(await bbsApi.createChatRoom({ name: name.trim() }, token));
      const createdRoomNo = chatRoomNo(data.room?.room_no);
      setRoomDialogMode(null);
      await loadSidebar({ quiet: true });
      navigate(`/room/${createdRoomNo}`);
    } catch (error) {
      setDialogError(errorMessage(error, "房间创建失败。"));
    } finally {
      setDialogLoading(false);
    }
  }

  async function createGroup(event) {
    event.preventDefault();
    if (!groupName.trim()) return;
    setGroupSaving(true);
    try {
      await bbsApi.createChatGroup({ name: groupName.trim(), sort_order: sidebar.groups.length }, token);
      setGroupName("");
      setGroupEditor(false);
      await loadSidebar({ quiet: true });
    } catch (error) {
      setComposerError(errorMessage(error, "分组创建失败。"));
    } finally {
      setGroupSaving(false);
    }
  }

  function startEditingGroup(group) {
    setGroupEditor(false);
    setGroupName("");
    setDeletingGroupId("");
    setEditingGroupId(chatId(group?.id));
    setEditingGroupName(group?.name || "");
  }

  function cancelEditingGroup() {
    setEditingGroupId("");
    setEditingGroupName("");
  }

  async function updateGroup(event, group) {
    event.preventDefault();
    const name = editingGroupName.trim();
    if (!name || !group?.id) return;
    setGroupSaving(true);
    try {
      await bbsApi.updateChatGroup(group.id, { name, sort_order: Number(group.sort_order || 0) }, token);
      await loadSidebar({ quiet: true });
      cancelEditingGroup();
    } catch (error) {
      setComposerError(errorMessage(error, "分组更新失败。"));
    } finally {
      setGroupSaving(false);
    }
  }

  async function reorderGroup(group, direction) {
    if (!group?.id || (direction !== -1 && direction !== 1)) return;
    setGroupSaving(true);
    try {
      await bbsApi.moveChatGroup(group.id, direction, token);
      await loadSidebar({ quiet: true });
    } catch (error) {
      setComposerError(errorMessage(error, "分组排序失败。"));
      loadSidebar({ quiet: true }).catch(() => {});
    } finally {
      setGroupSaving(false);
    }
  }

  function requestDeleteGroup(group) {
    cancelEditingGroup();
    setDeletingGroupId(chatId(group?.id));
  }

  async function deleteGroup(group) {
    if (!group?.id) return;
    setGroupSaving(true);
    try {
      await bbsApi.deleteChatGroup(group.id, token);
      await loadSidebar({ quiet: true });
      setDeletingGroupId("");
    } catch (error) {
      setComposerError(errorMessage(error, "分组删除失败。"));
    } finally {
      setGroupSaving(false);
    }
  }

  function toggleManageMode() {
    setManageMode((value) => !value);
    setGroupEditor(false);
    setGroupName("");
    cancelEditingGroup();
    setDeletingGroupId("");
  }

  async function placeRoom(roomNumber, groupId, sortOrder = 0) {
    if (placementSaving) return;
    setPlacementSaving(true);
    try {
      await bbsApi.placeChatRoom(roomNumber, { group_id: groupId || "0", sort_order: sortOrder }, token);
      await loadSidebar({ quiet: true });
    } catch (error) {
      setComposerError(errorMessage(error, "房间移动失败。"));
    } finally {
      setPlacementSaving(false);
    }
  }

  async function markAnnouncementSeen() {
    const requestedRoomNo = activeRoomNo;
    const requestedSession = roomSessionRef.current;
    const requestToken = token;
    const version = chatInteger(roomRef.current?.announcement_version);
    if (compareChatIntegers(version, "0") <= 0 || compareChatIntegers(version, membershipRef.current?.last_seen_announcement_version) <= 0) return;
    if (!isCurrentRoomOperation(requestedRoomNo, requestedSession, requestToken)) return;
    try {
      const data = await bbsApi.markChatAnnouncementSeen(requestedRoomNo, version, requestToken);
      if (!isCurrentRoomOperation(requestedRoomNo, requestedSession, requestToken)) return;
      if (data?.membership) updateMembership(data.membership);
    } catch (error) {
      if (isCurrentRoomOperation(requestedRoomNo, requestedSession, requestToken)) {
        setAnnouncementError(errorMessage(error, "公告状态同步失败。"));
      }
    }
  }

  async function saveAnnouncement(announcement) {
    const requestedRoomNo = activeRoomNo;
    const requestedSession = roomSessionRef.current;
    const requestToken = token;
    if (!isCurrentRoomOperation(requestedRoomNo, requestedSession, requestToken)) return;
    setAnnouncementSaving(true);
    setAnnouncementError("");
    try {
      const data = await bbsApi.updateChatAnnouncement(requestedRoomNo, announcement, requestToken);
      if (!isCurrentRoomOperation(requestedRoomNo, requestedSession, requestToken)) return;
      absorbUsers(data?.users || []);
      if (data?.room) updateRoom(data.room);
      scheduleSidebarRefresh();
    } catch (error) {
      if (isCurrentRoomOperation(requestedRoomNo, requestedSession, requestToken)) {
        setAnnouncementError(errorMessage(error, "公告保存失败。"));
      }
    } finally {
      if (isCurrentRoomOperation(requestedRoomNo, requestedSession, requestToken)) {
        setAnnouncementSaving(false);
      }
    }
  }

  function openRoomDialog(mode) {
    setRoomDialogMode(mode);
    setRoomDialogPreview(null);
    setDialogError("");
  }

  if (!token) {
    return (
      <main className="chat-auth-shell">
        <section className="chat-auth-panel panel">
          <ShieldCheck size={34} aria-hidden="true" />
          <h1>登录后进入房间</h1>
          <p>聊天记录和已读位置会在你的设备之间同步。</p>
          <Link to={`/user/signin?redirect=${encodeURIComponent(chatPath)}`}>
            前往登录
            <ArrowRight size={17} aria-hidden="true" />
          </Link>
        </section>
      </main>
    );
  }

  const unreadIndex = unreadChatIndex(messages, initialReadSeq);
  const canEditAnnouncement = Number(membership?.role || 0) === 1;
  const roomActive = Number(room?.status || 0) === 1;

  return (
    <main className={`chat-page ${sidebarOpen ? "is-sidebar-open" : ""}`}>
      <div className="chat-sidebar-wrap">
        <ChatSidebar
          groups={sidebar.groups}
          rooms={sidebar.rooms}
          users={users}
          activeRoomNo={activeRoomNo}
          loading={sidebarLoading}
          manageMode={manageMode}
          groupEditor={groupEditor}
          groupName={groupName}
          groupSaving={groupSaving}
          placementSaving={placementSaving}
          onGroupNameChange={setGroupName}
          onSubmitGroup={createGroup}
          onCancelGroup={() => { setGroupEditor(false); setGroupName(""); }}
          editingGroupId={editingGroupId}
          editingGroupName={editingGroupName}
          deletingGroupId={deletingGroupId}
          onStartEditGroup={startEditingGroup}
          onEditGroupNameChange={setEditingGroupName}
          onSubmitGroupEdit={updateGroup}
          onCancelGroupEdit={cancelEditingGroup}
          onMoveGroup={reorderGroup}
          onRequestDeleteGroup={requestDeleteGroup}
          onConfirmDeleteGroup={deleteGroup}
          onCancelDeleteGroup={() => setDeletingGroupId("")}
          onToggleManage={toggleManageMode}
          onCreateGroup={() => { setDeletingGroupId(""); cancelEditingGroup(); setGroupEditor(true); }}
          onPlaceRoom={placeRoom}
          onSelectRoom={(roomNumber) => { setSidebarOpen(false); navigate(`/room/${roomNumber}`); }}
          onOpenRoomDialog={openRoomDialog}
        />
      </div>

      <section className="chat-room-pane" aria-label={room?.name || activeRoomNo || "聊天"}>
        <header className="chat-room-header panel">
          <button className="chat-mobile-sidebar-btn" type="button" title="房间列表" aria-label="房间列表" onClick={() => setSidebarOpen((value) => !value)}>
            <PanelLeft size={20} aria-hidden="true" />
          </button>
          <div className="chat-room-header__copy">
            <span>{activeRoomNo || "实时通讯"}</span>
            <h1>{room?.name || preview?.name || (phase === "lobby" ? "选择一个房间" : phase === "loading" ? "正在加载房间" : "聊天室")}</h1>
            {room && <p>{memberCount} 位成员</p>}
          </div>
          <div className={`chat-connection chat-connection--${connectionStatus}`} title={connectionLabel(connectionStatus)}>
            {connectionStatus === "connected" ? <Wifi size={15} aria-hidden="true" /> : <WifiOff size={15} aria-hidden="true" />}
            <span>{connectionLabel(connectionStatus)}</span>
          </div>
          {room && (
            <div className="chat-room-header__actions">
              <button type="button" title="房间公告" aria-label="房间公告" onClick={() => { setAnnouncementError(""); setAnnouncementOpen(true); }}>
                <Megaphone size={19} aria-hidden="true" />
              </button>
              <button type="button" title="分享房间" aria-label="分享房间" onClick={() => setShareOpen(true)}>
                <Share2 size={19} aria-hidden="true" />
              </button>
            </div>
          )}
        </header>

        {phase === "loading" && (
          <section className="chat-room-state panel">
            <LoaderCircle className="chat-spin" size={30} aria-hidden="true" />
            <h2>正在同步房间</h2>
          </section>
        )}
        {phase === "error" && (
          <section className="chat-room-state panel">
            <AlertCircle size={30} aria-hidden="true" />
            <h2>{pageError}</h2>
            <button type="button" onClick={() => setReloadKey((value) => value + 1)}>重试</button>
          </section>
        )}
        {phase === "lobby" && (
          <section className="chat-room-state panel">
            <PanelLeft size={30} aria-hidden="true" />
            <h2>{sidebar.rooms.length ? "从左侧选择一个房间" : "开始第一段对话"}</h2>
            <p>{sidebar.rooms.length ? "选择已有房间继续讨论，或创建一个新的讨论空间。" : "创建新房间，或输入房间号加入已有讨论空间。"}</p>
            <button className="chat-primary-btn" type="button" onClick={() => openRoomDialog(sidebar.rooms.length ? "join" : "create")}>
              {sidebar.rooms.length ? "查找或创建房间" : "创建房间"}
            </button>
          </section>
        )}
        {phase === "preview" && preview && (
          <section className="chat-room-state chat-room-preview-state panel">
            <CircleCheck size={32} aria-hidden="true" />
            <span>{preview.room_no}</span>
            <h2>{preview.name}</h2>
            <p>{preview.member_count || 0} 位成员</p>
            <button className="chat-primary-btn" type="button" disabled={dialogLoading} onClick={() => joinRoom(preview.room_no)}>
              {dialogLoading ? "加入中..." : "加入房间"}
            </button>
            {dialogError && <small className="chat-form-error">{dialogError}</small>}
          </section>
        )}
        {phase === "ready" && (
          <>
            {repairing && <div className="chat-sync-banner"><LoaderCircle className="chat-spin" size={14} aria-hidden="true" />正在补齐消息</div>}
            {composerError && (
              <div className="chat-error-banner" role="status">
                <AlertCircle size={15} aria-hidden="true" />
                <span>{composerError}</span>
                <button type="button" aria-label="关闭提示" onClick={() => setComposerError("")}>×</button>
              </div>
            )}
            <ChatTimeline
              messages={messages}
              users={users}
              currentUserId={currentUserId}
              unreadIndex={unreadIndex}
              hasOlder={messagePage.hasOlder}
              loadingOlder={loadingOlder}
              hasNewer={messagePage.hasNewer}
              loadingNewer={loadingNewer}
              loading={false}
              scrollRef={scrollRef}
              onScroll={handleTimelineScroll}
              onLoadOlder={loadOlder}
              onLoadNewer={loadNewer}
              onJumpLatest={async () => {
                await loadNewer({ scrollToLatest: true });
                if (!scrollRef.current) return;
                scrollRef.current.scrollTop = scrollRef.current.scrollHeight;
                scheduleRead(latestChatSeq(messagesRef.current));
              }}
              deletingMessageId={deletingMessageId}
              onDeleteMessage={deleteMessage}
            />
            <form className="chat-composer panel" onSubmit={sendMessage}>
              <textarea
                aria-label="消息内容"
                maxLength={4000}
                placeholder={roomActive ? "输入消息" : "房间已关闭"}
                value={composer}
                disabled={!roomActive}
                onChange={(event) => {
                  composerSubmissionGuardRef.current.reset();
                  setComposer(event.target.value);
                }}
                onKeyDown={handleComposerKeyDown}
              />
              <footer>
                <span>{composer.length}/4000</span>
                <button className="chat-primary-btn" type="submit" disabled={!roomActive || !composer.trim()}>
                  <Send size={17} aria-hidden="true" />
                  发送
                </button>
              </footer>
            </form>
          </>
        )}
      </section>

      {sidebarOpen && <button className="chat-sidebar-scrim" type="button" aria-label="关闭房间列表" onClick={() => setSidebarOpen(false)} />}
      {roomDialogMode && (
        <ChatRoomDialog
          mode={roomDialogMode}
          preview={roomDialogPreview}
          loading={dialogLoading}
          error={dialogError}
          onClose={() => setRoomDialogMode(null)}
          onLookup={lookupRoom}
          onJoin={joinRoom}
          onCreate={createRoom}
        />
      )}
      {announcementOpen && room && (
        <ChatAnnouncementDialog
          room={room}
          canEdit={canEditAnnouncement}
          loading={announcementSaving}
          error={announcementError}
          onClose={() => setAnnouncementOpen(false)}
          onSave={saveAnnouncement}
          onSeen={markAnnouncementSeen}
        />
      )}
      {shareOpen && <ChatShareDialog roomNo={activeRoomNo} onClose={() => setShareOpen(false)} />}
    </main>
  );
}

export default ChatPage;
