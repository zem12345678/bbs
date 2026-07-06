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
  const enabledProviders = enabledAuthProviders({ providers });
  if (enabledProviders.length === 0) {
    return null;
  }

  return (
    <div className="oauth-login-grid">
      {enabledProviders.map((provider) => {
        const Icon = providerIcon(provider.provider);
        return (
          <button
            className="oauth-login-button"
            key={provider.provider}
            onClick={() => startOAuth(provider.provider)}
            type="button"
          >
            <Icon size={17} />
            <span>{provider.label || provider.provider}</span>
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
