import React from "react";
import { emojiTextParts, getCachedPublicEmojis, loadPublicEmojis } from "../../lib/emojis.js";

export default function EmojiText({ text = "" }) {
  const [emojis, setEmojis] = React.useState(getCachedPublicEmojis);

  React.useEffect(() => {
    let alive = true;
    loadPublicEmojis()
      .then((items) => {
        if (alive) setEmojis(items);
      })
      .catch(() => {});
    return () => {
      alive = false;
    };
  }, [text]);

  return emojiTextParts(text, emojis).map((part, index) =>
    part.type === "emoji" ? (
      <img
        alt={part.value}
        className="custom-emoji"
        draggable="false"
        key={`${part.value}-${index}`}
        loading="lazy"
        src={part.emoji.url}
        title={`:${part.emoji.name}:`}
      />
    ) : (
      <React.Fragment key={`text-${index}`}>{part.value}</React.Fragment>
    )
  );
}
