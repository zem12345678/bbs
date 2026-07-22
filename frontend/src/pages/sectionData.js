import {
  CircleHelp,
  ImagePlus,
  Paperclip,
  ShieldCheck
} from "lucide-react";

export const workspacePhotos = [
  "https://images.unsplash.com/photo-1497366754035-f200968a6e72?auto=format&fit=crop&w=380&q=80",
  "https://images.unsplash.com/photo-1516321318423-f06f85e504b3?auto=format&fit=crop&w=380&q=80",
  "https://images.unsplash.com/photo-1497366811353-6870744d04b2?auto=format&fit=crop&w=380&q=80"
];

export const pageImages = {
  首页: "https://images.unsplash.com/photo-1556761175-b413da4baf72?auto=format&fit=crop&w=920&q=80",
  圈子: "https://images.unsplash.com/photo-1521737604893-d14cc237f11d?auto=format&fit=crop&w=920&q=80",
  求助: "https://images.unsplash.com/photo-1517048676732-d65bc937f952?auto=format&fit=crop&w=920&q=80",
  资源: "https://images.unsplash.com/photo-1453928582365-b6ad33cbcf64?auto=format&fit=crop&w=920&q=80",
  商城: "https://images.unsplash.com/photo-1556742049-0cfed4f6a45d?auto=format&fit=crop&w=920&q=80",
  会员: "https://images.unsplash.com/photo-1552664730-d307ca884978?auto=format&fit=crop&w=920&q=80",
  更多: "https://images.unsplash.com/photo-1500530855697-b586d89ba3ee?auto=format&fit=crop&w=920&q=80"
};

export const memberBenefits = [
  { title: "悬赏问答", desc: "有效会员可发布积分悬赏问答并采纳答案。", icon: CircleHelp },
  { title: "付费附件", desc: "有效会员可为已发布话题附件设置积分售价。", icon: Paperclip },
  { title: "自定义背景", desc: "有效会员可保存个人主页背景图。", icon: ImagePlus },
  { title: "权益校验", desc: "会员到期或被撤销后，受限功能会立即停止使用。", icon: ShieldCheck }
];
