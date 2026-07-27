export function friendlyMallCheckoutError(error) {
  const message = mallCheckoutErrorMessage(error);
  const normalized = message.toLowerCase();
  const legacyCode = mallLegacyCode(error);
  if (!message) return "兑换失败，请稍后重试。";
  if (normalized.includes("order price changed") || message.includes("商品价格已变更")) return "商品价格已变更，请确认新金额后重试。";
  if (normalized.includes("insufficient stock") || message.includes("库存不足")) return "库存不足，请刷新商品或调整数量后重试。";
  if (normalized.includes("insufficient credits") || message.includes("积分不足")) return "积分不足，请确认余额后再兑换。";
  if (normalized.includes("product unavailable") || message.includes("商品暂不可用") || message.includes("商品不可用")) return "商品暂不可兑换，请刷新商品列表后重试。";
  if (normalized.includes("coupon unavailable") || message.includes("优惠券") || message.includes("优惠码")) return "优惠券暂不可用，请重新选择或清空优惠码。";
  if (normalized.includes("invalid order state") || message.includes("订单状态")) return "订单状态已变化，请刷新后重试。";
  if (normalized.includes("unsupported payment") || message.includes("支付方式")) return "当前支付方式暂不支持，请选择积分支付。";
  if (normalized.includes("credit charger not configured") || message.includes("积分支付服务")) return "积分支付服务暂未配置，请稍后在个人工作台继续支付。";
  if (normalized.includes("pending order already reserves product") || message.includes("商品已有待支付订单")) return "该商品已有待支付订单，请前往订单继续支付或取消后再兑换。";
  if (normalized.includes("pending membership order already exists") || message.includes("待支付会员订单")) return "该会员权益已有待支付订单，请前往订单继续支付或取消后再兑换。";
  if (normalized.includes("membership order refund unavailable") || message.includes("会员订单不支持售后")) return "会员权益订单不支持普通售后，请联系管理员处理。";
  if (normalized.includes("active theme entitlement already exists") || message.includes("主题权益已解锁")) return "该主题权益已解锁，请直接前往个人资料启用。";
  if (normalized.includes("pending theme order already exists") || message.includes("待支付主题订单")) return "该主题已有待支付订单，请前往订单继续支付或取消后再兑换。";
  if (normalized.includes("duplicate theme grant in order") || message.includes("重复主题权益")) return "同一主题权益每次只能兑换一份，请调整数量后重试。";
  if (normalized.includes("active badge entitlement already exists") || message.includes("徽章权益已解锁")) return "该徽章权益已解锁，请前往个人徽章查看。";
  if (normalized.includes("pending badge order already exists") || message.includes("待支付徽章订单")) return "该徽章已有待支付订单，请前往订单继续支付或取消后再兑换。";
  if (normalized.includes("duplicate badge grant in order") || message.includes("重复徽章权益")) return "同一徽章权益每次只能兑换一份，请调整数量后重试。";
  if (legacyCode === "FailedPrecondition" && (error?.httpCode === 412 || error?.status === 412)) return message;
  return message;
}

export function friendlyMallOrderActionError(error, fallback = "订单操作失败，请稍后重试。") {
  const message = friendlyMallCheckoutError(error);
  if (!message || message === "兑换失败，请稍后重试。") {
    return fallback;
  }
  return message;
}

export function friendlyMallReviewError(error, fallback = "评价发布失败，请稍后重试。") {
  const message = mallCheckoutErrorMessage(error);
  const normalized = message.toLowerCase();
  const legacyCode = mallLegacyCode(error);
  if (legacyCode === "AlreadyExists" || normalized.includes("duplicate reference")) return "该订单已评价过该商品，请勿重复提交。";
  if (normalized.includes("invalid order state") || message.includes("订单状态")) return "只有已完成且包含该商品的订单可以评价。";
  if (normalized.includes("order does not belong to user") || message.includes("不属于当前用户")) return "该订单不属于当前账号，无法评价。";
  if (normalized.includes("product not found") || message.includes("商品不存在")) return "商品不存在或已下架，暂时无法评价。";
  return message || fallback;
}

export function shouldRefreshMallInventoryAfterError(error) {
  const normalized = mallCheckoutErrorMessage(error).toLowerCase();
  const message = mallCheckoutErrorMessage(error);
  return normalized.includes("insufficient stock") || normalized.includes("product unavailable") || normalized.includes("order price changed") || message.includes("库存") || message.includes("商品");
}

export function shouldRefreshMallCouponsAfterError(error) {
  const message = mallCheckoutErrorMessage(error);
  return message.toLowerCase().includes("coupon unavailable") || message.includes("优惠券") || message.includes("优惠码");
}

function mallCheckoutErrorMessage(error) {
  if (typeof error?.message === "string") return error.message.trim();
  if (typeof error === "string") return error.trim();
  return "";
}

function mallLegacyCode(error) {
  return String(error?.meta?.legacy_code || error?.meta?.legacyCode || "").trim();
}
