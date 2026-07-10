import React from "react";
import { X } from "lucide-react";
import { bbsApi } from "../../api";
import { listItems } from "../../lib/apiShapes";

export default function TagAssist({ className = "", maxTags = 8, onChange, placeholder, value }) {
  const allTags = parseTags(value);
  const query = currentTagQuery(value);
  const tags = (query ? allTags.slice(0, -1) : allTags).slice(0, maxTags);
  const [suggestions, setSuggestions] = React.useState([]);
  const [loading, setLoading] = React.useState(false);

  React.useEffect(() => {
    if (!query) {
      setSuggestions([]);
      setLoading(false);
      return;
    }
    let alive = true;
    const timer = window.setTimeout(() => {
      setLoading(true);
      bbsApi
        .autocompleteTags({ query, limit: 8 })
        .then((data) => {
          if (!alive) return;
          const existing = new Set(tags.map((tag) => tag.toLowerCase()));
          setSuggestions(
            listItems(data)
              .map((item) => item.name || item.title || item)
              .filter(Boolean)
              .filter((name) => !existing.has(String(name).toLowerCase()))
              .slice(0, 6)
          );
        })
        .catch(() => {
          if (alive) setSuggestions([]);
        })
        .finally(() => {
          if (alive) setLoading(false);
        });
    }, 220);
    return () => {
      alive = false;
      window.clearTimeout(timer);
    };
  }, [query, tags.join("|")]);

  function applyTags(nextTags) {
    onChange(nextTags.slice(0, maxTags).join(" "));
  }

  function addTag(tag) {
    const normalized = String(tag || "").trim().replace(/^#/, "");
    if (!normalized) return;
    const existing = new Set(tags.map((item) => item.toLowerCase()));
    if (existing.has(normalized.toLowerCase())) {
      onChange(tags.join(" "));
      return;
    }
    applyTags([...tags, normalized]);
    setSuggestions([]);
  }

  function removeTag(tag) {
    applyTags(tags.filter((item) => item !== tag));
  }

  return (
    <div className="tag-assist">
      <input className={className} placeholder={placeholder} value={value} onChange={(event) => onChange(event.target.value)} />
      {tags.length > 0 && (
        <div className="tag-chip-row" aria-label="已选标签">
          {tags.map((tag) => (
            <button key={tag} type="button" onClick={() => removeTag(tag)}>
              #{tag}
              <X size={13} aria-hidden="true" />
            </button>
          ))}
        </div>
      )}
      {(suggestions.length > 0 || loading) && (
        <div className="tag-suggestions" role="listbox" aria-label="标签建议">
          {loading && suggestions.length === 0 && <span>匹配标签中...</span>}
          {suggestions.map((tag) => (
            <button key={tag} type="button" onClick={() => addTag(tag)}>
              #{tag}
            </button>
          ))}
        </div>
      )}
    </div>
  );
}

function parseTags(text = "") {
  return String(text)
    .split(/[,，\s#]+/)
    .map((tag) => tag.trim())
    .filter(Boolean);
}

function currentTagQuery(text = "") {
  const raw = String(text);
  if (!raw || /[,，\s#]$/.test(raw)) return "";
  const parts = raw.split(/[,，\s#]+/);
  return (parts[parts.length - 1] || "").trim();
}
