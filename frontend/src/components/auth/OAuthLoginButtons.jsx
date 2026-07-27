import React from "react";
import { Chrome, Github, LogIn, MessageCircle } from "lucide-react";
import { bbsApi } from "../../api";
import { oauthCallbackURL } from "../../lib/authRedirect";

export const defaultAuthConfig = {
  password_enabled: true,
  register_enabled: true,
  email_verification_required: false,
  webmaster_enabled: false,
  oauth_callback_hint: "",
  providers: [
    { provider: "github", label: "GitHub", enabled: false, min_account_years: 3 },
    { provider: "qq", label: "QQ", enabled: false },
    { provider: "google", label: "Google", enabled: false }
  ]
};

export function normalizeAuthConfig(data) {
  const providerItems = Array.isArray(data?.providers) ? data.providers : defaultAuthConfig.providers;
  const defaultProvidersByName = new Map(defaultAuthConfig.providers.map((provider) => [provider.provider, provider]));
  return {
    password_enabled: data?.password_enabled ?? data?.passwordEnabled ?? true,
    register_enabled: data?.register_enabled ?? data?.registerEnabled ?? true,
    email_verification_required: data?.email_verification_required ?? data?.emailVerificationRequired ?? false,
    webmaster_enabled: data?.webmaster_enabled ?? data?.webmasterEnabled ?? false,
    oauth_callback_hint: data?.oauth_callback_hint ?? data?.oauthCallbackHint ?? "",
    providers: providerItems
      .filter((provider) => provider?.provider)
      .map((provider) => ({
        ...defaultProvidersByName.get(provider.provider),
        ...provider,
        enabled: Boolean(provider.enabled)
      }))
  };
}

export function enabledAuthProviders(config) {
  return (config?.providers || []).filter((provider) => provider?.enabled && provider?.provider);
}

export function OAuthLoginButtons({ callbackHint = "", disabled = false, disabledReason = "", providers = [], redirectTarget = "/user/profile" }) {
  const visibleProviders = providers.filter((provider) => provider?.provider);
  const callbackURL = oauthCallbackURL(callbackHint, redirectTarget);
  if (visibleProviders.length === 0) {
    return null;
  }

  return (
    <div className="oauth-login-grid">
      {visibleProviders.map((provider) => {
        const Icon = providerIcon(provider.provider);
        const enabled = !disabled && Boolean(provider.enabled) && Boolean(callbackURL);
        const label = provider.label || provider.provider;
        return (
          <button
            className="oauth-login-button"
            disabled={!enabled}
            key={provider.provider}
            onClick={() => enabled && startOAuth(provider.provider, callbackURL)}
            title={enabled ? `${label} 登录` : disabledReason || (callbackURL ? `${label} 登录未开启或 OAuth 密钥未配置` : "第三方登录回调地址未配置")}
            type="button"
          >
            <Icon size={17} />
            <span>{label}</span>
          </button>
        );
      })}
    </div>
  );
}

function startOAuth(provider, callbackURL) {
  window.location.href = bbsApi.oauthStartUrl(provider, callbackURL);
}

function providerIcon(provider) {
  switch (provider) {
    case "github":
      return Github;
    case "google":
      return Chrome;
    case "qq":
      return MessageCircle;
    default:
      return LogIn;
  }
}
