export type RegistrationMode = "open" | "invite_only" | "closed";

const registrationModes = new Set<RegistrationMode>([
  "open",
  "invite_only",
  "closed"
]);

export function resolveRegistrationMode(
  configuredMode: unknown,
  legacyEnabled = true
): RegistrationMode {
  const normalized = String(configuredMode ?? "")
    .trim()
    .toLowerCase();
  if (registrationModes.has(normalized as RegistrationMode)) {
    return normalized as RegistrationMode;
  }
  if (normalized) {
    return "closed";
  }
  return legacyEnabled ? "open" : "closed";
}
