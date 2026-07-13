import React from "react";
import { profileThemeClass } from "../lib/postMappers.js";

export default function ProfilePreview({ person }) {
  const cover = person.background || person.backgroundUrl || person.avatar;
  return (
    <aside className={`profile-popover panel ${profileThemeClass(person.profileTheme)}`} aria-label={`${person.name} 资料卡`}>
      <img
        className="cover"
        src={cover}
        alt=""
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
