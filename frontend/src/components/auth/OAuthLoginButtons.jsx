import React from "react";
import { Chrome, Github, LogIn, MessageCircle } from "lucide-react";
import { bbsApi } from "../../api";

export const defaultAuthConfig = {
  password_enabled: true,
  register_enabled: true,
  providers: []
};

export function normalizeAuthConfig(data) {
  return {
    password_enabled: data?.password_enabled ?? data?.passwordEnabled ?? true,
    register_enabled: data?.register_enabled ?? data?.registerEnabled ?? true,
    providers: Array.isArray(data?.providers) ? data.providers : []
  };
}

export function enabledAuthProviders(config) {
  return (config?.providers || []).filter((provider) => provider?.enabled && provider?.provider);
}

export function OAuthLoginButtons({ providers = [] }) {
  const visibleProviders = providers.filter((provider) => provider?.provider);
  if (visibleProviders.length === 0) {
    return null;
  }

  return (
    <div className="oauth-login-grid">
      {visibleProviders.map((provider) => {
        const Icon = providerIcon(provider.provider);
        const enabled = Boolean(provider.enabled);
        const label = provider.label || provider.provider;
        return (
          <button
            className="oauth-login-button"
            disabled={!enabled}
            key={provider.provider}
            onClick={() => enabled && startOAuth(provider.provider)}
            title={enabled ? `${label} 登录` : `${label} 登录未开启或 OAuth 密钥未配置`}
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

function startOAuth(provider) {
  const redirect = `${window.location.origin}/auth/callback`;
  window.location.href = bbsApi.oauthStartUrl(provider, redirect);
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
