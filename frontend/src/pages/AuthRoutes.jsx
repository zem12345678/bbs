import React from "react";
import { Link, useLocation, useNavigate } from "react-router-dom";
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
  const [configState, setConfigState] = React.useState({
    loading: true,
    error: ""
  });

  React.useEffect(() => {
    let alive = true;
    setConfigState({ loading: true, error: "" });
    bbsApi
      .authConfig()
      .then((data) => {
        if (!alive) return;
        setConfig(normalizeAuthConfig(data));
        setConfigState({ loading: false, error: "" });
      })
      .catch(() => {
        if (!alive) return;
        setConfig(defaultAuthConfig);
        setConfigState({ loading: false, error: "登录配置暂时不可用，已使用默认账号入口。" });
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
    if (!passwordFormEnabled) {
      setState({ loading: false, error: signup ? "当前未开放账号注册。" : "当前未开放账号密码登录。" });
      return;
    }
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

  const configReady = !configState.loading;
  const passwordFormEnabled = configReady && config.password_enabled && (!signup || config.register_enabled);
  const anyOAuthProviderEnabled = enabledAuthProviders(config).length > 0;

  React.useEffect(() => {
    if (!configState.loading && signup && !config.register_enabled) {
      navigate("/user/signin", { replace: true });
    }
  }, [config.register_enabled, configState.loading, navigate, signup]);

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
    <section className="auth-page">
      <aside className="auth-page-intro panel">
        <div>
          <span className="auth-page-kicker">
            {signup ? <UserPlus size={16} /> : <LogIn size={16} />}
            账号
          </span>
          <h1>{signup ? "创建社区账号" : "登录社区账号"}</h1>
          <p>{signup ? "注册后可以发布内容、评论、收藏、关注作者并累积积分。" : "登录后继续参与讨论、查看消息和管理个人内容。"}</p>
        </div>
        <div className="auth-page-notes" aria-label="账号权益">
          <span>创作中心</span>
          <span>互动通知</span>
          <span>积分成长</span>
          {config.email_verification_required && <span>邮箱验证</span>}
        </div>
      </aside>
      <section className="auth-page-panel panel">
        <div className="auth-tabs" role="tablist" aria-label="账号入口">
          <button className={!signup ? "is-active" : ""} type="button" onClick={() => navigate("/user/signin")}>
            登录
          </button>
          <button
            className={signup ? "is-active" : ""}
            type="button"
            disabled={configState.loading || !config.register_enabled}
            onClick={() => navigate("/user/signup")}
            title={config.register_enabled ? "创建社区账号" : "当前未开放账号注册"}
          >
            注册
          </button>
        </div>
        {configState.loading && <p className="form-muted">正在读取登录配置...</p>}
        {configState.error && <p className="form-error">{configState.error}</p>}
        <OAuthLoginButtons disabled={configState.loading} disabledReason="正在读取登录配置" providers={config.providers} />
        {configReady && !anyOAuthProviderEnabled && <p className="form-muted">第三方登录暂未开启，请使用账号密码入口。</p>}
        {passwordFormEnabled ? (
          <form className="auth-form" onSubmit={submit} aria-busy={state.loading}>
            {signup ? (
              <>
                <label>
                  用户名
                  <input
                    autoComplete="username"
                    required
                    value={form.username}
                    onChange={(event) => updateField("username", event.target.value)}
                  />
                </label>
                <label>
                  邮箱
                  <input
                    autoComplete="email"
                    type="email"
                    required
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
                  required
                  value={form.account}
                  onChange={(event) => updateField("account", event.target.value)}
                />
              </label>
            )}
            <label>
              密码
              <input
                autoComplete={signup ? "new-password" : "current-password"}
                minLength={signup ? 8 : undefined}
                required
                type="password"
                value={form.password}
                onChange={(event) => updateField("password", event.target.value)}
              />
            </label>
            {signup && config.email_verification_required && <p className="form-muted">注册后需要完成邮箱验证，验证通过后可继续使用完整社区能力。</p>}
            {!signup && config.webmaster_enabled && <p className="form-muted">站长初始化账号已启用，可通过账号密码入口完成首次登录。</p>}
            {state.error && <p className="form-error">{state.error}</p>}
            <button type="submit" disabled={state.loading}>
              {state.loading ? "处理中..." : signup ? "创建账号" : "登录"}
            </button>
          </form>
        ) : (
          <p className="form-muted">{signup ? "当前未开放账号注册。" : "当前未开放账号密码登录。"}</p>
        )}
        <div className="auth-route-links">
          {signup ? (
            <Link to="/user/signin">已有账号，直接登录</Link>
          ) : (
            <>
              {config.register_enabled ? <Link to="/user/signup">创建新账号</Link> : <span className="auth-route-link-disabled">注册暂未开放</span>}
              {config.password_enabled ? <Link to="/user/password/forgot">忘记密码</Link> : <span className="auth-route-link-disabled">密码入口已关闭</span>}
            </>
          )}
        </div>
      </section>
    </section>
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

export function ForgotPasswordPage() {
  const [email, setEmail] = React.useState("");
  const [state, setState] = React.useState({
    loading: false,
    error: "",
    message: "",
    resetUrl: ""
  });

  async function submit(event) {
    event.preventDefault();
    const normalizedEmail = email.trim();
    if (!normalizedEmail) {
      setState({ loading: false, error: "请输入注册邮箱", message: "", resetUrl: "" });
      return;
    }
    setState({ loading: true, error: "", message: "", resetUrl: "" });
    try {
      const data = await bbsApi.requestPasswordReset({ email: normalizedEmail });
      const resetUrl = data?.reset_url || (data?.reset_token ? `/user/password/reset?token=${encodeURIComponent(data.reset_token)}` : "");
      setState({
        loading: false,
        error: "",
        message: "如果邮箱存在，重置链接会发送到该邮箱。",
        resetUrl
      });
    } catch (error) {
      setState({ loading: false, error: error.message || "提交失败", message: "", resetUrl: "" });
    }
  }

  return (
    <>
      <RouteHeader icon={RotateCcwKey} eyebrow="账号安全" title="找回密码" description="通过注册邮箱恢复账号访问权限。" />
      <section className="auth-route-card panel">
        <form className="auth-form" onSubmit={submit}>
          <label>
            注册邮箱
            <input autoComplete="email" type="email" value={email} onChange={(event) => setEmail(event.target.value)} />
          </label>
          {state.error && <p className="form-error">{state.error}</p>}
          {state.message && <p className="form-success">{state.message}</p>}
          <button type="submit" disabled={state.loading}>
            {state.loading ? "提交中..." : "发送重置链接"}
          </button>
        </form>
        {state.resetUrl && (
          <a className="route-link-button" href={state.resetUrl}>
            继续重置
          </a>
        )}
        <div className="auth-route-links">
          <Link to="/user/signin">返回登录</Link>
          <Link to="/user/signup">创建新账号</Link>
        </div>
      </section>
    </>
  );
}

export function ResetPasswordPage() {
  const location = useLocation();
  const initialToken = React.useMemo(() => new URLSearchParams(location.search).get("token") || "", [location.search]);
  const [form, setForm] = React.useState({
    token: initialToken,
    new_password: "",
    confirm_password: ""
  });
  const [state, setState] = React.useState({
    loading: false,
    error: "",
    message: ""
  });

  React.useEffect(() => {
    setForm((current) => ({ ...current, token: initialToken }));
  }, [initialToken]);

  function updateField(field, value) {
    setForm((current) => ({ ...current, [field]: value }));
  }

  async function submit(event) {
    event.preventDefault();
    if (!form.token.trim()) {
      setState({ loading: false, error: "缺少重置令牌", message: "" });
      return;
    }
    if (form.new_password.length < 8) {
      setState({ loading: false, error: "新密码至少 8 位", message: "" });
      return;
    }
    if (form.new_password !== form.confirm_password) {
      setState({ loading: false, error: "两次输入的新密码不一致", message: "" });
      return;
    }
    setState({ loading: true, error: "", message: "" });
    try {
      await bbsApi.resetPassword({
        token: form.token.trim(),
        new_password: form.new_password
      });
      setForm((current) => ({ ...current, new_password: "", confirm_password: "" }));
      setState({ loading: false, error: "", message: "密码已重置，可以使用新密码登录。" });
    } catch (error) {
      setState({ loading: false, error: error.message || "重置失败", message: "" });
    }
  }

  return (
    <>
      <RouteHeader icon={RotateCcwKey} eyebrow="账号安全" title="重置密码" description="设置新的账号密码后返回登录。" />
      <section className="auth-route-card panel">
        <form className="auth-form" onSubmit={submit}>
          {!initialToken && (
            <label>
              重置令牌
              <input value={form.token} onChange={(event) => updateField("token", event.target.value)} />
            </label>
          )}
          <label>
            新密码
            <input
              autoComplete="new-password"
              type="password"
              value={form.new_password}
              onChange={(event) => updateField("new_password", event.target.value)}
            />
          </label>
          <label>
            确认新密码
            <input
              autoComplete="new-password"
              type="password"
              value={form.confirm_password}
              onChange={(event) => updateField("confirm_password", event.target.value)}
            />
          </label>
          {state.error && <p className="form-error">{state.error}</p>}
          {state.message && <p className="form-success">{state.message}</p>}
          <button type="submit" disabled={state.loading}>
            {state.loading ? "重置中..." : "重置密码"}
          </button>
        </form>
        <div className="auth-route-links">
          <Link to="/user/signin">返回登录</Link>
          <Link to="/user/password/forgot">重新发送</Link>
        </div>
      </section>
    </>
  );
}

export function EmailVerifyPage({ auth, onAuthUserUpdate }) {
  const location = useLocation();
  const initialToken = React.useMemo(() => new URLSearchParams(location.search).get("token") || "", [location.search]);
  const [token, setToken] = React.useState(initialToken);
  const [state, setState] = React.useState({
    loading: false,
    submittedToken: "",
    error: "",
    message: ""
  });

  const submitToken = React.useCallback(
    async (nextToken) => {
      const normalizedToken = nextToken.trim();
      if (!normalizedToken) {
        setState({ loading: false, submittedToken: "", error: "缺少邮箱验证令牌", message: "" });
        return;
      }
      setState({ loading: true, submittedToken: normalizedToken, error: "", message: "" });
      try {
        const data = await bbsApi.verifyEmail({ token: normalizedToken });
        const user = data?.user;
        if (auth?.accessToken && user) {
          onAuthUserUpdate?.(user);
        }
        setState({ loading: false, submittedToken: normalizedToken, error: "", message: "邮箱已验证。" });
      } catch (error) {
        setState({ loading: false, submittedToken: normalizedToken, error: error.message || "邮箱验证失败", message: "" });
      }
    },
    [auth?.accessToken, onAuthUserUpdate]
  );

  React.useEffect(() => {
    setToken(initialToken);
    if (initialToken && initialToken !== state.submittedToken) {
      submitToken(initialToken);
    }
  }, [initialToken, state.submittedToken, submitToken]);

  function submit(event) {
    event.preventDefault();
    submitToken(token);
  }

  return (
    <>
      <RouteHeader icon={MailCheck} eyebrow="账号安全" title="邮箱验证" description="完成邮箱验证后，账号安全状态会同步到个人工作台。" />
      <section className="auth-route-card panel">
        <form className="auth-form" onSubmit={submit}>
          {!initialToken && (
            <label>
              验证令牌
              <input value={token} onChange={(event) => setToken(event.target.value)} />
            </label>
          )}
          {state.error && <p className="form-error">{state.error}</p>}
          {state.message && <p className="form-success">{state.message}</p>}
          <button type="submit" disabled={state.loading}>
            {state.loading ? "验证中..." : "验证邮箱"}
          </button>
        </form>
        <div className="auth-route-links">
          <Link to="/dashboard/profile">返回个人资料</Link>
          <Link to="/user/signin">返回登录</Link>
        </div>
      </section>
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
