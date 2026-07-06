import { workspacePhotos } from "../pages/sectionData";

export const people = [
  {
    name: "许恪",
    handle: "xuke",
    role: "架构师 @ 腾讯",
    bio: "后端架构师，分享服务治理、拆分边界和稳定性复盘",
    avatar: "https://images.unsplash.com/photo-1500648767791-00dcc994a43e?auto=format&fit=crop&w=140&q=80"
  },
  {
    name: "明扬",
    handle: "mingyang",
    role: "高级后端工程师 @ 字节跳动",
    bio: "后端性能优化作者，关注查询链路和缓存策略",
    avatar: "https://images.unsplash.com/photo-1506794778202-cad84cf45f1d?auto=format&fit=crop&w=140&q=80"
  },
  {
    name: "沈洛",
    handle: "shenluo",
    role: "前端工程师 @ 阿里巴巴",
    bio: "前端体验共建者，关注设计系统和可访问性",
    avatar: "https://images.unsplash.com/photo-1494790108377-be9c29b29330?auto=format&fit=crop&w=140&q=80"
  },
  {
    name: "嘉言",
    handle: "jiayan",
    role: "产品经理",
    bio: "关注社区增长和内容运营节奏",
    avatar: "https://images.unsplash.com/photo-1508214751196-bcfd4ca60f91?auto=format&fit=crop&w=140&q=80"
  },
  {
    name: "洛吉",
    handle: "luoji",
    role: "算法工程师",
    bio: "记录模型评测、工具链和上线观察",
    avatar: "https://images.unsplash.com/photo-1535713875002-d1d0cf377fde?auto=format&fit=crop&w=140&q=80"
  }
];

export const posts = [
  {
    author: people[0],
    level: "LV.7",
    time: "1天前",
    text: "圈子帖子详情页终于把正文、话题和三张配图顺下来了，列表里看到的是点进详情图序变了，现在评论区也能跟着同一条内容往下走。",
    images: workspacePhotos,
    tags: ["前端交互体验研究", "前端体验优化"],
    likes: 2,
    comments: 10
  },
  {
    author: people[1],
    level: "LV.6",
    time: "1天前",
    text: "今天把接口查询从 1.2s 优化到了 180ms，关键还是索引和缓存效果。@xuke 你之前说的慢 SQL 排查思路很有用。",
    tags: ["后端性能优化小组", "今日代码审查"],
    likes: 3,
    comments: "评论",
    praised: true
  },
  {
    author: people[2],
    level: "LV.5",
    time: "1天前",
    text: "把发布器的附件态、链接态和投票态梳理了一遍，准备明天补一组移动端断点验证。",
    tags: ["发布演练复盘", "产品体验共创"],
    likes: 8,
    comments: 6
  }
];
