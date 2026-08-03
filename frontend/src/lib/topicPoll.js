export const MIN_POLL_CHOICES = 2;
export const MAX_POLL_CHOICES = 10;
export const MAX_POLL_CHOICE_LENGTH = 80;

export function emptyPollDraft() {
  return {
    enabled: false,
    multiple: false,
    choices: ["", ""],
    expires_at: "",
    dirty: false
  };
}

export function pollDraftFromApi(poll) {
  if (!poll) return emptyPollDraft();
  const choices = Array.isArray(poll.choices) ? poll.choices.map((choice) => String(choice?.text || "")) : [];
  return {
    enabled: true,
    multiple: Boolean(poll.multiple),
    choices: choices.length >= MIN_POLL_CHOICES ? choices : ["", ""],
    expires_at: millisToLocalDateTime(poll.expires_at ?? poll.expiresAt),
    dirty: false
  };
}

export function pollPayloadFromDraft(draft, now = Date.now()) {
  if (!draft?.enabled) {
    return { payload: { enabled: false }, error: "" };
  }
  const choices = (draft.choices || []).map((choice) => String(choice || "").trim());
  if (choices.length < MIN_POLL_CHOICES || choices.length > MAX_POLL_CHOICES) {
    return { payload: null, error: `投票选项需为 ${MIN_POLL_CHOICES} 至 ${MAX_POLL_CHOICES} 个。` };
  }
  if (choices.some((choice) => !choice || [...choice].length > MAX_POLL_CHOICE_LENGTH)) {
    return { payload: null, error: `每个投票选项需为 1 至 ${MAX_POLL_CHOICE_LENGTH} 个字符。` };
  }
  const uniqueChoices = new Set(choices.map((choice) => choice.toLocaleLowerCase()));
  if (uniqueChoices.size !== choices.length) {
    return { payload: null, error: "投票选项不能重复。" };
  }
  let expiresAt = 0;
  if (draft.expires_at) {
    expiresAt = new Date(draft.expires_at).getTime();
    if (!Number.isFinite(expiresAt) || expiresAt <= now) {
      return { payload: null, error: "投票截止时间必须晚于当前时间。" };
    }
  }
  return {
    payload: {
      enabled: true,
      multiple: Boolean(draft.multiple),
      choices,
      expires_at: expiresAt || undefined
    },
    error: ""
  };
}

export function pollChoicePercent(votes, totalVoters) {
  const total = Number(totalVoters) || 0;
  if (total <= 0) return 0;
  return Math.min(100, Math.max(0, Math.round(((Number(votes) || 0) / total) * 100)));
}

function millisToLocalDateTime(value) {
  const millis = Number(value) || 0;
  if (millis <= 0) return "";
  const date = new Date(millis);
  if (Number.isNaN(date.getTime())) return "";
  const pad = (part) => String(part).padStart(2, "0");
  return `${date.getFullYear()}-${pad(date.getMonth() + 1)}-${pad(date.getDate())}T${pad(date.getHours())}:${pad(date.getMinutes())}`;
}
