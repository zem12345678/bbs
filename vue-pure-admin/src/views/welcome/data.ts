import GroupLine from "~icons/ri/group-line";
import Question from "~icons/ri/question-answer-line";
import CheckLine from "~icons/ri/chat-check-line";
import Smile from "~icons/ri/star-smile-line";

const cardVisuals = {
  users: {
    icon: GroupLine,
    bgColor: "#effaff",
    color: "#41b6ff"
  },
  content: {
    icon: Question,
    bgColor: "#fff5f4",
    color: "#e85f33"
  },
  comments: {
    icon: CheckLine,
    bgColor: "#eff8f4",
    color: "#26ce83"
  },
  pending_reports: {
    icon: Smile,
    bgColor: "#f6f4fe",
    color: "#7846e5"
  }
};

/** 后台首页兜底指标 */
const chartData = [
  {
    key: "users",
    ...cardVisuals.users,
    duration: 2200,
    name: "注册用户",
    value: 0,
    percent: "暂无新增",
    data: [0, 0, 0, 0, 0, 0, 0]
  },
  {
    key: "content",
    ...cardVisuals.content,
    duration: 1600,
    name: "内容总量",
    value: 0,
    percent: "暂无新增",
    data: [0, 0, 0, 0, 0, 0, 0]
  },
  {
    key: "comments",
    ...cardVisuals.comments,
    duration: 1500,
    name: "评论总量",
    value: 0,
    percent: "暂无新增",
    data: [0, 0, 0, 0, 0, 0, 0]
  },
  {
    key: "pending_reports",
    ...cardVisuals.pending_reports,
    duration: 100,
    name: "待处理举报",
    value: 0,
    percent: "暂无待处理",
    data: [0, 0, 0, 0, 0, 0, 0]
  }
];

export { cardVisuals };
export { chartData };
