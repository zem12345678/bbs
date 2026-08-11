import React from "react";
import EmojiText from "./EmojiText.jsx";

const imagePattern = /^!\[([^\]]*)\]\(([^)\s]+)(?:\s+"[^"]*")?\)$/;

export default function MarkdownPreview({ className = "", text = "" }) {
  const blocks = markdownBlocks(text);
  return (
    <div className={`markdown-preview ${className}`.trim()}>
      {blocks.length === 0 ? (
        <p className="markdown-preview-empty">暂无预览</p>
      ) : (
        blocks.map((block, index) => renderBlock(block, index))
      )}
    </div>
  );
}

function markdownBlocks(text) {
  const lines = String(text || "").replace(/\r\n/g, "\n").split("\n");
  const blocks = [];
  let paragraph = [];
  let list = [];
  let code = [];
  let inCode = false;

  function flushParagraph() {
    if (paragraph.length > 0) {
      blocks.push({ type: "paragraph", text: paragraph.join(" ") });
      paragraph = [];
    }
  }

  function flushList() {
    if (list.length > 0) {
      blocks.push({ type: "list", items: list });
      list = [];
    }
  }

  lines.forEach((line) => {
    const trimmed = line.trim();
    if (trimmed.startsWith("```")) {
      if (inCode) {
        blocks.push({ type: "code", text: code.join("\n") });
        code = [];
        inCode = false;
      } else {
        flushParagraph();
        flushList();
        inCode = true;
      }
      return;
    }
    if (inCode) {
      code.push(line);
      return;
    }
    if (!trimmed) {
      flushParagraph();
      flushList();
      return;
    }
    const image = imagePattern.exec(trimmed);
    if (image) {
      flushParagraph();
      flushList();
      blocks.push({ type: "image", alt: image[1], src: image[2] });
      return;
    }
    const listItem = /^[-*]\s+(.+)$/.exec(trimmed);
    if (listItem) {
      flushParagraph();
      list.push(listItem[1]);
      return;
    }
    flushList();
    if (trimmed.startsWith("### ")) {
      flushParagraph();
      blocks.push({ type: "heading", level: 3, text: trimmed.slice(4) });
      return;
    }
    if (trimmed.startsWith("## ")) {
      flushParagraph();
      blocks.push({ type: "heading", level: 2, text: trimmed.slice(3) });
      return;
    }
    if (trimmed.startsWith("# ")) {
      flushParagraph();
      blocks.push({ type: "heading", level: 1, text: trimmed.slice(2) });
      return;
    }
    if (trimmed.startsWith("> ")) {
      flushParagraph();
      blocks.push({ type: "quote", text: trimmed.slice(2) });
      return;
    }
    paragraph.push(trimmed);
  });

  flushParagraph();
  flushList();
  if (inCode && code.length > 0) {
    blocks.push({ type: "code", text: code.join("\n") });
  }
  return blocks;
}

function renderBlock(block, index) {
  switch (block.type) {
    case "heading":
      return block.level === 1 ? (
        <h2 key={index}><EmojiText text={block.text} /></h2>
      ) : (
        <h3 key={index}><EmojiText text={block.text} /></h3>
      );
    case "quote":
      return <blockquote key={index}><EmojiText text={block.text} /></blockquote>;
    case "list":
      return (
        <ul key={index}>
          {block.items.map((item, itemIndex) => (
            <li key={`${item}-${itemIndex}`}><EmojiText text={item} /></li>
          ))}
        </ul>
      );
    case "code":
      return (
        <pre key={index}>
          <code>{block.text}</code>
        </pre>
      );
    case "image":
      return <img alt={block.alt || ""} key={index} src={block.src} />;
    default:
      return <p key={index}><EmojiText text={block.text} /></p>;
  }
}
