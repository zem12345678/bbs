const markdownImagePattern = /!\[[^\]]*\]\(([^)\s]+)(?:\s+"[^"]*")?\)/g;

export function markdownImageUrls(text = "") {
  const urls = [];
  const pattern = new RegExp(markdownImagePattern);
  let match = pattern.exec(text);
  while (match) {
    urls.push(match[1]);
    match = pattern.exec(text);
  }
  return urls;
}

export function textWithoutMarkdownImages(text = "") {
  return text.replace(markdownImagePattern, "").replace(/\n{3,}/g, "\n\n").trim();
}

export function appendMarkdownImage(text = "", imageUrl, alt = "图片") {
  const base = text.trimEnd();
  return `${base}${base ? "\n\n" : ""}![${alt}](${imageUrl})\n`;
}
