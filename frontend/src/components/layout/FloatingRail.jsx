import React from "react";
import { useNavigate } from "react-router-dom";
import { MessageCircle, Monitor, MoreHorizontal, Package, Settings } from "lucide-react";

export default function FloatingRail() {
  const navigate = useNavigate();
  const buttons = [
    { label: "聊天室", icon: MessageCircle, path: "/chat" },
    { label: "显示模式", icon: Monitor },
    { label: "工具箱", icon: Package },
    { label: "设置", icon: Settings },
    { label: "更多", icon: MoreHorizontal }
  ];

  return (
    <div className="floating-rail" aria-label="快捷工具">
      {buttons.map(({ label, icon: Icon, path }) => (
        <button type="button" aria-label={label} title={label} key={label} onClick={path ? () => navigate(path) : undefined}>
          <Icon size={24} />
        </button>
      ))}
    </div>
  );
}
