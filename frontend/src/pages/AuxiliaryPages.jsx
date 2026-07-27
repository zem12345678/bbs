import React from "react";
import { useNavigate } from "react-router-dom";
import { BookOpen, CalendarCheck, CheckCircle2, ExternalLink, Gift, Info, Link, MessageCircle, Pencil, Wrench } from "lucide-react";
import { bbsApi } from "../api";
import { listItems, listTotal } from "../lib/apiShapes";
import { DataRows, EmptyState, RouteHeader } from "./RouteBlocks.jsx";

const pageMap = {
  links: {
    icon: Link,
    eyebrow: "友情链接",
    title: "社区外部链接",
    description: "展示合作社区、工具站和文档入口。",
    rows: []
  },
  tasks: {
    icon: CheckCircle2,
    eyebrow: "社区任务",
    title: "成长任务中心",
    description: "完成社区成长行为后领取任务奖励，到账记录会同步到积分明细。",
    rows: []
  },
  about: {
    icon: Info,
    eyebrow: "关于社区",
    title: "BBS 社区平台",
    description: "一个面向内容沉淀、圈子协作和技术交流的社区论坛。",
    rows: [
      { title: "内容形态", description: "话题、文章、评论、资源、问答和活动", meta: "社区" },
      { title: "技术架构", description: "Go 微服务、gRPC、PG、Redis、Kafka、ES 和 MongoDB", meta: "后端" },
      { title: "用户能力", description: "个人资料、创作中心、通知、收藏、点赞和积分成长", meta: "前台" }
    ]
  },
  install: {
    icon: Wrench,
    eyebrow: "安装引导",
    title: "社区上手引导",
    description: "面向普通社区用户的首次使用路径，后台初始化和系统配置放在独立管理端处理。",
    rows: [
      { title: "完善个人资料", description: "补齐昵称、头像和简介，让内容展示更完整", meta: "账号" },
      { title: "发布第一条话题", description: "选择分类和标签，把问题或经验沉淀到广场", meta: "创作" },
      { title: "管理互动记录", description: "在个人工作台查看点赞、收藏、通知和积分明细", meta: "工作台" }
    ]
  },
  redirect: {
    icon: ExternalLink,
    eyebrow: "跳转",
    title: "外链跳转确认",
    description: "用于安全提示、来源追踪和外链访问审计。",
    rows: [
      { title: "确认目标地址", description: "访问外部链接前先确认目标站点", meta: "安全" },
      { title: "记录访问来源", description: "便于统计社区资源点击和渠道效果", meta: "统计" }
    ]
  }
};

const LINK_PAGE_SIZE = 30;

export function AuxiliaryPage({ auth, kind = "about", siteConfig }) {
  const navigate = useNavigate();
  const page = pageMap[kind] || pageMap.about;
  const pageTitle = kind === "about" ? siteConfig?.siteName || page.title : page.title;
  const pageDescription =
    kind === "about" ? siteConfig?.siteDescription || page.description : page.description;
  const token = auth?.accessToken;
  const [state, setState] = React.useState({
    rows: page.rows,
    total: page.rows.length,
    offset: page.rows.length,
    loading: false,
    loadingMore: false,
    error: "",
    loadMoreError: "",
    actionError: "",
    claimingId: ""
  });

  React.useEffect(() => {
    if (kind !== "links" && kind !== "tasks") {
      setState({
        rows: page.rows,
        total: page.rows.length,
        offset: page.rows.length,
        loading: false,
        loadingMore: false,
        error: "",
        loadMoreError: "",
        actionError: "",
        claimingId: ""
      });
      return;
    }
    let alive = true;
    setState({ rows: [], total: 0, offset: 0, loading: true, loadingMore: false, error: "", loadMoreError: "", actionError: "", claimingId: "" });
    const loader =
      kind === "links"
        ? () => bbsApi.links({ limit: LINK_PAGE_SIZE, offset: 0 })
        : token
          ? () => bbsApi.myTasks({ limit: LINK_PAGE_SIZE, offset: 0 }, token)
          : () => bbsApi.tasks({ limit: LINK_PAGE_SIZE, offset: 0 });
    loader()
      .then((data) => {
        if (!alive) return;
        const items = listItems(data);
        const rows = items.map(auxiliaryRow);
        setState({
          rows,
          total: Math.max(items.length, listTotal(data, items)),
          offset: items.length,
          loading: false,
          loadingMore: false,
          error: "",
          loadMoreError: "",
          actionError: "",
          claimingId: ""
        });
      })
      .catch((error) => {
        if (!alive) return;
        setState({ rows: [], total: 0, offset: 0, loading: false, loadingMore: false, error: error.message || "数据加载失败", loadMoreError: "", actionError: "", claimingId: "" });
      });
    return () => {
      alive = false;
    };
  }, [kind, page.rows, token]);

  async function loadMoreLinks() {
    if (kind !== "links" || state.loading || state.loadingMore || state.offset >= state.total) {
      return;
    }
    const offset = state.offset;
    setState((current) => ({ ...current, loadingMore: true, loadMoreError: "" }));
    try {
      const data = await bbsApi.links({ limit: LINK_PAGE_SIZE, offset });
      const items = listItems(data);
      const nextRows = items.map((item, index) => auxiliaryRow(item, offset + index));
      setState((current) => {
        const rows = mergeAuxiliaryRows(current.rows, nextRows);
        const nextOffset = current.offset + items.length;
        return {
          ...current,
          rows,
          total: items.length > 0 ? Math.max(nextOffset, listTotal(data, items)) : current.offset,
          offset: nextOffset,
          loadingMore: false,
          loadMoreError: ""
        };
      });
    } catch (error) {
      setState((current) => ({ ...current, loadingMore: false, loadMoreError: error.message || "更多友情链接加载失败" }));
    }
  }

  async function handleTaskAction(task, action) {
    if (action.type === "signin") {
      navigate("/user/signin");
      return;
    }
    if (action.type === "checkin") {
      navigate("/member");
      return;
    }
    if (action.type === "publish-topic") {
      navigate("/topic/create");
      return;
    }
    if (action.type === "browse-topics") {
      navigate("/topics");
      return;
    }
    const taskId = String(task.id || task.key || "");
    if (!token || !taskId || state.claimingId) return;
    setState(current => ({ ...current, actionError: "", claimingId: taskId }));
    try {
      const data = await bbsApi.claimTask(taskId, token);
      const updated = data?.task && typeof data.task === "object" ? auxiliaryRow(data.task) : null;
      setState(current => ({
        ...current,
        rows: updated ? current.rows.map(row => (sameTask(row, task) ? updated : row)) : current.rows,
        claimingId: ""
      }));
    } catch (error) {
      setState(current => ({ ...current, actionError: error.message || "领取奖励失败", claimingId: "" }));
    }
  }

  return (
    <>
      <RouteHeader
        icon={page.icon || BookOpen}
        eyebrow={page.eyebrow}
        title={pageTitle}
        description={pageDescription}
      />
      {state.loading && <EmptyState title="正在加载..." />}
      {state.error && <EmptyState title="加载失败" description={state.error} />}
      {!state.loading && !state.error && state.rows.length === 0 && <EmptyState title={kind === "links" ? "暂无友情链接" : "暂无社区任务"} />}
      {!state.loading && !state.error && kind !== "tasks" && state.rows.length > 0 && <DataRows rows={state.rows} />}
      {!state.loading && !state.error && kind === "tasks" && state.rows.length > 0 && (
        <TaskRows rows={state.rows} signedIn={Boolean(token)} claimingId={state.claimingId} onAction={handleTaskAction} />
      )}
      {!state.loading && !state.error && kind === "links" && state.rows.length > 0 && state.offset < state.total && (
        <div className="dashboard-history-more">
          <span>{state.loadingMore ? "正在加载更多友情链接..." : state.loadMoreError || "继续查看更多友情链接。"}</span>
          <button aria-label="加载更多友情链接" type="button" disabled={state.loadingMore} onClick={loadMoreLinks}>
            {state.loadingMore ? "加载中" : "加载更多"}
          </button>
        </div>
      )}
      {state.actionError && <p className="task-action-error" role="status">{state.actionError}</p>}
    </>
  );
}

function TaskRows({ rows, signedIn, claimingId, onAction }) {
  return (
    <div className="data-rows">
      {rows.map((row, index) => {
        const action = taskAction(row, signedIn);
        const Icon = action.icon;
        const taskId = String(row.id || row.key || index);
        return (
          <article className="data-row panel task-row" key={row.rowKey || row.key || row.id || index}>
            <div>
              <strong>{row.title || "成长任务"}</strong>
              <p>{taskDescription(row)}</p>
            </div>
            <div className="task-row-actions">
              <span>{action.status}</span>
              {action.label && (
                <button type="button" disabled={action.type === "claim" && claimingId === taskId} onClick={() => onAction(row, action)}>
                  <Icon aria-hidden="true" size={16} />
                  {action.type === "claim" && claimingId === taskId ? "领取中" : action.label}
                </button>
              )}
            </div>
          </article>
        );
      })}
    </div>
  );
}

function taskAction(task, signedIn) {
  if (!signedIn) {
    return { type: "signin", label: "登录领取", status: "登录后查看完成状态", icon: Gift };
  }
  if (task.claimed) {
    return { type: "done", status: task.key === "daily_check_in" ? "本日已领取" : "已领取", icon: CheckCircle2 };
  }
  if (task.claimable) {
    return { type: "claim", label: "领取奖励", status: `可领取 +${taskReward(task)} 积分`, icon: Gift };
  }
  if (task.key === "first_topic") {
    return { type: "publish-topic", label: "去发布", status: "发布首个话题后领取", icon: Pencil };
  }
  if (task.key === "first_comment") {
    return { type: "browse-topics", label: "去评论", status: "发表首条评论后领取", icon: MessageCircle };
  }
  return { type: "checkin", label: "去签到", status: "完成今日签到后领取", icon: CalendarCheck };
}

function taskDescription(task) {
  const reward = taskReward(task);
  const description = task.description || "完成后领取积分奖励";
  return `${reward} 积分 · ${description}`;
}

function taskReward(task) {
  const reward = Number(task.reward_points ?? task.rewardPoints ?? 0);
  return Number.isFinite(reward) ? reward : 0;
}

function sameTask(left, right) {
  const leftID = left?.id || left?.key;
  const rightID = right?.id || right?.key;
  return String(leftID || "") === String(rightID || "");
}

function mergeAuxiliaryRows(rows, nextRows) {
  const rowIDs = new Set(rows.map(auxiliaryRowID));
  return rows.concat(
    nextRows.filter((row) => {
      const rowID = auxiliaryRowID(row);
      if (rowIDs.has(rowID)) return false;
      rowIDs.add(rowID);
      return true;
    })
  );
}

function auxiliaryRowID(row) {
  if (row?.id != null) return `id:${row.id}`;
  if (row?.key) return `key:${row.key}`;
  if (row?.url) return `url:${row.url}`;
  return `title:${row?.title || ""}`;
}

function auxiliaryRow(item, index) {
  const url = item.url ?? item.URL ?? "";
  const rowKey = item.id ?? item.key ?? url ?? index;
  return {
    ...item,
    url,
    id: item.id ?? item.task_id ?? item.taskId,
    key: item.key ?? rowKey,
    rowKey,
    title: item.title || item.name || `条目 #${index + 1}`,
    description: item.description || item.summary || url || "暂无说明",
    meta: item.meta || item.reward_points || item.rewardPoints || (url ? "外部链接" : item.status || "已接入")
  };
}
