export const defaultAuthConfig = {
  password_enabled: true,
  register_enabled: true,
  register_mode: "open",
  invite_required: false,
  email_verification_required: false,
  webmaster_enabled: false,
  oauth_callback_hint: "",
  providers: [
    { provider: "github", label: "GitHub", enabled: false, min_account_years: 3 },
    { provider: "qq", label: "QQ", enabled: false },
    { provider: "google", label: "Google", enabled: false }
  ]
};

const registerModes = new Set(["open", "invite_only", "closed"]);

export function normalizeAuthConfig(data) {
  const providerItems = Array.isArray(data?.providers)
    ? data.providers
    : defaultAuthConfig.providers;
  const defaultProvidersByName = new Map(
    defaultAuthConfig.providers.map(provider => [provider.provider, provider])
  );
  const rawMode = String(
    data?.register_mode ?? data?.registerMode ?? ""
  )
    .trim()
    .toLowerCase();
  const legacyRegisterEnabled =
    data?.register_enabled ?? data?.registerEnabled ?? true;
  const register_mode = registerModes.has(rawMode)
    ? rawMode
    : rawMode
      ? "closed"
      : legacyRegisterEnabled
        ? "open"
        : "closed";
  const passwordEnabled = Boolean(
    data?.password_enabled ?? data?.passwordEnabled ?? true
  );
  return {
    password_enabled: passwordEnabled,
    register_enabled: passwordEnabled && register_mode !== "closed",
    register_mode,
    invite_required: passwordEnabled && register_mode === "invite_only",
    email_verification_required: Boolean(
      data?.email_verification_required ?? data?.emailVerificationRequired ?? false
    ),
    webmaster_enabled: Boolean(
      data?.webmaster_enabled ?? data?.webmasterEnabled ?? false
    ),
    oauth_callback_hint:
      data?.oauth_callback_hint ?? data?.oauthCallbackHint ?? "",
    providers: providerItems
      .filter(provider => provider?.provider)
      .map(provider => ({
        ...defaultProvidersByName.get(provider.provider),
        ...provider,
        enabled: Boolean(provider.enabled)
      }))
  };
}

export function enabledAuthProviders(config) {
  return (config?.providers || []).filter(
    provider => provider?.enabled && provider?.provider
  );
}
