import React from "react";

export default function Avatar({ person, small = false }) {
  return (
    <span className={`avatar ${small ? "small" : ""}`}>
      <img src={person.avatar} alt="" />
      <strong>V</strong>
    </span>
  );
}
