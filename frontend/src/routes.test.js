import assert from "node:assert/strict";
import fs from "node:fs";
import test from "node:test";

import { pageRoutes, pathToPage } from "./routes.js";

test("maps chat workspace routes to the chat navigation section", () => {
  assert.equal(pageRoutes.find((route) => route.key === "chat")?.path, "/chat");
  assert.equal(pathToPage("/chat"), "聊天室");
  assert.equal(pathToPage("/room/AB12CD3E"), "聊天室");
});

test("maps username profile routes to the member navigation section", () => {
  assert.equal(pathToPage("/u/alice"), "会员");
  assert.equal(pathToPage("/u/alice/articles"), "会员");
});

test("keeps explicit chat entries on user message surfaces", () => {
  const messageSurfaces = [
    fs.readFileSync(new URL("./pages/UserRoutes.jsx", import.meta.url), "utf8"),
    fs.readFileSync(new URL("./pages/UserDashboardRoutes.jsx", import.meta.url), "utf8")
  ];

  for (const source of messageSurfaces) {
    assert.match(source, /进入聊天室/);
    assert.match(source, /navigate\("\/chat"\)/);
  }
});

test("keeps a desktop floating chat entry", () => {
  const source = fs.readFileSync(new URL("./components/layout/FloatingRail.jsx", import.meta.url), "utf8");

  assert.match(source, /label:\s*"聊天室"/);
  assert.match(source, /path:\s*"\/chat"/);
});

test("keeps chat, return, and logout actions in the current app shell", () => {
  const appSource = fs.readFileSync(new URL("./App.jsx", import.meta.url), "utf8");
  const sideNavSource = fs.readFileSync(new URL("./components/layout/PageColumns.jsx", import.meta.url), "utf8");

  assert.match(appSource, /AppSessionContext\.Provider value=\{\{ auth, onLogout: handleLogout \}\}/);
  assert.match(appSource, /<ChatPage auth=\{auth\} onLogout=\{handleLogout\}/);
  assert.match(sideNavSource, /label: "聊天室", path: "\/chat"/);
  assert.match(sideNavSource, /className="nav-logout-btn"/);
  assert.match(sideNavSource, /aria-label="退出登录"/);

  const chatSource = fs.readFileSync(new URL("./pages/ChatPage.jsx", import.meta.url), "utf8");
  assert.match(chatSource, /className="chat-session-logout"/);
  assert.match(chatSource, /className="chat-back-to-community"/);
  assert.match(chatSource, /navigate\("\/plaza"\)/);

  const chatSidebarSource = fs.readFileSync(new URL("./components/chat/ChatSidebar.jsx", import.meta.url), "utf8");
  assert.match(chatSidebarSource, /import \{ currentTheme, subscribeTheme, toggleTheme \}/);
  assert.match(chatSidebarSource, /className="chat-theme-toggle"/);
  assert.match(chatSidebarSource, /onClick=\{\(\) => setTheme\(toggleTheme\(\)\)\}/);
});

test("keeps public announcements and hashtag search connected to the app shell", () => {
  const appSource = fs.readFileSync(new URL("./App.jsx", import.meta.url), "utf8");
  const announcementSource = fs.readFileSync(new URL("./components/SiteAnnouncements.jsx", import.meta.url), "utf8");
  const contentSource = fs.readFileSync(new URL("./pages/ContentRoutes.jsx", import.meta.url), "utf8");

  assert.match(appSource, /import SiteAnnouncements from "\.\/components\/SiteAnnouncements\.jsx"/);
  assert.match(appSource, /<SiteAnnouncements \/>/);
  assert.match(appSource, /bbsApi\.trendingHashtags\(\{ limit: 8 \}\)/);
  assert.match(announcementSource, /bbsApi\s*\.announcements\(\{ limit: 20 \}\)/);
  assert.match(announcementSource, /announcementDismissalKey\(announcement\)/);
  assert.match(contentSource, /bbsApi\.searchHashtags\(query, \{ limit: SEARCH_PAGE_SIZE/);
  assert.match(contentSource, /className="search-hashtag-results panel"/);
});

test("keeps user safety controls connected to authenticated routes", () => {
  const appSource = fs.readFileSync(new URL("./App.jsx", import.meta.url), "utf8");
  const userSource = fs.readFileSync(new URL("./pages/UserRoutes.jsx", import.meta.url), "utf8");

  assert.match(appSource, /path="\/user\/safety"/);
  assert.match(userSource, /value: "safety", label: "屏蔽与静音"/);
  assert.match(userSource, /bbsApi\.userSafetyState\(profileUserId, auth\.accessToken\)/);
  assert.match(userSource, /bbsApi\.blockUser\(profileUserId, auth\.accessToken\)/);
  assert.match(userSource, /bbsApi\.muteUser\(profileUserId, auth\.accessToken\)/);
  assert.match(userSource, /function UserSafetyPanel\(\{ auth \}\)/);
  assert.match(userSource, /const relationSessionRef = React\.useRef\(0\)/);
  assert.match(userSource, /const requestSessionRef = React\.useRef\(0\)/);
  assert.match(userSource, /const page = state\.page \+ 1/);
  assert.match(userSource, /window\.confirm\(message\)/);
});

test("keeps named collection management inside the existing favorites route", () => {
  const source = fs.readFileSync(new URL("./pages/UserRoutes.jsx", import.meta.url), "utf8");

  assert.match(source, /activeValue === "favorites" && <UserFavoritesPanel auth=\{auth\}/);
  assert.match(source, /bbsApi\.createCollection/);
  assert.match(source, /bbsApi\.updateCollection/);
  assert.match(source, /bbsApi\.deleteCollection/);
  assert.match(source, /bbsApi\.addCollectionItem/);
  assert.match(source, /bbsApi\.removeCollectionItem/);
});

test("connects user-list management and timelines to member routes", () => {
  const appSource = fs.readFileSync(new URL("./App.jsx", import.meta.url), "utf8");
  const userSource = fs.readFileSync(new URL("./pages/UserRoutes.jsx", import.meta.url), "utf8");

  assert.equal(pathToPage("/user-lists/9223372036854775000"), "会员");
  assert.match(appSource, /path="\/user\/lists"/);
  assert.match(appSource, /path="\/user\/:userId\/lists"/);
  assert.match(appSource, /path="\/u\/:username\/lists"/);
  assert.match(appSource, /path="\/user-lists\/:listId"/);
  assert.match(appSource, /<UserListDetailPage auth=\{auth\}/);
  assert.match(userSource, /activeValue === "lists" && <UserListsPanel/);
  assert.match(userSource, /bbsApi\.createUserList/);
  assert.match(userSource, /bbsApi\.addUserListMember/);
  assert.match(userSource, /bbsApi\.copyUserList/);
  assert.match(userSource, /bbsApi\.userListFeed\(listId, \{ limit: USER_LIST_FEED_PAGE_SIZE, offset: state\.feedOffset \}/);
});

test("connects two-factor login and account security management", () => {
  const authSource = fs.readFileSync(new URL("./pages/AuthRoutes.jsx", import.meta.url), "utf8");
  const userSource = fs.readFileSync(new URL("./pages/UserRoutes.jsx", import.meta.url), "utf8");
  const apiSource = fs.readFileSync(new URL("./api.js", import.meta.url), "utf8");

  assert.match(authSource, /mfaChallengeFromResponse\(data\)/);
  assert.match(authSource, /bbsApi\.completeMfaLogin/);
  assert.match(authSource, /if \(mfaChallenge\)/);
  assert.match(authSource, /params\.get\("mfa_required"\) === "true"/);
  assert.match(authSource, /setMfaChallenge\(challenge\)/);
  assert.match(authSource, /async function submitMFA/);
  assert.match(userSource, /bbsApi\.mfaStatus/);
  assert.match(userSource, /bbsApi\.beginTotpEnrollment/);
  assert.match(userSource, /bbsApi\.confirmTotpEnrollment/);
  assert.match(userSource, /bbsApi\.regenerateMfaRecoveryCodes/);
  assert.match(userSource, /bbsApi\.disableTotp/);
  assert.match(userSource, /recoveryCodes\.map/);
  assert.match(apiSource, /request\("\/auth\/login\/mfa"/);
  assert.match(apiSource, /request\("\/users\/me\/mfa"/);
});

test("shows every joined room's latest message and time in the chat sidebar", () => {
  const source = fs.readFileSync(new URL("./components/chat/ChatSidebar.jsx", import.meta.url), "utf8");

  assert.match(source, /import \{ timeAgoMillis \} from "\.\.\/\.\.\/lib\/formatters"/);
  assert.match(source, /function displayLastMessageTime\(room\)/);
  assert.match(source, /Number\(message\?\.status\) === 2\) return "这条消息已删除"/);
  assert.match(source, /room\?\.last_message\?\.created_at/);
  assert.match(source, /const \[, refreshRelativeTime\] = React\.useState\(0\)/);
  assert.match(source, /window\.setInterval\(\(\) => refreshRelativeTime\(\(value\) => value \+ 1\), 60_000\)/);
  assert.match(source, /return \(\) => window\.clearInterval\(intervalId\)/);
  assert.match(source, /const lastMessageTime = displayLastMessageTime\(item\)/);
  assert.match(source, /<small>\{displayLastMessage\(item, userMap\)\}<\/small>/);
  assert.match(source, /<time>\{lastMessageTime\}<\/time>/);
});

test("loads sidebar popular channels and resources from backend rankings", () => {
  const sidebar = fs.readFileSync(new URL("./components/layout/PageColumns.jsx", import.meta.url), "utf8");
  const resources = fs.readFileSync(new URL("./pages/SectionPages.jsx", import.meta.url), "utf8");
  const resourceCard = fs.readFileSync(new URL("./pages/SectionBlocks.jsx", import.meta.url), "utf8");

  assert.match(sidebar, /bbsApi\.popularChatRooms\(\{ limit: 5 \}\)/);
  assert.match(sidebar, /bbsApi\.popularResources\(\{ limit: 5 \}\)/);
  assert.match(sidebar, /import \{ safeExternalURL \}/);
  assert.match(sidebar, /url: safeExternalURL\(item\?\.url \?\? item\?\.URL\)/);
  assert.match(sidebar, /function recordPopularResourceVisit\(resource\)/);
  assert.match(sidebar, /onClick=\{\(\) => recordPopularResourceVisit\(resource\)\}/);
  assert.doesNotMatch(sidebar, /const hotChatChannels =/);
  assert.doesNotMatch(sidebar, /const hotResources =/);
  assert.match(resources, /const RESOURCE_ACTIVITY_LIMIT = 3/);
  assert.match(resources, /bbsApi\s*\.\s*popularResources\(\{ limit: RESOURCE_ACTIVITY_LIMIT \}\)/);
  assert.match(resources, /const activityItems = resourceActivityItems\(activityState\.items\)/);
  assert.match(resources, /<TrendBar key=\{resource\.key\} label=\{resource\.label\} value=\{resource\.value\} \/>/);
  assert.doesNotMatch(resources, /76 - index \* 14/);
  assert.match(resources, /bbsApi\.recordResourceVisit\(resource\.id\)/);
  assert.match(resourceCard, /export function ResourceCard\(\{ resource, onVisit \}\)/);
  assert.match(resourceCard, /onClick=\{onVisit\}/);
});

test("links home recent community items to detail pages", () => {
  const source = fs.readFileSync(new URL("./pages/SectionPages.jsx", import.meta.url), "utf8");
  const home = source.slice(source.indexOf("export function HomePage"), source.indexOf("export function CirclesPage"));
  const mapper = source.slice(source.indexOf("function homeContentItem"), source.indexOf("function topicToQuestion"));

  assert.match(source, /import \{ Link, useNavigate, useSearchParams \}/);
  assert.match(home, /<Link className="timeline-item-link" to=\{item\.path\}>/);
  assert.match(home, /item\.path \?/);
  assert.match(mapper, /const id = toId\(item\?\.id\)/);
  assert.match(mapper, /path: id \? `\/\$\{type === "文章" \? "article" : "topic"\}\/\$\{encodeURIComponent\(id\)\}` : ""/);
});

test("links help reply cues to answered topic detail pages", () => {
  const source = fs.readFileSync(new URL("./pages/SectionPages.jsx", import.meta.url), "utf8");
  const blocks = fs.readFileSync(new URL("./pages/SectionBlocks.jsx", import.meta.url), "utf8");
  const help = source.slice(source.indexOf("export function HelpPage"), source.indexOf("export function ResourcesPage"));
  const mapper = source.slice(source.indexOf("function topicToQuestion"), source.indexOf("function linkToResource"));

  assert.match(help, /const replyCueQuestions = questions\.filter\(\(question\) => question\.answers > 0\)\.slice\(0, 4\)/);
  assert.match(help, /replyCueQuestions\.map\(\(question\) => \(/);
  assert.match(help, /actionLabel="查看话题"/);
  assert.match(help, /onAction=\{question\.path \? \(\) => navigate\(question\.path\) : undefined\}/);
  assert.doesNotMatch(help, /questions\.slice\(0, 4\)\.map/);
  assert.match(mapper, /const id = toId\(topic\?\.id\)/);
  assert.match(mapper, /path: id \? `\/topic\/\$\{encodeURIComponent\(id\)\}` : ""/);
  assert.match(blocks, /const detailPath = question\.path \|\| \(question\.id \? `\/topic\/\$\{question\.id\}` : "\/help"\)/);
});

test("content list pages support keyword search through backend search APIs", () => {
  const source = fs.readFileSync(new URL("./pages/ContentRoutes.jsx", import.meta.url), "utf8");
  const listPage = source.slice(
    source.indexOf("export function ContentListPage"),
    source.indexOf("export function ContentDetailPage")
  );

  assert.match(listPage, /const \[searchParams, setSearchParams\] = useSearchParams\(\)/);
  assert.match(listPage, /const keyword = searchParams\.get\("q"\)\?\.trim\(\) \|\| ""/);
  assert.match(listPage, /const listSearchEnabled = filter === "all"/);
  assert.match(listPage, /const activeKeyword = listSearchEnabled \? keyword : ""/);
  assert.match(listPage, /bbsApi\.searchArticles\(activeKeyword, \{ page, page_size: CONTENT_PAGE_SIZE \}\)/);
  assert.match(listPage, /bbsApi\.searchTopics\(activeKeyword, \{ page, page_size: CONTENT_PAGE_SIZE \}\)/);
  assert.match(listPage, /const mapper = isArticle \? searchHitToPost : topicSearchHitToPost/);
  assert.match(listPage, /<form className="search-page-form panel" role="search" onSubmit=\{submitListSearch\}>/);
  assert.match(listPage, /setSearchParams\(next/);
  assert.match(listPage, /\{!activeKeyword && <PillTabs items=\{sortTabs\}/);
});

test("paginates auxiliary links and tasks through their real APIs", () => {
  const source = fs.readFileSync(new URL("./pages/AuxiliaryPages.jsx", import.meta.url), "utf8");
  const loaderStart = source.indexOf("function loadAuxiliaryPage");
  const loader = source.slice(loaderStart, source.indexOf("function auxiliaryListLabel", loaderStart));
  const loadMoreStart = source.indexOf("async function loadMoreRows");
  const loadMore = source.slice(loadMoreStart, source.indexOf("async function handleTaskAction", loadMoreStart));

  assert.match(source, /const AUXILIARY_PAGE_SIZE = 30;/);
  assert.match(source, /loadAuxiliaryPage\(kind, token, 0\)/);
  assert.match(loader, /const params = \{ limit: AUXILIARY_PAGE_SIZE, offset \}/);
  assert.match(loader, /if \(kind === "links"\) return bbsApi\.links\(params\)/);
  assert.match(loader, /if \(token\) return bbsApi\.myTasks\(params, token\)/);
  assert.match(loader, /return bbsApi\.tasks\(params\)/);
  assert.match(loadMore, /kind !== "links" && kind !== "tasks"/);
  assert.match(loadMore, /const data = await loadAuxiliaryPage\(kind, token, offset\)/);
  assert.match(loadMore, /const rows = mergeAuxiliaryRows\(current\.rows, nextRows\)/);
  assert.match(source, /const canLoadMore = \(kind === "links" \|\| kind === "tasks"\)/);
  assert.match(source, /aria-label=\{`加载更多\$\{listLabel\}`\}/);
  assert.match(source, /onClick=\{loadMoreRows\}/);
  assert.doesNotMatch(source, /async function loadMoreLinks/);
});

test("chat message fallbacks ignore stale room sessions", () => {
  const source = fs.readFileSync(new URL("./pages/ChatPage.jsx", import.meta.url), "utf8");
  const fallbackStart = source.indexOf("const sendChatMessageFallback");
  const fallback = source.slice(fallbackStart, source.indexOf("const advanceChatReadFallback", fallbackStart));
  const sendStart = source.indexOf("async function sendMessage");
  const send = source.slice(sendStart, source.indexOf("async function deleteMessage", sendStart));
  const realtimeStart = source.indexOf("const handleRealtimeEvent");
  const realtime = source.slice(realtimeStart, source.indexOf("eventHandlerRef.current", realtimeStart));

  assert.match(source, /roomMatches\(message, activeRoomNoRef\.current, roomRef\.current\)/);
  assert.match(fallback, /async \(roomNo, roomSession, clientMessageId, body\)/);
  assert.match(fallback, /await bbsApi\.sendChatMessage\(roomNo, \{ client_message_id: clientMessageId, body \}, token\)/);
  assert.match(fallback, /if \(!isCurrentRoomSession\(roomNo, roomSession\)\) return false/);

  assert.match(send, /const requestedRoomNo = activeRoomNo/);
  assert.match(send, /const requestedSession = roomSessionRef\.current/);
  assert.match(send, /if \(!isCurrentRoomSession\(requestedRoomNo, requestedSession\)\) return/);
  assert.match(send, /roomSession: requestedSession/);
  assert.match(send, /sendChatMessageFallback\(requestedRoomNo, requestedSession, clientMessageId, body\)/);
  assert.match(send, /if \(!isCurrentRoomSession\(requestedRoomNo, requestedSession\)\) \{\s*composerSubmissionGuardRef\.current\.release\(clientMessageId\)/);

  assert.match(realtime, /sendChatMessageFallback\(pending\.roomNo, pending\.roomSession, pending\.clientMessageId, pending\.body\)/);
  assert.match(realtime, /if \(!isCurrentRoomSession\(pending\.roomNo, pending\.roomSession\)\) \{\s*composerSubmissionGuardRef\.current\.release\(pending\.clientMessageId\)/);
});

test("chat keeps room entry paginated and jumps to the latest bounded window", () => {
  const source = fs.readFileSync(new URL("./pages/ChatPage.jsx", import.meta.url), "utf8");
  const loadRoomStart = source.indexOf("async function loadRoom");
  const loadRoom = source.slice(loadRoomStart, source.indexOf("const rememberEvent", loadRoomStart));
  const jumpStart = source.indexOf("async function jumpToLatest");
  const jump = source.slice(jumpStart, source.indexOf("async function sendMessage", jumpStart));
  const timeline = fs.readFileSync(new URL("./components/chat/ChatTimeline.jsx", import.meta.url), "utf8");

  assert.match(source, /const INITIAL_BEFORE = 30;/);
  assert.match(source, /const INITIAL_AFTER = 30;/);
  assert.match(loadRoom, /\{ anchor_seq: readSeq, before: INITIAL_BEFORE, after: INITIAL_AFTER \}/);
  assert.match(loadRoom, /applyMessagePage\(page, true\)/);
  assert.doesNotMatch(loadRoom, /repairActiveRef\.current\(\)/);

  assert.match(jump, /\{ anchor_seq: knownLatest, before: INITIAL_BEFORE, after: INITIAL_AFTER \}/);
  assert.match(jump, /applyMessagePage\(data, true\)/);
  assert.match(jump, /scheduleRead\(latestChatSeq\(next\)\)/);
  assert.match(timeline, /disabled=\{loadingNewer\}/);
});

test("chat room actions ignore stale room and auth sessions", () => {
  const source = fs.readFileSync(new URL("./pages/ChatPage.jsx", import.meta.url), "utf8");
  const readFallbackStart = source.indexOf("const advanceChatReadFallback");
  const readFallback = source.slice(readFallbackStart, source.indexOf("const replayPendingMessages", readFallbackStart));
  const advanceReadStart = source.indexOf("const advanceRead");
  const advanceRead = source.slice(advanceReadStart, source.indexOf("const scheduleRead", advanceReadStart));
  const deleteStart = source.indexOf("async function deleteMessage");
  const deleteAction = source.slice(deleteStart, source.indexOf("function handleComposerKeyDown", deleteStart));
  const seenStart = source.indexOf("async function markAnnouncementSeen");
  const seen = source.slice(seenStart, source.indexOf("async function saveAnnouncement", seenStart));
  const saveStart = source.indexOf("async function saveAnnouncement");
  const save = source.slice(saveStart, source.indexOf("function openRoomDialog", saveStart));

  assert.match(source, /const activeTokenRef = React\.useRef\(token\)/);
  assert.match(source, /activeTokenRef\.current = token/);
  assert.match(source, /requestToken === activeTokenRef\.current && isCurrentRoomSession\(roomNo, session\)/);
  assert.match(source, /roomMatches\(deleted, activeRoomNoRef\.current, roomRef\.current\)/);
  assert.match(source, /const currentRoomNo = activeRoomNoRef\.current/);
  assert.match(source, /roomMatches\(payload, activeRoomNoRef\.current, roomRef\.current\)/);

  assert.match(readFallback, /async \(roomNo, roomSession, requestToken, readSeq\)/);
  assert.match(readFallback, /if \(!isCurrentRoomOperation\(roomNo, roomSession, requestToken\)\) return false/);
  assert.match(advanceRead, /const requestedSession = roomSessionRef\.current/);
  assert.match(advanceRead, /const requestToken = token/);
  assert.match(advanceRead, /isCurrentRoomOperation\(requestedRoomNo, requestedSession, requestToken\)/);
  assert.match(advanceRead, /roomSession: requestedSession, requestToken, readSeq: target/);
  assert.match(advanceRead, /advanceChatReadFallback\(requestedRoomNo, requestedSession, requestToken, target\)/);

  for (const action of [deleteAction, seen, save]) {
    assert.match(action, /const requestedRoomNo = activeRoomNo/);
    assert.match(action, /const requestedSession = roomSessionRef\.current/);
    assert.match(action, /const requestToken = token/);
    assert.match(action, /isCurrentRoomOperation\(requestedRoomNo, requestedSession, requestToken\)/);
  }
  assert.match(deleteAction, /deleteChatMessage\(requestedRoomNo, messageId, requestToken\)/);
  assert.match(seen, /markChatAnnouncementSeen\(requestedRoomNo, version, requestToken\)/);
  assert.match(save, /updateChatAnnouncement\(requestedRoomNo, announcement, requestToken\)/);
  assert.match(save, /finally \{\s*if \(isCurrentRoomOperation\(requestedRoomNo, requestedSession, requestToken\)\) \{/);
});

test("chat leaves the current room through a guarded membership request", () => {
  const source = fs.readFileSync(new URL("./pages/ChatPage.jsx", import.meta.url), "utf8");
  const leaveStart = source.indexOf("async function leaveRoom");
  const leave = source.slice(leaveStart, source.indexOf("async function joinRoom", leaveStart));
  const dialogs = fs.readFileSync(new URL("./components/chat/ChatDialogs.jsx", import.meta.url), "utf8");

  assert.ok(leaveStart >= 0, "leaveRoom is present");
  assert.match(source, /const leaveRequestRef = React\.useRef\(null\)/);
  assert.match(source, /leaveRequestRef\.current = null;[\s\S]*?setLeaveDialogOpen\(false\);[\s\S]*?setLeavingRoom\(false\);/);
  assert.match(leave, /const requestedRoomNo = activeRoomNo/);
  assert.match(leave, /const requestedSession = roomSessionRef\.current/);
  assert.match(leave, /const requestToken = token/);
  assert.match(leave, /isCurrentRoomOperation\(requestedRoomNo, requestedSession, requestToken\)/);
  assert.match(leave, /leaveRequestRef\.current = request/);
  assert.match(leave, /await bbsApi\.leaveChatRoom\(requestedRoomNo, requestToken\)/);
  assert.match(leave, /await loadSidebar\(\{ quiet: true \}\)\.catch\(\(\) => null\)/);
  assert.match(leave, /navigate\("\/chat"\)/);
  assert.match(source, /event\.type === "room\.member\.joined" \|\| event\.type === "room\.member\.left"/);
  assert.match(source, /title="离开房间" aria-label="离开房间"/);
  assert.match(source, /<ChatLeaveDialog\b/);
  assert.match(dialogs, /export function ChatLeaveDialog/);
  assert.match(dialogs, /房间和聊天记录不会删除/);
});

test("chat sidebar ignores stale auth and superseded responses", () => {
  const source = fs.readFileSync(new URL("./pages/ChatPage.jsx", import.meta.url), "utf8");
  const sidebarStart = source.indexOf("const loadSidebar");
  const sidebar = source.slice(sidebarStart, source.indexOf("const scheduleSidebarRefresh", sidebarStart));

  assert.match(source, /const sidebarRequestVersionRef = React\.useRef\(0\)/);
  assert.match(source, /const sidebarLoadingRequestVersionRef = React\.useRef\(0\)/);
  assert.match(source, /sidebarRequestVersionRef\.current \+= 1;\s*sidebarLoadingRequestVersionRef\.current = 0;[\s\S]*?setSidebar\(\{ groups: \[\], rooms: \[\] \}\);/);
  assert.match(sidebar, /const requestToken = token/);
  assert.match(sidebar, /if \(requestToken !== activeTokenRef\.current\) return null/);
  assert.match(sidebar, /const requestVersion = \+\+sidebarRequestVersionRef\.current/);
  assert.match(sidebar, /requestToken === activeTokenRef\.current && requestVersion === sidebarRequestVersionRef\.current/);
  assert.match(sidebar, /await bbsApi\.chatSidebar\(requestToken\)/);
  assert.match(sidebar, /if \(!isCurrentRequest\(\)\) return null/);
  assert.match(sidebar, /catch \(error\) \{\s*if \(!isCurrentRequest\(\)\) return null/);
  assert.match(sidebar, /sidebarLoadingRequestVersionRef\.current === requestVersion/);
  assert.match(source, /React\.useEffect\(\(\) => \(\) => \{\s*sidebarRequestVersionRef\.current \+= 1/);
});

test("chat room dialog actions ignore stale room and auth sessions", () => {
  const source = fs.readFileSync(new URL("./pages/ChatPage.jsx", import.meta.url), "utf8");
  const joinStart = source.indexOf("async function joinRoom");
  const join = source.slice(joinStart, source.indexOf("async function lookupRoom", joinStart));
  const lookupStart = source.indexOf("async function lookupRoom");
  const lookup = source.slice(lookupStart, source.indexOf("async function createRoom", lookupStart));
  const createStart = source.indexOf("async function createRoom");
  const create = source.slice(createStart, source.indexOf("async function createGroup", createStart));
  const dialogStart = source.indexOf("function openRoomDialog");
  const dialog = source.slice(dialogStart, source.indexOf("if (!token)", dialogStart));

  assert.match(source, /const roomDialogRequestVersionRef = React\.useRef\(0\)/);
  assert.match(source, /roomSessionRef\.current \+= 1;[\s\S]*?invalidateRoomDialogRequests\(\);[\s\S]*?setRoomDialogMode\(null\);/);
  assert.match(source, /requestToken === activeTokenRef\.current &&[\s\S]*?roomSession === roomSessionRef\.current &&[\s\S]*?requestVersion === roomDialogRequestVersionRef\.current/);
  assert.match(source, /requestVersion: \+\+roomDialogRequestVersionRef\.current/);

  for (const action of [join, lookup, create]) {
    assert.match(action, /const \{ requestToken, requestedSession, requestVersion \} = startRoomDialogRequest\(\)/);
    assert.match(action, /const isCurrentRequest = \(\) => isCurrentRoomDialogRequest\(requestToken, requestedSession, requestVersion\)/);
    assert.match(action, /if \(!isCurrentRequest\(\)\) return/);
    assert.match(action, /finally \{\s*if \(isCurrentRequest\(\)\) \{/);
  }

  assert.match(join, /joinChatRoom\(normalized, requestToken\)/);
  assert.match(join, /if \(normalized !== activeRoomNoRef\.current\) navigate/);
  assert.match(lookup, /lookupChatRoom\(normalized, requestToken\)/);
  assert.match(lookup, /setRoomDialogPreview\(data\)/);
  assert.match(create, /createChatRoom\(\{ name: roomName \}, requestToken\)/);
  assert.match(create, /await loadSidebar\(\{ quiet: true \}\)\.catch\(\(\) => null\)/);
  assert.match(create, /if \(!isCurrentRequest\(\)\) return;\s*navigate\(`\/room\/\$\{createdRoomNo\}`\)/);
  assert.match(dialog, /function openRoomDialog\(mode\) \{\s*invalidateRoomDialogRequests\(\);/);
  assert.match(dialog, /function closeRoomDialog\(\) \{\s*invalidateRoomDialogRequests\(\);[\s\S]*?setRoomDialogMode\(null\);/);
  assert.match(source, /onClose=\{closeRoomDialog\}/);
});

test("chat sidebar mutations ignore stale auth sessions", () => {
  const source = fs.readFileSync(new URL("./pages/ChatPage.jsx", import.meta.url), "utf8");
  const createStart = source.indexOf("async function createGroup");
  const create = source.slice(createStart, source.indexOf("function startEditingGroup", createStart));
  const updateStart = source.indexOf("async function updateGroup");
  const update = source.slice(updateStart, source.indexOf("async function reorderGroup", updateStart));
  const reorderStart = source.indexOf("async function reorderGroup");
  const reorder = source.slice(reorderStart, source.indexOf("function requestDeleteGroup", reorderStart));
  const deleteStart = source.indexOf("async function deleteGroup");
  const deleteAction = source.slice(deleteStart, source.indexOf("function toggleManageMode", deleteStart));
  const placeStart = source.indexOf("async function placeRoom");
  const place = source.slice(placeStart, source.indexOf("async function markAnnouncementSeen", placeStart));

  assert.match(source, /const groupMutationRequestVersionRef = React\.useRef\(0\)/);
  assert.match(source, /const placementRequestVersionRef = React\.useRef\(0\)/);
  assert.match(source, /groupMutationRequestVersionRef\.current \+= 1;\s*placementRequestVersionRef\.current \+= 1;[\s\S]*?setGroupSaving\(false\);[\s\S]*?setPlacementSaving\(false\);/);
  assert.match(source, /requestToken === activeTokenRef\.current && requestVersion === groupMutationRequestVersionRef\.current/);
  assert.match(source, /requestToken === activeTokenRef\.current && requestVersion === placementRequestVersionRef\.current/);

  for (const action of [create, update, reorder, deleteAction]) {
    assert.match(action, /const \{ requestToken, requestVersion \} = startGroupMutationRequest\(\)/);
    assert.match(action, /const isCurrentRequest = \(\) => isCurrentGroupMutationRequest\(requestToken, requestVersion\)/);
    assert.match(action, /if \(!isCurrentRequest\(\)\) return/);
    assert.match(action, /await loadSidebar\(\{ quiet: true \}\)\.catch\(\(\) => null\)/);
    assert.match(action, /catch \(error\) \{\s*if \(isCurrentRequest\(\)\) \{/);
    assert.match(action, /finally \{\s*if \(isCurrentRequest\(\)\) \{\s*setGroupSaving\(false\)/);
  }

  assert.match(create, /createChatGroup\(\{ name, sort_order: sortOrder \}, requestToken\)/);
  assert.match(update, /updateChatGroup\(group\.id, \{ name, sort_order: Number\(group\.sort_order \|\| 0\) \}, requestToken\)/);
  assert.match(reorder, /moveChatGroup\(group\.id, direction, requestToken\)/);
  assert.match(deleteAction, /deleteChatGroup\(group\.id, requestToken\)/);
  assert.match(place, /const \{ requestToken, requestVersion \} = startPlacementRequest\(\)/);
  assert.match(place, /const isCurrentRequest = \(\) => isCurrentPlacementRequest\(requestToken, requestVersion\)/);
  assert.match(place, /placeChatRoom\(roomNumber, \{ group_id: groupId \|\| "0", sort_order: sortOrder \}, requestToken\)/);
  assert.match(place, /finally \{\s*if \(isCurrentRequest\(\)\) \{\s*setPlacementSaving\(false\)/);
});

test("shop page uses shared mall order status helpers", () => {
  const source = fs.readFileSync(new URL("./pages/SectionPages.jsx", import.meta.url), "utf8");

  assert.match(source, /mallOrderCanPay/);
  assert.match(source, /mallOrderStatusLabel/);
  assert.doesNotMatch(source, /function formatOrderStatus/);
  assert.doesNotMatch(source, /function orderAwaitingPayment/);
});

test("order history ignores superseded list responses", () => {
  const source = fs.readFileSync(new URL("./pages/UserDashboardRoutes.jsx", import.meta.url), "utf8");

  assert.match(source, /orderLoadRequestVersionRef/);
  assert.match(source, /const requestVersion = \+\+orderLoadRequestVersionRef\.current/);
  assert.match(source, /requestVersion === orderLoadRequestVersionRef\.current/);
  assert.match(source, /if \(!isCurrent\(\)\) return/);
});

test("dashboard content actions ignore stale auth sessions", () => {
  const source = fs.readFileSync(new URL("./pages/UserDashboardRoutes.jsx", import.meta.url), "utf8");
  const contentPanel = source.slice(source.indexOf("function ContentManagerPanel"), source.indexOf("function InteractionsPanel"));
  const loadItemsStart = contentPanel.indexOf("const loadItems");
  const loadItems = contentPanel.slice(loadItemsStart, contentPanel.indexOf("React.useLayoutEffect", loadItemsStart));
  const actionStart = contentPanel.indexOf("async function runContentAction");
  const action = contentPanel.slice(actionStart, contentPanel.indexOf("\n\n  return", actionStart));

  assert.match(contentPanel, /const contentSessionRef = React\.useRef\(0\)/);
  assert.match(contentPanel, /const contentTokenRef = React\.useRef\(auth\.accessToken\)/);
  assert.match(contentPanel, /contentTokenRef\.current = auth\.accessToken/);
  assert.match(contentPanel, /function isCurrentContentSessionRequest\(requestToken, session\)/);
  assert.match(contentPanel, /React\.useLayoutEffect\(\(\) => \{\s*contentSessionRef\.current \+= 1/);

  assert.match(loadItems, /const requestToken = auth\.accessToken/);
  assert.match(loadItems, /const contentSession = contentSessionRef\.current/);
  assert.match(loadItems, /const isCurrentRequest = \(\) => alive && isCurrentContentSessionRequest\(requestToken, contentSession\)/);
  assert.match(loadItems, /loader\(\{ status, limit: DASHBOARD_HISTORY_PAGE_SIZE, offset \}, requestToken\)/);
  assert.match(loadItems, /then\(\(data\) => \{\s*if \(!isCurrentRequest\(\)\) return/);
  assert.match(loadItems, /catch\(\(error\) => \{\s*if \(!isCurrentRequest\(\)\) return/);

  assert.match(action, /const requestToken = auth\.accessToken/);
  assert.match(action, /const contentSession = contentSessionRef\.current/);
  assert.match(action, /const isCurrentRequest = \(\) => isCurrentContentSessionRequest\(requestToken, contentSession\)/);
  assert.match(action, /if \(!requestToken \|\| !isCurrentRequest\(\)\) return/);
  assert.match(action, /await bbsApi[\s\S]*?requestToken/);
  assert.match(action, /await bbsApi[\s\S]*?;\s*}\s*if \(!isCurrentRequest\(\)\) return/);
  assert.match(action, /catch \(error\) \{\s*if \(!isCurrentRequest\(\)\) return/);
  assert.doesNotMatch(action, /auth\.accessToken\)/);
});

test("dashboard interactions ignore stale auth sessions", () => {
  const source = fs.readFileSync(new URL("./pages/UserDashboardRoutes.jsx", import.meta.url), "utf8");
  const interactionsPanel = source.slice(source.indexOf("function InteractionsPanel"), source.indexOf("function MessagesPanel"));
  const loadInteractionsStart = interactionsPanel.indexOf("const loadInteractions");
  const loadInteractions = interactionsPanel.slice(loadInteractionsStart, interactionsPanel.indexOf("React.useLayoutEffect", loadInteractionsStart));
  const actionStart = interactionsPanel.indexOf("async function removeInteraction");
  const action = interactionsPanel.slice(actionStart, interactionsPanel.indexOf("\n\n  return", actionStart));

  assert.match(interactionsPanel, /const interactionSessionRef = React\.useRef\(0\)/);
  assert.match(interactionsPanel, /const interactionTokenRef = React\.useRef\(auth\.accessToken\)/);
  assert.match(interactionsPanel, /interactionTokenRef\.current = auth\.accessToken/);
  assert.match(interactionsPanel, /function isCurrentInteractionSessionRequest\(requestToken, session\)/);
  assert.match(interactionsPanel, /React\.useLayoutEffect\(\(\) => \{\s*interactionSessionRef\.current \+= 1/);

  assert.match(loadInteractions, /const requestToken = auth\.accessToken/);
  assert.match(loadInteractions, /const interactionSession = interactionSessionRef\.current/);
  assert.match(loadInteractions, /const isCurrentRequest = \(\) => alive && isCurrentInteractionSessionRequest\(requestToken, interactionSession\)/);
  assert.match(loadInteractions, /loader\(\{ limit: DASHBOARD_HISTORY_PAGE_SIZE, offset \}, requestToken\)/);
  assert.match(loadInteractions, /then\(async \(data\) => \{\s*if \(!isCurrentRequest\(\)\) return/);
  assert.match(loadInteractions, /await Promise\.all\([\s\S]*?\)\)\)\.filter\(Boolean\);\s*if \(!isCurrentRequest\(\)\) return/);
  assert.match(loadInteractions, /catch\(\(error\) => \{\s*if \(!isCurrentRequest\(\)\) return/);

  assert.match(action, /const requestToken = auth\.accessToken/);
  assert.match(action, /const interactionSession = interactionSessionRef\.current/);
  assert.match(action, /const isCurrentRequest = \(\) => isCurrentInteractionSessionRequest\(requestToken, interactionSession\)/);
  assert.match(action, /if \(!requestToken \|\| !isCurrentRequest\(\)\) return/);
  assert.match(action, /await bbsApi[\s\S]*?requestToken/);
  assert.match(action, /await bbsApi[\s\S]*?;\s*}\s*if \(!isCurrentRequest\(\)\) return/);
  assert.match(action, /catch \(error\) \{\s*if \(!isCurrentRequest\(\)\) return/);
  assert.doesNotMatch(action, /auth\.accessToken\)/);
});

test("dashboard messages ignore stale auth sessions", () => {
  const source = fs.readFileSync(new URL("./pages/UserDashboardRoutes.jsx", import.meta.url), "utf8");
  const messagesPanel = source.slice(source.indexOf("function MessagesPanel"), source.indexOf("function OrdersPanel"));
  const loadMessagesStart = messagesPanel.indexOf("const loadMessages");
  const loadMessages = messagesPanel.slice(loadMessagesStart, messagesPanel.indexOf("React.useLayoutEffect", loadMessagesStart));

  assert.match(messagesPanel, /const messageSessionRef = React\.useRef\(0\)/);
  assert.match(messagesPanel, /const messageTokenRef = React\.useRef\(auth\.accessToken\)/);
  assert.match(messagesPanel, /messageTokenRef\.current = auth\.accessToken/);
  assert.match(messagesPanel, /function isCurrentMessageSessionRequest\(requestToken, session\)/);
  assert.match(messagesPanel, /React\.useLayoutEffect\(\(\) => \{\s*messageSessionRef\.current \+= 1/);

  assert.match(loadMessages, /const requestToken = auth\.accessToken/);
  assert.match(loadMessages, /const messageSession = messageSessionRef\.current/);
  assert.match(loadMessages, /const isCurrentRequest = \(\) => alive && isCurrentMessageSessionRequest\(requestToken, messageSession\)/);
  assert.match(loadMessages, /notifications\(\{ limit: DASHBOARD_HISTORY_PAGE_SIZE, offset \}, requestToken\)/);
  assert.match(loadMessages, /then\(\(data\) => \{\s*if \(!isCurrentRequest\(\)\) return/);
  assert.match(loadMessages, /catch\(\(error\) => \{\s*if \(!isCurrentRequest\(\)\) return/);

  for (const name of ["markRead", "markAllRead", "openNotification"]) {
    const start = messagesPanel.indexOf(`async function ${name}`);
    const end = messagesPanel.indexOf("\n\n  ", start + 1);
    const action = messagesPanel.slice(start, end === -1 ? undefined : end);

    assert.ok(start >= 0, `${name} is present`);
    assert.match(action, /const requestToken = auth\.accessToken/);
    assert.match(action, /const messageSession = messageSessionRef\.current/);
    assert.match(action, /const isCurrentRequest = \(\) => isCurrentMessageSessionRequest\(requestToken, messageSession\)/);
    assert.match(action, /if \(!requestToken \|\| !isCurrentRequest\(\)\) return/);
    assert.match(action, /await bbsApi[\s\S]*?requestToken/);
    assert.match(action, /await bbsApi[\s\S]*?;\s*if \(!isCurrentRequest\(\)\) return/);
    assert.match(action, /catch \(error\) \{\s*if \(!isCurrentRequest\(\)\) return/);
    assert.doesNotMatch(action, /auth\.accessToken\)/);
  }
});

test("dashboard serializes order mutations before button state rerenders", () => {
  const source = fs.readFileSync(new URL("./pages/UserDashboardRoutes.jsx", import.meta.url), "utf8");
  const ordersPanel = source.slice(source.indexOf("function OrdersPanel"), source.indexOf("function EntitlementsPanel"));
  const loadOrdersStart = ordersPanel.indexOf("const loadOrders");
  const loadOrders = ordersPanel.slice(loadOrdersStart, ordersPanel.indexOf("React.useEffect(() => ()", loadOrdersStart));

  assert.match(ordersPanel, /const orderActionSubmittingRef = React\.useRef\(false\)/);
  assert.match(ordersPanel, /orderActionSubmittingRef\.current = true/);
  assert.match(ordersPanel, /finally \{\s*if \(isCurrentRequest\(\)\) orderActionSubmittingRef\.current = false/);
  assert.match(ordersPanel, /disabled=\{orderActionBusy\}/);
  assert.doesNotMatch(loadOrders, /action:\s*""/);
});

test("dashboard order actions ignore stale auth sessions", () => {
  const source = fs.readFileSync(new URL("./pages/UserDashboardRoutes.jsx", import.meta.url), "utf8");
  const ordersPanel = source.slice(source.indexOf("function OrdersPanel"), source.indexOf("function EntitlementsPanel"));

  assert.match(ordersPanel, /const orderSessionRef = React\.useRef\(0\)/);
  assert.match(ordersPanel, /const orderTokenRef = React\.useRef\(auth\.accessToken\)/);
  assert.match(ordersPanel, /orderTokenRef\.current = auth\.accessToken/);
  assert.match(ordersPanel, /function isCurrentOrderSessionRequest\(requestToken, session\)/);
  assert.match(ordersPanel, /React\.useLayoutEffect\(\(\) => \{\s*orderSessionRef\.current \+= 1;\s*orderActionSubmittingRef\.current = false/);

  for (const name of ["payOrder", "cancelOrder", "confirmOrder", "submitRefund", "cancelRefund"]) {
    const start = ordersPanel.indexOf(`async function ${name}`);
    const end = ordersPanel.indexOf("\n\n  ", start + 1);
    const action = ordersPanel.slice(start, end === -1 ? undefined : end);

    assert.ok(start >= 0, `${name} is present`);
    assert.match(action, /const requestToken = auth\.accessToken/);
    assert.match(action, /const orderSession = orderSessionRef\.current/);
    assert.match(action, /const isCurrentRequest = \(\) => isCurrentOrderSessionRequest\(requestToken, orderSession\)/);
    assert.match(action, /if \(!requestToken \|\| !isCurrentRequest\(\)/);
    assert.match(action, /await bbsApi[\s\S]*?requestToken/);
    assert.match(action, /await bbsApi[\s\S]*?;\s*if \(!isCurrentRequest\(\)\) return/);
    assert.match(action, /catch \(error\) \{\s*if \(!isCurrentRequest\(\)\) return/);
    assert.match(action, /finally \{\s*if \(isCurrentRequest\(\)\) orderActionSubmittingRef\.current = false/);
    assert.doesNotMatch(action, /auth\.accessToken\)/);
  }

  for (const name of ["payOrder", "cancelOrder"]) {
    const start = ordersPanel.indexOf(`async function ${name}`);
    const end = ordersPanel.indexOf("\n\n  ", start + 1);
    const action = ordersPanel.slice(start, end === -1 ? undefined : end);

    assert.match(action, /const requestUserId = auth\?\.user\?\.id/);
    assert.match(action, /clearCheckoutAttemptForOrder\(\{ userId: requestUserId/);
  }
});

test("dashboard addresses ignore stale auth sessions", () => {
  const source = fs.readFileSync(new URL("./pages/UserDashboardRoutes.jsx", import.meta.url), "utf8");
  const addressesPanel = source.slice(source.indexOf("function AddressesPanel"), source.indexOf("function RefundsPanel"));
  const loadAddressesStart = addressesPanel.indexOf("const loadAddresses");
  const loadAddresses = addressesPanel.slice(loadAddressesStart, addressesPanel.indexOf("React.useLayoutEffect", loadAddressesStart));

  assert.match(addressesPanel, /const addressSessionRef = React\.useRef\(0\)/);
  assert.match(addressesPanel, /const addressTokenRef = React\.useRef\(auth\.accessToken\)/);
  assert.match(addressesPanel, /addressTokenRef\.current = auth\.accessToken/);
  assert.match(addressesPanel, /function isCurrentAddressSessionRequest\(requestToken, session\)/);
  assert.match(addressesPanel, /React\.useLayoutEffect\(\(\) => \{\s*addressSessionRef\.current \+= 1/);

  assert.match(loadAddresses, /const requestToken = auth\.accessToken/);
  assert.match(loadAddresses, /const addressSession = addressSessionRef\.current/);
  assert.match(loadAddresses, /const isCurrentRequest = \(\) => alive && isCurrentAddressSessionRequest\(requestToken, addressSession\)/);
  assert.match(loadAddresses, /mallAddresses\(\{ limit: DASHBOARD_HISTORY_PAGE_SIZE, offset \}, requestToken\)/);
  assert.match(loadAddresses, /then\(\(data\) => \{\s*if \(!isCurrentRequest\(\)\) return/);
  assert.match(loadAddresses, /catch\(\(error\) => \{\s*if \(!isCurrentRequest\(\)\) return/);

  for (const name of ["saveAddress", "setDefaultAddress", "deleteAddress"]) {
    const start = addressesPanel.indexOf(`async function ${name}`);
    const end = addressesPanel.indexOf("\n\n  ", start + 1);
    const action = addressesPanel.slice(start, end === -1 ? undefined : end);

    assert.ok(start >= 0, `${name} is present`);
    assert.match(action, /const requestToken = auth\.accessToken/);
    assert.match(action, /const addressSession = addressSessionRef\.current/);
    assert.match(action, /const isCurrentRequest = \(\) => isCurrentAddressSessionRequest\(requestToken, addressSession\)/);
    assert.match(action, /if \(!requestToken \|\| !isCurrentRequest\(\)/);
    assert.match(action, /await bbsApi[\s\S]*?requestToken/);
    assert.match(action, /await bbsApi[\s\S]*?;\s*if \(!isCurrentRequest\(\)\) return/);
    assert.match(action, /catch \(error\) \{\s*if \(!isCurrentRequest\(\)\) return/);
    assert.doesNotMatch(action, /auth\.accessToken\)/);
  }
});

test("dashboard refund actions ignore stale auth sessions", () => {
  const source = fs.readFileSync(new URL("./pages/UserDashboardRoutes.jsx", import.meta.url), "utf8");
  const refundsPanel = source.slice(source.indexOf("function RefundsPanel"), source.indexOf("function ReviewsPanel"));
  const loadRefundsStart = refundsPanel.indexOf("const loadRefunds");
  const loadRefunds = refundsPanel.slice(loadRefundsStart, refundsPanel.indexOf("React.useLayoutEffect", loadRefundsStart));
  const actionStart = refundsPanel.indexOf("async function cancelRefund");
  const action = refundsPanel.slice(actionStart, refundsPanel.indexOf("\n\n  return", actionStart));

  assert.match(refundsPanel, /const refundSessionRef = React\.useRef\(0\)/);
  assert.match(refundsPanel, /const refundTokenRef = React\.useRef\(auth\.accessToken\)/);
  assert.match(refundsPanel, /refundTokenRef\.current = auth\.accessToken/);
  assert.match(refundsPanel, /function isCurrentRefundSessionRequest\(requestToken, session\)/);
  assert.match(refundsPanel, /React\.useLayoutEffect\(\(\) => \{\s*refundSessionRef\.current \+= 1/);

  assert.match(loadRefunds, /const requestToken = auth\.accessToken/);
  assert.match(loadRefunds, /const refundSession = refundSessionRef\.current/);
  assert.match(loadRefunds, /const isCurrentRequest = \(\) => alive && isCurrentRefundSessionRequest\(requestToken, refundSession\)/);
  assert.match(loadRefunds, /mallRefunds\(\{ limit: DASHBOARD_HISTORY_PAGE_SIZE, offset, status \}, requestToken\)/);
  assert.match(loadRefunds, /then\(\(data\) => \{\s*if \(!isCurrentRequest\(\)\) return/);
  assert.match(loadRefunds, /catch\(\(error\) => \{\s*if \(!isCurrentRequest\(\)\) return/);

  assert.match(action, /const requestToken = auth\.accessToken/);
  assert.match(action, /const refundSession = refundSessionRef\.current/);
  assert.match(action, /const isCurrentRequest = \(\) => isCurrentRefundSessionRequest\(requestToken, refundSession\)/);
  assert.match(action, /if \(!requestToken \|\| !isCurrentRequest\(\) \|\| !id/);
  assert.match(action, /await bbsApi\.cancelMallRefund\(id, requestToken\);\s*if \(!isCurrentRequest\(\)\) return/);
  assert.match(action, /catch \(error\) \{\s*if \(!isCurrentRequest\(\)\) return/);
  assert.doesNotMatch(action, /auth\.accessToken\)/);
});

test("profile actions ignore stale auth sessions", () => {
  const source = fs.readFileSync(new URL("./pages/UserDashboardRoutes.jsx", import.meta.url), "utf8");
  const profilePanel = source.slice(source.indexOf("function ProfilePanel"), source.indexOf("function ModerationSection"));

  assert.match(profilePanel, /const profileSessionRef = React\.useRef\(0\)/);
  assert.match(profilePanel, /const profileTokenRef = React\.useRef\(auth\.accessToken\)/);
  assert.match(profilePanel, /profileTokenRef\.current = auth\.accessToken/);
  assert.match(profilePanel, /function isCurrentProfileSessionRequest\(requestToken, session\)/);
  assert.match(profilePanel, /React\.useLayoutEffect\(\(\) => \{\s*profileSessionRef\.current \+= 1;/);

  for (const name of ["submit", "uploadAvatar", "uploadBackground", "requestVerification"]) {
    const start = profilePanel.indexOf(`async function ${name}`);
    const end = profilePanel.indexOf("\n\n  ", start + 1);
    const action = profilePanel.slice(start, end === -1 ? undefined : end);

    assert.ok(start >= 0, `${name} is present`);
    assert.match(action, /const requestToken = auth\.accessToken/);
    assert.match(action, /const profileSession = profileSessionRef\.current/);
    assert.match(action, /const isCurrentRequest = \(\) => isCurrentProfileSessionRequest\(requestToken, profileSession\)/);
    assert.match(action, /if \(!requestToken \|\| !isCurrentRequest\(\)\) return/);
    assert.match(action, /await bbsApi[\s\S]*?requestToken/);
    assert.match(action, /await bbsApi[\s\S]*?;\s*if \(!isCurrentRequest\(\)\) return/);
    assert.match(action, /catch \(error\) \{\s*if \(!isCurrentRequest\(\)\) return/);
    assert.doesNotMatch(action, /auth\.accessToken\)/);
  }
});

test("shop serializes review submission and image upload before button state rerenders", () => {
  const source = fs.readFileSync(new URL("./pages/SectionPages.jsx", import.meta.url), "utf8");
  const actions = ["submitProductReview", "uploadReviewImage"].map((name) => {
    const start = source.indexOf(`async function ${name}`);
    const end = source.indexOf("\n\n  async function ", start + 1);
    assert.ok(start >= 0, `${name} is present`);
    return source.slice(start, end === -1 ? undefined : end);
  });

  assert.match(source, /const reviewActionSubmittingRef = React\.useRef\(false\)/);
  assert.match(source, /const \[reviewActionBusy, setReviewActionBusy\] = React\.useState\(false\)/);
  assert.match(source, /if \(!token \|\| !detailProduct\?\.id \|\| reviewActionSubmittingRef\.current\) return/);
  assert.match(source, /if \(!file \|\| reviewActionSubmittingRef\.current\) return/);
  assert.match(source, /disabled=\{reviewActionBusy\}/);
  for (const action of actions) {
    assert.match(action, /reviewActionSubmittingRef\.current = true;\s*setReviewActionBusy\(true\)/);
    assert.match(action, /finally \{\s*reviewActionSubmittingRef\.current = false;\s*setReviewActionBusy\(false\)/);
  }
});

test("shop ignores stale product-detail review responses", () => {
  const source = fs.readFileSync(new URL("./pages/SectionPages.jsx", import.meta.url), "utf8");

  assert.match(source, /const detailReviewSessionRef = React\.useRef\(0\)/);
  assert.match(source, /React\.useLayoutEffect\(\(\) => \{\s*detailReviewSessionRef\.current \+= 1;\s*\}, \[detailProduct\?\.id, token\]\)/);
  assert.match(source, /function isCurrentDetailReviewRequest\(productId, session\)/);

  const reviewActions = new Map();
  for (const name of [
    "loadMoreProductReviews",
    "loadMoreMyProductReviews",
    "loadMoreProductReviewOrders",
    "submitProductReview",
    "uploadReviewImage"
  ]) {
    const start = source.indexOf(`async function ${name}`);
    const end = source.indexOf("\n\n  async function ", start + 1);
    const action = source.slice(start, end === -1 ? undefined : end);

    assert.ok(start >= 0, `${name} is present`);
    assert.match(action, /(?:const|let) reviewSession = detailReviewSessionRef\.current/);
    reviewActions.set(name, action);
  }

  for (const name of ["loadMoreProductReviews", "loadMoreMyProductReviews", "loadMoreProductReviewOrders", "uploadReviewImage"]) {
    const action = reviewActions.get(name);
    assert.match(action, /await bbsApi[\s\S]*?;\s*if \(!isCurrentRequest\(\)\) return/);
    assert.match(action, /catch \(error\) \{\s*if \(!isCurrentRequest\(\)\) return/);
  }

  const submit = reviewActions.get("submitProductReview");
  assert.match(submit, /await bbsApi\.createMallProductReview\([\s\S]*?\);\s*if \(!isCurrentRequest\(\)\) return/);
  assert.match(submit, /reviewSession = \+\+detailReviewSessionRef\.current;\s*setProductReviewOrders/);
  assert.match(submit, /await Promise\.allSettled\([\s\S]*?\);\s*if \(!isCurrentRequest\(\)\) return/);
  assert.match(submit, /catch \(error\) \{\s*if \(!isCurrentRequest\(\)\) return/);
});

test("shop ignores stale catalog pages after a query refresh", () => {
  const source = fs.readFileSync(new URL("./pages/SectionPages.jsx", import.meta.url), "utf8");
  const catalogLoad = source.slice(source.indexOf("async function reloadProducts"), source.indexOf("function applyCartData"));

  assert.match(source, /const productLoadRequestVersionRef = React\.useRef\(0\)/);
  assert.match(source, /const productQueryRef = React\.useRef\(\{ keyword: filters\.keyword, category: filters\.category \}\)/);
  assert.match(source, /function isCurrentProductRequest\(query, requestVersion\)/);
  assert.match(source, /React\.useLayoutEffect\(\(\) => \{\s*productLoadRequestVersionRef\.current \+= 1;\s*\}, \[filters\.category, filters\.keyword\]\)/);
  assert.match(source, /const requestVersion = \+\+productLoadRequestVersionRef\.current;\s*const isCurrentRequest = \(\) => alive && isCurrentProductRequest/);

  for (const name of ["reloadProducts", "loadMoreProducts"]) {
    const start = catalogLoad.indexOf(`async function ${name}`);
    const end = catalogLoad.indexOf("\n\n  async function ", start + 1);
    const action = catalogLoad.slice(start, end === -1 ? undefined : end);

    assert.ok(start >= 0, `${name} is present`);
    assert.match(
      action,
      name === "reloadProducts"
        ? /const query = \{ \.\.\.productQueryRef\.current \}/
        : /const query = \{ keyword: filters\.keyword, category: filters\.category \}/
    );
    assert.match(action, /const requestVersion = \+\+productLoadRequestVersionRef\.current/);
    assert.match(action, /await bbsApi\.mallProducts\([\s\S]*?\);\s*if \(!isCurrentRequest\(\)\) return/);
    assert.match(action, /catch \(error\) \{\s*if \(!isCurrentRequest\(\)\) return/);
  }
});

test("shop owns checkout completion across auth sessions", () => {
  const source = fs.readFileSync(new URL("./pages/SectionPages.jsx", import.meta.url), "utf8");
  const start = source.indexOf("async function redeemProduct");
  const end = source.indexOf("\n\n  function cancelCheckout", start);
  const redeem = source.slice(start, end);

  assert.ok(start >= 0, "redeemProduct is present");
  assert.ok(end > start, "redeemProduct has a bounded body");
  assert.match(source, /const \[checkoutActionBusy, setCheckoutActionBusy\] = React\.useState\(false\)/);
  assert.match(source, /const checkoutSubmittingRef = React\.useRef\(0\)/);
  assert.match(source, /const checkoutRequestIdRef = React\.useRef\(0\)/);
  assert.match(source, /const shopSessionRef = React\.useRef\(0\)/);
  assert.match(source, /const shopTokenRef = React\.useRef\(token\)/);
  assert.match(source, /shopTokenRef\.current = token/);
  assert.match(source, /function isCurrentShopSessionRequest\(requestToken, session\)/);
  assert.match(source, /React\.useLayoutEffect\(\(\) => \{\s*shopSessionRef\.current \+= 1;\s*\}, \[token\]\)/);
  assert.match(source, /checkoutSubmittingRef\.current = 0;\s*setCheckoutActionBusy\(false\);\s*setBusyProductId\(null\)/);
  assert.match(source, /const checkoutBusy = checkoutActionBusy \|\|/);

  assert.match(redeem, /const requestToken = token/);
  assert.match(redeem, /const requestUserId = auth\?\.user\?\.id/);
  assert.match(redeem, /const shopSession = shopSessionRef\.current/);
  assert.match(redeem, /const isCurrentRequest = \(\) => isCurrentShopSessionRequest\(requestToken, shopSession\)/);
  assert.match(redeem, /const requestID = \+\+checkoutRequestIdRef\.current;\s*checkoutSubmittingRef\.current = requestID/);
  assert.match(redeem, /setCheckoutActionBusy\(true\)/);
  assert.match(redeem, /checkoutAttemptKey\(\{\s*userId: requestUserId/);
  assert.match(redeem, /recordCheckoutAttemptOrder\(\{ userId: requestUserId/);
  assert.match(redeem, /clearCheckoutAttemptKey\(\{ userId: requestUserId/);
  assert.match(redeem, /checkoutMallCart\(orderPayload, requestToken\)/);
  assert.match(redeem, /await bbsApi\.payMallOrder\([\s\S]*?requestToken\s*\)/);
  assert.match(redeem, /if \(!isCurrentRequest\(\)\) return;\s*if \(checkout\.mode === "cart"\) applyCartData/);
  assert.match(redeem, /if \(checkoutSubmittingRef\.current !== requestID\) return;\s*checkoutSubmittingRef\.current = 0;\s*setCheckoutActionBusy\(false\)/);
  assert.doesNotMatch(redeem, /checkoutMallCart\(orderPayload, token\)/);
  assert.doesNotMatch(redeem, /payMallOrder\([\s\S]*?,\s*token\s*\)/);
});

test("shop guards authenticated storefront side effects across auth sessions", () => {
  const source = fs.readFileSync(new URL("./pages/SectionPages.jsx", import.meta.url), "utf8");
  const shopAction = (name) => {
    const start = source.indexOf(`async function ${name}`);
    const nextAsync = source.indexOf("\n\n  async function ", start + 1);
    const nextFunction = source.indexOf("\n\n  function ", start + 1);
    const end = Math.min(...[nextAsync, nextFunction].filter((index) => index > start));

    assert.ok(start >= 0, `${name} is present`);
    assert.ok(end > start, `${name} has a bounded body`);
    return source.slice(start, end);
  };

  for (const name of [
    "reloadFavorites",
    "loadMoreFavorites",
    "addToCart",
    "toggleProductFavorite",
    "refreshCheckoutProduct",
    "updateCartQuantity",
    "removeCartItem",
    "clearCart",
    "reloadAddresses",
    "loadMoreAddresses",
    "loadMoreMyCoupons",
    "claimCoupon",
    "saveAddress",
    "setDefaultAddress",
    "deleteAddress"
  ]) {
    const action = shopAction(name);

    assert.match(action, /const requestToken = token/);
    assert.match(action, /const session = shopSessionRef\.current/);
    assert.match(action, /const isCurrentRequest = \(\) => isCurrentShopSessionRequest\(requestToken, session\)/);
    assert.match(action, /if \(!isCurrentRequest\(\)\) return/);
    assert.doesNotMatch(action, /bbsApi\.[^(]+\([\s\S]*?, token\)/);
  }

  assert.match(shopAction("refreshCoupons"), /async function refreshCoupons\(isCurrentRequest = \(\) => true\)/);
  assert.match(shopAction("refreshCoupons"), /if \(!isCurrentRequest\(\)\) return \[\];\s*const data = await bbsApi\.mallCoupons/);
  assert.match(shopAction("syncCheckoutAfterMallError"), /refreshCoupons\(isCurrentRequest\)/);
  assert.match(shopAction("claimCoupon"), /Promise\.allSettled\(\[refreshCoupons\(isCurrentRequest\), refreshMyCoupons\(\)\]\)/);

  for (const name of ["claimCoupon", "saveAddress", "setDefaultAddress", "deleteAddress"]) {
    assert.match(shopAction(name), /finally \{\s*if \(isCurrentRequest\(\)\)/);
  }
});
