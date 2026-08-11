import React from "react";
import { Smile } from "lucide-react";
import { captureEmojiSelection, getCachedPublicEmojis, insertEmojiToken, loadPublicEmojis } from "../../lib/emojis.js";

export default function EmojiPicker({ disabled = false, inputRef, maxLength, onChange, value = "" }) {
  const [open, setOpen] = React.useState(false);
  const [query, setQuery] = React.useState("");
  const [emojis, setEmojis] = React.useState(getCachedPublicEmojis);
  const [state, setState] = React.useState({ loading: false, error: "" });
  const pickerRef = React.useRef(null);
  const selectionRef = React.useRef(null);
  const panelId = React.useId();

  React.useEffect(() => {
    if (!open) return undefined;
    let alive = true;
    setState({ loading: emojis.length === 0, error: "" });
    loadPublicEmojis()
      .then((items) => {
        if (!alive) return;
        setEmojis(items);
        setState({ loading: false, error: "" });
      })
      .catch((error) => {
        if (alive) setState({ loading: false, error: error.message || "表情加载失败" });
      });
    return () => {
      alive = false;
    };
  }, [open]);

  React.useEffect(() => {
    if (!open) return undefined;
    function close(event) {
      if (event.key === "Escape" || (event.type === "pointerdown" && !pickerRef.current?.contains(event.target))) {
        setOpen(false);
      }
    }
    document.addEventListener("pointerdown", close);
    document.addEventListener("keydown", close);
    return () => {
      document.removeEventListener("pointerdown", close);
      document.removeEventListener("keydown", close);
    };
  }, [open]);

  const normalizedQuery = query.trim().toLocaleLowerCase();
  const visibleEmojis = normalizedQuery
    ? emojis.filter((emoji) =>
        [emoji.name, ...(emoji.aliases || [])].some((name) => name.toLocaleLowerCase().includes(normalizedQuery))
      )
    : emojis;

  function selectEmoji(emoji) {
    const input = inputRef?.current;
    const selection = selectionRef.current || captureEmojiSelection(input);
    const result = insertEmojiToken(value, emoji.name, selection.start, selection.end, maxLength);
    if (!result) {
      setState({ loading: false, error: `内容最多 ${Math.trunc(Number(maxLength))} 字，无法插入该表情` });
      return;
    }
    onChange?.(result.value);
    setOpen(false);
    window.requestAnimationFrame(() => {
      inputRef?.current?.focus();
      inputRef?.current?.setSelectionRange(result.selection, result.selection);
    });
  }

  return (
    <div className={`emoji-picker ${open ? "is-open" : ""}`.trim()} ref={pickerRef}>
      <button
        aria-controls={panelId}
        aria-expanded={open}
        aria-label="插入自定义表情"
        className={`emoji-picker__trigger ${open ? "is-active" : ""}`.trim()}
        disabled={disabled}
        title="插入自定义表情"
        type="button"
        onClick={() => {
          if (!open) selectionRef.current = captureEmojiSelection(inputRef?.current);
          setOpen((value) => !value);
        }}
      >
        <Smile size={18} aria-hidden="true" />
      </button>
      {open && (
        <div className="emoji-picker__popover" id={panelId} role="dialog" aria-label="自定义表情">
          <input
            aria-label="搜索自定义表情"
            autoFocus
            placeholder="搜索表情"
            type="search"
            value={query}
            onChange={(event) => setQuery(event.target.value)}
          />
          {state.loading && <p>正在加载...</p>}
          {state.error && <p className="form-error">{state.error}</p>}
          {!state.loading && !state.error && visibleEmojis.length === 0 && <p>暂无可用表情</p>}
          {visibleEmojis.length > 0 && (
            <div className="emoji-picker__grid">
              {visibleEmojis.map((emoji) => (
                <button aria-label={`:${emoji.name}:`} key={emoji.name} title={`:${emoji.name}:`} type="button" onClick={() => selectEmoji(emoji)}>
                  <img alt="" className="custom-emoji" loading="lazy" src={emoji.url} />
                </button>
              ))}
            </div>
          )}
        </div>
      )}
    </div>
  );
}
