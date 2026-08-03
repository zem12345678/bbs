import React from "react";
import { Chrome, Github, LogIn, MessageCircle } from "lucide-react";
import { bbsApi } from "../../api";
import { oauthCallbackURL } from "../../lib/authRedirect";
export { defaultAuthConfig, enabledAuthProviders, normalizeAuthConfig } from "../../lib/authConfig.js";

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
