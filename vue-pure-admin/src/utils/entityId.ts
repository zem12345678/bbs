export type EntityId = string | number;

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
