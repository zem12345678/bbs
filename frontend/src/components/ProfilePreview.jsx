import React from "react";

export default function ProfilePreview({ person }) {
  return (
    <aside className="profile-popover panel" aria-label={`${person.name} 资料卡`}>
      <img
        className="cover"
        src="https://images.unsplash.com/photo-1497366216548-37526070297c?auto=format&fit=crop&w=760&q=80"
        alt="办公桌俯拍"
      />
      <div className="profile-inner">
        <div className="profile-avatar">
          <img src={person.avatar} alt="" />
          <strong>V</strong>
        </div>
        <h2>{person.name}</h2>
        <p className="handle">@{person.handle}</p>
        <p className="bio">{person.bio}</p>
        <div className="stats">
          <span>
            帖子
            <strong>0</strong>
          </span>
          <span>
            关注中
            <strong>0</strong>
          </span>
          <span>
            关注者
            <strong>0</strong>
          </span>
        </div>
      </div>
    </aside>
  );
}
