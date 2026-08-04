import React from "react";
import { Bell } from "lucide-react";
import { bbsApi } from "../../api";

const NOTIFICATION_PREFERENCES = [
  { type: "system", label: "系统通知", description: "平台维护和运营通知" },
  { type: "follow", label: "新增关注", description: "有人关注你的账号" },
  { type: "comment", label: "内容评论", description: "有人评论你的文章或话题" },
  { type: "reply", label: "评论回复", description: "有人回复你的评论" },
  { type: "like", label: "点赞", description: "有人点赞你的内容" },
  { type: "favorite", label: "收藏", description: "有人收藏你的内容" },
  { type: "qa_answer_accepted", label: "回答被采纳", description: "你的回答被采纳并获得积分" },
  { type: "mall_order_paid", label: "订单支付", description: "商城订单支付完成" },
  { type: "mall_order_shipped", label: "订单发货", description: "商城订单已发货" },
  { type: "mall_order_completed", label: "订单完成", description: "商城订单已完成" },
  { type: "mall_refund_approved", label: "售后通过", description: "商城退款申请已通过" },
  { type: "mall_refund_rejected", label: "售后拒绝", description: "商城退款申请未通过" },
  { type: "mall_digital_entitlement_revoked", label: "权益撤销", description: "数字权益被撤销" },
  { type: "mall_review_published", label: "评价展示", description: "商品评价已展示" },
  { type: "mall_review_hidden", label: "评价隐藏", description: "商品评价被隐藏" }
];

export default function NotificationPreferencesPanel({ auth }) {
  const [items, setItems] = React.useState(defaultPreferences());
  const [state, setState] = React.useState({ loading: true, saving: false, error: "", message: "" });

  React.useEffect(() => {
    let alive = true;
    if (!auth?.accessToken) {
      setState({ loading: false, saving: false, error: "", message: "" });
      return undefined;
    }
    setState({ loading: true, saving: false, error: "", message: "" });
    bbsApi.notificationPreferences(auth.accessToken)
      .then((data) => {
        if (!alive) return;
        setItems(mergePreferences(data?.items || data?.preferences));
        setState({ loading: false, saving: false, error: "", message: "" });
      })
      .catch((error) => {
        if (!alive) return;
        setState({ loading: false, saving: false, error: error.message || "通知设置加载失败", message: "" });
      });
    return () => {
      alive = false;
    };
  }, [auth?.accessToken]);

  function toggle(type) {
    setItems((current) => current.map((item) => (item.type === type ? { ...item, enabled: !item.enabled } : item)));
    setState((current) => ({ ...current, message: "", error: "" }));
  }

  async function save() {
    if (!auth?.accessToken) return;
    setState((current) => ({ ...current, saving: true, error: "", message: "" }));
    try {
      const data = await bbsApi.updateNotificationPreferences(items, auth.accessToken);
      setItems(mergePreferences(data?.items || data?.preferences));
      setState({ loading: false, saving: false, error: "", message: "通知设置已保存。" });
    } catch (error) {
      setState({ loading: false, saving: false, error: error.message || "通知设置保存失败", message: "" });
    }
  }

  return (
    <section className="account-security panel notification-preferences-panel">
      <header>
        <strong><Bell size={18} aria-hidden="true" /> 通知设置</strong>
        <p>选择希望接收的站内通知类型，关闭后只影响之后产生的通知。</p>
      </header>
      {state.loading ? (
        <p>正在加载通知设置...</p>
      ) : (
        <>
          <div className="notification-preferences-list">
            {items.map((item) => {
              const definition = NOTIFICATION_PREFERENCES.find((candidate) => candidate.type === item.type);
              return (
                <label className="notification-preference-row" key={item.type}>
                  <span>
                    <strong>{definition?.label || item.type}</strong>
                    <small>{definition?.description || "站内通知"}</small>
                  </span>
                  <input type="checkbox" checked={item.enabled} onChange={() => toggle(item.type)} />
                </label>
              );
            })}
          </div>
          {state.error && <p className="form-error">{state.error}</p>}
          {state.message && <p className="form-success">{state.message}</p>}
          <button type="button" disabled={state.saving} onClick={save}>
            {state.saving ? "保存中..." : "保存通知设置"}
          </button>
        </>
      )}
    </section>
  );
}

function defaultPreferences() {
  return NOTIFICATION_PREFERENCES.map(({ type }) => ({ type, enabled: true }));
}

function mergePreferences(items) {
  const overrides = new Map((Array.isArray(items) ? items : []).map((item) => [item.type || item.notification_type, item.enabled !== false]));
  return defaultPreferences().map((item) => ({ ...item, enabled: overrides.has(item.type) ? overrides.get(item.type) : true }));
}
