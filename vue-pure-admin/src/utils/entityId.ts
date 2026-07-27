export type EntityId = string | number;

const maxInt64EntityId = "9223372036854775807";

export function normalizeEntityId(value: unknown): EntityId | undefined {
  if (typeof value === "number" && Number.isFinite(value) && value > 0) {
    return value;
  }
  if (typeof value === "string") {
    const trimmed = value.trim();
    if (trimmed && trimmed !== "0") {
      return trimmed;
    }
  }
  return undefined;
}

export function entityIdText(value: unknown, fallback = "-") {
  const id = normalizeEntityId(value);
  return id === undefined ? fallback : String(id);
}

export function normalizeDecimalEntityId(value: unknown): string | undefined {
  let text = "";
  if (typeof value === "string") {
    text = value.trim();
  } else if (typeof value === "number" && Number.isSafeInteger(value)) {
    text = String(value);
  }
  if (!/^[1-9]\d*$/.test(text)) return undefined;
  if (
    text.length > maxInt64EntityId.length ||
    (text.length === maxInt64EntityId.length && text > maxInt64EntityId)
  ) {
    return undefined;
  }
  return text;
}
