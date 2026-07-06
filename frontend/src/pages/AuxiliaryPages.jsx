import React from "react";
import { BookOpen, CheckCircle2, ExternalLink, Info, Link, Wrench } from "lucide-react";
import { bbsApi } from "../api";
import { listItems } from "../lib/apiShapes";
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
    description: "承接发帖、评论、采纳、上传资源等成长任务，完成后自动记录到积分明细。",
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

export function AuxiliaryPage({ kind = "about" }) {
  const page = pageMap[kind] || pageMap.about;
  const dynamicLoader = kind === "links" ? bbsApi.links : kind === "tasks" ? bbsApi.tasks : null;
  const [state, setState] = React.useState({ rows: page.rows, loading: false, error: "" });

  React.useEffect(() => {
    if (!dynamicLoader) {
      setState({ rows: page.rows, loading: false, error: "" });
      return;
    }
    let alive = true;
    setState({ rows: [], loading: true, error: "" });
    dynamicLoader({ limit: 30, offset: 0 })
      .then((data) => {
        if (!alive) return;
        const rows = listItems(data).map(auxiliaryRow);
        setState({ rows, loading: false, error: "" });
      })
      .catch((error) => {
        if (!alive) return;
        setState({ rows: [], loading: false, error: error.message || "数据加载失败" });
      });
    return () => {
      alive = false;
    };
  }, [dynamicLoader, kind, page.rows]);

  return (
    <>
      <RouteHeader icon={page.icon || BookOpen} eyebrow={page.eyebrow} title={page.title} description={page.description} />
      {state.loading && <EmptyState title="正在加载..." />}
      {state.error && <EmptyState title="加载失败" description={state.error} />}
      {!state.loading && !state.error && state.rows.length === 0 && <EmptyState title={kind === "links" ? "暂无友情链接" : "暂无社区任务"} />}
      {!state.loading && !state.error && state.rows.length > 0 && <DataRows rows={state.rows} />}
    </>
  );
}

function auxiliaryRow(item, index) {
  return {
    key: item.id || item.key || index,
    title: item.title || item.name || `条目 #${index + 1}`,
    description: item.description || item.summary || item.url || "暂无说明",
    meta: item.meta || item.reward_points || item.rewardPoints || item.status || item.url || "已接入"
  };
}
