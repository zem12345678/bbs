import React from "react";
import { Link, useNavigate } from "react-router-dom";
import { LogIn, MailCheck, RotateCcwKey, UserPlus } from "lucide-react";
import { bbsApi } from "../api";
import { defaultAuthConfig, enabledAuthProviders, normalizeAuthConfig, OAuthLoginButtons } from "../components/auth/OAuthLoginButtons.jsx";
import { userDisplayName } from "../lib/postMappers";
import { EmptyState, RouteHeader } from "./RouteBlocks.jsx";

export function AuthRoutePage({ auth, mode = "signin", onAuthSuccess }) {
  const navigate = useNavigate();
  const signup = mode === "signup";
  const [form, setForm] = React.useState({
    account: "",
    username: "",
    email: "",
    nickname: "",
    password: ""
  });
  const [state, setState] = React.useState({
    loading: false,
    error: ""
  });
  const [config, setConfig] = React.useState(defaultAuthConfig);

  React.useEffect(() => {
    let alive = true;
    bbsApi
      .authConfig()
      .then((data) => {
        if (!alive) return;
        setConfig(normalizeAuthConfig(data));
      })
      .catch(() => {
        if (!alive) return;
        setConfig(defaultAuthConfig);
      });
    return () => {
      alive = false;
    };
  }, []);

  function updateField(field, value) {
    setForm((current) => ({ ...current, [field]: value }));
  }

  async function submit(event) {
    event.preventDefault();
    setState({ loading: true, error: "" });
    try {
      const data = signup
        ? await bbsApi.register({
            username: form.username.trim(),
            email: form.email.trim(),
            nickname: form.nickname.trim() || form.username.trim(),
            password: form.password
          })
        : await bbsApi.login({
            account: form.account.trim(),
            password: form.password
          });
      onAuthSuccess(data);
      navigate("/user/profile");
    } catch (error) {
      setState({ loading: false, error: error.message || (signup ? "注册失败" : "登录失败") });
    }
  }

  const enabledProviders = enabledAuthProviders(config);
  const passwordFormEnabled = config.password_enabled && (!signup || config.register_enabled);

  if (auth) {
    return (
      <EmptyState
        title={`已登录为 ${userDisplayName(auth.user)}`}
        description="可以进入个人中心维护资料、查看互动和积分。"
        action={
          <Link className="route-link-button" to="/user/profile">
            进入个人中心
          </Link>
        }
      />
    );
  }

  return (
    <>
      <RouteHeader
        icon={signup ? UserPlus : LogIn}
        eyebrow="账号"
        title={signup ? "创建社区账号" : "登录社区账号"}
        description={signup ? "注册后可以发布内容、评论、收藏、关注作者并累积积分。" : "登录后继续参与讨论、查看消息和管理个人内容。"}
      />
      <section className="auth-route-card panel">
        {passwordFormEnabled ? (
          <form className="auth-form" onSubmit={submit}>
            {signup ? (
              <>
                <label>
                  用户名
                  <input
                    autoComplete="username"
                    value={form.username}
                    onChange={(event) => updateField("username", event.target.value)}
                  />
                </label>
                <label>
                  邮箱
                  <input
                    autoComplete="email"
                    type="email"
                    value={form.email}
                    onChange={(event) => updateField("email", event.target.value)}
                  />
                </label>
                <label>
                  昵称
                  <input value={form.nickname} onChange={(event) => updateField("nickname", event.target.value)} />
                </label>
              </>
            ) : (
              <label>
                用户名或邮箱
                <input
                  autoComplete="username"
                  value={form.account}
                  onChange={(event) => updateField("account", event.target.value)}
                />
              </label>
            )}
            <label>
              密码
              <input
                autoComplete={signup ? "new-password" : "current-password"}
                type="password"
                value={form.password}
                onChange={(event) => updateField("password", event.target.value)}
              />
            </label>
            {state.error && <p className="form-error">{state.error}</p>}
            <button type="submit" disabled={state.loading}>
              {state.loading ? "处理中..." : signup ? "创建账号" : "登录"}
            </button>
          </form>
        ) : (
          <p className="form-muted">{signup ? "当前未开放账号注册。" : "当前未开放账号密码登录。"}</p>
        )}
        {enabledProviders.length > 0 && <OAuthLoginButtons providers={enabledProviders} />}
        <div className="auth-route-links">
          {signup ? (
            <Link to="/user/signin">已有账号，去登录</Link>
          ) : (
            <>
              <Link to="/user/signup">创建新账号</Link>
              <Link to="/user/password/forgot">忘记密码</Link>
            </>
          )}
        </div>
      </section>
    </>
  );
}

export function AuthCallbackPage({ auth, onAuthSuccess }) {
  const navigate = useNavigate();
  const [error, setError] = React.useState("");

  React.useEffect(() => {
    const params = new URLSearchParams(window.location.hash.replace(/^#/, ""));
    const token = params.get("access_token");
    const expiresAt = params.get("expires_at");
    const oauthError = params.get("error");
    window.history.replaceState(null, "", "/auth/callback");
    if (token) {
      onAuthSuccess({ access_token: token, expires_at: Number(expiresAt || 0), user: auth?.user || null });
      navigate("/user/profile", { replace: true });
      return;
    }
    setError(oauthError || "第三方登录未完成");
  }, []);

  return (
    <>
      <RouteHeader icon={LogIn} eyebrow="账号" title="第三方登录" description="正在完成账号登录状态同步。" />
      <EmptyState
        title={error ? "登录失败" : "正在登录"}
        description={error || "请稍候。"}
        action={
          <Link className="route-link-button" to="/user/signin">
            返回登录
          </Link>
        }
      />
    </>
  );
}

export function AuthPendingPage({ kind = "forgot" }) {
  const meta = {
    forgot: {
      icon: RotateCcwKey,
      title: "找回密码",
      description: "通过邮箱验证身份后重置账号密码。"
    },
    reset: {
      icon: RotateCcwKey,
      title: "重置密码",
      description: "请输入新的安全密码并完成账号恢复。"
    },
    verify: {
      icon: MailCheck,
      title: "邮箱验证",
      description: "验证邮箱后可继续使用账号安全能力。"
    }
  }[kind];

  return (
    <>
      <RouteHeader icon={meta.icon} eyebrow="账号安全" title={meta.title} description={meta.description} />
      <EmptyState
        title="暂不可用"
        description="当前可以先登录后在个人中心修改密码。"
        action={
          <Link className="route-link-button" to="/user/signin">
            返回登录
          </Link>
        }
      />
    </>
  );
}
