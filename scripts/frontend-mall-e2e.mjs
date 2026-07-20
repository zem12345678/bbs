import { spawn } from "node:child_process";
import { existsSync } from "node:fs";
import { mkdtemp, readdir, rm, writeFile } from "node:fs/promises";
import net from "node:net";
import os from "node:os";
import path from "node:path";
import { fileURLToPath } from "node:url";

import { normalizeAuthResponse } from "../frontend/src/lib/authStorage.js";
import { friendlyMallReviewError } from "../frontend/src/lib/mallErrors.js";

const API_BASE = (process.env.API_BASE || process.env.VITE_API_BASE || "http://127.0.0.1:18080/api/v1").replace(/\/$/, "");
const FRONTEND_BASE = (process.env.FRONTEND_BASE || "http://127.0.0.1:8850").replace(/\/$/, "");
const MALL_POSTGRES_DSN =
  process.env.MALL_POSTGRES_DSN ||
  process.env.BBS_MALL_POSTGRES_DSN ||
  "postgres://bbs_mall_app:local_mall_pass@127.0.0.1:5432/bbs?sslmode=disable&search_path=bbs_mall";
const PSQL_BIN = process.env.PSQL_BIN || "psql";
const AUTH_STORAGE_KEY = "bbs.community.auth";
const SCRIPT_DIR = path.dirname(fileURLToPath(import.meta.url));
const REPO_ROOT = path.dirname(SCRIPT_DIR);
const FRONTEND_DIR = path.join(REPO_ROOT, "frontend");
const VITE_BIN = path.join(FRONTEND_DIR, "node_modules", "vite", "bin", "vite.js");

const CHECKOUT_PRICE = 20;
const COUPON_DISCOUNT = 5;
const ZERO_CREDIT_CHECKOUT_PRICE = 5;
const CREDIT_TOP_UP = 200;
const INSUFFICIENT_CHECKOUT_PRICE = 260;
const PAYMENT_RECOVERY_TOP_UP = 300;
const MEMBERSHIP_DURATION_MS = 30 * 24 * 60 * 60 * 1000;
const DEFAULT_QA_REWARD_CREDITS = 10;
const CDP_COMMAND_TIMEOUT_MS = 30000;
const ORDER_HISTORY_PAGE_SIZE = 50;
const ORDER_HISTORY_FIXTURE_ORDER_COUNT = ORDER_HISTORY_PAGE_SIZE + 1;
const ENTITLEMENT_HISTORY_PAGE_SIZE = 50;
const ENTITLEMENT_HISTORY_FIXTURE_COUNT = ENTITLEMENT_HISTORY_PAGE_SIZE + 1;
const COUPON_HISTORY_PAGE_SIZE = 50;
const COUPON_HISTORY_FIXTURE_COUNT = COUPON_HISTORY_PAGE_SIZE + 1;
const REFUND_HISTORY_PAGE_SIZE = 50;
const REFUND_HISTORY_FIXTURE_COUNT = REFUND_HISTORY_PAGE_SIZE + 1;
const REVIEW_HISTORY_PAGE_SIZE = 50;
const REVIEW_HISTORY_FIXTURE_COUNT = REVIEW_HISTORY_PAGE_SIZE + 1;
const CREDIT_HISTORY_PAGE_SIZE = 50;
const CREDIT_HISTORY_FIXTURE_COUNT = CREDIT_HISTORY_PAGE_SIZE + 1;
const ADDRESS_HISTORY_PAGE_SIZE = 50;
const ADDRESS_HISTORY_FIXTURE_COUNT = ADDRESS_HISTORY_PAGE_SIZE + 1;

async function main() {
  await assertHttpReachable(`${API_BASE}/mall/products?limit=1&offset=0`, "api-gateway");

  const frontendServer = await ensureFrontendServer();
  try {
    const fixture = await createCommercialFixture();
    const chromePath = await findChromeExecutable();
    const inactiveFavoriteResult = await assertInactiveProductFavoriteRejected(fixture);
    const duplicateGrantCartResult = await assertDuplicateDigitalGrantCartRejected(fixture);
    const cartReplayResult = await assertCartCheckoutReplayRequestLocked(fixture);
    const result = await runBrowserCheckout(chromePath, fixture);

    console.log(
      JSON.stringify(
        {
          ok: true,
          productId: fixture.product.id,
          orderHistoryProductId: fixture.orderHistoryProduct.id,
          orderHistoryFixtureOrderCount: fixture.orderHistory.count,
          entitlementHistoryProductId: fixture.entitlementHistoryProduct.id,
          entitlementHistoryFixtureCount: fixture.entitlementHistory.count,
          couponHistoryFixtureCount: fixture.couponHistory.count,
          refundHistoryProductId: fixture.refundHistoryProduct.id,
          refundHistoryFixtureCount: fixture.refundHistory.count,
          reviewHistoryProductId: fixture.reviewHistoryProduct.id,
          reviewHistoryFixtureCount: fixture.reviewHistory.count,
          creditHistoryFixtureCount: fixture.creditHistory.count,
          addressHistoryFixtureCount: result.addressHistoryFixtureCount,
          cartProductId: fixture.cartProduct.id,
          refundProductId: fixture.refundProduct.id,
          rejectedRefundProductId: fixture.rejectedRefundProduct.id,
          inactiveFavoriteProductId: fixture.inactiveFavoriteProduct.id,
          inactiveFavoriteApiStatus: inactiveFavoriteResult.status,
          inactiveFavoriteApiMessage: inactiveFavoriteResult.message,
          digitalProductId: fixture.digitalProduct.id,
          duplicateDigitalProductId: fixture.duplicateDigitalProduct.id,
          duplicateGrantCartApiStatus: duplicateGrantCartResult.status,
          duplicateGrantCartApiMessage: duplicateGrantCartResult.message,
          cartReplayOrderId: cartReplayResult.orderId,
          cartReplayDuplicateOrderId: cartReplayResult.duplicateOrderId,
          cartReplayConflictApiStatus: cartReplayResult.conflictStatus,
          cartReplayConflictApiMessage: cartReplayResult.conflictMessage,
          themeProductId: fixture.themeProduct.id,
          membershipProductId: fixture.membershipProduct.id,
          couponCode: fixture.coupon.code,
          directCouponCode: fixture.directCoupon.code,
          zeroCreditCouponCode: fixture.zeroCreditCoupon.code,
          cancelCouponCode: fixture.cancelCoupon.code,
          archivedCouponStatus: result.archivedCouponStatus,
          archivedCouponRejectStatus: result.archivedCouponRejectStatus,
          archivedCouponRejectText: result.archivedCouponRejectText,
          userId: fixture.auth.user.id,
          checkInDay: result.checkInDay,
          checkInLedgerId: result.checkInLedgerId,
          checkInBalanceBefore: result.checkInBalanceBefore,
          checkInBalanceAfter: result.checkInBalanceAfter,
          checkInDuplicate: result.checkInDuplicate,
          checkInButtonText: result.checkInButtonText,
          orderId: result.orderId,
          orderNo: result.orderNo,
          orderHistoryInitialTotal: result.orderHistoryInitialTotal,
          orderHistoryLoadedOrderNo: result.orderHistoryLoadedOrderNo,
          orderHistorySelectedOrderNo: result.orderHistorySelectedOrderNo,
          entitlementHistoryInitialTotal: result.entitlementHistoryInitialTotal,
          entitlementHistoryLoadedCode: result.entitlementHistoryLoadedCode,
          couponHistoryInitialTotal: result.couponHistoryInitialTotal,
          couponHistoryLoadedCode: result.couponHistoryLoadedCode,
          refundHistoryInitialTotal: result.refundHistoryInitialTotal,
          refundHistoryLoadedNote: result.refundHistoryLoadedNote,
          reviewHistoryInitialTotal: result.reviewHistoryInitialTotal,
          reviewHistoryLoadedContent: result.reviewHistoryLoadedContent,
          creditHistoryInitialTotal: result.creditHistoryInitialTotal,
          creditHistoryLoadedReason: result.creditHistoryLoadedReason,
          addressHistoryInitialTotal: result.addressHistoryInitialTotal,
          addressHistoryLoadedDetail: result.addressHistoryLoadedDetail,
          directCouponOrderId: result.directCouponOrderId,
          directCouponText: result.directCouponText,
          directCouponReuseHttpStatus: result.directCouponReuseHttpStatus,
          directCouponReuseText: result.directCouponReuseText,
          zeroCreditCouponOrderId: result.zeroCreditCouponOrderId,
          zeroCreditCouponPaymentId: result.zeroCreditCouponPaymentId,
          zeroCreditCouponUsageId: result.zeroCreditCouponUsageId,
          zeroCreditCouponBalanceBefore: result.zeroCreditCouponBalanceBefore,
          zeroCreditCouponBalanceAfter: result.zeroCreditCouponBalanceAfter,
          zeroCreditCouponLockedStock: result.zeroCreditCouponLockedStock,
          zeroCreditCouponRefundId: result.zeroCreditCouponRefundId,
          zeroCreditCouponRefundBalanceBefore: result.zeroCreditCouponRefundBalanceBefore,
          zeroCreditCouponRefundBalanceAfter: result.zeroCreditCouponRefundBalanceAfter,
          zeroCreditCouponRefundRestoredStock: result.zeroCreditCouponRefundRestoredStock,
          zeroCreditCouponRefundNotificationTitles: result.zeroCreditCouponRefundNotificationTitles,
          dashboardPayOrderId: result.dashboardPayOrderId,
          dashboardPayText: result.dashboardPayText,
          dashboardPayLockedStock: result.dashboardPayLockedStock,
          dashboardPayNotificationTitles: result.dashboardPayNotificationTitles,
          payingOrderResumeOrderId: result.payingOrderResumeOrderId,
          payingOrderResumePaymentKey: result.payingOrderResumePaymentKey,
          payingOrderResumeLedgerCount: result.payingOrderResumeLedgerCount,
          insufficientPaymentOrderId: result.insufficientPaymentOrderId,
          insufficientPaymentText: result.insufficientPaymentText,
          insufficientPaymentLockedStock: result.insufficientPaymentLockedStock,
          insufficientPaymentBalanceBeforeFailure: result.insufficientPaymentBalanceBeforeFailure,
          insufficientPaymentBalanceAfterTopUp: result.insufficientPaymentBalanceAfterTopUp,
          insufficientPaymentFailedPaymentId: result.insufficientPaymentFailedPaymentId,
          insufficientPaymentRecoveredPaymentId: result.insufficientPaymentRecoveredPaymentId,
          insufficientPaymentNotificationTitles: result.insufficientPaymentNotificationTitles,
          cancelCouponOrderId: result.cancelCouponOrderId,
          cancelCouponText: result.cancelCouponText,
          cancelCouponLockedStock: result.cancelCouponLockedStock,
          cancelCouponRestoredStock: result.cancelCouponRestoredStock,
          cancelCouponReleasedUsageId: result.cancelCouponReleasedUsageId,
          cancelCouponReclaimedUsageId: result.cancelCouponReclaimedUsageId,
          paidText: result.paidText,
          fulfillmentText: result.fulfillmentText,
          promotedAddressText: result.promotedAddressText,
          reviewText: result.reviewText,
          publicReviewText: result.publicReviewText,
          reviewDuplicateHttpStatus: result.reviewDuplicateHttpStatus,
          reviewDuplicateLegacyCode: result.reviewDuplicateLegacyCode,
          reviewDuplicateText: result.reviewDuplicateText,
          reviewHiddenNotificationTitles: result.reviewHiddenNotificationTitles,
          publicReviewHiddenText: result.publicReviewHiddenText,
          reviewNotificationTitles: result.reviewNotificationTitles,
          cartOrderId: result.cartOrderId,
          cartText: result.cartText,
          cartNotificationTitles: result.cartNotificationTitles,
          refundOrderId: result.refundOrderId,
          refundText: result.refundText,
          refundConfirmActionHidden: result.refundConfirmActionHidden,
          refundConfirmApiStatus: result.refundConfirmApiStatus,
          refundConfirmApiMessage: result.refundConfirmApiMessage,
          refundLockedStock: result.refundLockedStock,
          refundRestoredStock: result.refundRestoredStock,
          refundNotificationTitles: result.refundNotificationTitles,
          refundCreditLedgerId: result.refundCreditLedgerId,
          refundCreditLedgerSourceEventId: result.refundCreditLedgerSourceEventId,
          refundCreditLedgerCountAfterRetry: result.refundCreditLedgerCountAfterRetry,
          refundBalanceAfterRetry: result.refundBalanceAfterRetry,
          rejectedRefundOrderId: result.rejectedRefundOrderId,
          rejectedRefundCanceledId: result.rejectedRefundCanceledId,
          rejectedRefundReplacementId: result.rejectedRefundReplacementId,
          rejectedRefundCancelBalanceBefore: result.rejectedRefundCancelBalanceBefore,
          rejectedRefundCancelBalanceAfter: result.rejectedRefundCancelBalanceAfter,
          rejectedRefundCancelStockBefore: result.rejectedRefundCancelStockBefore,
          rejectedRefundCancelStockAfter: result.rejectedRefundCancelStockAfter,
          rejectedRefundOrderStatusAfterRejection: result.rejectedRefundOrderStatusAfterRejection,
          rejectedRefundText: result.rejectedRefundText,
          rejectedRefundNotificationTitles: result.rejectedRefundNotificationTitles,
          digitalOrderId: result.digitalOrderId,
          digitalOrderNo: result.digitalOrderNo,
          digitalGrantKey: fixture.digitalGrantKey,
          digitalEntitlementCode: result.digitalEntitlementCode,
          digitalEntitlementCodes: result.digitalEntitlementCodes,
          digitalEntitlementCount: result.digitalEntitlementCount,
          digitalRefundId: result.digitalRefundId,
          digitalText: result.digitalText,
          digitalRevokedText: result.digitalRevokedText,
          publicBadgeText: result.publicBadgeText,
          publicBadgeRevokedText: result.publicBadgeRevokedText,
          themeOrderId: result.themeOrderId,
          themeOrderNo: result.themeOrderNo,
          themeGrantKey: fixture.themeGrantKey,
          themeEntitlementCode: result.themeEntitlementCode,
          themeUseActionText: result.themeUseActionText,
          themeProfileClass: result.themeProfileClass,
          themeRevokedProfileClass: result.themeRevokedProfileClass,
          themeRevocationReason: result.themeRevocationReason,
          membershipOrderId: result.membershipOrderId,
          membershipOrderNo: result.membershipOrderNo,
          membershipGrantKey: fixture.membershipGrantKey,
          membershipPendingDuplicateOrderRejected: result.membershipPendingDuplicateOrderRejected,
          membershipDuplicatePaymentApiStatus: result.membershipDuplicatePaymentApiStatus,
          membershipDuplicatePaymentApiMessage: result.membershipDuplicatePaymentApiMessage,
          membershipEntitlementCode: result.membershipEntitlementCode,
          membershipExpiresAt: result.membershipExpiresAt,
          membershipRenewalExpiresAt: result.membershipRenewalExpiresAt,
          membershipRenewalExpiresAtAfterFirstRevoke: result.membershipRenewalExpiresAtAfterFirstRevoke,
          membershipRenewalBackgroundRetainedAfterFirstRevoke: result.membershipRenewalBackgroundRetainedAfterFirstRevoke,
          membershipRenewalBountyReleaseLedgerId: result.membershipRenewalBountyReleaseLedgerId,
          membershipRefundApiStatus: result.membershipRefundApiStatus,
          membershipRefundApiMessage: result.membershipRefundApiMessage,
          membershipRefundActionHidden: result.membershipRefundActionHidden,
          membershipUseActionText: result.membershipUseActionText,
          membershipBackgroundUrl: result.membershipBackgroundUrl,
          membershipProfileBackgroundStyle: result.membershipProfileBackgroundStyle,
          membershipRevokedCachedProfileBackgroundStyle: result.membershipRevokedCachedProfileBackgroundStyle,
          membershipRevokedDashboardProfileBackgroundStyle: result.membershipRevokedDashboardProfileBackgroundStyle,
          membershipRevokedProfileBackgroundStyle: result.membershipRevokedProfileBackgroundStyle,
          membershipRevokedHeaderProfileNickname: result.membershipRevokedHeaderProfileNickname,
          membershipAdminMuteBackgroundUrl: result.membershipAdminMuteBackgroundUrl,
          membershipAdminUnmuteBackgroundUrl: result.membershipAdminUnmuteBackgroundUrl,
          membershipText: result.membershipText,
          membershipRevocationReason: result.membershipRevocationReason,
          membershipRevokedBackgroundApiStatus: result.membershipRevokedBackgroundApiStatus,
          membershipRevokedBackgroundApiMessage: result.membershipRevokedBackgroundApiMessage,
          membershipRevokedApiStatus: result.membershipRevokedApiStatus,
          membershipRevokedApiMessage: result.membershipRevokedApiMessage,
          membershipRevokedDraftPublishApiStatus: result.membershipRevokedDraftPublishApiStatus,
          membershipRevokedDraftPublishApiMessage: result.membershipRevokedDraftPublishApiMessage,
          membershipRevokedEditApiStatus: result.membershipRevokedEditApiStatus,
          membershipRevokedEditApiMessage: result.membershipRevokedEditApiMessage,
          membershipRevokedUnacceptApiStatus: result.membershipRevokedUnacceptApiStatus,
          membershipRevokedUnacceptApiMessage: result.membershipRevokedUnacceptApiMessage,
          membershipRevokedText: result.membershipRevokedText,
          defaultQuestionTopicId: result.defaultQuestionTopicId,
          defaultQuestionAcceptedCommentId: result.defaultQuestionAcceptedCommentId,
          defaultQuestionReserveLedgerId: result.defaultQuestionReserveLedgerId,
          defaultQuestionAnswererLedgerId: result.defaultQuestionAnswererLedgerId,
          defaultQuestionReserveLedgerCount: result.defaultQuestionReserveLedgerCount,
          defaultQuestionBalanceBefore: result.defaultQuestionBalanceBefore,
          defaultQuestionBalanceAfterPublish: result.defaultQuestionBalanceAfterPublish,
          defaultQuestionBalanceAfterAccept: result.defaultQuestionBalanceAfterAccept,
          bountyDraftTopicId: result.bountyDraftTopicId,
          bountyDraftTopicTitle: result.bountyDraftTopicTitle,
          bountyTopicId: result.bountyTopicId,
          bountyTopicTitle: result.bountyTopicTitle,
          bountyAcceptedCommentId: result.bountyAcceptedCommentId,
          bountyAcceptedTopicStatus: result.bountyAcceptedTopicStatus,
          bountyAnswererId: fixture.answererAuth.user.id,
          bountyQuestionerLedgerId: result.bountyQuestionerLedgerId,
          bountyAnswererLedgerId: result.bountyAnswererLedgerId,
          bountyReversalLedgerId: result.bountyReversalLedgerId,
          bountyReacceptedAnswererLedgerId: result.bountyReacceptedAnswererLedgerId,
          bountyReleasedTopicId: result.bountyReleasedTopicId,
          bountyReleaseLedgerId: result.bountyReleaseLedgerId,
          bountyInsufficientCreditBalance: result.bountyInsufficientCreditBalance,
          bountyInsufficientCreditText: result.bountyInsufficientCreditText,
          bountyText: result.bountyText,
          attachmentBrowserE2E: result.attachmentEnabled,
          attachmentTopicId: result.attachmentTopicId,
          attachmentId: result.attachmentId,
          attachmentMembershipOrderId: result.attachmentMembershipOrderId,
          attachmentMembershipEntitlementId: result.attachmentMembershipEntitlementId,
          attachmentPriceCredits: result.attachmentPriceCredits,
          attachmentBuyerChargedCredits: result.attachmentBuyerChargedCredits,
          attachmentAuthorEarnedCredits: result.attachmentAuthorEarnedCredits,
          attachmentAuthorSaleLedgerId: result.attachmentAuthorSaleLedgerId,
          attachmentAuthorSaleTotal: result.attachmentAuthorSaleTotal,
          attachmentAuthorTotalEarnedCredits: result.attachmentAuthorTotalEarnedCredits,
          attachmentArchived: result.attachmentArchived,
          attachmentRevokedSaleApiStatus: result.attachmentRevokedSaleApiStatus,
          attachmentRevokedSaleText: result.attachmentRevokedSaleText,
          attachmentRevokedSaleBuyerBalanceUnchanged: result.attachmentRevokedSaleBuyerBalanceUnchanged,
          attachmentRevokedSaleAuthorBalanceUnchanged: result.attachmentRevokedSaleAuthorBalanceUnchanged,
          attachmentRevokedAttachmentArchived: result.attachmentRevokedAttachmentArchived,
          notificationTitles: result.notificationTitles
        },
        null,
        2
      )
    );
  } finally {
    await frontendServer?.stop();
  }
}

async function createCommercialFixture() {
  const stamp = Date.now();
  const admin = await apiRequest("/admin/auth/login", {
    method: "POST",
    body: {
      account: process.env.ADMIN_ACCOUNT || "admin",
      password: process.env.ADMIN_PASSWORD || "Admin123!"
    }
  });
  const adminToken = admin.access_token || admin.accessToken;
  if (!adminToken) {
    throw new Error("Admin login did not return access_token");
  }

  const slug = `e2e-${stamp}`;
  const sku = `E2E-${stamp}`;
  const couponCode = `E2E${stamp}`;
  const directCouponCode = `DIRECT${stamp}`;
  const zeroCreditCouponCode = `ZERO${stamp}`;
  const cancelCouponCode = `CANCEL${stamp}`;
  const productTitle = `E2E Browser Product ${stamp}`;
  const directCouponProductTitle = `E2E Direct Coupon Product ${stamp}`;
  const zeroCreditCouponProductTitle = `E2E Zero Credit Coupon Product ${stamp}`;
  const dashboardPayProductTitle = `E2E Dashboard Pay Product ${stamp}`;
  const orderHistoryProductTitle = `E2E Order History Product ${stamp}`;
  const entitlementHistoryGrantKey = `digital-history-${stamp}`;
  const entitlementHistoryProductTitle = `E2E Entitlement History Product ${stamp}`;
  const reviewHistoryGrantKey = `review-history-${stamp}`;
  const reviewHistoryProductTitle = `E2E Review History Product ${stamp}`;
  const insufficientCreditProductTitle = `E2E Insufficient Credit Product ${stamp}`;
  const cancelCouponProductTitle = `E2E Cancel Coupon Product ${stamp}`;
  const cartProductTitle = `E2E Cart Product ${stamp}`;
  const refundProductTitle = `E2E Refund Product ${stamp}`;
  const refundHistoryProductTitle = `E2E Refund History Product ${stamp}`;
  const rejectedRefundProductTitle = `E2E Rejected Refund Product ${stamp}`;
  const inactiveFavoriteProductTitle = `E2E Draft Favorite Product ${stamp}`;
  const digitalGrantKey = `badge-e2e-${stamp}`;
  const digitalProductTitle = `E2E Badge Entitlement ${stamp}`;
  const duplicateDigitalProductTitle = `E2E Badge Entitlement Duplicate ${stamp}`;
  const themeGrantKey = "theme-pro";
  const themeProductTitle = `E2E Theme Pro Entitlement ${stamp}`;
  const membershipGrantKey = `vip-e2e-${stamp}`;
  const membershipProductTitle = `E2E Membership Month ${stamp}`;

  const category = await apiRequest("/admin/mall/categories", {
    method: "POST",
    token: adminToken,
    body: {
      slug,
      name: `E2E Mall ${stamp}`,
      description: "Browser E2E mall category",
      status: 2,
      sort: 999
    }
  });

  const product = await apiRequest("/admin/mall/products", {
    method: "POST",
    token: adminToken,
    body: {
      sku,
      title: productTitle,
      description: "Browser E2E mall product",
      category: slug,
      cover_url: "",
      price_credits: CHECKOUT_PRICE,
      stock: 5,
      status: 2,
      sort: 9999
    }
  });

  const directCouponProduct = await apiRequest("/admin/mall/products", {
    method: "POST",
    token: adminToken,
    body: {
      sku: `${sku}-DIRECT`,
      title: directCouponProductTitle,
      description: "Browser E2E direct coupon product",
      category: "digital",
      cover_url: "",
      price_credits: CHECKOUT_PRICE,
      stock: 5,
      status: 2,
      sort: 9999
    }
  });

  const zeroCreditCouponProduct = await apiRequest("/admin/mall/products", {
    method: "POST",
    token: adminToken,
    body: {
      sku: `${sku}-ZERO-COUPON`,
      title: zeroCreditCouponProductTitle,
      description: "Browser E2E zero-credit coupon product",
      category: "digital",
      cover_url: "",
      price_credits: ZERO_CREDIT_CHECKOUT_PRICE,
      stock: 5,
      status: 2,
      sort: 9999
    }
  });

  const dashboardPayProduct = await apiRequest("/admin/mall/products", {
    method: "POST",
    token: adminToken,
    body: {
      sku: `${sku}-DASHBOARD-PAY`,
      title: dashboardPayProductTitle,
      description: "Browser E2E dashboard payment product",
      category: slug,
      cover_url: "",
      price_credits: CHECKOUT_PRICE,
      stock: 5,
      status: 2,
      sort: 9999
    }
  });

  const orderHistoryProduct = await apiRequest("/admin/mall/products", {
    method: "POST",
    token: adminToken,
    body: {
      sku: `${sku}-ORDER-HISTORY`,
      title: orderHistoryProductTitle,
      description: "Browser E2E order history pagination product",
      category: slug,
      cover_url: "",
      price_credits: 1,
      stock: ORDER_HISTORY_FIXTURE_ORDER_COUNT + 2,
      status: 2,
      sort: 9999
    }
  });

  const entitlementHistoryProduct = await apiRequest("/admin/mall/products", {
    method: "POST",
    token: adminToken,
    body: {
      sku: entitlementHistoryGrantKey,
      title: entitlementHistoryProductTitle,
      description: "Browser E2E entitlement history pagination product",
      category: "digital",
      cover_url: "",
      grant_type: "digital",
      grant_key: entitlementHistoryGrantKey,
      price_credits: 0,
      stock: ENTITLEMENT_HISTORY_FIXTURE_COUNT + 2,
      status: 2,
      sort: 9999
    }
  });

  const reviewHistoryProduct = await apiRequest("/admin/mall/products", {
    method: "POST",
    token: adminToken,
    body: {
      sku: reviewHistoryGrantKey,
      title: reviewHistoryProductTitle,
      description: "Browser E2E review history pagination product",
      category: "digital",
      cover_url: "",
      grant_type: "digital",
      grant_key: reviewHistoryGrantKey,
      price_credits: 0,
      stock: REVIEW_HISTORY_FIXTURE_COUNT + 2,
      status: 2,
      sort: 9999
    }
  });

  const insufficientCreditProduct = await apiRequest("/admin/mall/products", {
    method: "POST",
    token: adminToken,
    body: {
      sku: `${sku}-INSUFFICIENT-CREDIT`,
      title: insufficientCreditProductTitle,
      description: "Browser E2E insufficient credit recovery product",
      category: slug,
      cover_url: "",
      price_credits: INSUFFICIENT_CHECKOUT_PRICE,
      stock: 5,
      status: 2,
      sort: 9999
    }
  });

  const cancelCouponProduct = await apiRequest("/admin/mall/products", {
    method: "POST",
    token: adminToken,
    body: {
      sku: `${sku}-CANCEL-COUPON`,
      title: cancelCouponProductTitle,
      description: "Browser E2E coupon cancellation product",
      category: slug,
      cover_url: "",
      price_credits: CHECKOUT_PRICE,
      stock: 5,
      status: 2,
      sort: 9999
    }
  });

  const refundHistoryProduct = await apiRequest("/admin/mall/products", {
    method: "POST",
    token: adminToken,
    body: {
      sku: `${sku}-REFUND-HISTORY`,
      title: refundHistoryProductTitle,
      description: "Browser E2E refund history pagination product",
      category: slug,
      cover_url: "",
      price_credits: 0,
      stock: REFUND_HISTORY_FIXTURE_COUNT + 2,
      status: 2,
      sort: 9998
    }
  });

  const refundProduct = await apiRequest("/admin/mall/products", {
    method: "POST",
    token: adminToken,
    body: {
      sku: `${sku}-REFUND`,
      title: refundProductTitle,
      description: "Browser E2E mall refund product",
      category: slug,
      cover_url: "",
      price_credits: CHECKOUT_PRICE,
      stock: 5,
      status: 2,
      sort: 9998
    }
  });

  const cartProduct = await apiRequest("/admin/mall/products", {
    method: "POST",
    token: adminToken,
    body: {
      sku: `${sku}-CART`,
      title: cartProductTitle,
      description: "Browser E2E mall cart product",
      category: slug,
      cover_url: "",
      price_credits: CHECKOUT_PRICE,
      stock: 5,
      status: 2,
      sort: 9997
    }
  });

  const rejectedRefundProduct = await apiRequest("/admin/mall/products", {
    method: "POST",
    token: adminToken,
    body: {
      sku: `${sku}-REJECTED`,
      title: rejectedRefundProductTitle,
      description: "Browser E2E rejected mall refund product",
      category: slug,
      cover_url: "",
      price_credits: CHECKOUT_PRICE,
      stock: 5,
      status: 2,
      sort: 9996
    }
  });

  const inactiveFavoriteProduct = await apiRequest("/admin/mall/products", {
    method: "POST",
    token: adminToken,
    body: {
      sku: `${sku}-DRAFT-FAVORITE`,
      title: inactiveFavoriteProductTitle,
      description: "Browser E2E draft product favorite guard",
      category: slug,
      cover_url: "",
      price_credits: CHECKOUT_PRICE,
      stock: 5,
      status: 1,
      sort: 9995
    }
  });

  const digitalProduct = await apiRequest("/admin/mall/products", {
    method: "POST",
    token: adminToken,
    body: {
      sku: digitalGrantKey,
      title: digitalProductTitle,
      description: "Browser E2E digital badge entitlement",
      category: "badge",
      cover_url: "",
      grant_type: "badge",
      grant_key: digitalGrantKey,
      price_credits: CHECKOUT_PRICE,
      stock: 5,
      status: 2,
      sort: 9995
    }
  });

  const duplicateDigitalProduct = await apiRequest("/admin/mall/products", {
    method: "POST",
    token: adminToken,
    body: {
      sku: `${digitalGrantKey}-DUP`,
      title: duplicateDigitalProductTitle,
      description: "Browser E2E duplicate badge entitlement guard",
      category: "badge",
      cover_url: "",
      grant_type: "badge",
      grant_key: digitalGrantKey,
      price_credits: CHECKOUT_PRICE,
      stock: 5,
      status: 2,
      sort: 9995
    }
  });

  const membershipProduct = await apiRequest("/admin/mall/products", {
    method: "POST",
    token: adminToken,
    body: {
      sku: membershipGrantKey,
      title: membershipProductTitle,
      description: "Browser E2E membership month entitlement",
      category: "digital",
      cover_url: "",
      grant_type: "membership",
      grant_key: membershipGrantKey,
      price_credits: CHECKOUT_PRICE,
      stock: 5,
      status: 2,
      sort: 9994
    }
  });

  const themeProduct = await apiRequest("/admin/mall/products", {
    method: "POST",
    token: adminToken,
    body: {
      sku: `${sku}-THEME-PRO`,
      title: themeProductTitle,
      description: "Browser E2E profile theme entitlement",
      category: "digital",
      cover_url: "",
      grant_type: "theme",
      grant_key: themeGrantKey,
      price_credits: CHECKOUT_PRICE,
      stock: 5,
      status: 2,
      sort: 9993
    }
  });

  const coupon = await apiRequest("/admin/mall/coupons", {
    method: "POST",
    token: adminToken,
    body: {
      code: couponCode,
      name: `${couponCode} Browser E2E Coupon`,
      description: "Browser E2E coupon",
      discount_credits: COUPON_DISCOUNT,
      min_order_credits: CHECKOUT_PRICE,
      total_quota: 10,
      per_user_limit: 1,
      status: 2,
      starts_at: 0,
      ends_at: 0
    }
  });

  const directCoupon = await apiRequest("/admin/mall/coupons", {
    method: "POST",
    token: adminToken,
    body: {
      code: directCouponCode,
      name: `${directCouponCode} Direct Checkout Coupon`,
      description: "Browser E2E direct checkout coupon",
      discount_credits: COUPON_DISCOUNT,
      min_order_credits: CHECKOUT_PRICE,
      total_quota: 10,
      per_user_limit: 1,
      status: 2,
      starts_at: 0,
      ends_at: 0
    }
  });

  const zeroCreditCoupon = await apiRequest("/admin/mall/coupons", {
    method: "POST",
    token: adminToken,
    body: {
      code: zeroCreditCouponCode,
      name: `${zeroCreditCouponCode} Zero Credit Checkout Coupon`,
      description: "Browser E2E zero-credit checkout coupon",
      discount_credits: ZERO_CREDIT_CHECKOUT_PRICE,
      min_order_credits: ZERO_CREDIT_CHECKOUT_PRICE,
      total_quota: 1,
      per_user_limit: 1,
      status: 2,
      starts_at: 0,
      ends_at: 0
    }
  });

  const cancelCoupon = await apiRequest("/admin/mall/coupons", {
    method: "POST",
    token: adminToken,
    body: {
      code: cancelCouponCode,
      name: `${cancelCouponCode} Cancel Recovery Coupon`,
      description: "Browser E2E coupon cancellation recovery",
      discount_credits: COUPON_DISCOUNT,
      min_order_credits: CHECKOUT_PRICE,
      total_quota: 1,
      per_user_limit: 1,
      status: 2,
      starts_at: 0,
      ends_at: 0
    }
  });

  const password = `Passw0rd!${stamp}`;
  const registered = await apiRequest("/auth/register", {
    method: "POST",
    body: {
      username: `e2e${stamp}`,
      email: `e2e${stamp}@example.com`,
      password,
      nickname: `E2E ${stamp}`
    }
  });

  const auth = normalizeAuthResponse(registered);
  if (!auth.accessToken || !auth.user?.id) {
    throw new Error("User registration did not return auth payload");
  }

  const answererRegistered = await apiRequest("/auth/register", {
    method: "POST",
    body: {
      username: `e2eanswer${stamp}`,
      email: `e2eanswer${stamp}@example.com`,
      password,
      nickname: `E2E Answer ${stamp}`
    }
  });
  const answererAuth = normalizeAuthResponse(answererRegistered);
  if (!answererAuth.accessToken || !answererAuth.user?.id) {
    throw new Error("Answerer registration did not return auth payload");
  }

  await apiRequest(`/admin/credits/users/${encodeURIComponent(auth.user.id)}/adjust`, {
    method: "POST",
    token: adminToken,
    body: {
      delta: CREDIT_TOP_UP,
      reason: "browser_mall_topup",
      description: "Browser mall checkout top-up",
      source_event_id: `browser-mall-credit-${stamp}`
    }
  });

  const creditHistory = await createCreditHistoryFixture(auth, adminToken, stamp);
  const couponHistory = await createCouponHistoryFixture(auth, adminToken, stamp);
  const refundHistory = await createRefundHistoryFixture(auth, refundHistoryProduct.product, stamp);
  const reviewHistory = await createReviewHistoryFixture(auth, reviewHistoryProduct.product, stamp);
  const orderHistory = await createOrderHistoryFixture(auth, orderHistoryProduct.product, stamp);
  const entitlementHistory = await createEntitlementHistoryFixture(auth, entitlementHistoryProduct.product, entitlementHistoryGrantKey, stamp);

  return {
    auth,
    answererAuth,
    adminToken,
    creditHistory,
    category: category.category,
    product: product.product,
    directCouponProduct: directCouponProduct.product,
    zeroCreditCouponProduct: zeroCreditCouponProduct.product,
    dashboardPayProduct: dashboardPayProduct.product,
    orderHistoryProduct: orderHistoryProduct.product,
    orderHistory,
    entitlementHistoryProduct: entitlementHistoryProduct.product,
    entitlementHistory,
    couponHistory,
    refundHistoryProduct: refundHistoryProduct.product,
    refundHistory,
    reviewHistoryProduct: reviewHistoryProduct.product,
    reviewHistory,
    insufficientCreditProduct: insufficientCreditProduct.product,
    cancelCouponProduct: cancelCouponProduct.product,
    cartProduct: cartProduct.product,
    refundProduct: refundProduct.product,
    rejectedRefundProduct: rejectedRefundProduct.product,
    inactiveFavoriteProduct: inactiveFavoriteProduct.product,
    digitalProduct: digitalProduct.product,
    duplicateDigitalProduct: duplicateDigitalProduct.product,
    digitalGrantKey,
    themeProduct: themeProduct.product,
    themeGrantKey,
    membershipProduct: membershipProduct.product,
    membershipGrantKey,
    coupon: coupon.coupon,
    directCoupon: directCoupon.coupon,
    zeroCreditCoupon: zeroCreditCoupon.coupon,
    cancelCoupon: cancelCoupon.coupon,
    password
  };
}

async function createCreditHistoryFixture(auth, adminToken, stamp) {
  if (!auth?.accessToken || !auth?.user?.id || !adminToken) {
    throw new Error(`Credit history fixture is missing auth or admin token: ${JSON.stringify({ hasToken: Boolean(auth?.accessToken), userId: auth?.user?.id, hasAdminToken: Boolean(adminToken) })}`);
  }
  for (let index = 0; index < CREDIT_HISTORY_FIXTURE_COUNT; index += 1) {
    const suffix = String(index).padStart(2, "0");
    await apiRequest(`/admin/credits/users/${encodeURIComponent(auth.user.id)}/adjust`, {
      method: "POST",
      token: adminToken,
      body: {
        delta: 1,
        reason: `e2e_credit_history_${stamp}_${suffix}`,
        description: `Browser E2E credit history ${stamp}-${suffix}`,
        source_event_id: `e2e-credit-history-${stamp}-${suffix}`
      }
    });
  }
  return {
    count: CREDIT_HISTORY_FIXTURE_COUNT
  };
}

async function createCouponHistoryFixture(auth, adminToken, stamp) {
  if (!auth?.accessToken || !adminToken) {
    throw new Error(`Coupon history fixture is missing auth or admin token: ${JSON.stringify({ hasToken: Boolean(auth?.accessToken), hasAdminToken: Boolean(adminToken) })}`);
  }
  for (let index = 0; index < COUPON_HISTORY_FIXTURE_COUNT; index += 1) {
    const code = `HISTORY${stamp}${String(index).padStart(2, "0")}`;
    const created = await apiRequest("/admin/mall/coupons", {
      method: "POST",
      token: adminToken,
      body: {
        code,
        name: `E2E Coupon History ${stamp}-${index}`,
        description: "Browser E2E coupon history pagination coupon",
        discount_credits: 1,
        min_order_credits: 0,
        total_quota: 1,
        per_user_limit: 1,
        status: 2,
        starts_at: 0,
        ends_at: 0
      }
    });
    const coupon = created?.coupon || created;
    if (!coupon?.id) {
      throw new Error(`Coupon history fixture coupon ${index} did not return an id: ${JSON.stringify(created)}`);
    }
    const claimed = await apiRequest(`/mall/coupons/${encodeURIComponent(coupon.id)}/claim`, {
      method: "POST",
      token: auth.accessToken
    });
    const usage = claimed?.usage || claimed;
    if (couponUsageStatusValue(usage?.status ?? usage?.Status) !== 4) {
      throw new Error(`Coupon history fixture usage ${coupon.id} status = ${usage?.status ?? usage?.Status ?? "unknown"}, want claimed`);
    }
  }
  return {
    count: COUPON_HISTORY_FIXTURE_COUNT
  };
}

async function createRefundHistoryFixture(auth, product, stamp) {
  if (!auth?.accessToken || !product?.id) {
    throw new Error(`Refund history fixture is missing auth or product: ${JSON.stringify({ hasToken: Boolean(auth?.accessToken), productId: product?.id })}`);
  }
  for (let index = 0; index < REFUND_HISTORY_FIXTURE_COUNT; index += 1) {
    const created = await apiRequest("/mall/orders", {
      method: "POST",
      token: auth.accessToken,
      body: {
        idempotency_key: `e2e-refund-history-order-${stamp}-${index}`,
        receiver: "E2E Refund History",
        phone: "13800000000",
        address: "Shanghai Pudong Refund History Road 1",
        items: [{ product_id: product.id, quantity: 1 }]
      }
    });
    const order = created?.order || created;
    if (!order?.id) {
      throw new Error(`Refund history fixture order ${index} did not return an id: ${JSON.stringify(created)}`);
    }
    const paid = await apiRequest(`/mall/orders/${encodeURIComponent(order.id)}/pay`, {
      method: "POST",
      token: auth.accessToken,
      body: {
        payment_method: "credits",
        idempotency_key: `e2e-refund-history-pay-${stamp}-${index}`
      }
    });
    const paidOrder = paid?.order || paid;
    if (mallOrderStatusValue(paidOrder?.status) !== 3) {
      throw new Error(`Refund history fixture order ${order.id} status = ${paidOrder?.status ?? "unknown"}, want paid`);
    }
    const note = `E2E refund history ${stamp}-${String(index).padStart(2, "0")}`;
    const requested = await apiRequest(`/mall/orders/${encodeURIComponent(order.id)}/refunds`, {
      method: "POST",
      token: auth.accessToken,
      body: {
        reason: "other",
        note
      }
    });
    const refund = requested?.refund || requested;
    if (!refund?.id || refundStatusValue(refund.status ?? refund.Status) !== 1) {
      throw new Error(`Refund history fixture request for order ${order.id} did not return requested status: ${JSON.stringify(requested)}`);
    }
  }
  return {
    count: REFUND_HISTORY_FIXTURE_COUNT
  };
}

async function createReviewHistoryFixture(auth, product, stamp) {
  if (!auth?.accessToken || !product?.id) {
    throw new Error(`Review history fixture is missing auth or product: ${JSON.stringify({ hasToken: Boolean(auth?.accessToken), productId: product?.id })}`);
  }
  for (let index = 0; index < REVIEW_HISTORY_FIXTURE_COUNT; index += 1) {
    const created = await apiRequest("/mall/orders", {
      method: "POST",
      token: auth.accessToken,
      body: {
        idempotency_key: `e2e-review-history-order-${stamp}-${index}`,
        items: [{ product_id: product.id, quantity: 1 }]
      }
    });
    const order = created?.order || created;
    if (!order?.id) {
      throw new Error(`Review history fixture order ${index} did not return an id: ${JSON.stringify(created)}`);
    }
    const paid = await apiRequest(`/mall/orders/${encodeURIComponent(order.id)}/pay`, {
      method: "POST",
      token: auth.accessToken,
      body: {
        payment_method: "credits",
        idempotency_key: `e2e-review-history-pay-${stamp}-${index}`
      }
    });
    const paidOrder = paid?.order || paid;
    if (mallOrderStatusValue(paidOrder?.status) !== 6) {
      throw new Error(`Review history fixture order ${order.id} status = ${paidOrder?.status ?? "unknown"}, want completed`);
    }
    const content = `E2E review history ${stamp}-${String(index).padStart(2, "0")}`;
    const submitted = await apiRequest(`/mall/products/${encodeURIComponent(product.id)}/reviews`, {
      method: "POST",
      token: auth.accessToken,
      body: {
        order_id: order.id,
        rating: 5,
        content
      }
    });
    const review = submitted?.review || submitted;
    if (!review?.id || Number(review.status ?? review.Status) !== 1) {
      throw new Error(`Review history fixture review for order ${order.id} did not return pending status: ${JSON.stringify(submitted)}`);
    }
  }
  return {
    count: REVIEW_HISTORY_FIXTURE_COUNT
  };
}

async function createOrderHistoryFixture(auth, product, stamp) {
  if (!auth?.accessToken || !product?.id) {
    throw new Error(`Order history fixture is missing auth or product: ${JSON.stringify({ hasToken: Boolean(auth?.accessToken), productId: product?.id })}`);
  }
  for (let index = 0; index < ORDER_HISTORY_FIXTURE_ORDER_COUNT; index += 1) {
    const data = await apiRequest("/mall/orders", {
      method: "POST",
      token: auth.accessToken,
      body: {
        idempotency_key: `e2e-order-history-${stamp}-${index}`,
        receiver: "E2E Order History",
        phone: "13800000000",
        address: "Shanghai Pudong Order History Road 1",
        items: [{ product_id: product.id, quantity: 1 }]
      }
    });
    const order = data?.order || data;
    if (!order?.id) {
      throw new Error(`Order history fixture order ${index} did not return an id: ${JSON.stringify(data)}`);
    }
  }
  return {
    count: ORDER_HISTORY_FIXTURE_ORDER_COUNT
  };
}

async function createEntitlementHistoryFixture(auth, product, grantKey, stamp) {
  if (!auth?.accessToken || !product?.id || !grantKey) {
    throw new Error(`Entitlement history fixture is missing auth, product or grant key: ${JSON.stringify({ hasToken: Boolean(auth?.accessToken), productId: product?.id, grantKey })}`);
  }
  const fixture = { auth };
  for (let index = 0; index < ENTITLEMENT_HISTORY_FIXTURE_COUNT; index += 1) {
    const created = await apiRequest("/mall/orders", {
      method: "POST",
      token: auth.accessToken,
      body: {
        idempotency_key: `e2e-entitlement-history-order-${stamp}-${index}`,
        items: [{ product_id: product.id, quantity: 1 }]
      }
    });
    const order = created?.order || created;
    if (!order?.id) {
      throw new Error(`Entitlement history fixture order ${index} did not return an id: ${JSON.stringify(created)}`);
    }
    const paid = await apiRequest(`/mall/orders/${encodeURIComponent(order.id)}/pay`, {
      method: "POST",
      token: auth.accessToken,
      body: {
        payment_method: "credits",
        idempotency_key: `e2e-entitlement-history-pay-${stamp}-${index}`
      }
    });
    const paidOrder = paid?.order || paid;
    if (mallOrderStatusValue(paidOrder?.status) !== 6) {
      throw new Error(`Entitlement history fixture order ${order.id} status = ${paidOrder?.status ?? "unknown"}, want completed`);
    }
    await waitForDigitalEntitlement(fixture, order.id, product.id, grantKey, "ACTIVE");
  }
  return {
    count: ENTITLEMENT_HISTORY_FIXTURE_COUNT
  };
}

async function assertInactiveProductFavoriteRejected(fixture) {
  const productId = fixture.inactiveFavoriteProduct?.id;
  if (!productId) {
    throw new Error(`Draft favorite product fixture missing id: ${JSON.stringify(fixture.inactiveFavoriteProduct)}`);
  }
  const failure = await apiRequestFailure(`/mall/products/${encodeURIComponent(productId)}/favorite`, {
    method: "POST",
    token: fixture.auth.accessToken,
    expectedStatus: 412,
    label: "draft product favorite"
  });
  const combined = `${failure.message} ${failure.rawBody}`.toLowerCase();
  if (!combined.includes("product unavailable")) {
    throw new Error(`Draft product favorite did not return product unavailable: ${failure.rawBody.slice(0, 800)}`);
  }
  return {
    status: failure.status,
    message: failure.message || "product unavailable"
  };
}

async function assertDuplicateDigitalGrantCartRejected(fixture) {
  const firstProductId = fixture.digitalProduct?.id;
  const duplicateProductId = fixture.duplicateDigitalProduct?.id;
  if (!firstProductId || !duplicateProductId) {
    throw new Error(`Duplicate digital cart fixture missing product ids: ${JSON.stringify({ firstProductId, duplicateProductId })}`);
  }
  const token = fixture.auth.accessToken;
  try {
    await apiRequest(`/mall/cart/items/${encodeURIComponent(firstProductId)}`, {
      method: "PUT",
      token,
      body: { quantity: 1 }
    });
    const failure = await apiRequestFailure(`/mall/cart/items/${encodeURIComponent(duplicateProductId)}`, {
      method: "PUT",
      token,
      body: { quantity: 1 },
      expectedStatus: 412,
      label: "duplicate badge grant cart item"
    });
    const combined = `${failure.message} ${failure.rawBody}`.toLowerCase();
    if (!combined.includes("duplicate badge grant")) {
      throw new Error(`Duplicate badge grant cart write did not return duplicate grant error: ${failure.rawBody.slice(0, 800)}`);
    }
    return {
      status: failure.status,
      message: failure.message || "duplicate badge grant in order"
    };
  } finally {
    await Promise.allSettled([
      apiRequest(`/mall/cart/items/${encodeURIComponent(firstProductId)}`, { method: "DELETE", token }),
      apiRequest(`/mall/cart/items/${encodeURIComponent(duplicateProductId)}`, { method: "DELETE", token })
    ]);
  }
}

async function assertCartCheckoutReplayRequestLocked(fixture) {
  const productId = fixture.cartProduct?.id;
  if (!productId) {
    throw new Error(`Cart replay product fixture missing id: ${JSON.stringify(fixture.cartProduct)}`);
  }
  const token = fixture.auth.accessToken;
  const idempotencyKey = `cart-replay-${Date.now()}`;
  const request = {
    idempotency_key: idempotencyKey,
    receiver: "E2E Replay",
    phone: "13800000000",
    address: "Shanghai Zhangjiang Road 1"
  };
  let orderId = "";
  try {
    await apiRequest("/mall/cart", { method: "DELETE", token });
    await apiRequest(`/mall/cart/items/${encodeURIComponent(productId)}`, {
      method: "PUT",
      token,
      body: { quantity: 1 }
    });
    const created = await apiRequest("/mall/cart/checkout", { method: "POST", token, body: request });
    const order = created?.order || created;
    orderId = String(order?.id || "");
    if (!orderId) {
      throw new Error(`Cart replay checkout did not return order.id: ${JSON.stringify(created)}`);
    }

    const duplicate = await apiRequest("/mall/cart/checkout", { method: "POST", token, body: request });
    const duplicateOrder = duplicate?.order || duplicate;
    const duplicateOrderId = String(duplicateOrder?.id || "");
    if (!duplicate?.duplicate || duplicateOrderId !== orderId) {
      throw new Error(`Matching cart replay did not return the original order: ${JSON.stringify(duplicate)}`);
    }

    const conflict = await apiRequestFailure("/mall/cart/checkout", {
      method: "POST",
      token,
      body: { ...request, address: "Shanghai Zhangjiang Road 2" },
      expectedStatus: 409,
      label: "cart checkout idempotency replay conflict"
    });
    const combined = `${conflict.message} ${conflict.rawBody}`.toLowerCase();
    if (!combined.includes("duplicate reference")) {
      throw new Error(`Cart replay conflict did not return duplicate reference: ${conflict.rawBody.slice(0, 800)}`);
    }
    const orderDetail = await apiRequest(`/mall/orders/${encodeURIComponent(orderId)}`, { token });
    const persistedOrder = orderDetail?.order || orderDetail;
    if (String(persistedOrder?.address || "") !== request.address) {
      throw new Error(`Cart replay changed persisted order address: ${JSON.stringify(persistedOrder)}`);
    }
    return {
      orderId,
      duplicateOrderId,
      conflictStatus: conflict.status,
      conflictMessage: conflict.message || "duplicate reference"
    };
  } finally {
    if (orderId) {
      await apiRequest(`/mall/orders/${encodeURIComponent(orderId)}/cancel`, { method: "POST", token }).catch(() => {});
    }
    await apiRequest("/mall/cart", { method: "DELETE", token }).catch(() => {});
  }
}

async function runBrowserCheckout(chromePath, fixture) {
  const port = await getFreePort();
  const userDataDir = await mkdtemp(path.join(os.tmpdir(), "bbs-mall-e2e-"));
  const chrome = spawn(
    chromePath,
    [
      "--headless=new",
      "--disable-gpu",
      "--no-first-run",
      "--no-default-browser-check",
      "--disable-background-networking",
      "--disable-extensions",
      "--disable-sync",
      "--no-sandbox",
      `--remote-debugging-port=${port}`,
      `--user-data-dir=${userDataDir}`,
      "about:blank"
    ],
    { stdio: ["ignore", "pipe", "pipe"], windowsHide: true }
  );

  const stderr = [];
  chrome.stderr?.on("data", (chunk) => stderr.push(String(chunk)));

  let page;
  try {
    const browserWs = await waitForBrowserWebSocket(port, chrome, stderr);
    const pageWs = await createPageWebSocket(port, browserWs);
    page = new CDPClient(pageWs);
    await page.connect();
    const expectedBrowserIssues = [];
    const issues = collectBrowserIssues(page, expectedBrowserIssues);
    await page.send("Page.enable");
    await page.send("DOM.enable");
    await page.send("Runtime.enable");
    await page.send("Network.enable");
    await page.send("Log.enable");
    await page.send("Page.addScriptToEvaluateOnNewDocument", {
      source: `if (!window.localStorage.getItem(${JSON.stringify(AUTH_STORAGE_KEY)})) { window.localStorage.setItem(${JSON.stringify(AUTH_STORAGE_KEY)}, ${JSON.stringify(JSON.stringify(fixture.auth))}); }`
    });

    const campaignUrl = `${FRONTEND_BASE}/shop?category=${encodeURIComponent(fixture.category.slug)}&keyword=${encodeURIComponent("Browser Product")}`;
    await navigate(page, campaignUrl);
    await waitForText(page, fixture.product.title, "campaign filtered product");

    const defaultQuestionResult = await runBrowserDefaultQuestionRewardFlow(page, fixture);

    const shopUrl = `${FRONTEND_BASE}/shop?product_id=${encodeURIComponent(fixture.product.id)}&coupon_code=${encodeURIComponent(fixture.coupon.code)}`;
    await navigate(page, shopUrl);
    await waitForText(page, fixture.product.title, "product detail");
    await waitForText(page, "商品详情", "product detail panel");
    await clickButton(page, "^收藏商品$");
    await waitForText(page, "商品已收藏", "product favorited");
    await clickButtonInArticle(page, fixture.product.title, "^查看详情$");
    await waitForText(page, "商品详情", "favorite product detail target");
    await waitForText(page, fixture.product.title, "favorite product detail target title");
    await clickButton(page, "^取消收藏$");
    await waitForText(page, "已取消收藏", "product unfavorited");

    const directCouponResult = await runBrowserDirectCouponCheckout(page, fixture);
    const zeroCreditCouponResult = await runBrowserZeroCreditCouponCheckout(page, fixture);
    const dashboardPayResult = await runBrowserDashboardPaymentFlow(page, fixture);
    const payingOrderResumeResult = await runBrowserPayingOrderResumeFlow(page, fixture);
    const insufficientPaymentResult = await runBrowserInsufficientCreditRecoveryFlow(page, fixture, expectedBrowserIssues);
    const cancelCouponResult = await runBrowserCouponCancellationFlow(page, fixture);
    const archivedCouponResult = await runBrowserCouponArchiveFlow(page, fixture);
    await navigate(page, shopUrl);
    await waitForText(page, fixture.product.title, "product detail after direct coupon checkout");

    await clickButtonInArticle(page, fixture.coupon.code, "^领取$");
    await waitForText(page, "优惠券已领取|已经在你的券包里", "coupon claimed");

    await navigate(page, `${FRONTEND_BASE}/dashboard/coupons`);
    await waitForText(page, "优惠券|个人工作台", "dashboard coupons panel");
    await waitForText(page, fixture.coupon.code, "claimed coupon in dashboard");
    await clickButtonInArticle(page, fixture.coupon.code, "^去使用$");
    await waitForText(page, "优惠券使用引导", "coupon usage guide");
    await waitForText(page, "带券兑换", "coupon usage guide action");
    await clickButton(page, "^带券兑换$");
    await waitForText(page, "确认兑换", "coupon guide checkout preview");
    await waitForText(page, `已预估优惠 ${COUPON_DISCOUNT} 积分`, "coupon guide checkout discount preview");
    await clickButton(page, "^取消$");

    await fillByLabel(page, "收件人", "浏览器联调");
    await fillByLabel(page, "联系电话", "13800000000");
    await fillByLabel(page, "省份", "上海");
    await fillByLabel(page, "城市", "上海");
    await fillByLabel(page, "区县", "浦东新区");
    await fillByLabel(page, "详细地址", "张江路 1 号");
    await clickButton(page, "保存为收货地址|保存修改");
    await waitForText(page, "收货地址已保存|收货地址已更新", "address saved");

    const updatedAddressDetail = "张江路 2 号";
    await navigate(page, `${FRONTEND_BASE}/dashboard/addresses`);
    await waitForText(page, "新增收货地址|编辑收货地址", "dashboard address book");
    await waitForText(page, "浏览器联调", "dashboard saved address receiver");
    await waitForText(page, "默认地址|默认", "dashboard default address");
    await clickButtonInArticle(page, "浏览器联调", "^编辑$");
    await waitForText(page, "编辑收货地址", "dashboard address edit form");
    await fillByLabel(page, "详细地址", updatedAddressDetail);
    await clickButton(page, "^保存修改$");
    await waitForText(page, "收货地址已更新", "dashboard address updated");
    await waitForText(page, updatedAddressDetail, "dashboard updated address detail");

    const temporaryAddressReceiver = "浏览器联调备用";
    const temporaryAddressDetail = "金科路 9 号";
    await fillByLabel(page, "收件人", temporaryAddressReceiver);
    await fillByLabel(page, "联系电话", "13900000000");
    await fillByLabel(page, "省份", "上海");
    await fillByLabel(page, "城市", "上海");
    await fillByLabel(page, "区县", "浦东新区");
    await fillByLabel(page, "详细地址", temporaryAddressDetail);
    await clickButton(page, "^保存地址$");
    await waitForText(page, "收货地址已新增", "dashboard secondary address created");
    await waitForText(page, temporaryAddressDetail, "dashboard secondary address detail");
    await clickButtonInArticle(page, temporaryAddressReceiver, "^设默认$");
    await waitForText(page, "默认收货地址已更新", "dashboard secondary address set default");
    await waitForText(page, `${temporaryAddressReceiver} · 默认地址`, "dashboard secondary address default");
    await clickButtonInArticle(page, temporaryAddressReceiver, "^删除$");
    await waitForText(page, "收货地址已删除", "dashboard secondary address deleted");
    await waitForText(page, "浏览器联调 · 默认地址", "dashboard primary address promoted default");
    await waitForAddressDeleted(page, temporaryAddressReceiver, "dashboard secondary address row removed");
    const addressesAfterDelete = listItems(await apiRequest("/mall/addresses?limit=20&offset=0", { token: fixture.auth.accessToken }));
    if (addressesAfterDelete.some((address) => String(address?.receiver || "") === temporaryAddressReceiver)) {
      throw new Error(`Deleted secondary address still returned by API: ${JSON.stringify(addressesAfterDelete)}`);
    }
    const promotedAddress = addressesAfterDelete.find((address) => String(address?.receiver || "") === "浏览器联调");
    if (!promotedAddress || !(promotedAddress.is_default || promotedAddress.isDefault) || String(promotedAddress.detail || "") !== updatedAddressDetail) {
      throw new Error(`Primary address was not promoted after deleting default address: ${JSON.stringify(addressesAfterDelete)}`);
    }
    const promotedAddressText = `${promotedAddress.receiver} · ${promotedAddress.detail}`;

    await navigate(page, shopUrl);
    await waitForText(page, fixture.product.title, "product detail after address edit");
    await waitForText(page, updatedAddressDetail, "updated default address in shop");

    await clickButton(page, "^立即兑换$");
    await waitForText(page, "确认兑换", "checkout panel");
    await waitForText(page, `已预估优惠 ${COUPON_DISCOUNT} 积分`, "coupon applied");
    await clickButton(page, "^确认兑换$");
    await waitForText(page, "兑换成功|订单已创建", "order paid");

    const paidText = await bodyText(page);
    const order = await latestMallOrderForProduct(fixture, fixture.product.id);
    if (!order?.id) {
      throw new Error("Paid mall order was not returned by user order API");
    }
    const orderNo = order.order_no || order.orderNo || String(order.id);

    await clickButton(page, "查看订单");
    await waitForText(page, "个人工作台", "dashboard shell");
    await waitForText(page, "已支付", "paid order row");
    await waitForText(page, fixture.product.title, "order item title");
    await waitForText(page, "支付记录|支付成功", "payment evidence");
    await waitForText(page, "再次兑换", "repeat purchase action");
    await clickButton(page, "^再次兑换$");
    await waitForText(page, "商品详情", "repeat purchase product detail");
    await waitForText(page, fixture.product.title, "repeat purchase product title");

    await shipMallOrder(fixture, order.id);
    await navigate(page, `${FRONTEND_BASE}/dashboard/orders?order_id=${encodeURIComponent(order.id)}`);
    await waitForText(page, "已发货", "shipped order row");
    await waitForText(page, orderNo, "shipped order number");
    await waitForText(page, "E2E Express|E2E", "shipping evidence");
    await clickButtonInArticle(page, orderNo, "^确认收货$", "shipped order confirmation action");
    await waitForText(page, "已确认收货，订单已完成。|已完成", "order completed");

    const notifications = await waitForMallOrderNotifications(fixture, order.id, ["订单已支付", "订单已发货", "订单已完成"]);
    const notificationTitles = notifications.map((item) => item.title || item.type || "").filter(Boolean);
    await navigate(page, `${FRONTEND_BASE}/dashboard/messages`);
    await waitForText(page, "站内通知|站内消息", "messages panel");
    await waitForText(page, order.order_no || order.orderNo || String(order.id), "mall order notification content");
    await waitForText(page, "订单已支付|订单已发货|订单已完成", "mall order notification titles");
    await clickButtonInArticle(page, order.order_no || order.orderNo || String(order.id), "查看订单");
    await waitForText(page, "订单详情", "notification order target detail");
    await waitForText(page, "查看商品", "order detail product target action");
    await waitForText(page, "评价商品", "review action");

    const fulfillmentText = await bodyText(page);
    await clickButton(page, "^评价商品$");
    await waitForText(page, "商品评价", "product review panel");
    await waitForText(page, "发布评价", "review form");
    const reviewContent = `浏览器联调评价 ${Date.now()}：兑换、履约和确认收货链路顺畅。`;
    await fillByLabel(page, "评价内容", reviewContent);
    await waitForButtonEnabled(page, "^发布评价$", "review submit enabled");
    await clickButton(page, "^发布评价$");
    await waitForText(page, "评价已提交", "review submitted");

    const createdReview = await latestProductReviewForOrder(fixture, order.id, fixture.product.id);
    await publishMallReview(fixture, createdReview.id);
    const reviewNotifications = await waitForMallReviewNotifications(fixture, createdReview.id, ["商品评价已展示"]);
    const reviewNotificationTitles = reviewNotifications.map((item) => item.title || item.type || "").filter(Boolean);
    await navigate(page, `${FRONTEND_BASE}/dashboard/messages`);
    await waitForText(page, "站内通知|站内消息", "messages panel after review publish");
    await waitForText(page, "商品评价已展示", "product review notification title");
    await clickButtonInArticle(page, "商品评价已展示", "查看评价");
    await waitForText(page, "个人工作台", "dashboard shell after review notification");
    await waitForText(page, "个人列表|评价", "review notification target");
    await waitForText(page, "当前定位", "focused review marker");
    await waitForText(page, reviewContent, "focused review content");

    const reviewText = await bodyText(page);
    await navigate(page, `${FRONTEND_BASE}/shop?product_id=${encodeURIComponent(fixture.product.id)}`);
    await waitForText(page, "商品详情", "review product detail after publish");
    await waitForText(page, fixture.product.title, "review product title after publish");
    const publicReviewText = await waitForPublicProductReview(page, reviewContent, "published review in public product reviews");
    const duplicateReview = await assertDuplicateProductReviewRejected(fixture, order.id, fixture.product.id, reviewContent);
    await hideMallReview(fixture, createdReview.id);
    const hiddenReviewNotifications = await waitForMallReviewNotifications(fixture, createdReview.id, ["商品评价未展示"]);
    const reviewHiddenNotificationTitles = hiddenReviewNotifications.map((item) => item.title || item.type || "").filter(Boolean);
    await navigate(page, `${FRONTEND_BASE}/dashboard/messages`);
    await waitForText(page, "商品评价未展示", "product review hidden notification title");
    await clickButtonInArticle(page, "商品评价未展示", "查看评价");
    await waitForText(page, "个人列表|评价", "hidden review notification target");
    await waitForText(page, "当前定位", "hidden review focused marker");
    await waitForText(page, "已隐藏", "hidden review status badge");
    await waitForText(page, reviewContent, "hidden review still visible to owner");
    await navigate(page, `${FRONTEND_BASE}/shop?product_id=${encodeURIComponent(fixture.product.id)}&review_hidden=${Date.now()}`);
    await waitForText(page, "商品详情", "review product detail after hide");
    await waitForText(page, fixture.product.title, "review product title after hide");
    const publicReviewHiddenText = await waitForPublicProductReviewHidden(page, reviewContent, "hidden review removed from public product reviews");
    const cartResult = await runBrowserCartCheckout(page, fixture);
    const refundResult = await runBrowserRefundFlow(page, fixture);
    const rejectedRefundResult = await runBrowserRejectedRefundFlow(page, fixture);
    const digitalResult = await runBrowserDigitalEntitlementFlow(page, fixture, expectedBrowserIssues);
    const refundHistoryPaginationResult = await runBrowserRefundHistoryPaginationFlow(page, fixture);
    const reviewHistoryPaginationResult = await runBrowserReviewHistoryPaginationFlow(page, fixture);
    const themeResult = await runBrowserThemeEntitlementFlow(page, fixture);
    const membershipResult = await runBrowserMembershipBountyFlow(page, fixture, expectedBrowserIssues);
    const couponHistoryPaginationResult = await runBrowserCouponHistoryPaginationFlow(page, fixture);
    const orderHistoryPaginationResult = await runBrowserOrderHistoryPaginationFlow(page, fixture);
    const entitlementHistoryPaginationResult = await runBrowserEntitlementHistoryPaginationFlow(page, fixture);
    const creditHistoryPaginationResult = await runBrowserCreditHistoryPaginationFlow(page, fixture);
    const addressHistoryPaginationResult = await runBrowserAddressHistoryPaginationFlow(page, fixture);
    const attachmentMembership = truthyEnv("MALL_E2E_ATTACHMENTS") ? await activateMembershipForAttachment(fixture) : null;
    const attachmentResult = truthyEnv("MALL_E2E_ATTACHMENTS")
      ? await runBrowserAttachmentFlow(page, fixture, membershipResult.topicId, userDataDir)
      : { enabled: false };
    let attachmentRevokedSaleResult = { tested: false };
    if (attachmentMembership) {
      const revoked = await revokeMallDigitalEntitlement(
        fixture,
        attachmentMembership.entitlementId,
        `Browser E2E attachment membership revoke ${Date.now()}`
      );
      if (String(revoked?.status || "").toUpperCase() !== "REVOKED") {
        throw new Error(`Attachment membership revoke status = ${revoked?.status ?? "unknown"}, want REVOKED`);
      }
      attachmentRevokedSaleResult = await runBrowserRevokedAttachmentSaleFlow(page, fixture, attachmentResult, expectedBrowserIssues);
    }
    const checkInResult = await runBrowserCheckInFlow(page, fixture);
    const seriousIssues = issues.filter(isSeriousBrowserIssue);
    if (seriousIssues.length > 0) {
      throw new Error(`Browser reported ${seriousIssues.length} serious issue(s): ${JSON.stringify(seriousIssues.slice(0, 5), null, 2)}`);
    }
    return {
      orderId: String(order.id),
      orderNo: order.order_no || order.orderNo || "",
      orderHistoryInitialTotal: orderHistoryPaginationResult.initialTotal,
      orderHistoryLoadedOrderNo: orderHistoryPaginationResult.loadedOrderNo,
      orderHistorySelectedOrderNo: orderHistoryPaginationResult.selectedOrderNo,
      entitlementHistoryInitialTotal: entitlementHistoryPaginationResult.initialTotal,
      entitlementHistoryLoadedCode: entitlementHistoryPaginationResult.loadedCode,
      couponHistoryInitialTotal: couponHistoryPaginationResult.initialTotal,
      couponHistoryLoadedCode: couponHistoryPaginationResult.loadedCode,
      creditHistoryInitialTotal: creditHistoryPaginationResult.initialTotal,
      creditHistoryLoadedReason: creditHistoryPaginationResult.loadedReason,
      addressHistoryFixtureCount: addressHistoryPaginationResult.fixtureCount,
      addressHistoryInitialTotal: addressHistoryPaginationResult.initialTotal,
      addressHistoryLoadedDetail: addressHistoryPaginationResult.loadedDetail,
      directCouponOrderId: directCouponResult.orderId,
      directCouponText: directCouponResult.text,
      directCouponReuseHttpStatus: directCouponResult.reuseStatus,
      directCouponReuseText: directCouponResult.reuseText,
      zeroCreditCouponOrderId: zeroCreditCouponResult.orderId,
      zeroCreditCouponPaymentId: zeroCreditCouponResult.paymentId,
      zeroCreditCouponUsageId: zeroCreditCouponResult.usageId,
      zeroCreditCouponBalanceBefore: zeroCreditCouponResult.balanceBefore,
      zeroCreditCouponBalanceAfter: zeroCreditCouponResult.balanceAfter,
      zeroCreditCouponLockedStock: zeroCreditCouponResult.lockedStock,
      zeroCreditCouponRefundId: zeroCreditCouponResult.refundId,
      zeroCreditCouponRefundBalanceBefore: zeroCreditCouponResult.refundBalanceBefore,
      zeroCreditCouponRefundBalanceAfter: zeroCreditCouponResult.refundBalanceAfter,
      zeroCreditCouponRefundRestoredStock: zeroCreditCouponResult.refundRestoredStock,
      zeroCreditCouponRefundNotificationTitles: zeroCreditCouponResult.refundNotificationTitles,
      dashboardPayOrderId: dashboardPayResult.orderId,
      dashboardPayText: dashboardPayResult.text,
      dashboardPayLockedStock: dashboardPayResult.lockedStock,
      dashboardPayNotificationTitles: dashboardPayResult.notificationTitles,
      payingOrderResumeOrderId: payingOrderResumeResult.orderId,
      payingOrderResumePaymentKey: payingOrderResumeResult.paymentKey,
      payingOrderResumeLedgerCount: payingOrderResumeResult.ledgerCount,
      insufficientPaymentOrderId: insufficientPaymentResult.orderId,
      insufficientPaymentText: insufficientPaymentResult.text,
      insufficientPaymentLockedStock: insufficientPaymentResult.lockedStock,
      insufficientPaymentBalanceBeforeFailure: insufficientPaymentResult.balanceBeforeFailure,
      insufficientPaymentBalanceAfterTopUp: insufficientPaymentResult.balanceAfterTopUp,
      insufficientPaymentFailedPaymentId: insufficientPaymentResult.failedPaymentId,
      insufficientPaymentRecoveredPaymentId: insufficientPaymentResult.recoveredPaymentId,
      insufficientPaymentNotificationTitles: insufficientPaymentResult.notificationTitles,
      cancelCouponOrderId: cancelCouponResult.orderId,
      cancelCouponText: cancelCouponResult.text,
      cancelCouponLockedStock: cancelCouponResult.lockedStock,
      cancelCouponRestoredStock: cancelCouponResult.restoredStock,
      cancelCouponReleasedUsageId: cancelCouponResult.releasedUsageId,
      cancelCouponReclaimedUsageId: cancelCouponResult.reclaimedUsageId,
      archivedCouponStatus: archivedCouponResult.status,
      archivedCouponRejectStatus: archivedCouponResult.rejectStatus,
      archivedCouponRejectText: archivedCouponResult.rejectText,
      paidText: summarizeCheckoutText(paidText),
      fulfillmentText: summarizeOrderLifecycleText(fulfillmentText),
      promotedAddressText,
      reviewText: summarizeReviewText(reviewText),
      reviewHistoryInitialTotal: reviewHistoryPaginationResult.initialTotal,
      reviewHistoryLoadedContent: reviewHistoryPaginationResult.loadedContent,
      publicReviewText: summarizePublicProductReviewText(publicReviewText, reviewContent),
      reviewDuplicateHttpStatus: duplicateReview.status,
      reviewDuplicateLegacyCode: duplicateReview.legacyCode,
      reviewDuplicateText: duplicateReview.frontendText,
      reviewHiddenNotificationTitles,
      publicReviewHiddenText,
      reviewNotificationTitles,
      cartOrderId: cartResult.orderId,
      cartText: cartResult.cartText,
      cartNotificationTitles: cartResult.notificationTitles,
      refundOrderId: refundResult.orderId,
      refundText: refundResult.refundText,
      refundConfirmActionHidden: refundResult.confirmActionHidden,
      refundConfirmApiStatus: refundResult.confirmApiStatus,
      refundConfirmApiMessage: refundResult.confirmApiMessage,
      refundLockedStock: refundResult.lockedStock,
      refundRestoredStock: refundResult.restoredStock,
      refundNotificationTitles: refundResult.notificationTitles,
      refundCreditLedgerId: refundResult.creditLedgerId,
      refundCreditLedgerSourceEventId: refundResult.creditLedgerSourceEventId,
      refundCreditLedgerCountAfterRetry: refundResult.creditLedgerCountAfterRetry,
      refundBalanceAfterRetry: refundResult.balanceAfterRetry,
      refundHistoryInitialTotal: refundHistoryPaginationResult.initialTotal,
      refundHistoryLoadedNote: refundHistoryPaginationResult.loadedNote,
      rejectedRefundOrderId: rejectedRefundResult.orderId,
      rejectedRefundCanceledId: rejectedRefundResult.canceledRefundId,
      rejectedRefundReplacementId: rejectedRefundResult.replacementRefundId,
      rejectedRefundCancelBalanceBefore: rejectedRefundResult.cancelBalanceBefore,
      rejectedRefundCancelBalanceAfter: rejectedRefundResult.cancelBalanceAfter,
      rejectedRefundCancelStockBefore: rejectedRefundResult.cancelStockBefore,
      rejectedRefundCancelStockAfter: rejectedRefundResult.cancelStockAfter,
      rejectedRefundOrderStatusAfterRejection: rejectedRefundResult.orderStatusAfterRejection,
      rejectedRefundText: rejectedRefundResult.refundText,
      rejectedRefundNotificationTitles: rejectedRefundResult.notificationTitles,
      digitalOrderId: digitalResult.orderId,
      digitalOrderNo: digitalResult.orderNo,
      digitalEntitlementCode: digitalResult.entitlementCode,
      digitalEntitlementCodes: digitalResult.entitlementCodes,
      digitalEntitlementCount: digitalResult.entitlementCount,
      digitalRefundId: digitalResult.refundId,
      digitalText: digitalResult.digitalText,
      digitalRevokedText: digitalResult.revokedText,
      publicBadgeText: digitalResult.publicBadgeText,
      publicBadgeRevokedText: digitalResult.publicBadgeRevokedText,
      themeOrderId: themeResult.orderId,
      themeOrderNo: themeResult.orderNo,
      themeEntitlementCode: themeResult.entitlementCode,
      themeUseActionText: themeResult.useActionText,
      themeProfileClass: themeResult.profileClass,
      themeRevokedProfileClass: themeResult.revokedProfileClass,
      themeRevocationReason: themeResult.revocationReason,
      membershipOrderId: membershipResult.orderId,
      membershipOrderNo: membershipResult.orderNo,
      membershipPendingDuplicateOrderRejected: membershipResult.pendingDuplicateOrderRejected,
      membershipDuplicatePaymentApiStatus: membershipResult.duplicatePaymentApiStatus,
      membershipDuplicatePaymentApiMessage: membershipResult.duplicatePaymentApiMessage,
      membershipEntitlementCode: membershipResult.entitlementCode,
      membershipExpiresAt: membershipResult.expiresAt,
      membershipRenewalExpiresAt: membershipResult.renewalExpiresAt,
      membershipRenewalExpiresAtAfterFirstRevoke: membershipResult.renewalExpiresAtAfterFirstRevoke,
      membershipRenewalBackgroundRetainedAfterFirstRevoke: membershipResult.renewalBackgroundRetainedAfterFirstRevoke,
      membershipRenewalBountyReleaseLedgerId: membershipResult.renewalBountyReleaseLedgerId,
      membershipRefundApiStatus: membershipResult.refundApiStatus,
      membershipRefundApiMessage: membershipResult.refundApiMessage,
      membershipRefundActionHidden: membershipResult.refundActionHidden,
      membershipUseActionText: membershipResult.useActionText,
      membershipBackgroundUrl: membershipResult.membershipBackgroundUrl,
      membershipProfileBackgroundStyle: membershipResult.membershipProfileBackgroundStyle,
      membershipRevokedCachedProfileBackgroundStyle: membershipResult.membershipRevokedCachedProfileBackgroundStyle,
      membershipRevokedDashboardProfileBackgroundStyle: membershipResult.membershipRevokedDashboardProfileBackgroundStyle,
      membershipRevokedProfileBackgroundStyle: membershipResult.membershipRevokedProfileBackgroundStyle,
      membershipRevokedHeaderProfileNickname: membershipResult.headerProfileNickname,
      membershipAdminMuteBackgroundUrl: membershipResult.adminMuteBackgroundUrl,
      membershipAdminUnmuteBackgroundUrl: membershipResult.adminUnmuteBackgroundUrl,
      membershipText: membershipResult.membershipText,
      membershipRevocationReason: membershipResult.revocationReason,
      membershipRevokedBackgroundApiStatus: membershipResult.revokedBackgroundApiStatus,
      membershipRevokedBackgroundApiMessage: membershipResult.revokedBackgroundApiMessage,
      membershipRevokedApiStatus: membershipResult.revokedApiStatus,
      membershipRevokedApiMessage: membershipResult.revokedApiMessage,
      membershipRevokedDraftPublishApiStatus: membershipResult.revokedDraftPublishApiStatus,
      membershipRevokedDraftPublishApiMessage: membershipResult.revokedDraftPublishApiMessage,
      membershipRevokedEditApiStatus: membershipResult.revokedEditApiStatus,
      membershipRevokedEditApiMessage: membershipResult.revokedEditApiMessage,
      membershipRevokedUnacceptApiStatus: membershipResult.revokedUnacceptApiStatus,
      membershipRevokedUnacceptApiMessage: membershipResult.revokedUnacceptApiMessage,
      membershipRevokedText: membershipResult.revokedText,
      defaultQuestionTopicId: defaultQuestionResult.topicId,
      defaultQuestionAcceptedCommentId: defaultQuestionResult.acceptedCommentId,
      defaultQuestionReserveLedgerId: defaultQuestionResult.reserveLedgerId,
      defaultQuestionAnswererLedgerId: defaultQuestionResult.answererLedgerId,
      defaultQuestionReserveLedgerCount: defaultQuestionResult.reserveLedgerCount,
      defaultQuestionBalanceBefore: defaultQuestionResult.balanceBefore,
      defaultQuestionBalanceAfterPublish: defaultQuestionResult.balanceAfterPublish,
      defaultQuestionBalanceAfterAccept: defaultQuestionResult.balanceAfterAccept,
      bountyDraftTopicId: membershipResult.draftTopicId,
      bountyDraftTopicTitle: membershipResult.draftTopicTitle,
      bountyTopicId: membershipResult.topicId,
      bountyTopicTitle: membershipResult.topicTitle,
      bountyAcceptedCommentId: membershipResult.bountyAcceptedCommentId,
      bountyAcceptedTopicStatus: membershipResult.bountyAcceptedTopicStatus,
      bountyQuestionerLedgerId: membershipResult.bountyQuestionerLedgerId,
      bountyAnswererLedgerId: membershipResult.bountyAnswererLedgerId,
      bountyReversalLedgerId: membershipResult.bountyReversalLedgerId,
      bountyReacceptedAnswererLedgerId: membershipResult.bountyReacceptedAnswererLedgerId,
      bountyReleasedTopicId: membershipResult.bountyReleasedTopicId,
      bountyReleaseLedgerId: membershipResult.bountyReleaseLedgerId,
      bountyInsufficientCreditBalance: membershipResult.bountyInsufficientCreditBalance,
      bountyInsufficientCreditText: membershipResult.bountyInsufficientCreditText,
      bountyText: membershipResult.bountyText,
      attachmentEnabled: attachmentResult.enabled,
      attachmentTopicId: attachmentResult.topicId || "",
      attachmentId: attachmentResult.attachmentId || "",
      attachmentMembershipOrderId: attachmentMembership?.orderId || "",
      attachmentMembershipEntitlementId: attachmentMembership?.entitlementId || "",
      attachmentPriceCredits: attachmentResult.priceCredits || 0,
      attachmentBuyerChargedCredits: attachmentResult.buyerChargedCredits || 0,
      attachmentAuthorEarnedCredits: attachmentResult.authorEarnedCredits || 0,
      attachmentAuthorSaleLedgerId: attachmentResult.authorSaleLedgerId || "",
      attachmentAuthorSaleTotal: attachmentResult.authorSaleTotal || 0,
      attachmentAuthorTotalEarnedCredits: attachmentResult.authorTotalEarnedCredits || 0,
      attachmentArchived: Boolean(attachmentResult.archived),
      attachmentRevokedSaleApiStatus: attachmentRevokedSaleResult.apiStatus || 0,
      attachmentRevokedSaleText: attachmentRevokedSaleResult.text || "",
      attachmentRevokedSaleBuyerBalanceUnchanged: Boolean(attachmentRevokedSaleResult.buyerBalanceUnchanged),
      attachmentRevokedSaleAuthorBalanceUnchanged: Boolean(attachmentRevokedSaleResult.authorBalanceUnchanged),
      attachmentRevokedAttachmentArchived: Boolean(attachmentRevokedSaleResult.archived),
      checkInDay: checkInResult.day,
      checkInLedgerId: checkInResult.ledgerId,
      checkInBalanceBefore: checkInResult.balanceBefore,
      checkInBalanceAfter: checkInResult.balanceAfter,
      checkInDuplicate: checkInResult.duplicate,
      checkInButtonText: checkInResult.buttonText,
      notificationTitles
    };
  } finally {
    await page?.close().catch(() => {});
    chrome.kill();
    await delay(250);
    await rm(userDataDir, { recursive: true, force: true }).catch(() => {});
  }
}

async function activateMembershipForAttachment(fixture) {
  const productId = fixture.membershipProduct?.id;
  const unitPrice = Number(fixture.membershipProduct?.price_credits ?? fixture.membershipProduct?.priceCredits ?? 0);
  if (!productId || !Number.isSafeInteger(unitPrice) || unitPrice < 0) {
    throw new Error(`Attachment membership product fixture is invalid: ${JSON.stringify(fixture.membershipProduct)}`);
  }
  const balance = await currentCreditBalance(fixture);
  if (balance < unitPrice) {
    await topUpUserCredits(fixture, unitPrice - balance, `browser-attachment-membership-topup-${Date.now()}`);
  }
  const orderData = await apiRequest("/mall/orders", {
    method: "POST",
    token: fixture.auth.accessToken,
    body: {
      idempotency_key: `browser-attachment-membership-order-${Date.now()}`,
      items: [{ product_id: productId, quantity: 1 }]
    }
  });
  const order = orderData?.order || orderData;
  if (!order?.id) {
    throw new Error(`Attachment membership order did not return an id: ${JSON.stringify(orderData)}`);
  }
  const paymentData = await apiRequest(`/mall/orders/${encodeURIComponent(order.id)}/pay`, {
    method: "POST",
    token: fixture.auth.accessToken,
    body: {
      payment_method: "credits",
      idempotency_key: `browser-attachment-membership-pay-${order.id}-${Date.now()}`
    }
  });
  const paidOrder = paymentData?.order || paymentData;
  if (mallOrderStatusValue(paidOrder?.status) !== 6) {
    throw new Error(`Attachment membership payment status = ${paidOrder?.status ?? "unknown"}, want completed`);
  }
  const entitlement = await waitForDigitalEntitlement(fixture, order.id, productId, fixture.membershipGrantKey, "ACTIVE");
  if (!entitlement?.id) {
    throw new Error(`Attachment membership order did not issue an active entitlement: ${JSON.stringify(entitlement)}`);
  }
  return { orderId: String(order.id), entitlementId: String(entitlement.id) };
}

async function runBrowserAttachmentFlow(page, fixture, topicId, tempDir) {
  if (!topicId) {
    throw new Error("Attachment browser flow requires a published topic");
  }

  const sourceName = `browser-paid-attachment-${Date.now()}.txt`;
  const sourcePath = path.join(tempDir, sourceName);
  const revokedSourceName = `browser-revoked-attachment-${Date.now()}.txt`;
  const revokedSourcePath = path.join(tempDir, revokedSourceName);
  const uploadPriceCredits = 3;
  const priceCredits = 5;
  await writeFile(sourcePath, `Browser attachment E2E ${Date.now()}\n`, "utf8");
  await writeFile(revokedSourcePath, `Browser revoked attachment E2E ${Date.now()}\n`, "utf8");

  await setBrowserAuth(page, fixture.auth);
  await navigate(page, `${FRONTEND_BASE}/topic/${encodeURIComponent(topicId)}?attachment_e2e=${Date.now()}`);
  await waitForText(page, "附件", "attachment panel for author");
  await waitForSelector(page, ".topic-attachment-upload input[type='file']", "attachment upload input");
  await fillBySelector(page, ".topic-attachment-upload input[type='number']", String(uploadPriceCredits));
  await setFileInputFiles(page, ".topic-attachment-upload input[type='file']", sourcePath);
  await waitForText(page, "附件已添加", "attachment upload notice");
  await waitForText(page, sourceName, "uploaded attachment name");

  const attachmentRows = listItems(await apiRequest(`/topics/${encodeURIComponent(topicId)}/attachments`));
  const attachment = attachmentRows.find((item) => String(item?.original_name || item?.originalName || "") === sourceName);
  if (!attachment?.id) {
    throw new Error(`Uploaded browser attachment was not returned by topic API: ${JSON.stringify(attachmentRows)}`);
  }
  const attachmentPrice = Number(attachment?.price_credits ?? attachment?.priceCredits ?? 0);
  if (attachmentPrice !== uploadPriceCredits) {
    throw new Error(`Uploaded browser attachment price = ${attachmentPrice}, want ${uploadPriceCredits}`);
  }

  await fillBySelector(page, ".topic-attachment-upload input[type='number']", String(priceCredits));
  await setFileInputFiles(page, ".topic-attachment-upload input[type='file']", revokedSourcePath);
  await waitForText(page, revokedSourceName, "uploaded attachment reserved for revoked membership sale");
  const attachmentsWithRevokedSale = listItems(await apiRequest(`/topics/${encodeURIComponent(topicId)}/attachments`));
  const revokedAttachment = attachmentsWithRevokedSale.find(
    (item) => String(item?.original_name || item?.originalName || "") === revokedSourceName
  );
  if (!revokedAttachment?.id) {
    throw new Error(`Revoked-sale browser attachment was not returned by topic API: ${JSON.stringify(attachmentsWithRevokedSale)}`);
  }
  const revokedAttachmentPrice = Number(revokedAttachment?.price_credits ?? revokedAttachment?.priceCredits ?? 0);
  if (revokedAttachmentPrice !== priceCredits) {
    throw new Error(`Revoked-sale browser attachment price = ${revokedAttachmentPrice}, want ${priceCredits}`);
  }

  const priceLabel = `${sourceName} 的积分价格`;
  const savePriceLabel = `保存附件 ${sourceName} 的积分价格`;
  await fillInputByAriaLabel(page, priceLabel, String(priceCredits));
  await clickByAriaLabel(page, savePriceLabel);
  await waitForText(page, "附件积分价格已更新", "attachment price update notice");

  const buyerBalanceBefore = await currentCreditBalance({ ...fixture, auth: fixture.answererAuth });
  const authorBalanceBeforeSale = await currentCreditBalance(fixture);
  const attachmentSaleEventId = `attachment-download:${attachment.id}:${fixture.answererAuth.user.id}`;
  if (buyerBalanceBefore < priceCredits) {
    throw new Error(`Attachment buyer balance = ${buyerBalanceBefore}, want at least ${priceCredits}`);
  }
  await setBrowserAuth(page, fixture.answererAuth);
  await navigate(page, `${FRONTEND_BASE}/topic/${encodeURIComponent(topicId)}?attachment_buyer_e2e=${Date.now()}`);
  await waitForText(page, sourceName, "attachment visible to buyer");
  await clickButtonInArticle(page, sourceName, "^下载$");
  await waitForText(page, "附件下载已开始", "paid attachment first download notice");
  const buyerBalanceAfterFirstDownload = await currentCreditBalance({ ...fixture, auth: fixture.answererAuth });
  if (buyerBalanceAfterFirstDownload !== buyerBalanceBefore - priceCredits) {
    throw new Error(`First browser attachment download balance = ${buyerBalanceAfterFirstDownload}, want ${buyerBalanceBefore - priceCredits}`);
  }
  const authorBalanceAfterFirstSale = await currentCreditBalance(fixture);
  if (authorBalanceAfterFirstSale !== authorBalanceBeforeSale + priceCredits) {
    throw new Error(`First browser attachment sale balance = ${authorBalanceAfterFirstSale}, want ${authorBalanceBeforeSale + priceCredits}`);
  }
  const authorSaleLedger = await waitForCreditLedgerEntry(
    fixture.auth.accessToken,
    (item) =>
      creditLedgerReason(item) === "attachment_sale" &&
      String(item.source_type ?? item.sourceType ?? "") === "attachment" &&
      String(item.source_id ?? item.sourceId ?? "") === String(attachment.id) &&
      creditLedgerSourceEventId(item) === attachmentSaleEventId &&
      creditLedgerDelta(item) === priceCredits,
    "attachment author sale ledger",
  );
  await clickButtonInArticle(page, sourceName, "^下载$");
  await waitForText(page, "附件下载已开始", "paid attachment replay download notice");
  const buyerBalanceAfterReplay = await currentCreditBalance({ ...fixture, auth: fixture.answererAuth });
  if (buyerBalanceAfterReplay !== buyerBalanceAfterFirstDownload) {
    throw new Error(`Browser attachment replay changed buyer balance from ${buyerBalanceAfterFirstDownload} to ${buyerBalanceAfterReplay}`);
  }
  const authorBalanceAfterReplay = await currentCreditBalance(fixture);
  if (authorBalanceAfterReplay !== authorBalanceAfterFirstSale) {
    throw new Error(`Browser attachment replay changed author balance from ${authorBalanceAfterFirstSale} to ${authorBalanceAfterReplay}`);
  }
  const authorSaleEntries = await creditLedgerEntriesForSource(fixture.auth.accessToken, attachmentSaleEventId, "attachment_sale");
  if (authorSaleEntries.length !== 1) {
    throw new Error(`Browser attachment replay created ${authorSaleEntries.length} author sale ledger entries, want 1`);
  }
  const buyerDownloads = listItems(await apiRequest("/attachments/downloads?limit=20&offset=0", { token: fixture.answererAuth.accessToken }));
  const buyerDownload = buyerDownloads.find((item) => String(item?.attachment?.id || item?.attachment_id || item?.attachmentId || "") === String(attachment.id));
  const chargedCredits = Number(buyerDownload?.charged_credits ?? buyerDownload?.chargedCredits ?? 0);
  if (!buyerDownload || String(buyerDownload?.status || "").toUpperCase() !== "AUTHORIZED" || chargedCredits !== priceCredits) {
    throw new Error(`Browser attachment download history mismatch: ${JSON.stringify(buyerDownload)}`);
  }
  const authorSalesResponse = await apiRequest("/attachments/sales?limit=20&offset=0", { token: fixture.auth.accessToken });
  const authorSales = listItems(authorSalesResponse);
  const authorSaleTotal = Number(authorSalesResponse?.total ?? authorSalesResponse?.count ?? 0);
  const authorTotalEarnedCredits = Number(authorSalesResponse?.total_earned_credits ?? authorSalesResponse?.totalEarnedCredits ?? 0);
  const authorSaleRecords = authorSales.filter((item) => String(item?.attachment?.id || item?.attachment_id || item?.attachmentId || "") === String(attachment.id));
  const authorSaleRecord = authorSaleRecords[0];
  if (
    authorSaleRecords.length !== 1 ||
    Number(authorSaleRecord?.earned_credits ?? authorSaleRecord?.earnedCredits ?? 0) !== priceCredits ||
    Number(authorSaleRecord?.sold_at ?? authorSaleRecord?.soldAt ?? 0) <= 0
  ) {
    throw new Error(`Browser attachment sale history mismatch: ${JSON.stringify(authorSaleRecords)}`);
  }
  if (authorSaleTotal !== 1 || authorTotalEarnedCredits !== priceCredits) {
    throw new Error("Browser attachment sale summary mismatch: " + JSON.stringify(authorSalesResponse));
  }
  if (JSON.stringify(authorSalesResponse).includes('"object_key"')) {
    throw new Error("Browser attachment sale history exposed object_key");
  }

  await setBrowserAuth(page, fixture.auth);
  await navigate(page, `${FRONTEND_BASE}/member?attachment_sales_e2e=${Date.now()}`);
  await waitForText(page, "附件售卖记录", "attachment sale history section");
  await waitForText(page, "累计收益 " + priceCredits + " 积分 · 1 笔", "attachment sale history summary");
  await waitForText(page, sourceName, "attachment sale history filename");
  await waitForText(page, `收益 ${priceCredits} 积分`, "attachment sale history earned credits");

  await setBrowserAuth(page, fixture.auth);
  await navigate(page, `${FRONTEND_BASE}/topic/${encodeURIComponent(topicId)}?attachment_archive_e2e=${Date.now()}`);
  await waitForText(page, sourceName, "attachment visible before archive");
  await clickByAriaLabel(page, `归档附件 ${sourceName}`);
  await waitForText(page, "附件已归档", "attachment archive notice");
  await waitFor(page, `!document.body?.innerText?.includes(${JSON.stringify(sourceName)})`, "archived attachment removed from topic", 20000);
  const attachmentsAfterArchive = listItems(await apiRequest(`/topics/${encodeURIComponent(topicId)}/attachments`));
  if (attachmentsAfterArchive.some((item) => String(item?.id || "") === String(attachment.id))) {
    throw new Error(`Archived browser attachment was still returned by topic API: ${JSON.stringify(attachmentsAfterArchive)}`);
  }
  const archivedAuthorSalesResponse = await apiRequest("/attachments/sales?limit=20&offset=0", { token: fixture.auth.accessToken });
  const archivedAuthorSales = listItems(archivedAuthorSalesResponse);
  const archivedAuthorSaleTotal = Number(archivedAuthorSalesResponse?.total ?? archivedAuthorSalesResponse?.count ?? 0);
  const archivedAuthorTotalEarnedCredits = Number(
    archivedAuthorSalesResponse?.total_earned_credits ?? archivedAuthorSalesResponse?.totalEarnedCredits ?? 0
  );
  const archivedAuthorSale = archivedAuthorSales.find((item) => String(item?.attachment?.id || item?.attachment_id || item?.attachmentId || "") === String(attachment.id));
  if (
    !archivedAuthorSale ||
    String(archivedAuthorSale?.attachment?.status || "").toUpperCase() !== "ARCHIVED" ||
    archivedAuthorSaleTotal !== authorSaleTotal ||
    archivedAuthorTotalEarnedCredits !== authorTotalEarnedCredits
  ) {
    throw new Error(`Archived browser attachment sale history mismatch: ${JSON.stringify(archivedAuthorSale)}`);
  }
  await navigate(page, `${FRONTEND_BASE}/member?attachment_sales_archived_e2e=${Date.now()}`);
  await waitForText(page, "附件已归档", "archived attachment sale history state");

  const buyerBalanceBeforeArchivedReplay = await currentCreditBalance({ ...fixture, auth: fixture.answererAuth });
  await setBrowserAuth(page, fixture.answererAuth);
  await navigate(page, `${FRONTEND_BASE}/member?attachment_archived_replay_e2e=${Date.now()}`);
  await waitForText(page, sourceName, "archived attachment in buyer download history");
  await clickByAriaLabel(page, `重新下载附件 ${sourceName}`);
  await waitForText(page, "附件下载已开始", "archived attachment replay notice");
  const buyerBalanceAfterArchivedReplay = await currentCreditBalance({ ...fixture, auth: fixture.answererAuth });
  if (buyerBalanceAfterArchivedReplay !== buyerBalanceBeforeArchivedReplay) {
    throw new Error(`Archived browser attachment replay changed buyer balance from ${buyerBalanceBeforeArchivedReplay} to ${buyerBalanceAfterArchivedReplay}`);
  }

  return {
    enabled: true,
    topicId: String(topicId),
    attachmentId: String(attachment.id),
    priceCredits,
    buyerChargedCredits: chargedCredits,
    authorEarnedCredits: authorBalanceAfterFirstSale - authorBalanceBeforeSale,
    authorSaleLedgerId: String(authorSaleLedger.id ?? authorSaleLedger.ID ?? ""),
    authorSaleTotal,
    authorTotalEarnedCredits,
    revokedAttachmentId: String(revokedAttachment.id),
    revokedAttachmentName: revokedSourceName,
    revokedAttachmentPrice,
    archived: true,
    archivedReplay: true
  };
}

async function runBrowserRevokedAttachmentSaleFlow(page, fixture, attachmentResult, expectedBrowserIssues = []) {
  const attachmentId = String(attachmentResult?.revokedAttachmentId || "");
  const sourceName = String(attachmentResult?.revokedAttachmentName || "");
  const priceCredits = Number(attachmentResult?.revokedAttachmentPrice || 0);
  if (!attachmentId || !sourceName || !Number.isSafeInteger(priceCredits) || priceCredits <= 0) {
    throw new Error(`Revoked attachment fixture is invalid: ${JSON.stringify(attachmentResult)}`);
  }

  const buyerFixture = { ...fixture, auth: fixture.answererAuth };
  const buyerBalanceBefore = await currentCreditBalance(buyerFixture);
  const authorBalanceBefore = await currentCreditBalance(fixture);
  const sourceEventID = `attachment-download:${attachmentId}:${fixture.answererAuth.user.id}`;
  const apiFailure = await apiRequestFailure(`/attachments/${encodeURIComponent(attachmentId)}/download`, {
    token: fixture.answererAuth.accessToken,
    expectedStatus: 412,
    label: "revoked paid attachment sale"
  });
  const expectedApiMessage = "paid attachment sales unavailable because the author membership entitlement is inactive";
  if (!`${apiFailure.message}\n${apiFailure.rawBody}`.includes(expectedApiMessage)) {
    throw new Error(`Revoked paid attachment sale error mismatch: ${JSON.stringify(apiFailure)}`);
  }

  await setBrowserAuth(page, fixture.answererAuth);
  await navigate(page, `${FRONTEND_BASE}/topic/${encodeURIComponent(attachmentResult.topicId)}?attachment_revoked_sale_e2e=${Date.now()}`);
  await waitForText(page, sourceName, "revoked attachment visible to buyer");
  const frontendText = "该付费附件作者的会员权益已失效，暂时无法购买。";
  const stopExpectingRevokedSaleFailure = expectBrowserHttpFailure(
    expectedBrowserIssues,
    `${API_BASE}/attachments/${encodeURIComponent(attachmentId)}/download`,
    412
  );
  try {
    await clickButtonInArticle(page, sourceName, "^下载$");
    await waitForText(page, frontendText, "revoked paid attachment frontend error");
    await delay(250);
  } finally {
    stopExpectingRevokedSaleFailure();
  }

  const buyerBalanceAfter = await currentCreditBalance(buyerFixture);
  const authorBalanceAfter = await currentCreditBalance(fixture);
  if (buyerBalanceAfter !== buyerBalanceBefore || authorBalanceAfter !== authorBalanceBefore) {
    throw new Error(
      `Revoked paid attachment sale changed balances: buyer ${buyerBalanceBefore}->${buyerBalanceAfter}, author ${authorBalanceBefore}->${authorBalanceAfter}`
    );
  }
  const buyerDownloads = listItems(await apiRequest("/attachments/downloads?limit=20&offset=0", { token: fixture.answererAuth.accessToken }));
  if (buyerDownloads.some((item) => String(item?.attachment?.id || item?.attachment_id || item?.attachmentId || "") === attachmentId)) {
    throw new Error(`Revoked paid attachment sale created a download record for ${attachmentId}`);
  }
  const authorSaleEntries = await creditLedgerEntriesForSource(fixture.auth.accessToken, sourceEventID, "attachment_sale");
  if (authorSaleEntries.length !== 0) {
    throw new Error(`Revoked paid attachment sale created ${authorSaleEntries.length} author ledger entries`);
  }
  const authorSales = listItems(await apiRequest("/attachments/sales?limit=20&offset=0", { token: fixture.auth.accessToken }));
  if (authorSales.some((item) => String(item?.attachment?.id || item?.attachment_id || item?.attachmentId || "") === attachmentId)) {
    throw new Error(`Revoked paid attachment sale created a sales history record for ${attachmentId}`);
  }

  await setBrowserAuth(page, fixture.auth);
  await navigate(page, `${FRONTEND_BASE}/topic/${encodeURIComponent(attachmentResult.topicId)}?attachment_revoked_archive_e2e=${Date.now()}`);
  await waitForText(page, sourceName, "revoked attachment visible before archive");
  await clickByAriaLabel(page, `归档附件 ${sourceName}`);
  await waitForText(page, "附件已归档", "revoked attachment archive notice");
  await waitFor(page, `!document.body?.innerText?.includes(${JSON.stringify(sourceName)})`, "revoked attachment removed from topic", 20000);
  const attachmentsAfterArchive = listItems(await apiRequest(`/topics/${encodeURIComponent(attachmentResult.topicId)}/attachments`));
  if (attachmentsAfterArchive.some((item) => String(item?.id || "") === attachmentId)) {
    throw new Error(`Revoked attachment was still returned by topic API after archive: ${JSON.stringify(attachmentsAfterArchive)}`);
  }

  return {
    tested: true,
    apiStatus: apiFailure.status,
    text: frontendText,
    buyerBalanceUnchanged: true,
    authorBalanceUnchanged: true,
    archived: true
  };
}

async function runBrowserCheckInFlow(page, fixture) {
  const token = fixture.auth.accessToken;
  const beforeStatus = await apiRequest("/credits/check-in", { token });
  if (Boolean(beforeStatus?.checked_in ?? beforeStatus?.checkedIn)) {
    throw new Error("Fresh browser e2e account was already checked in");
  }
  const balanceBefore = await currentCreditBalance(fixture);

  await setBrowserAuth(page, fixture.auth);
  await navigate(page, `${FRONTEND_BASE}/member?check_in_e2e=${Date.now()}`);
  await waitForText(page, "把贡献转成持续权益", "member page for daily check-in");
  await waitForText(page, "今日签到可领取 5 积分", "daily check-in available state");
  await waitForButtonEnabled(page, "^签到 \\+5$", "daily check-in action enabled");
  await clickButton(page, "^签到 \\+5$");
  await waitForText(page, "今日已签到，连续", "daily check-in completion state");
  await waitFor(
    page,
    `document.querySelector(".check-in-action")?.disabled === true`,
    "daily check-in action disabled after completion",
  );
  await waitForText(page, "每日签到", "daily check-in ledger row");

  const afterStatus = await apiRequest("/credits/check-in", { token });
  if (!Boolean(afterStatus?.checked_in ?? afterStatus?.checkedIn)) {
    throw new Error(`Daily check-in status remained incomplete: ${JSON.stringify(afterStatus)}`);
  }
  const checkIn = afterStatus?.check_in ?? afterStatus?.checkIn;
  const day = String(checkIn?.latest_day ?? checkIn?.latestDay ?? "").trim();
  if (!day) {
    throw new Error(`Daily check-in did not return latest_day: ${JSON.stringify(afterStatus)}`);
  }
  const sourceEventID = `credit.checkin:${fixture.auth.user.id}:${day}`;
  const ledger = await waitForCreditLedgerEntry(
    token,
    (entry) =>
      creditLedgerReason(entry) === "daily_check_in" &&
      creditLedgerSourceEventId(entry) === sourceEventID &&
      creditLedgerDelta(entry) === 5,
    "daily check-in credit ledger",
  );
  const balanceAfter = await currentCreditBalance(fixture);
  if (balanceAfter !== balanceBefore + 5) {
    throw new Error(`Daily check-in balance = ${balanceAfter}, want ${balanceBefore + 5}`);
  }

  const duplicate = await apiRequest("/credits/check-in", { method: "POST", token });
  if (!duplicate?.duplicate) {
    throw new Error(`Daily check-in replay was not marked duplicate: ${JSON.stringify(duplicate)}`);
  }
  const duplicateLedgerID = String(duplicate?.ledger?.id ?? duplicate?.ledger?.ID ?? "");
  const ledgerID = String(ledger?.id ?? ledger?.ID ?? "");
  if (!ledgerID || duplicateLedgerID !== ledgerID) {
    throw new Error(`Daily check-in replay ledger = ${duplicateLedgerID || "empty"}, want ${ledgerID || "original ledger"}`);
  }
  const entries = await creditLedgerEntriesForSource(token, sourceEventID, "daily_check_in");
  if (entries.length !== 1) {
    throw new Error(`Daily check-in ledger entries = ${entries.length}, want 1 for ${sourceEventID}`);
  }

  await navigate(page, `${FRONTEND_BASE}/member?check_in_reload=${Date.now()}`);
  await waitForText(page, "今日已签到，连续", "daily check-in persisted state after reload");
  await waitFor(
    page,
    `(() => {
      const button = document.querySelector(".check-in-action");
      return Boolean(button?.disabled && (button.innerText || "").trim() === "今日已签到");
    })()`,
    "daily check-in action remains disabled after reload",
  );
  const buttonText = await evaluate(page, `document.querySelector(".check-in-action")?.innerText?.trim() || ""`);

  return {
    day,
    ledgerId: ledgerID,
    balanceBefore,
    balanceAfter,
    duplicate: true,
    buttonText
  };
}

function collectBrowserIssues(page, expectedBrowserIssues = []) {
  const issues = [];
  page.on("Runtime.exceptionThrown", (event) => {
    issues.push({
      type: "pageerror",
      text: event.exceptionDetails?.exception?.description || event.exceptionDetails?.text || "Runtime exception"
    });
  });
  page.on("Runtime.consoleAPICalled", (event) => {
    if (event.type === "error" || event.type === "warning") {
      issues.push({
        type: `console:${event.type}`,
        text: (event.args || []).map((arg) => arg.value || arg.description || "").join(" ").slice(0, 800)
      });
    }
  });
  page.on("Log.entryAdded", (event) => {
    const entry = event.entry || {};
    if (entry.level === "error" || entry.level === "warning") {
      const issue = { type: `log:${entry.level}`, text: entry.text || "", url: entry.url || "" };
      issue.expected = isExpectedBrowserIssue(expectedBrowserIssues, issue);
      issues.push(issue);
    }
  });
  page.on("Network.responseReceived", (event) => {
    const response = event.response || {};
    if (response.status >= 400 && (response.url?.startsWith(API_BASE) || response.url?.startsWith(FRONTEND_BASE))) {
      const issue = { type: "http", status: response.status, url: response.url };
      issue.expected = isExpectedBrowserIssue(expectedBrowserIssues, issue);
      issues.push(issue);
    }
  });
  return issues;
}

function expectBrowserHttpFailure(expectedBrowserIssues, url, status) {
  const expected = { url, status };
  expectedBrowserIssues.push(expected);
  return () => {
    const index = expectedBrowserIssues.indexOf(expected);
    if (index >= 0) expectedBrowserIssues.splice(index, 1);
  };
}

function isExpectedBrowserIssue(expectedBrowserIssues, issue) {
  const text = `${issue.url || ""} ${issue.text || ""}`;
  return expectedBrowserIssues.some((expected) => {
    if (expected.url && !text.includes(expected.url)) return false;
    if (expected.status && issue.status && Number(issue.status) !== Number(expected.status)) return false;
    if (expected.status && !issue.status && !text.includes(String(expected.status))) return false;
    return true;
  });
}

function isSeriousBrowserIssue(issue) {
  if (issue.expected) return false;
  const text = `${issue.text || ""} ${issue.url || ""}`;
  if (/favicon|manifest|websocket|ws:\/\//i.test(text)) return false;
  if (/Download the React DevTools/i.test(text)) return false;
  return true;
}

function summarizeCheckoutText(text) {
  const lines = text
    .split(/\r?\n/)
    .map((line) => line.trim())
    .filter(Boolean);
  return lines.find((line) => line.includes("兑换成功") || line.includes("订单已创建")) || "";
}

function summarizeOrderLifecycleText(text) {
  const lines = text
    .split(/\r?\n/)
    .map((line) => line.trim())
    .filter(Boolean);
  return lines.find((line) => line.includes("已确认收货") || line.includes("已完成")) || "";
}

function summarizeReviewText(text) {
  const lines = text
    .split(/\r?\n/)
    .map((line) => line.trim())
    .filter(Boolean);
  return lines.find((line) => line.includes("当前定位")) ||
    lines.find((line) => line.includes("已展示")) ||
    lines.find((line) => line.includes("评价已提交")) ||
    "";
}

function summarizePublicProductReviewText(text, reviewContent) {
  const lines = text
    .split(/\r?\n/)
    .map((line) => line.trim())
    .filter(Boolean);
  return lines.find((line) => line.includes(reviewContent)) ||
    lines.find((line) => line.includes("星评价")) ||
    "";
}

function summarizeCartText(text) {
  const lines = text
    .split(/\r?\n/)
    .map((line) => line.trim())
    .filter(Boolean);
  return lines.find((line) => line.includes("兑换成功") || line.includes("订单已创建")) || "";
}

function summarizeDashboardPaymentText(text) {
  const lines = text
    .split(/\r?\n/)
    .map((line) => line.trim())
    .filter(Boolean);
  return lines.find((line) => line.includes("订单已支付")) ||
    lines.find((line) => line.includes("支付成功")) ||
    "";
}

function summarizeInsufficientPaymentRecoveryText(text) {
  const lines = text
    .split(/\r?\n/)
    .map((line) => line.trim())
    .filter(Boolean);
  return lines.find((line) => line.includes("支付成功")) ||
    lines.find((line) => line.includes("订单已支付")) ||
    lines.find((line) => line.includes("支付失败")) ||
    "";
}

function summarizeCouponCancellationText(text) {
  const lines = text
    .split(/\r?\n/)
    .map((line) => line.trim())
    .filter(Boolean);
  return lines.find((line) => line.includes("优惠券已领取")) ||
    lines.find((line) => line.includes("已释放")) ||
    lines.find((line) => line.includes("订单已取消")) ||
    "";
}

function summarizeRefundText(text) {
  const lines = text
    .split(/\r?\n/)
    .map((line) => line.trim())
    .filter(Boolean);
  return lines.find((line) => line.includes("已退款")) || lines.find((line) => line.includes("售后退款已通过")) || "";
}

function summarizeRejectedRefundText(text) {
  const lines = text
    .split(/\r?\n/)
    .map((line) => line.trim())
    .filter(Boolean);
  return lines.find((line) => line.includes("售后已拒绝")) || lines.find((line) => line.includes("售后申请已拒绝")) || "";
}

function summarizeDigitalEntitlementText(text, grantKey, entitlementCode) {
  const lines = text
    .split(/\r?\n/)
    .map((line) => line.trim())
    .filter(Boolean);
  return lines.find((line) => line.includes(grantKey)) ||
    (entitlementCode ? lines.find((line) => line.includes(entitlementCode)) : "") ||
    lines.find((line) => line.includes("可用")) ||
    "";
}

function summarizePublicBadgeText(text, badgeTitle) {
  const lines = text
    .split(/\r?\n/)
    .map((line) => line.trim())
    .filter(Boolean);
  return lines.find((line) => line.includes(badgeTitle)) ||
    lines.find((line) => line.includes("通过商城数字权益获得")) ||
    lines.find((line) => line.includes("暂无公开徽章")) ||
    "";
}

function summarizeMembershipBountyText(text, topicTitle) {
  const lines = text
    .split(/\r?\n/)
    .map((line) => line.trim())
    .filter(Boolean);
  return lines.find((line) => line.includes(topicTitle)) ||
    lines.find((line) => line.includes("会员权益可用")) ||
    lines.find((line) => line.includes("积分悬赏")) ||
    "";
}

function membershipEffectiveExpiryLabel(expiresAt) {
  const date = new Date(expiresAt);
  if (Number.isNaN(date.getTime())) {
    throw new Error(`Membership effective expiry is invalid: ${expiresAt}`);
  }
  return `会员有效至 ${date.toLocaleDateString("zh-CN", { year: "numeric", month: "2-digit", day: "2-digit" })}`;
}

async function runBrowserDefaultQuestionRewardFlow(page, fixture) {
  const topicTitle = `E2E Default Question Reward ${Date.now()}`;
  const topicBody = "浏览器联调基础采纳奖励：发布时冻结基础积分，采纳后结算给答主。";
  const balanceBefore = await currentCreditBalance(fixture);

  await navigate(page, `${FRONTEND_BASE}/question/create`);
  await waitForText(page, "发布求助|创作中心", "default reward question editor");
  await fillBySelector(page, ".compose-title", topicTitle);
  await fillBySelector(page, ".editor-body", topicBody);
  await fillByLabel(page, "悬赏积分", "0");
  await waitForText(page, "发布后冻结 10 积分作为基础采纳奖励", "default reward reservation hint");
  await waitForButtonEnabled(page, "^发布$", "default reward question submit enabled");
  await clickButton(page, "^发布$");
  await waitForText(page, topicTitle, "default reward question detail");

  const topic = await latestTopicForTitle(fixture, topicTitle);
  if (!topic?.id) {
    throw new Error(`Default reward question was not returned by topic API: ${topicTitle}`);
  }
  const bountyScore = Number(topic.bounty_score ?? topic.bountyScore ?? 0);
  if (bountyScore !== 0) {
    throw new Error(`Default reward question bounty score = ${bountyScore}, want 0`);
  }
  const reserveLedger = await waitForCreditLedgerEntry(
    fixture.auth.accessToken,
    (item) =>
      creditLedgerReason(item) === "qa_bounty_reserved" &&
      String(item.source_type ?? item.sourceType ?? "") === "topic" &&
      String(item.source_id ?? item.sourceId ?? "") === String(topic.id) &&
      creditLedgerDelta(item) === -DEFAULT_QA_REWARD_CREDITS,
    "default question reserved ledger",
  );
  const balanceAfterPublish = await currentCreditBalance(fixture);
  if (balanceAfterPublish !== balanceBefore - DEFAULT_QA_REWARD_CREDITS) {
    throw new Error(
      `Default reward question balance after publish = ${balanceAfterPublish}, want ${balanceBefore - DEFAULT_QA_REWARD_CREDITS}`,
    );
  }

  const answerNeedle = `浏览器联调基础奖励答案 ${Date.now()}`;
  const answerResp = await apiRequest(`/topics/${encodeURIComponent(topic.id)}/comments`, {
    method: "POST",
    token: fixture.answererAuth.accessToken,
    body: {
      content: `${answerNeedle}：采纳后应获得 ${DEFAULT_QA_REWARD_CREDITS} 积分。`,
      parent_id: 0,
    },
  });
  const answerComment = answerResp?.comment || answerResp;
  if (!answerComment?.id) {
    throw new Error(`Default reward answer comment creation did not return comment.id: ${JSON.stringify(answerResp)}`);
  }
  const answererFixture = { ...fixture, auth: fixture.answererAuth };
  const questionerBalanceBeforeAccept = await currentCreditBalance(fixture);
  const answererBalanceBeforeAccept = await currentCreditBalance(answererFixture);

  await navigate(page, `${FRONTEND_BASE}/topic/${encodeURIComponent(topic.id)}?default_reward_e2e=${Date.now()}`);
  await waitForText(page, topicTitle, "default reward question before acceptance");
  await waitForText(page, answerNeedle, "default reward answer before acceptance");
  await clickButtonWhenEnabled(page, "^采纳答案$", "default reward answer accept button enabled");
  await waitForText(page, "已采纳", "default reward accepted answer badge");
  await waitForText(page, "已解决", "default reward question resolved state");

  await waitForTopicAccepted(topic.id, answerComment.id, "default reward accepted topic");
  const answererLedger = await waitForCreditLedgerEntry(
    fixture.answererAuth.accessToken,
    (item) =>
      creditLedgerReason(item) === "qa_answer_accepted" &&
      String(item.source_type ?? item.sourceType ?? "") === "comment" &&
      String(item.source_id ?? item.sourceId ?? "") === String(answerComment.id) &&
      creditLedgerDelta(item) === DEFAULT_QA_REWARD_CREDITS,
    "default reward answerer ledger",
  );
  const balanceAfterAccept = await currentCreditBalance(fixture);
  const answererBalanceAfterAccept = await currentCreditBalance(answererFixture);
  if (balanceAfterAccept !== questionerBalanceBeforeAccept) {
    throw new Error(
      `Default reward questioner balance after accept = ${balanceAfterAccept}, want unchanged ${questionerBalanceBeforeAccept}`,
    );
  }
  if (answererBalanceAfterAccept !== answererBalanceBeforeAccept + DEFAULT_QA_REWARD_CREDITS) {
    throw new Error(
      `Default reward answerer balance after accept = ${answererBalanceAfterAccept}, want ${answererBalanceBeforeAccept + DEFAULT_QA_REWARD_CREDITS}`,
    );
  }
  const reserveEntries = await creditLedgerEntriesForSource(
    fixture.auth.accessToken,
    `content.qa.bounty:${topic.id}`,
    "qa_bounty_reserved",
  );
  if (reserveEntries.length !== 1) {
    throw new Error(`Default reward reserve ledger count = ${reserveEntries.length}, want 1`);
  }

  return {
    topicId: String(topic.id),
    acceptedCommentId: String(answerComment.id),
    reserveLedgerId: String(reserveLedger.id ?? reserveLedger.ID ?? ""),
    answererLedgerId: String(answererLedger.id ?? answererLedger.ID ?? ""),
    reserveLedgerCount: reserveEntries.length,
    balanceBefore,
    balanceAfterPublish,
    balanceAfterAccept,
  };
}

async function runBrowserDirectCouponCheckout(page, fixture) {
  const shopUrl = `${FRONTEND_BASE}/shop?product_id=${encodeURIComponent(fixture.directCouponProduct.id)}&coupon_code=${encodeURIComponent(fixture.directCoupon.code)}`;
  await navigate(page, shopUrl);
  await waitForText(page, fixture.directCouponProduct.title, "direct coupon product detail");
  await waitForText(page, "商品详情", "direct coupon product detail panel");
  await clickButton(page, "^立即兑换$");
  await waitForText(page, "确认兑换", "direct coupon checkout panel");
  await waitForText(page, "优惠码将提交给系统校验", "direct coupon backend validation hint");
  await waitForButtonEnabled(page, "^确认兑换$", "direct coupon checkout enabled");
  await clickButton(page, "^确认兑换$");
  await waitForText(page, `已优惠 ${COUPON_DISCOUNT} 积分|兑换成功|订单已创建`, "direct coupon order paid");

  const order = await latestMallOrderForProduct(fixture, fixture.directCouponProduct.id);
  if (!order?.id) {
    throw new Error("Direct coupon mall order was not returned by user order API");
  }
  const couponCode = order.coupon_code || order.couponCode || "";
  const discountCredits = Number(order.discount_credits ?? order.discountCredits ?? 0);
  if (couponCode !== fixture.directCoupon.code || discountCredits !== COUPON_DISCOUNT) {
    throw new Error(`Direct coupon order did not apply expected discount: ${JSON.stringify({ couponCode, discountCredits })}`);
  }
  const reuseRejection = await assertDirectCouponReuseRejected(fixture);

  return {
    orderId: String(order.id),
    orderNo: order.order_no || order.orderNo || "",
    text: summarizeCheckoutText(await bodyText(page)),
    reuseStatus: reuseRejection.status,
    reuseText: reuseRejection.message
  };
}

async function runBrowserZeroCreditCouponCheckout(page, fixture) {
  const product = fixture.zeroCreditCouponProduct;
  const coupon = fixture.zeroCreditCoupon;
  const initialStock = await currentMallProductStock(product.id);
  const shopUrl = `${FRONTEND_BASE}/shop?product_id=${encodeURIComponent(product.id)}&coupon_code=${encodeURIComponent(coupon.code)}`;

  await navigate(page, shopUrl);
  await waitForText(page, product.title, "zero-credit coupon product detail");
  await waitForText(page, coupon.code, "zero-credit coupon visible in shop");
  await clickButtonInArticle(page, coupon.code, "^领取$");
  await waitForText(page, "优惠券已领取|已经在你的券包里", "zero-credit coupon claimed");
  await waitForCouponUsageStatus(fixture, coupon.id, 4, "", "zero-credit coupon claimed usage");

  const balanceBefore = await currentCreditBalance(fixture);
  await clickButton(page, "^立即兑换$");
  await waitForText(page, "确认兑换", "zero-credit coupon checkout panel");
  await waitForText(page, `已预估优惠 ${ZERO_CREDIT_CHECKOUT_PRICE} 积分`, "zero-credit coupon discount preview");
  const checkoutText = await bodyText(page);
  if (!/应付积分\s*0/.test(checkoutText)) {
    throw new Error(`Zero-credit coupon checkout did not render a zero payable amount: ${checkoutText.slice(0, 1200)}`);
  }
  await waitForButtonEnabled(page, "^确认兑换$", "zero-credit coupon checkout enabled");
  await clickButton(page, "^确认兑换$");
  await waitForText(page, `已优惠 ${ZERO_CREDIT_CHECKOUT_PRICE} 积分`, "zero-credit coupon checkout paid");

  const order = await latestMallOrderForProduct(fixture, product.id);
  if (!order?.id) {
    throw new Error("Zero-credit coupon mall order was not returned by user order API");
  }
  const settledOrder = await waitForMallOrderStatus(fixture, order.id, [3, 6], "zero-credit coupon order settled in API");
  const originalCredits = Number(settledOrder.original_credits ?? settledOrder.originalCredits ?? 0);
  const discountCredits = Number(settledOrder.discount_credits ?? settledOrder.discountCredits ?? 0);
  const totalCredits = Number(settledOrder.total_credits ?? settledOrder.totalCredits ?? 0);
  const couponCode = settledOrder.coupon_code || settledOrder.couponCode || "";
  if (
    originalCredits !== ZERO_CREDIT_CHECKOUT_PRICE ||
    discountCredits !== ZERO_CREDIT_CHECKOUT_PRICE ||
    totalCredits !== 0 ||
    couponCode !== coupon.code
  ) {
    throw new Error(
      `Zero-credit coupon order snapshot mismatch: ${JSON.stringify({ originalCredits, discountCredits, totalCredits, couponCode })}`
    );
  }

  const payment = await waitForMallOrderPaymentStatus(fixture, order.id, 2, "zero-credit coupon payment completed");
  const paymentAmount = Number(payment.amount_credits ?? payment.amountCredits ?? 0);
  const paymentIdempotencyKey = String(payment.idempotency_key ?? payment.idempotencyKey ?? "").trim();
  if (paymentAmount !== 0 || !paymentIdempotencyKey) {
    throw new Error(`Zero-credit coupon payment mismatch: ${JSON.stringify({ paymentAmount, paymentIdempotencyKey })}`);
  }
  const debitEntries = await creditLedgerEntriesForSource(
    fixture.auth.accessToken,
    `mall.order.pay:${order.id}:${paymentIdempotencyKey}`,
    "mall_order_paid"
  );
  if (debitEntries.length !== 0) {
    throw new Error(`Zero-credit coupon payment created debit ledger entries: ${JSON.stringify(debitEntries)}`);
  }

  const usedUsage = await waitForCouponUsageStatus(fixture, coupon.id, 2, order.id, "zero-credit coupon used usage");
  const usageDiscount = Number(usedUsage.discount_credits ?? usedUsage.discountCredits ?? 0);
  if (usageDiscount !== ZERO_CREDIT_CHECKOUT_PRICE) {
    throw new Error(`Zero-credit coupon usage discount = ${usageDiscount}, want ${ZERO_CREDIT_CHECKOUT_PRICE}`);
  }
  const lockedStock = await waitForMallProductStock(product.id, initialStock - 1, "zero-credit coupon product stock locked after paid order");
  const balanceAfter = await currentCreditBalance(fixture);
  if (balanceAfter !== balanceBefore) {
    throw new Error(`Zero-credit coupon checkout changed balance from ${balanceBefore} to ${balanceAfter}`);
  }

  const refundNote = `Browser E2E zero-credit refund ${Date.now()}`;
  const adminNote = `Browser E2E zero-credit refund approved ${Date.now()}`;
  const refund = await createMallRefund(fixture, order.id, refundNote);
  const refundAmount = Number(refund.amount_credits ?? refund.amountCredits ?? 0);
  if (refundAmount !== 0) {
    throw new Error(`Zero-credit coupon refund amount = ${refundAmount}, want 0`);
  }
  const refundBalanceBefore = await currentCreditBalance(fixture);
  if (refundBalanceBefore !== balanceAfter) {
    throw new Error(`Zero-credit coupon refund started with balance ${refundBalanceBefore}, want ${balanceAfter}`);
  }
  await approveMallRefund(fixture, refund.id, adminNote);
  await waitForMallOrderStatus(fixture, order.id, 8, "zero-credit coupon order refunded in API");
  const refundEntries = await creditLedgerEntriesForSource(
    fixture.auth.accessToken,
    `mall.refund:${refund.id}`,
    "mall_order_refund"
  );
  if (refundEntries.length !== 0) {
    throw new Error(`Zero-credit coupon refund created credit ledger entries: ${JSON.stringify(refundEntries)}`);
  }
  const refundRestoredStock = await waitForMallProductStock(product.id, initialStock, "zero-credit coupon product stock restored after refund");
  const refundBalanceAfter = await currentCreditBalance(fixture);
  if (refundBalanceAfter !== refundBalanceBefore) {
    throw new Error(`Zero-credit coupon refund changed balance from ${refundBalanceBefore} to ${refundBalanceAfter}`);
  }
  await approveMallRefund(fixture, refund.id, `${adminNote} retry`);
  await waitForMallProductStock(product.id, initialStock, "zero-credit coupon refund retry kept stock restored");
  const refundEntriesAfterRetry = await creditLedgerEntriesForSource(
    fixture.auth.accessToken,
    `mall.refund:${refund.id}`,
    "mall_order_refund"
  );
  if (refundEntriesAfterRetry.length !== 0) {
    throw new Error(`Zero-credit coupon refund retry created credit ledger entries: ${JSON.stringify(refundEntriesAfterRetry)}`);
  }
  const refundBalanceAfterRetry = await currentCreditBalance(fixture);
  if (refundBalanceAfterRetry !== refundBalanceAfter) {
    throw new Error(`Zero-credit coupon refund retry changed balance from ${refundBalanceAfter} to ${refundBalanceAfterRetry}`);
  }
  const refundNotifications = await waitForMallOrderNotifications(fixture, order.id, ["售后退款已通过"]);

  return {
    orderId: String(order.id),
    paymentId: String(payment.id || payment.ID || ""),
    usageId: String(usedUsage.id || usedUsage.ID || ""),
    balanceBefore,
    balanceAfter,
    lockedStock,
    refundId: String(refund.id || refund.ID || ""),
    refundBalanceBefore,
    refundBalanceAfter,
    refundRestoredStock,
    refundNotificationTitles: refundNotifications.map((item) => item.title || item.type || "").filter(Boolean)
  };
}

async function assertDirectCouponReuseRejected(fixture) {
  const failure = await apiRequestFailure("/mall/orders", {
    method: "POST",
    token: fixture.auth.accessToken,
    expectedStatus: 412,
    label: "direct coupon reuse",
    body: {
      idempotency_key: `direct-coupon-reuse-${Date.now()}`,
      items: [{ product_id: fixture.directCouponProduct.id, quantity: 1 }],
      coupon_code: fixture.directCoupon.code
    }
  });
  const legacyCode = String(failure.meta?.legacy_code || failure.meta?.legacyCode || "");
  const combined = `${failure.message} ${failure.rawBody}`.toLowerCase();
  if (legacyCode !== "FailedPrecondition" || !combined.includes("coupon unavailable")) {
    throw new Error(`Direct coupon reuse rejection mismatch: ${failure.rawBody.slice(0, 800)}`);
  }
  return {
    status: failure.status,
    message: failure.message || "coupon unavailable"
  };
}

async function runBrowserDashboardPaymentFlow(page, fixture) {
  const product = fixture.dashboardPayProduct;
  const initialStock = await currentMallProductStock(product.id);
  const orderData = await apiRequest("/mall/orders", {
    method: "POST",
    token: fixture.auth.accessToken,
    body: {
      idempotency_key: `web-dashboard-pay-${Date.now()}`,
      receiver: "浏览器联调补付",
      phone: "13500000000",
      address: "上海 上海 浦东新区 补付路 1 号",
      items: [{ product_id: product.id, quantity: 1 }]
    }
  });
  const order = orderData?.order || orderData;
  if (!order?.id) {
    throw new Error("Dashboard payment mall order was not returned by order API");
  }
  const orderStatus = mallOrderStatusValue(order.status);
  if (orderStatus !== 1 || Number(order.total_credits ?? order.totalCredits ?? 0) !== CHECKOUT_PRICE) {
    throw new Error(`Dashboard payment order snapshot mismatch: ${JSON.stringify({ orderStatus, totalCredits: order.total_credits ?? order.totalCredits })}`);
  }
  const orderNo = order.order_no || order.orderNo || String(order.id);
  const lockedStock = await waitForMallProductStock(product.id, initialStock - 1, "dashboard payment product stock locked by pending order");

  await navigate(page, `${FRONTEND_BASE}/dashboard/orders?order_id=${encodeURIComponent(order.id)}`);
  await waitForText(page, "个人工作台", "dashboard payment shell");
  await waitForText(page, orderNo, "dashboard payment order number");
  await waitForText(page, product.title, "dashboard payment order item title");
  await waitForText(page, "待支付", "dashboard payment pending order status");
  await waitForText(page, "继续支付", "dashboard payment action");
  await clickButtonInArticle(page, orderNo, "^继续支付$");
  await waitForText(page, "订单已支付，积分流水已同步。|已支付", "dashboard payment success");

  const paidOrder = await waitForMallOrderStatus(fixture, order.id, 3, "dashboard payment order paid in API");
  const notifications = await waitForMallOrderNotifications(fixture, order.id, ["订单已支付"]);
  await waitForMallProductStock(product.id, initialStock - 1, "dashboard payment product stock remains locked after paid");
  await navigate(page, `${FRONTEND_BASE}/dashboard/orders?order_id=${encodeURIComponent(order.id)}`);
  await waitForText(page, "支付记录|支付成功", "dashboard payment records");
  await waitForText(page, product.title, "paid dashboard payment item title");
  await waitForText(page, "再次兑换", "paid dashboard payment repeat action");

  return {
    orderId: String(order.id),
    orderNo: paidOrder.order_no || paidOrder.orderNo || orderNo,
    lockedStock,
    text: summarizeDashboardPaymentText(await bodyText(page)),
    notificationTitles: notifications.map((item) => item.title || item.type || "").filter(Boolean)
  };
}

async function runBrowserCouponHistoryPaginationFlow(page, fixture) {
  const firstPage = await apiRequest(`/mall/coupons/mine?status=4&limit=${COUPON_HISTORY_PAGE_SIZE}&offset=0`, {
    token: fixture.auth.accessToken
  });
  const firstPageItems = listItems(firstPage);
  const initialTotal = Number(firstPage?.total ?? firstPage?.count ?? firstPageItems.length);
  if (initialTotal <= COUPON_HISTORY_PAGE_SIZE || firstPageItems.length !== COUPON_HISTORY_PAGE_SIZE) {
    throw new Error(`Coupon history fixture did not produce a full first page: ${JSON.stringify({ initialTotal, itemCount: firstPageItems.length })}`);
  }
  const secondPage = await apiRequest(`/mall/coupons/mine?status=4&limit=${COUPON_HISTORY_PAGE_SIZE}&offset=${firstPageItems.length}`, {
    token: fixture.auth.accessToken
  });
  const firstPageCodes = new Set(firstPageItems.map(couponUsageCode).filter(Boolean));
  const loadedCode = listItems(secondPage).map(couponUsageCode).find((code) => code && !firstPageCodes.has(code));
  if (!loadedCode) {
    throw new Error(`Coupon history second page did not contain an item outside the first page: ${JSON.stringify({ firstPageItems, secondPageItems: listItems(secondPage) })}`);
  }

  await navigate(page, `${FRONTEND_BASE}/dashboard/coupons?coupon_history_pagination=${Date.now()}`);
  await waitForText(page, "个人列表|优惠券", "coupon history dashboard");
  await waitForButtonEnabled(page, "^加载更多$", "coupon history load more");
  const initialText = await bodyText(page);
  if (initialText.includes(loadedCode)) {
    throw new Error(`Second-page coupon ${loadedCode} rendered before loading the next page`);
  }
  await clickButton(page, "^加载更多$");
  await waitForText(page, loadedCode, "coupon history second page");

  return {
    initialTotal,
    loadedCode
  };
}

async function runBrowserCreditHistoryPaginationFlow(page, fixture) {
  const firstPage = await apiRequest(`/credits/ledger?limit=${CREDIT_HISTORY_PAGE_SIZE}&offset=0`, {
    token: fixture.auth.accessToken
  });
  const firstPageItems = listItems(firstPage);
  const initialTotal = Number(firstPage?.total ?? firstPage?.count ?? firstPageItems.length);
  if (initialTotal <= CREDIT_HISTORY_PAGE_SIZE || firstPageItems.length !== CREDIT_HISTORY_PAGE_SIZE) {
    throw new Error(`Credit history fixture did not produce a full first page: ${JSON.stringify({ initialTotal, itemCount: firstPageItems.length })}`);
  }
  const secondPage = await apiRequest(`/credits/ledger?limit=${CREDIT_HISTORY_PAGE_SIZE}&offset=${firstPageItems.length}`, {
    token: fixture.auth.accessToken
  });
  const firstPageReasons = new Set(firstPageItems.map((item) => String(item?.reason || "").trim()).filter(Boolean));
  const loadedReason = listItems(secondPage)
    .map((item) => String(item?.reason || "").trim())
    .find((reason) => reason && !firstPageReasons.has(reason));
  if (!loadedReason) {
    throw new Error(`Credit history second page did not contain an item outside the first page: ${JSON.stringify({ firstPageItems, secondPageItems: listItems(secondPage) })}`);
  }

  await navigate(page, `${FRONTEND_BASE}/dashboard/scores?credit_history_pagination=${Date.now()}`);
  await waitForText(page, "当前积分|积分明细", "credit history dashboard");
  await waitForButtonEnabled(page, "^加载更多$", "credit history load more");
  const initialText = await bodyText(page);
  if (initialText.includes(loadedReason)) {
    throw new Error(`Second-page credit reason ${loadedReason} rendered before loading the next page`);
  }
  await clickButton(page, "^加载更多$");
  await waitForText(page, loadedReason, "credit history second page");

  return {
    initialTotal,
    loadedReason
  };
}

async function runBrowserAddressHistoryPaginationFlow(page, fixture) {
  const addressFixture = await createAddressHistoryFixture(fixture);
  const firstPage = await apiRequest(`/mall/addresses?limit=${ADDRESS_HISTORY_PAGE_SIZE}&offset=0`, {
    token: fixture.auth.accessToken
  });
  const firstPageItems = listItems(firstPage);
  const initialTotal = Number(firstPage?.total ?? firstPage?.count ?? firstPageItems.length);
  if (initialTotal <= ADDRESS_HISTORY_PAGE_SIZE || firstPageItems.length !== ADDRESS_HISTORY_PAGE_SIZE) {
    throw new Error(`Address history fixture did not produce a full first page: ${JSON.stringify({ initialTotal, itemCount: firstPageItems.length })}`);
  }
  const secondPage = await apiRequest(`/mall/addresses?limit=${ADDRESS_HISTORY_PAGE_SIZE}&offset=${firstPageItems.length}`, {
    token: fixture.auth.accessToken
  });
  const firstPageDetails = new Set(firstPageItems.map((item) => String(item?.detail || "").trim()).filter(Boolean));
  const loadedDetail = listItems(secondPage)
    .map((item) => String(item?.detail || "").trim())
    .find((detail) => detail && !firstPageDetails.has(detail));
  if (!loadedDetail) {
    throw new Error(`Address history second page did not contain an item outside the first page: ${JSON.stringify({ firstPageItems, secondPageItems: listItems(secondPage) })}`);
  }

  await navigate(page, `${FRONTEND_BASE}/dashboard/addresses?address_history_pagination=${Date.now()}`);
  await waitForText(page, "新增收货地址|编辑收货地址", "address history dashboard");
  await waitForButtonEnabled(page, "^加载更多$", "address history load more");
  const initialText = await bodyText(page);
  if (initialText.includes(loadedDetail)) {
    throw new Error(`Second-page address ${loadedDetail} rendered before loading the next page`);
  }
  await clickButton(page, "^加载更多$");
  await waitForText(page, loadedDetail, "address history second page");

  return {
    fixtureCount: addressFixture.count,
    initialTotal,
    loadedDetail
  };
}

async function createAddressHistoryFixture(fixture) {
  const before = listItems(await apiRequest("/mall/addresses?limit=1&offset=0", { token: fixture.auth.accessToken }));
  const defaultAddress = before.find((item) => item?.is_default || item?.isDefault);
  const defaultAddressID = defaultAddress?.id ?? defaultAddress?.Id;
  if (!defaultAddressID) {
    throw new Error(`Address history fixture requires an existing default address: ${JSON.stringify(before)}`);
  }
  for (let index = 0; index < ADDRESS_HISTORY_FIXTURE_COUNT; index += 1) {
    const suffix = String(index).padStart(2, "0");
    const created = await apiRequest("/mall/addresses", {
      method: "POST",
      token: fixture.auth.accessToken,
      body: {
        receiver: `E2E Address History ${suffix}`,
        phone: "13800000000",
        province: "上海",
        city: "上海",
        district: "浦东新区",
        detail: `E2E address history ${Date.now()}-${suffix}`,
        postal_code: "200120",
        is_default: false
      }
    });
    const address = created?.address || created;
    if (!address?.id || address?.is_default || address?.isDefault) {
      throw new Error(`Address history fixture address ${index} was not created as non-default: ${JSON.stringify(created)}`);
    }
  }
  const after = listItems(await apiRequest("/mall/addresses?limit=1&offset=0", { token: fixture.auth.accessToken }));
  const currentDefault = after.find((item) => item?.is_default || item?.isDefault);
  if (String(currentDefault?.id ?? currentDefault?.Id ?? "") !== String(defaultAddressID)) {
    throw new Error(`Address history fixture changed the default address: ${JSON.stringify({ defaultAddressID, after })}`);
  }
  return {
    count: ADDRESS_HISTORY_FIXTURE_COUNT
  };
}

async function runBrowserOrderHistoryPaginationFlow(page, fixture) {
  const firstPage = await apiRequest(`/mall/orders?limit=${ORDER_HISTORY_PAGE_SIZE}&offset=0`, {
    token: fixture.auth.accessToken
  });
  const firstPageItems = listItems(firstPage);
  const initialTotal = Number(firstPage?.total ?? firstPage?.count ?? firstPageItems.length);
  if (initialTotal <= ORDER_HISTORY_PAGE_SIZE || firstPageItems.length !== ORDER_HISTORY_PAGE_SIZE) {
    throw new Error(`Order history fixture did not produce a full first page: ${JSON.stringify({ initialTotal, itemCount: firstPageItems.length })}`);
  }
  const selectedOrderNo = firstPageItems[0]?.order_no || firstPageItems[0]?.orderNo;
  if (!selectedOrderNo) {
    throw new Error(`First order page did not return an order number: ${JSON.stringify(firstPageItems[0])}`);
  }
  const secondPage = await apiRequest(`/mall/orders?limit=${ORDER_HISTORY_PAGE_SIZE}&offset=${firstPageItems.length}`, {
    token: fixture.auth.accessToken
  });
  const firstPageOrderNos = new Set(firstPageItems.map((item) => String(item?.order_no || item?.orderNo || "")).filter(Boolean));
  const loadedOrderNo = listItems(secondPage)
    .map((item) => String(item?.order_no || item?.orderNo || ""))
    .find((orderNo) => orderNo && !firstPageOrderNos.has(orderNo));
  if (!loadedOrderNo) {
    throw new Error(`Order history second page did not contain an item outside the first page: ${JSON.stringify({ firstPageItems, secondPageItems: listItems(secondPage) })}`);
  }

  await navigate(page, `${FRONTEND_BASE}/dashboard/orders?order_history_pagination=${Date.now()}`);
  await waitForText(page, selectedOrderNo, "order history first page");
  await waitForButtonEnabled(page, "^加载更多$", "order history load more");
  const initialText = await bodyText(page);
  if (initialText.includes(loadedOrderNo)) {
    throw new Error(`Second-page order ${loadedOrderNo} rendered before loading the next page`);
  }

  await clickButtonInArticle(page, selectedOrderNo, "^订单详情$", "order history selected order detail");
  await waitForText(page, "订单详情", "order history selected detail");
  await waitForButtonEnabled(page, "^加载更多$", "order history load more after selecting detail");
  await clickButton(page, "^加载更多$");
  await waitForText(page, loadedOrderNo, "order history second page");
  await waitForText(page, "订单详情", "selected order detail retained after pagination");
  const loadedText = await bodyText(page);
  if (!loadedText.includes(selectedOrderNo)) {
    throw new Error(`Selected order ${selectedOrderNo} disappeared after loading more order history`);
  }

  return {
    initialTotal,
    loadedOrderNo,
    selectedOrderNo: String(selectedOrderNo)
  };
}

async function runBrowserEntitlementHistoryPaginationFlow(page, fixture) {
  const firstPage = await apiRequest(`/mall/digital-entitlements?status=ACTIVE&limit=${ENTITLEMENT_HISTORY_PAGE_SIZE}&offset=0`, {
    token: fixture.auth.accessToken
  });
  const firstPageItems = listItems(firstPage);
  const initialTotal = Number(firstPage?.total ?? firstPage?.count ?? firstPageItems.length);
  if (initialTotal <= ENTITLEMENT_HISTORY_PAGE_SIZE || firstPageItems.length !== ENTITLEMENT_HISTORY_PAGE_SIZE) {
    throw new Error(`Entitlement history fixture did not produce a full first page: ${JSON.stringify({ initialTotal, itemCount: firstPageItems.length })}`);
  }
  const secondPage = await apiRequest(`/mall/digital-entitlements?status=ACTIVE&limit=${ENTITLEMENT_HISTORY_PAGE_SIZE}&offset=${firstPageItems.length}`, {
    token: fixture.auth.accessToken
  });
  const firstPageCodes = new Set(firstPageItems.map((item) => String(item?.fulfillment_code || item?.fulfillmentCode || "")).filter(Boolean));
  const loadedCode = listItems(secondPage)
    .map((item) => String(item?.fulfillment_code || item?.fulfillmentCode || ""))
    .find((code) => code && !firstPageCodes.has(code));
  if (!loadedCode) {
    throw new Error(`Entitlement history second page did not contain an item outside the first page: ${JSON.stringify({ firstPageItems, secondPageItems: listItems(secondPage) })}`);
  }

  await navigate(page, `${FRONTEND_BASE}/dashboard/entitlements?entitlement_history_pagination=${Date.now()}`);
  await waitForText(page, "个人列表|数字权益|权益", "entitlement history dashboard");
  await waitForButtonEnabled(page, "^加载更多$", "entitlement history load more");
  const initialText = await bodyText(page);
  if (initialText.includes(loadedCode)) {
    throw new Error(`Second-page entitlement ${loadedCode} rendered before loading the next page`);
  }
  await clickButton(page, "^加载更多$");
  await waitForText(page, loadedCode, "entitlement history second page");

  return {
    initialTotal,
    loadedCode
  };
}

async function runBrowserPayingOrderResumeFlow(page, fixture) {
  const product = fixture.dashboardPayProduct;
  const initialStock = await currentMallProductStock(product.id);
  const orderData = await apiRequest("/mall/orders", {
    method: "POST",
    token: fixture.auth.accessToken,
    body: {
      idempotency_key: `web-dashboard-resume-order-${Date.now()}`,
      receiver: "浏览器联调支付续办",
      phone: "13500000000",
      address: "上海 上海 浦东新区 续办路 1 号",
      items: [{ product_id: product.id, quantity: 1 }]
    }
  });
  const order = orderData?.order || orderData;
  if (!order?.id) {
    throw new Error("Dashboard payment resume order was not returned by order API");
  }
  if (mallOrderStatusValue(order.status) !== 1) {
    throw new Error(`Dashboard payment resume order was not pending: ${JSON.stringify(order)}`);
  }
  const orderNo = order.order_no || order.orderNo || String(order.id);
  const originalPaymentKey = `web-dashboard-resume-original-${order.id}-${Date.now()}`;
  await markOrderPayingForBrowserResume(order, fixture.auth.user.id, originalPaymentKey);
  await waitForMallOrderStatus(fixture, order.id, 2, "dashboard payment resume order paying");
  await waitForMallProductStock(product.id, initialStock - 1, "dashboard payment resume product stock locked");

  await navigate(page, `${FRONTEND_BASE}/dashboard/orders?order_id=${encodeURIComponent(order.id)}&paying_resume=${Date.now()}`);
  await waitForText(page, "个人工作台", "dashboard payment resume shell");
  await waitForText(page, orderNo, "dashboard payment resume order number");
  await waitForText(page, "支付中", "dashboard payment resume paying status");
  await waitForText(page, "继续支付", "dashboard payment resume action");
  await clickButtonInArticle(page, orderNo, "^继续支付$");
  await waitForText(page, "订单已支付，积分流水已同步。|已支付", "dashboard payment resume success");

  await waitForMallOrderStatus(fixture, order.id, 3, "dashboard payment resume order paid");
  const payment = await waitForMallOrderPaymentStatus(fixture, order.id, 2, "dashboard payment resume succeeded payment");
  const paymentKey = String(payment.idempotency_key ?? payment.idempotencyKey ?? "").trim();
  if (paymentKey !== originalPaymentKey) {
    throw new Error(`Dashboard payment resume used idempotency key ${paymentKey}, want ${originalPaymentKey}`);
  }
  const payments = listItems(await apiRequest(`/mall/orders/${encodeURIComponent(order.id)}/payments`, {
    token: fixture.auth.accessToken
  }));
  if (payments.length !== 1) {
    throw new Error(`Dashboard payment resume created ${payments.length} payment records, want one: ${JSON.stringify(payments)}`);
  }
  const debitEntries = await creditLedgerEntriesForSource(
    fixture.auth.accessToken,
    `mall.order.pay:${order.id}:${originalPaymentKey}`,
    "mall_order_paid"
  );
  if (debitEntries.length !== 1 || creditLedgerDelta(debitEntries[0]) !== -CHECKOUT_PRICE) {
    throw new Error(`Dashboard payment resume debit ledger mismatch: ${JSON.stringify(debitEntries)}`);
  }
  await waitForMallProductStock(product.id, initialStock - 1, "dashboard payment resume product stock remains locked");

  return {
    orderId: String(order.id),
    paymentKey,
    ledgerCount: debitEntries.length
  };
}

async function runBrowserInsufficientCreditRecoveryFlow(page, fixture, expectedBrowserIssues = []) {
  const product = fixture.insufficientCreditProduct;
  const initialStock = await currentMallProductStock(product.id);
  const balanceBeforeFailure = await currentCreditBalance(fixture);
  if (balanceBeforeFailure >= INSUFFICIENT_CHECKOUT_PRICE) {
    throw new Error(`Insufficient-credit fixture balance=${balanceBeforeFailure}, want below ${INSUFFICIENT_CHECKOUT_PRICE}`);
  }

  const orderData = await apiRequest("/mall/orders", {
    method: "POST",
    token: fixture.auth.accessToken,
    body: {
      idempotency_key: `web-insufficient-credit-${Date.now()}`,
      receiver: "浏览器联调余额不足",
      phone: "13700000000",
      address: "上海 上海 浦东新区 余额路 1 号",
      items: [{ product_id: product.id, quantity: 1 }]
    }
  });
  const order = orderData?.order || orderData;
  if (!order?.id) {
    throw new Error("Insufficient-credit mall order was not returned by order API");
  }
  const orderStatus = mallOrderStatusValue(order.status);
  const totalCredits = Number(order.total_credits ?? order.totalCredits ?? 0);
  if (orderStatus !== 1 || totalCredits !== INSUFFICIENT_CHECKOUT_PRICE) {
    throw new Error(`Insufficient-credit order snapshot mismatch: ${JSON.stringify({ orderStatus, totalCredits })}`);
  }
  const orderNo = order.order_no || order.orderNo || String(order.id);
  const lockedStock = await waitForMallProductStock(product.id, initialStock - 1, "insufficient-credit product stock locked by pending order");

  await navigate(page, `${FRONTEND_BASE}/dashboard/orders?order_id=${encodeURIComponent(order.id)}`);
  await waitForText(page, "个人工作台", "insufficient-credit dashboard shell");
  await waitForText(page, orderNo, "insufficient-credit order number");
  await waitForText(page, product.title, "insufficient-credit order item title");
  await waitForText(page, "待支付", "insufficient-credit pending order status");
  expectBrowserHttpFailure(expectedBrowserIssues, `${API_BASE}/mall/orders/${encodeURIComponent(order.id)}/pay`, 412);
  await clickButtonInArticle(page, orderNo, "^继续支付$");
  await waitForText(page, "积分不足|订单支付失败", "insufficient-credit payment failure notice");

  const failedPayment = await waitForMallOrderPaymentStatus(fixture, order.id, 3, "insufficient-credit failed payment");
  await waitForMallOrderStatus(fixture, order.id, 1, "insufficient-credit order remains pending after failed payment");
  await navigate(page, `${FRONTEND_BASE}/dashboard/orders?order_id=${encodeURIComponent(order.id)}&payment_failed=${Date.now()}`);
  await waitForText(page, "支付记录|支付失败", "insufficient-credit failed payment record");
  await waitForText(page, "支付失败", "insufficient-credit failed payment status");
  await waitForText(page, "继续支付", "insufficient-credit retry action");

  const balanceAfterTopUp = await topUpUserCredits(fixture, PAYMENT_RECOVERY_TOP_UP, `browser-mall-payment-recovery-${Date.now()}`);
  if (balanceAfterTopUp < INSUFFICIENT_CHECKOUT_PRICE) {
    throw new Error(`Payment recovery top-up balance=${balanceAfterTopUp}, want at least ${INSUFFICIENT_CHECKOUT_PRICE}`);
  }

  await clickButtonInArticle(page, orderNo, "^继续支付$");
  await waitForText(page, "订单已支付，积分流水已同步。|已支付", "insufficient-credit recovered payment success");
  const paidOrder = await waitForMallOrderStatus(fixture, order.id, 3, "insufficient-credit order paid after recovery");
  const recoveredPayment = await waitForMallOrderPaymentStatus(fixture, order.id, 2, "insufficient-credit recovered payment");
  const notifications = await waitForMallOrderNotifications(fixture, order.id, ["订单已支付"]);
  await waitForMallProductStock(product.id, initialStock - 1, "insufficient-credit product stock remains locked after recovered payment");
  await navigate(page, `${FRONTEND_BASE}/dashboard/orders?order_id=${encodeURIComponent(order.id)}&payment_recovered=${Date.now()}`);
  await waitForText(page, "支付记录|支付成功", "insufficient-credit recovered payment record");
  await waitForText(page, product.title, "recovered insufficient-credit item title");

  return {
    orderId: String(order.id),
    orderNo: paidOrder.order_no || paidOrder.orderNo || orderNo,
    lockedStock,
    balanceBeforeFailure,
    balanceAfterTopUp,
    failedPaymentId: String(failedPayment.id || failedPayment.ID || ""),
    recoveredPaymentId: String(recoveredPayment.id || recoveredPayment.ID || ""),
    text: summarizeInsufficientPaymentRecoveryText(await bodyText(page)),
    notificationTitles: notifications.map((item) => item.title || item.type || "").filter(Boolean)
  };
}

async function runBrowserCouponCancellationFlow(page, fixture) {
  const product = fixture.cancelCouponProduct;
  const coupon = fixture.cancelCoupon;
  const shopUrl = `${FRONTEND_BASE}/shop?product_id=${encodeURIComponent(product.id)}&coupon_code=${encodeURIComponent(coupon.code)}`;
  const initialStock = await currentMallProductStock(product.id);

  await navigate(page, shopUrl);
  await waitForText(page, product.title, "cancel coupon product detail");
  await waitForText(page, coupon.code, "cancel coupon visible in shop");
  await clickButtonInArticle(page, coupon.code, "^领取$");
  await waitForText(page, "优惠券已领取|已经在你的券包里", "cancel coupon claimed");
  await waitForCouponUsageStatus(fixture, coupon.id, 4, "", "cancel coupon claimed usage");

  const orderData = await apiRequest("/mall/orders", {
    method: "POST",
    token: fixture.auth.accessToken,
    body: {
      idempotency_key: `web-cancel-coupon-${Date.now()}`,
      coupon_code: coupon.code,
      receiver: "浏览器联调取消",
      phone: "13600000000",
      address: "上海 上海 浦东新区 取消路 1 号",
      items: [{ product_id: product.id, quantity: 1 }]
    }
  });
  const order = orderData?.order || orderData;
  if (!order?.id) {
    throw new Error("Cancel coupon mall order was not returned by order API");
  }
  const orderStatus = mallOrderStatusValue(order.status);
  const discountCredits = Number(order.discount_credits ?? order.discountCredits ?? 0);
  const couponCode = order.coupon_code || order.couponCode || "";
  if (orderStatus !== 1 || discountCredits !== COUPON_DISCOUNT || couponCode !== coupon.code) {
    throw new Error(`Cancel coupon order snapshot mismatch: ${JSON.stringify({ orderStatus, discountCredits, couponCode })}`);
  }
  const orderNo = order.order_no || order.orderNo || String(order.id);
  await waitForCouponUsageStatus(fixture, coupon.id, 1, order.id, "cancel coupon reserved usage");
  const lockedStock = await waitForMallProductStock(product.id, initialStock - 1, "cancel coupon product stock locked by unpaid order");

  await navigate(page, `${FRONTEND_BASE}/dashboard/orders?order_id=${encodeURIComponent(order.id)}`);
  await waitForText(page, "个人工作台", "cancel coupon dashboard shell");
  await waitForText(page, orderNo, "cancel coupon order number");
  await waitForText(page, product.title, "cancel coupon order item title");
  await waitForText(page, "待支付", "cancel coupon pending order status");
  await waitForText(page, coupon.code, "cancel coupon order discount");
  await clickButtonInArticle(page, orderNo, "^取消订单$");
  await waitForText(page, "订单已取消|已取消", "cancel coupon order canceled");

  await waitForMallOrderStatus(fixture, order.id, 4, "cancel coupon order canceled in API");
  const restoredStock = await waitForMallProductStock(product.id, initialStock, "cancel coupon product stock restored after cancel");
  const releasedUsage = await waitForCouponUsageStatus(fixture, coupon.id, 3, order.id, "cancel coupon released usage");

  await navigate(page, `${FRONTEND_BASE}/dashboard/coupons?status=3&order_id=${encodeURIComponent(order.id)}`);
  await waitForText(page, "优惠券|个人工作台", "released coupon dashboard");
  await waitForText(page, coupon.code, "released coupon code in dashboard");
  await waitForText(page, "已释放", "released coupon status in dashboard");
  await waitForText(page, `订单 #${order.id}`, "released coupon order reference");

  await navigate(page, `${shopUrl}&reclaim=${Date.now()}`);
  await waitForText(page, product.title, "cancel coupon product detail after release");
  await waitForText(page, coupon.code, "cancel coupon visible for reclaim");
  await clickButtonInArticle(page, coupon.code, "^领取$");
  await waitForText(page, "优惠券已领取|已经在你的券包里", "cancel coupon reclaimed after release");
  const reclaimedUsage = await waitForCouponUsageStatus(fixture, coupon.id, 4, "", "cancel coupon reclaimed usage");

  return {
    orderId: String(order.id),
    orderNo,
    lockedStock,
    restoredStock,
    releasedUsageId: String(releasedUsage.id || releasedUsage.ID || ""),
    reclaimedUsageId: String(reclaimedUsage.id || reclaimedUsage.ID || ""),
    text: summarizeCouponCancellationText(await bodyText(page))
  };
}

async function runBrowserCouponArchiveFlow(page, fixture) {
  const product = fixture.cancelCouponProduct;
  const coupon = fixture.cancelCoupon;
  const archived = await apiRequest(`/admin/mall/coupons/${encodeURIComponent(coupon.id)}`, {
    method: "PUT",
    token: fixture.adminToken,
    body: {
      code: coupon.code,
      name: coupon.name,
      description: coupon.description,
      discount_credits: Number(coupon.discount_credits ?? coupon.discountCredits ?? 0),
      min_order_credits: Number(coupon.min_order_credits ?? coupon.minOrderCredits ?? 0),
      total_quota: Number(coupon.total_quota ?? coupon.totalQuota ?? 0),
      per_user_limit: Number(coupon.per_user_limit ?? coupon.perUserLimit ?? 0),
      status: 3,
      starts_at: Number(coupon.starts_at ?? coupon.startsAt ?? 0),
      ends_at: Number(coupon.ends_at ?? coupon.endsAt ?? 0)
    }
  });
  const archivedCoupon = archived?.coupon || archived;
  if (Number(archivedCoupon?.status ?? archivedCoupon?.Status) !== 3) {
    throw new Error(`Archived coupon did not return archived status: ${JSON.stringify(archivedCoupon)}`);
  }

  const usage = await waitForCouponUsageStatus(fixture, coupon.id, 4, "", "archived coupon claimed usage");
  const usageCoupon = usage?.coupon || usage?.Coupon || {};
  if (Number(usageCoupon?.status ?? usageCoupon?.Status) !== 3) {
    throw new Error(`Archived coupon was not reflected in the user coupon wallet: ${JSON.stringify(usage)}`);
  }

  const shopUrl = `${FRONTEND_BASE}/shop?product_id=${encodeURIComponent(product.id)}&coupon_code=${encodeURIComponent(coupon.code)}&archived_coupon=${Date.now()}`;
  await navigate(page, shopUrl);
  await waitForText(page, product.title, "archived coupon product detail");
  await waitForText(page, "该优惠券已失效", "archived coupon customer state");
  await clickButton(page, "^立即兑换$");
  await waitForText(page, "确认兑换", "archived coupon checkout panel");
  await waitFor(
    page,
    `Array.from(document.querySelectorAll("button")).some((button) => (button.innerText || button.textContent || "").trim() === "确认兑换" && button.disabled)`,
    "archived coupon checkout disabled"
  );

  const rejected = await apiRequestFailure("/mall/orders", {
    method: "POST",
    token: fixture.auth.accessToken,
    expectedStatus: 412,
    label: "archived coupon checkout",
    body: {
      idempotency_key: `archived-coupon-${Date.now()}`,
      coupon_code: coupon.code,
      receiver: "浏览器联调归档",
      phone: "13700000000",
      address: "上海 上海 浦东新区 归档路 1 号",
      items: [{ product_id: product.id, quantity: 1 }]
    }
  });
  const combined = `${rejected.message} ${rejected.rawBody}`.toLowerCase();
  if (!combined.includes("coupon unavailable")) {
    throw new Error(`Archived coupon checkout rejection mismatch: ${rejected.rawBody.slice(0, 800)}`);
  }

  const publicCoupons = listItems(await apiRequest("/mall/coupons?limit=100&offset=0"));
  if (publicCoupons.some((item) => String(item?.id ?? item?.Id) === String(coupon.id))) {
    throw new Error(`Archived coupon is still listed for public claim: ${JSON.stringify(publicCoupons.slice(0, 20))}`);
  }

  return {
    status: Number(archivedCoupon.status ?? archivedCoupon.Status),
    rejectStatus: rejected.status,
    rejectText: rejected.message || "coupon unavailable"
  };
}

async function runBrowserCartCheckout(page, fixture) {
  const shopUrl = `${FRONTEND_BASE}/shop?product_id=${encodeURIComponent(fixture.cartProduct.id)}`;
  await navigate(page, shopUrl);
  await waitForText(page, fixture.cartProduct.title, "cart product detail");
  await waitForText(page, "商品详情", "cart product detail panel");
  await clickButtonNearText(page, fixture.cartProduct.title, "^加购物车$");
  await waitForText(page, "商品已加入购物车|购物车", "cart item added");
  await waitForText(page, fixture.cartProduct.title, "cart item title");
  await waitForButtonEnabled(page, "^结算购物车$", "cart checkout enabled");
  await clickButton(page, "^结算购物车$");
  await waitForText(page, "确认兑换", "cart checkout panel");
  await waitForText(page, "x 1", "cart checkout quantity");
  await clickButton(page, "^确认兑换$");
  await waitForText(page, "兑换成功|订单已创建", "cart order paid");
  const paidText = await bodyText(page);

  const cartOrder = await latestMallOrderForProduct(fixture, fixture.cartProduct.id);
  if (!cartOrder?.id) {
    throw new Error("Cart mall order was not returned by user order API");
  }
  const cartItems = await apiRequest("/mall/cart", {
    token: fixture.auth.accessToken
  });
  if (listItems(cartItems).some((item) => String(item?.product_id ?? item?.productId ?? item?.product?.id ?? "") === String(fixture.cartProduct.id))) {
    throw new Error("Cart checkout did not clear purchased cart item");
  }

  const notifications = await waitForMallOrderNotifications(fixture, cartOrder.id, ["订单已支付"]);
  await navigate(page, `${FRONTEND_BASE}/dashboard/orders?order_id=${encodeURIComponent(cartOrder.id)}`);
  await waitForText(page, "个人工作台", "cart dashboard shell");
  await waitForText(page, fixture.cartProduct.title, "cart order item title");
  await waitForText(page, "已支付", "cart paid order row");
  await waitForText(page, "支付记录|支付成功", "cart payment evidence");

  return {
    orderId: String(cartOrder.id),
    orderNo: cartOrder.order_no || cartOrder.orderNo || "",
    cartText: summarizeCartText(paidText),
    notificationTitles: notifications.map((item) => item.title || item.type || "").filter(Boolean)
  };
}

async function runBrowserRefundFlow(page, fixture) {
  const refundNote = `浏览器联调售后 ${Date.now()}：验证用户申请、运营审核和退款通知链路。`;
  const adminNote = `Browser E2E refund approved ${Date.now()}`;
  const initialStock = await currentMallProductStock(fixture.refundProduct.id);
  const shopUrl = `${FRONTEND_BASE}/shop?product_id=${encodeURIComponent(fixture.refundProduct.id)}`;
  await navigate(page, shopUrl);
  await waitForText(page, fixture.refundProduct.title, "refund product detail");
  await waitForText(page, "商品详情", "refund product detail panel");
  await clickButton(page, "^立即兑换$");
  await waitForText(page, "确认兑换", "refund checkout panel");
  await clickButton(page, "^确认兑换$");
  await waitForText(page, "兑换成功|订单已创建", "refund order paid");

  const order = await latestMallOrderForProduct(fixture, fixture.refundProduct.id);
  if (!order?.id) {
    throw new Error("Refund mall order was not returned by user order API");
  }
  const orderNo = order.order_no || order.orderNo || String(order.id);
  const orderedQuantity = orderQuantityForProduct(order, fixture.refundProduct.id);
  if (orderedQuantity !== 1) {
    throw new Error(`Refund mall order quantity = ${orderedQuantity}, want 1`);
  }
  const lockedStock = await waitForMallProductStock(fixture.refundProduct.id, initialStock - orderedQuantity, "refund product stock locked by order");
  await shipMallOrder(fixture, order.id);

  await clickButton(page, "查看订单");
  await waitForText(page, "个人工作台", "refund dashboard shell");
  await waitForText(page, "已发货", "refund shipped order row");
  await waitForText(page, fixture.refundProduct.title, "refund order item title");
  await waitForText(page, "申请售后", "refund action");
  await clickButton(page, "^申请售后$");
  await waitForText(page, "售后原因", "refund request form");
  await fillByLabel(page, "问题说明", refundNote);
  await waitForButtonEnabled(page, "^提交申请$", "refund submit enabled");
  await clickButton(page, "^提交申请$");
  await waitForText(page, "售后申请已提交|售后待审核", "refund request submitted");
  await assertButtonAbsentInArticle(page, orderNo, "^确认收货$", "refund pending confirmation action hidden in row");
  const confirmFailure = await apiRequestFailure(`/mall/orders/${encodeURIComponent(order.id)}/confirm`, {
    method: "POST",
    token: fixture.auth.accessToken,
    expectedStatus: 412,
    label: "refund pending order confirmation"
  });
  const confirmFailureText = `${confirmFailure.message} ${confirmFailure.rawBody}`.toLowerCase();
  if (!confirmFailureText.includes("invalid order state")) {
    throw new Error(`Refund pending order confirmation did not return invalid state: ${confirmFailure.rawBody.slice(0, 800)}`);
  }

  const refund = await latestRefundForOrder(fixture, order.id);
  if (!refund?.id) {
    throw new Error(`Refund request was not returned for order ${order.id}`);
  }

  await approveMallRefund(fixture, refund.id, adminNote);
  const refundAmount = Number(refund.amount_credits ?? refund.amountCredits ?? CHECKOUT_PRICE);
  if (!Number.isFinite(refundAmount) || refundAmount <= 0) {
    throw new Error(`Refund ${refund.id} did not expose a positive amount: ${JSON.stringify(refund)}`);
  }
  const creditLedger = await assertRefundCreditLedgerIdempotent(fixture, refund.id, refundAmount, adminNote);
  const restoredStock = await waitForMallProductStock(fixture.refundProduct.id, initialStock, "refund product stock restored after refund");
  const notifications = await waitForMallOrderNotifications(fixture, order.id, ["售后退款已通过"]);

  await navigate(page, `${FRONTEND_BASE}/dashboard/refunds`);
  await waitForText(page, "待审核|处理中|已拒绝", "refunds panel");
  await waitForText(page, orderNo, "approved refund order number");
  await waitForText(page, "已退款", "approved refund status");
  await waitForText(page, adminNote, "approved refund admin note");
  const refundText = await bodyText(page);

  await navigate(page, `${FRONTEND_BASE}/dashboard/orders?order_id=${encodeURIComponent(order.id)}`);
  await waitForText(page, "已退款", "refunded order status");
  await waitForText(page, "售后进度", "refunded order detail");
  await navigate(page, `${FRONTEND_BASE}/dashboard/messages`);
  await waitForText(page, "售后退款已通过", "refund notification title");
  await waitForText(page, orderNo, "refund notification content");
  await navigate(page, shopUrl);
  await waitForText(page, fixture.refundProduct.title, "refund product detail after stock restore");
  await waitForText(page, `库存\\s*${restoredStock}`, "restored refund product stock in storefront");

  return {
    orderId: String(order.id),
    orderNo,
    lockedStock,
    restoredStock,
    refundText: summarizeRefundText(refundText),
    confirmActionHidden: true,
    confirmApiStatus: confirmFailure.status,
    confirmApiMessage: confirmFailure.message || "invalid order state",
    notificationTitles: notifications.map((item) => item.title || item.type || "").filter(Boolean),
    creditLedgerId: creditLedger.ledgerId,
    creditLedgerSourceEventId: creditLedger.sourceEventId,
    creditLedgerCountAfterRetry: creditLedger.countAfterRetry,
    balanceAfterRetry: creditLedger.balanceAfterRetry
  };
}

async function runBrowserRejectedRefundFlow(page, fixture) {
  const refundNote = `浏览器联调拒绝售后 ${Date.now()}：验证运营拒绝后用户能看到审核原因。`;
  const replacementRefundNote = `浏览器联调重新申请售后 ${Date.now()}：验证撤回后可再次提交。`;
  const adminNote = `Browser E2E refund rejected ${Date.now()} - duplicate or unsupported reason`;
  const initialStock = await currentMallProductStock(fixture.rejectedRefundProduct.id);
  const shopUrl = `${FRONTEND_BASE}/shop?product_id=${encodeURIComponent(fixture.rejectedRefundProduct.id)}`;
  await navigate(page, shopUrl);
  await waitForText(page, fixture.rejectedRefundProduct.title, "rejected refund product detail");
  await waitForText(page, "商品详情", "rejected refund product detail panel");
  await clickButton(page, "^立即兑换$");
  await waitForText(page, "确认兑换", "rejected refund checkout panel");
  await clickButton(page, "^确认兑换$");
  await waitForText(page, "兑换成功|订单已创建", "rejected refund order paid");

  const order = await latestMallOrderForProduct(fixture, fixture.rejectedRefundProduct.id);
  if (!order?.id) {
    throw new Error("Rejected refund mall order was not returned by user order API");
  }
  const orderNo = order.order_no || order.orderNo || String(order.id);
  const orderedQuantity = orderQuantityForProduct(order, fixture.rejectedRefundProduct.id);
  if (orderedQuantity !== 1) {
    throw new Error(`Rejected refund mall order quantity = ${orderedQuantity}, want 1`);
  }
  const lockedStock = await waitForMallProductStock(
    fixture.rejectedRefundProduct.id,
    initialStock - orderedQuantity,
    "rejected refund product stock locked by order"
  );
  const balanceAfterCheckout = await currentCreditBalance(fixture);
  await shipMallOrder(fixture, order.id);

  await clickButton(page, "查看订单");
  await waitForText(page, "个人工作台", "rejected refund dashboard shell");
  await waitForText(page, "已发货", "rejected refund shipped order row");
  await waitForText(page, fixture.rejectedRefundProduct.title, "rejected refund order item title");
  await waitForText(page, "申请售后", "rejected refund action");
  await clickButton(page, "^申请售后$");
  await waitForText(page, "售后原因", "rejected refund request form");
  await fillByLabel(page, "问题说明", refundNote);
  await waitForButtonEnabled(page, "^提交申请$", "rejected refund submit enabled");
  await clickButton(page, "^提交申请$");
  await waitForText(page, "售后申请已提交|售后待审核", "rejected refund request submitted");

  const refund = await latestRefundForOrder(fixture, order.id);
  if (!refund?.id) {
    throw new Error(`Rejected refund request was not returned for order ${order.id}`);
  }

  const cancelBalanceBefore = await currentCreditBalance(fixture);
  const cancelStockBefore = await currentMallProductStock(fixture.rejectedRefundProduct.id);
  if (cancelBalanceBefore !== balanceAfterCheckout) {
    throw new Error(`Refund request changed balance from ${balanceAfterCheckout} to ${cancelBalanceBefore} before cancellation`);
  }
  if (cancelStockBefore !== lockedStock) {
    throw new Error(`Refund request changed stock from ${lockedStock} to ${cancelStockBefore} before cancellation`);
  }
  await waitForText(page, "撤回申请", "refund cancellation action");
  await clickButtonInArticle(page, orderNo, "^撤回申请$");
  await waitForText(page, "售后申请已撤回|售后已撤回", "refund cancellation completed");
  const canceledRefund = await waitForRefundStatus(fixture, refund.id, 5, "refund canceled by user");
  const cancelBalanceAfter = await currentCreditBalance(fixture);
  const cancelStockAfter = await currentMallProductStock(fixture.rejectedRefundProduct.id);
  if (cancelBalanceAfter !== cancelBalanceBefore) {
    throw new Error(`Refund cancellation changed balance from ${cancelBalanceBefore} to ${cancelBalanceAfter}`);
  }
  if (cancelStockAfter !== cancelStockBefore) {
    throw new Error(`Refund cancellation changed stock from ${cancelStockBefore} to ${cancelStockAfter}`);
  }

  await waitForText(page, "申请售后", "refund reapplication action");
  await clickButtonInArticle(page, orderNo, "^申请售后$");
  await waitForText(page, "售后原因", "replacement refund request form");
  await fillByLabel(page, "问题说明", replacementRefundNote);
  await waitForButtonEnabled(page, "^提交申请$", "replacement refund submit enabled");
  await clickButton(page, "^提交申请$");
  await waitForText(page, "售后申请已提交|售后待审核", "replacement refund request submitted");
  const replacementRefund = await waitForNewRefundForOrder(fixture, order.id, refund.id, "replacement refund request");
  if (refundStatusValue(replacementRefund.status) !== 1) {
    throw new Error(`Replacement refund ${replacementRefund.id} status = ${replacementRefund.status}, want requested`);
  }
  if (String(replacementRefund.id) === String(canceledRefund.id)) {
    throw new Error(`Replacement refund id = ${replacementRefund.id}, want a new request after cancellation`);
  }

  await rejectMallRefund(fixture, replacementRefund.id, adminNote);
  const notifications = await waitForMallOrderNotifications(fixture, order.id, ["售后申请已拒绝"]);

  await navigate(page, `${FRONTEND_BASE}/dashboard/orders?order_id=${encodeURIComponent(order.id)}`);
  await waitForText(page, "已发货", "rejected refund order resumed after rejection");
  await waitForText(page, "确认收货", "rejected refund confirmation action restored");
  await clickButtonInArticle(page, orderNo, "^确认收货$");
  await waitForText(page, "已确认收货，订单已完成。|已完成", "rejected refund order confirmed");
  const completedOrder = await waitForMallOrderStatus(fixture, order.id, 6, "rejected refund order completed after confirmation");

  await navigate(page, `${FRONTEND_BASE}/dashboard/refunds`);
  await waitForText(page, "待审核|处理中|已拒绝", "refunds panel for rejected refund");
  await waitForText(page, orderNo, "rejected refund order number");
  await waitForText(page, "售后已拒绝", "rejected refund status");
  await waitForText(page, adminNote, "rejected refund admin note");
  const refundText = await bodyText(page);

  await navigate(page, `${FRONTEND_BASE}/dashboard/messages`);
  await waitForText(page, "售后申请已拒绝", "rejected refund notification title");
  await waitForText(page, orderNo, "rejected refund notification content");
  await waitForText(page, adminNote, "rejected refund notification admin note");
  await clickButtonInArticle(page, "售后申请已拒绝", "查看售后");
  await waitForText(page, "个人工作台", "rejected refund notification target shell");
  await waitForText(page, "个人列表|售后", "rejected refund notification target");
  await waitForText(page, "当前定位", "rejected refund focused marker");
  await waitForText(page, orderNo, "focused rejected refund order number");
  await waitForText(page, "售后已拒绝", "focused rejected refund status");
  await waitForText(page, adminNote, "focused rejected refund admin note");

  return {
    orderId: String(order.id),
    orderNo,
    refundId: String(replacementRefund.id),
    canceledRefundId: String(canceledRefund.id),
    replacementRefundId: String(replacementRefund.id),
    cancelBalanceBefore,
    cancelBalanceAfter,
    cancelStockBefore,
    cancelStockAfter,
    orderStatusAfterRejection: mallOrderStatusValue(completedOrder.status),
    refundText: summarizeRejectedRefundText(refundText),
    notificationTitles: notifications.map((item) => item.title || item.type || "").filter(Boolean)
  };
}

async function runBrowserReviewHistoryPaginationFlow(page, fixture) {
  const firstPage = await apiRequest(`/mall/reviews?limit=${REVIEW_HISTORY_PAGE_SIZE}&offset=0`, {
    token: fixture.auth.accessToken
  });
  const firstPageItems = listItems(firstPage);
  const initialTotal = Number(firstPage?.total ?? firstPage?.count ?? firstPageItems.length);
  if (initialTotal <= REVIEW_HISTORY_PAGE_SIZE || firstPageItems.length !== REVIEW_HISTORY_PAGE_SIZE) {
    throw new Error(`Review history fixture did not produce a full first page: ${JSON.stringify({ initialTotal, itemCount: firstPageItems.length })}`);
  }
  const secondPage = await apiRequest(`/mall/reviews?limit=${REVIEW_HISTORY_PAGE_SIZE}&offset=${firstPageItems.length}`, {
    token: fixture.auth.accessToken
  });
  const firstPageContents = new Set(firstPageItems.map((item) => String(item?.content || "").trim()).filter(Boolean));
  const loadedContent = listItems(secondPage)
    .map((item) => String(item?.content || "").trim())
    .find((content) => content && !firstPageContents.has(content));
  if (!loadedContent) {
    throw new Error(`Review history second page did not contain an item outside the first page: ${JSON.stringify({ firstPageItems, secondPageItems: listItems(secondPage) })}`);
  }

  await navigate(page, `${FRONTEND_BASE}/dashboard/reviews?review_history_pagination=${Date.now()}`);
  await waitForText(page, "个人列表|评价", "review history dashboard");
  await waitForButtonEnabled(page, "^加载更多$", "review history load more");
  const initialText = await bodyText(page);
  if (initialText.includes(loadedContent)) {
    throw new Error(`Second-page review ${loadedContent} rendered before loading the next page`);
  }
  await clickButton(page, "^加载更多$");
  await waitForText(page, loadedContent, "review history second page");

  return {
    initialTotal,
    loadedContent
  };
}

async function runBrowserRefundHistoryPaginationFlow(page, fixture) {
  const firstPage = await apiRequest(`/mall/refunds?limit=${REFUND_HISTORY_PAGE_SIZE}&offset=0`, {
    token: fixture.auth.accessToken
  });
  const firstPageItems = listItems(firstPage);
  const initialTotal = Number(firstPage?.total ?? firstPage?.count ?? firstPageItems.length);
  if (initialTotal <= REFUND_HISTORY_PAGE_SIZE || firstPageItems.length !== REFUND_HISTORY_PAGE_SIZE) {
    throw new Error(`Refund history fixture did not produce a full first page: ${JSON.stringify({ initialTotal, itemCount: firstPageItems.length })}`);
  }
  const secondPage = await apiRequest(`/mall/refunds?limit=${REFUND_HISTORY_PAGE_SIZE}&offset=${firstPageItems.length}`, {
    token: fixture.auth.accessToken
  });
  const firstPageNotes = new Set(firstPageItems.map((item) => String(item?.user_note || item?.userNote || "").trim()).filter(Boolean));
  const loadedNote = listItems(secondPage)
    .map((item) => String(item?.user_note || item?.userNote || "").trim())
    .find((note) => note && !firstPageNotes.has(note));
  if (!loadedNote) {
    throw new Error(`Refund history second page did not contain an item outside the first page: ${JSON.stringify({ firstPageItems, secondPageItems: listItems(secondPage) })}`);
  }

  await navigate(page, `${FRONTEND_BASE}/dashboard/refunds?refund_history_pagination=${Date.now()}`);
  await waitForText(page, "个人列表|售后", "refund history dashboard");
  await waitForButtonEnabled(page, "^加载更多$", "refund history load more");
  const initialText = await bodyText(page);
  if (initialText.includes(loadedNote)) {
    throw new Error(`Second-page refund ${loadedNote} rendered before loading the next page`);
  }
  await clickButton(page, "^加载更多$");
  await waitForText(page, loadedNote, "refund history second page");

  return {
    initialTotal,
    loadedNote
  };
}

async function runBrowserDigitalEntitlementFlow(page, fixture, expectedBrowserIssues = []) {
  const duplicateBadgeQuantity = 2;
  const digitalQuantity = 1;
  const refundNote = `浏览器联调数字权益售后 ${Date.now()}：验证退款后权益撤销。`;
  const adminNote = `Browser E2E digital refund approved ${Date.now()}`;
  const shopUrl = `${FRONTEND_BASE}/shop?product_id=${encodeURIComponent(fixture.digitalProduct.id)}`;
  await navigate(page, shopUrl);
  await waitForText(page, fixture.digitalProduct.title, "digital product detail");
  await waitForText(page, "商品详情", "digital product detail panel");
  await clickButtonNearText(page, fixture.digitalProduct.title, "^加购物车$");
  await waitForText(page, "商品已加入购物车", "digital badge cart item added");
  await waitForButtonNearTextEnabled(page, fixture.digitalProduct.title, "^加购物车$", "digital badge add-to-cart ready");
  const stopExpectingDuplicateBadgeCartFailure = expectBrowserHttpFailure(expectedBrowserIssues, `${API_BASE}/mall/cart/items/${encodeURIComponent(fixture.digitalProduct.id)}`, 412);
  try {
    await clickButtonNearText(page, fixture.digitalProduct.title, "^加购物车$");
    await waitForText(page, "同一徽章权益每次只能兑换一份", "duplicate badge cart quantity error");
    await delay(250);
  } finally {
    stopExpectingDuplicateBadgeCartFailure();
  }
  await clickButton(page, "^立即兑换$");
  await waitForText(page, "确认兑换", "digital checkout panel");
  await waitForText(page, "徽章权益在线发放，无需收货地址|数字权益在线发放，无需收货地址", "digital checkout fulfillment hint");
  await fillByLabel(page, "数量", String(duplicateBadgeQuantity));
  await waitForText(page, `${CHECKOUT_PRICE * duplicateBadgeQuantity} 积分`, "duplicate badge checkout quantity total");
  const stopExpectingDuplicateBadgeCheckoutFailure = expectBrowserHttpFailure(expectedBrowserIssues, `${API_BASE}/mall/orders`, 412);
  try {
    await clickButton(page, "^确认兑换$");
    await waitForText(page, "同一徽章权益每次只能兑换一份", "duplicate badge quantity checkout error");
    await delay(250);
  } finally {
    stopExpectingDuplicateBadgeCheckoutFailure();
  }
  await fillByLabel(page, "数量", String(digitalQuantity));
  await waitForText(page, `${CHECKOUT_PRICE * digitalQuantity} 积分`, "digital checkout quantity total");
  await waitForButtonEnabled(page, "^确认兑换$", "digital checkout enabled after duplicate quantity");
  await clickButton(page, "^确认兑换$");
  await waitForText(page, "兑换成功|订单已创建", "digital order paid");

  const order = await latestMallOrderForProduct(fixture, fixture.digitalProduct.id);
  if (!order?.id) {
    throw new Error("Digital mall order was not returned by user order API");
  }
  const orderedQuantity = orderQuantityForProduct(order, fixture.digitalProduct.id);
  if (orderedQuantity !== digitalQuantity) {
    throw new Error(`Digital mall order quantity = ${orderedQuantity}, want ${digitalQuantity}`);
  }
  const totalCredits = Number(order.total_credits ?? order.totalCredits ?? 0);
  if (totalCredits !== CHECKOUT_PRICE * digitalQuantity) {
    throw new Error(`Digital mall order total = ${totalCredits}, want ${CHECKOUT_PRICE * digitalQuantity}`);
  }
  if (String(order.receiver || "").trim() || String(order.phone || "").trim() || String(order.address || "").trim()) {
    throw new Error(`Granted digital order unexpectedly stored shipping information: ${JSON.stringify({ receiver: order.receiver, phone: order.phone, address: order.address })}`);
  }
  const orderNo = order.order_no || order.orderNo || String(order.id);
  const entitlements = await waitForDigitalEntitlements(fixture, order.id, fixture.digitalProduct.id, fixture.digitalGrantKey, "ACTIVE", digitalQuantity);
  const entitlementCodes = entitlements.map((item) => item?.fulfillment_code || item?.fulfillmentCode || "").filter(Boolean);
  const entitlementCode = entitlementCodes[0] || "";

  await clickButton(page, "查看订单");
  await waitForText(page, "个人工作台", "digital dashboard shell");
  await waitForText(page, orderNo, "digital order number");
  await waitForText(page, fixture.digitalProduct.title, "digital order item title");
  await waitForText(page, `x\\s*${digitalQuantity}|${digitalQuantity}\\s*件|等\\s*${digitalQuantity}\\s*项`, "digital order quantity or entitlement count");
  await waitForText(page, `授权：徽章权益 · ${fixture.digitalGrantKey}`, "digital order grant snapshot");
  await waitForText(page, "数字权益", "digital order entitlement section");
  for (const code of entitlementCodes) {
    await waitForText(page, code, `digital entitlement fulfillment code ${code} in order`);
  }

  await navigate(page, `${FRONTEND_BASE}/dashboard/entitlements`);
  await waitForText(page, "个人列表|数字权益|权益", "entitlements dashboard");
  await waitForText(page, "已过期", "expired entitlement filter tab");
  await waitForText(page, "全部权益|徽章|主题|会员", "entitlement grant type filters");
  await clickButton(page, "^徽章$");
  await waitForText(page, fixture.digitalProduct.title, "entitlement title");
  await waitForText(page, fixture.digitalGrantKey, "entitlement grant key");
  for (const code of entitlementCodes) {
    await waitForText(page, code, `entitlement fulfillment code ${code}`);
  }
  await waitForText(page, "可用", "active entitlement state");
  const activeText = summarizeDigitalEntitlementText(await bodyText(page), fixture.digitalGrantKey, entitlementCode);
  await clickButtonInArticle(page, fixture.digitalProduct.title, "查看商品");
  await waitForText(page, "商品详情", "entitlement product target detail");
  await waitForText(page, fixture.digitalProduct.title, "entitlement product target title");
  const publicBadgesUrl = `${FRONTEND_BASE}/user/${encodeURIComponent(fixture.auth.user.id)}/badges`;
  await navigate(page, publicBadgesUrl);
  await waitForText(page, "用户空间", "public badge profile shell");
  await waitForPublicBadgePanelReady(page, fixture.digitalProduct.title, "public badge panel");
  await waitForText(page, fixture.digitalProduct.title, "public badge title");
  await waitForText(page, "通过商城数字权益获得。|通过商城数字权益获得", "public badge description");
  await waitForText(page, "徽章权益", "public badge grant label");
  const publicBadgeText = summarizePublicBadgeText(await bodyText(page), fixture.digitalProduct.title);

  const refund = await createMallRefund(fixture, order.id, refundNote);
  await approveMallRefund(fixture, refund.id, adminNote);
  await waitForDigitalEntitlements(fixture, order.id, fixture.digitalProduct.id, fixture.digitalGrantKey, "REVOKED", digitalQuantity);
  await waitForMallOrderNotifications(fixture, order.id, ["售后退款已通过"]);
  await navigate(page, publicBadgesUrl);
  await waitForText(page, "用户空间", "public badge profile shell after refund");
  await waitForPublicBadgePanelReady(page, fixture.digitalProduct.title, "public badge panel after refund");
  await waitFor(page, `(() => {
    const text = document.body?.innerText || "";
    return !text.includes(${JSON.stringify(fixture.digitalProduct.title)});
  })()`, "public badge removed after refund");
  const publicBadgeRevokedText = summarizePublicBadgeText(await bodyText(page), fixture.digitalProduct.title);

  await navigate(page, `${FRONTEND_BASE}/dashboard/entitlements`);
  await waitForText(page, "个人列表|数字权益|权益", "entitlements dashboard after refund");
  await clickButton(page, "^已撤销$");
  await clickButton(page, "^徽章$");
  await waitForText(page, fixture.digitalProduct.title, "revoked entitlement title");
  await waitForText(page, fixture.digitalGrantKey, "revoked entitlement grant key");
  for (const code of entitlementCodes) {
    await waitForText(page, code, `revoked entitlement fulfillment code ${code}`);
  }
  await waitForText(page, "已撤销|退款失效", "revoked entitlement state");
  const revokedText = summarizeDigitalEntitlementText(await bodyText(page), fixture.digitalGrantKey, entitlementCode);
  await clickButtonInArticle(page, fixture.digitalProduct.title, "查看售后");
  await waitForText(page, "个人列表|售后", "revoked entitlement refund target");
  await waitForText(page, "当前定位", "revoked entitlement focused refund marker");
  await waitForText(page, orderNo, "revoked entitlement focused refund order");
  await waitForText(page, "已退款", "revoked entitlement focused refund status");
  await waitForText(page, adminNote, "revoked entitlement focused refund admin note");

  await navigate(page, `${FRONTEND_BASE}/dashboard/orders?order_id=${encodeURIComponent(order.id)}`);
  await waitForText(page, orderNo, "refunded digital order number");
  await waitForText(page, "已退款", "refunded digital order status");
  await waitForText(page, "已撤销|退款失效", "refunded digital order entitlement state");
  await navigate(page, `${FRONTEND_BASE}/dashboard/messages`);
  await waitForText(page, "售后退款已通过", "digital refund notification title");
  await waitForText(page, orderNo, "digital refund notification order number");
  await waitForText(page, "数字权益已撤销", "digital refund notification revocation hint");
  await waitForText(page, fixture.digitalGrantKey, "digital refund notification grant key");
  for (const code of entitlementCodes) {
    await waitForText(page, code, `digital refund notification fulfillment code ${code}`);
  }

  return {
    orderId: String(order.id),
    orderNo,
    entitlementCode,
    entitlementCodes,
    entitlementCount: entitlements.length,
    refundId: String(refund.id),
    digitalText: activeText,
    revokedText,
    publicBadgeText,
    publicBadgeRevokedText
  };
}

async function runBrowserThemeEntitlementFlow(page, fixture) {
  const shopUrl = `${FRONTEND_BASE}/shop?product_id=${encodeURIComponent(fixture.themeProduct.id)}`;
  await navigate(page, shopUrl);
  await waitForText(page, fixture.themeProduct.title, "theme product detail");
  await waitForText(page, "商品详情", "theme product detail panel");
  await waitForText(page, "主题权益", "theme product grant label");
  await clickButton(page, "^立即兑换$");
  await waitForText(page, "确认兑换", "theme checkout panel");
  await waitForText(page, "主题权益在线发放，无需收货地址|数字权益在线发放，无需收货地址", "theme checkout fulfillment hint");
  await waitForButtonEnabled(page, "^确认兑换$", "theme checkout enabled");
  await clickButton(page, "^确认兑换$");
  await waitForText(page, "兑换成功|订单已创建", "theme order paid");

  const order = await latestMallOrderForProduct(fixture, fixture.themeProduct.id);
  if (!order?.id) {
    throw new Error("Theme mall order was not returned by user order API");
  }
  const orderNo = order.order_no || order.orderNo || String(order.id);
  const entitlement = await waitForDigitalEntitlement(fixture, order.id, fixture.themeProduct.id, fixture.themeGrantKey, "ACTIVE");
  const entitlementCode = entitlement?.fulfillment_code || entitlement?.fulfillmentCode || "";

  await navigate(page, `${FRONTEND_BASE}/dashboard/entitlements`);
  await waitForText(page, "个人列表|数字权益|权益", "theme entitlements dashboard");
  await clickButton(page, "^主题$");
  await waitForText(page, fixture.themeProduct.title, "theme dashboard entitlement title");
  await waitForText(page, fixture.themeGrantKey, "theme dashboard entitlement grant key");
  await waitForText(page, "可用", "theme dashboard entitlement active state");
  const useActionText = await clickButtonInArticle(page, fixture.themeProduct.title, "^启用主题$");

  await waitForText(page, "个人资料", "profile settings panel");
  await waitForText(page, "高级主题已解锁", "theme access available");
  await fillByLabel(page, "主题", fixture.themeGrantKey);
  const selectedTheme = await fieldValueByLabel(page, "主题");
  if (selectedTheme !== fixture.themeGrantKey) {
    throw new Error(`Profile theme select value = ${selectedTheme}, want ${fixture.themeGrantKey}`);
  }
  await clickButton(page, "^保存资料$");
  await waitForText(page, "资料已保存", "profile theme saved");

  const publicProfileUrl = `${FRONTEND_BASE}/user/${encodeURIComponent(fixture.auth.user.id)}`;
  await navigate(page, publicProfileUrl);
  await waitForText(page, "用户空间", "public profile after theme save");
  const profileClass = await waitForProfileThemeClass(page, "profile-theme-pro", "public profile theme pro class");

  const revocationReason = `Browser E2E theme revoke ${Date.now()}`;
  const revokedEntitlement = await revokeMallDigitalEntitlement(fixture, entitlement.id, revocationReason);
  if (String(revokedEntitlement?.status || "").toUpperCase() !== "REVOKED") {
    throw new Error(`Theme entitlement revoke status = ${revokedEntitlement?.status ?? "unknown"}, want REVOKED`);
  }
  await waitForDigitalEntitlement(fixture, order.id, fixture.themeProduct.id, fixture.themeGrantKey, "REVOKED");

  await navigate(page, `${publicProfileUrl}?theme_revoked=${Date.now()}`);
  await waitForText(page, "用户空间", "public profile after theme revoke");
  const revokedProfileClass = await waitForProfileThemeClass(page, "profile-theme-default", "public profile default class after theme revoke");

  await navigate(page, `${FRONTEND_BASE}/dashboard/profile`);
  await waitForText(page, "个人资料", "profile settings panel after theme revoke");
  await waitForText(page, "购买 theme-pro 后可使用高级主题", "theme access unavailable after revoke");

  return {
    orderId: String(order.id),
    orderNo,
    entitlementCode,
    useActionText,
    profileClass,
    revokedProfileClass,
    revocationReason
  };
}

async function runBrowserMembershipBountyFlow(page, fixture, expectedBrowserIssues = []) {
  const shopUrl = `${FRONTEND_BASE}/shop?product_id=${encodeURIComponent(fixture.membershipProduct.id)}`;
  const bountyScore = 7;
  const topicTitle = `E2E Membership Bounty ${Date.now()}`;
  const topicBody = `浏览器联调会员悬赏问答：先保存草稿，再兑换 ${fixture.membershipGrantKey} 后发布带悬赏问题。`;

  await navigate(page, `${FRONTEND_BASE}/question/create`);
  await waitForText(page, "发布求助|创作中心", "question draft editor");
  await fillBySelector(page, ".compose-title", topicTitle);
  await fillBySelector(page, ".editor-body", topicBody);
  await fillBySelector(page, ".tag-assist input", "会员悬赏 联调");
  await fillByLabel(page, "悬赏积分", String(bountyScore));
  await setCheckboxByLabel(page, "立即发布", false);
  await waitForText(page, "悬赏需会员权益", "question editor membership gate before membership");
  await waitForButtonEnabled(page, "^保存草稿$", "bounty question draft submit enabled");
  await clickButton(page, "^保存草稿$");
  await waitForText(page, "编辑求助|创作中心", "bounty question draft edit page");
  const loadedDraftTitle = await fieldValueBySelector(page, ".compose-title");
  if (loadedDraftTitle !== topicTitle) {
    throw new Error(`Loaded bounty draft title = ${loadedDraftTitle}, want ${topicTitle}`);
  }
  const loadedDraftBody = await fieldValueBySelector(page, ".editor-body");
  if (loadedDraftBody !== topicBody) {
    throw new Error(`Loaded bounty draft body = ${loadedDraftBody}, want ${topicBody}`);
  }
  const loadedDraftBounty = await fieldValueByLabel(page, "悬赏积分");
  if (loadedDraftBounty !== String(bountyScore)) {
    throw new Error(`Loaded bounty draft score = ${loadedDraftBounty}, want ${bountyScore}`);
  }

  const draftTopic = await latestMyTopicForTitle(fixture, topicTitle);
  if (!draftTopic?.id) {
    throw new Error(`Membership bounty draft was not returned by user topic API: ${topicTitle}`);
  }
  const draftStatus = Number(draftTopic.status ?? 0);
  if (draftStatus !== 1) {
    throw new Error(`Membership bounty draft status = ${draftStatus}, want 1`);
  }
  const draftType = String(draftTopic.type || draftTopic.topic_type || draftTopic.topicType || "").toLowerCase();
  if (draftType !== "qa") {
    throw new Error(`Membership bounty draft type = ${draftType || "unknown"}, want qa`);
  }
  const draftBounty = Number(draftTopic.bounty_score ?? draftTopic.bountyScore ?? 0);
  if (draftBounty !== bountyScore) {
    throw new Error(`Membership bounty draft score = ${draftBounty}, want ${bountyScore}`);
  }

  const pendingMembershipOrderData = await apiRequest("/mall/orders", {
    method: "POST",
    token: fixture.auth.accessToken,
    body: {
      idempotency_key: `pending-membership-${Date.now()}`,
      items: [{ product_id: fixture.membershipProduct.id, quantity: 1 }]
    }
  });
  const pendingMembershipOrder = pendingMembershipOrderData?.order || pendingMembershipOrderData;
  if (!pendingMembershipOrder?.id) {
    throw new Error(`Pending membership order creation did not return order.id: ${JSON.stringify(pendingMembershipOrderData)}`);
  }
  await waitForMallOrderStatus(fixture, pendingMembershipOrder.id, 1, "pending membership order before duplicate checkout");
  const duplicatePaymentOrder = await createDuplicatePendingMembershipOrderFixture(fixture);
  await waitForMallOrderStatus(fixture, duplicatePaymentOrder.id, 1, "duplicate membership payment fixture pending order");
  const duplicatePaymentRejection = await assertPendingMembershipPaymentRejected(fixture, duplicatePaymentOrder.id);

  await navigate(page, shopUrl);
  await waitForText(page, fixture.membershipProduct.title, "membership product detail");
  await waitForText(page, "商品详情", "membership product detail panel");
  await waitForText(page, "会员权益", "membership product grant label");
  await clickButton(page, "^立即兑换$");
  await waitForText(page, "确认兑换", "membership checkout panel");
  await waitForText(page, "会员权益在线发放，无需收货地址", "membership checkout fulfillment hint");
  const stopExpectingPendingMembershipCheckoutFailure = expectBrowserHttpFailure(expectedBrowserIssues, `${API_BASE}/mall/orders`, 412);
  try {
    await clickButton(page, "^确认兑换$");
    await waitForText(page, "该会员权益已有待支付订单", "pending membership checkout error");
    await delay(250);
  } finally {
    stopExpectingPendingMembershipCheckoutFailure();
  }
  await apiRequest(`/mall/orders/${encodeURIComponent(pendingMembershipOrder.id)}/cancel`, {
    method: "POST",
    token: fixture.auth.accessToken
  });
  await waitForMallOrderStatus(fixture, pendingMembershipOrder.id, 4, "pending membership order canceled before paid checkout");
  await apiRequest(`/mall/orders/${encodeURIComponent(duplicatePaymentOrder.id)}/cancel`, {
    method: "POST",
    token: fixture.auth.accessToken
  });
  await waitForMallOrderStatus(fixture, duplicatePaymentOrder.id, 4, "duplicate membership payment fixture canceled before paid checkout");

  await navigate(page, shopUrl);
  await waitForText(page, fixture.membershipProduct.title, "membership product detail after pending duplicate");
  await waitForText(page, "商品详情", "membership product detail panel after pending duplicate");
  await waitForText(page, "会员权益", "membership product grant label after pending duplicate");
  await clickButton(page, "^立即兑换$");
  await waitForText(page, "确认兑换", "membership checkout panel after pending duplicate");
  await waitForText(page, "会员权益在线发放，无需收货地址", "membership checkout fulfillment hint after pending duplicate");
  await waitForButtonEnabled(page, "^确认兑换$", "membership checkout enabled");
  await clickButton(page, "^确认兑换$");
  await waitForText(page, "兑换成功|订单已创建", "membership order paid");

  const order = await latestMallOrderForProduct(fixture, fixture.membershipProduct.id);
  if (!order?.id) {
    throw new Error("Membership mall order was not returned by user order API");
  }
  const orderNo = order.order_no || order.orderNo || String(order.id);
  const entitlement = await waitForDigitalEntitlement(fixture, order.id, fixture.membershipProduct.id, fixture.membershipGrantKey, "ACTIVE");
  const entitlementCode = entitlement?.fulfillment_code || entitlement?.fulfillmentCode || "";
  const entitlementExpiresAt = Number(entitlement?.expires_at ?? entitlement?.expiresAt ?? 0);
  const entitlementGrantType = String(entitlement?.grant_type || entitlement?.grantType || "").toLowerCase();
  if (entitlementGrantType && entitlementGrantType !== "membership") {
    throw new Error(`Membership entitlement grant_type = ${entitlementGrantType}, want membership`);
  }
  if (!entitlementExpiresAt || entitlementExpiresAt <= Date.now()) {
    throw new Error(`Membership entitlement expires_at = ${entitlementExpiresAt}, want future timestamp`);
  }
  const membershipRefundRejection = await assertMembershipOrderRejectsRefund(fixture, order.id);

  await navigate(page, shopUrl);
  await waitForText(page, fixture.membershipProduct.title, "membership renewal product detail");
  await waitForText(page, "商品详情", "membership renewal product detail panel");
  await waitForButtonEnabled(page, "^立即兑换$", "membership renewal checkout action");
  await clickButton(page, "^立即兑换$");
  await waitForText(page, "确认兑换", "membership renewal checkout panel");
  await clickButton(page, "^确认兑换$");
  await waitForText(page, "兑换成功|订单已创建", "membership renewal order paid");
  const renewalOrder = await latestMallOrderForProduct(fixture, fixture.membershipProduct.id);
  if (!renewalOrder?.id || String(renewalOrder.id) === String(order.id)) {
    throw new Error("Membership renewal did not create a distinct order");
  }
  const renewalEntitlement = await waitForDigitalEntitlement(fixture, renewalOrder.id, fixture.membershipProduct.id, fixture.membershipGrantKey, "ACTIVE");
  const renewalExpiresAt = Number(renewalEntitlement?.expires_at ?? renewalEntitlement?.expiresAt ?? 0);
  if (renewalExpiresAt < entitlementExpiresAt + MEMBERSHIP_DURATION_MS - 60000) {
    throw new Error(`Membership renewal expires_at = ${renewalExpiresAt}, want at least ${entitlementExpiresAt + MEMBERSHIP_DURATION_MS - 60000}`);
  }

  await navigate(page, `${FRONTEND_BASE}/dashboard/orders?order_id=${encodeURIComponent(order.id)}`);
  await waitForText(page, "个人工作台", "membership dashboard shell");
  await waitForText(page, orderNo, "membership order number");
  await waitForText(page, fixture.membershipProduct.title, "membership order item title");
  await waitForText(page, `授权：会员权益 · ${fixture.membershipGrantKey}|会员权益`, "membership order grant snapshot");
  if (entitlementCode) {
    await waitForText(page, entitlementCode, "membership order entitlement code");
  }
  await assertButtonAbsentInArticle(page, orderNo, "^申请售后$", "membership order refund action hidden in row");
  await assertButtonAbsentInOrderDetail(page, orderNo, "^申请售后$", "membership order refund action hidden in detail");

  await navigate(page, `${FRONTEND_BASE}/member`);
  await waitForText(page, "已购会员权益", "member page entitlements panel");
  await waitForText(page, fixture.membershipProduct.title, "member page membership entitlement title");
  await waitForText(page, "会员权益", "member page active membership count");
  await waitForText(page, "有效至", "member page membership expiry");
  await waitForText(page, membershipEffectiveExpiryLabel(renewalExpiresAt), "member page consolidated membership expiry");
  const membershipText = summarizeDigitalEntitlementText(await bodyText(page), fixture.membershipGrantKey, entitlementCode);

  await navigate(page, `${FRONTEND_BASE}/dashboard/entitlements`);
  await waitForText(page, "个人列表|数字权益|权益", "membership entitlements dashboard");
  await clickButtonInTabList(page, "权益类型", "^会员$", "membership entitlement filter");
  await waitForText(page, fixture.membershipProduct.title, "membership dashboard entitlement title");
  await waitForText(page, fixture.membershipGrantKey, "membership dashboard entitlement grant key");
  await waitForText(page, "有效至", "membership dashboard entitlement expiry");
  const useActionText = await clickButtonInArticle(page, fixture.membershipProduct.title, "^设置背景$");

  const membershipBackgroundUrl = `${FRONTEND_BASE}/uploads/e2e-membership-bg-${Date.now()}.webp`;
  await waitForText(page, "个人资料", "profile settings panel for membership background");
  await waitForText(page, "会员背景已解锁", "profile background membership access available");
  await fillByLabel(page, "背景图 URL", membershipBackgroundUrl);
  await clickButton(page, "^保存资料$");
  await waitForText(page, "资料已保存", "membership profile background saved");
  const publicProfileUrl = `${FRONTEND_BASE}/user/${encodeURIComponent(fixture.auth.user.id)}`;
  await navigate(page, publicProfileUrl);
  await waitForText(page, "用户空间", "public profile after membership background save");
  const membershipProfileBackgroundStyle = await waitForProfileBackgroundStyle(page, membershipBackgroundUrl, "public profile membership background");

  await navigate(page, `${FRONTEND_BASE}/question/edit/${encodeURIComponent(draftTopic.id)}`);
  await waitForText(page, "编辑求助|创作中心", "bounty question draft editor after membership");
  await waitForText(page, "会员权益可用", "question editor membership gate active");
  const loadedPublishTitle = await fieldValueBySelector(page, ".compose-title");
  if (loadedPublishTitle !== topicTitle) {
    throw new Error(`Loaded bounty draft title = ${loadedPublishTitle}, want ${topicTitle}`);
  }
  const bountyInsufficientCreditBalance = await currentCreditBalance(fixture);
  const oversizedBountyScore = bountyInsufficientCreditBalance + 100;
  await fillByLabel(page, "悬赏积分", String(oversizedBountyScore));
  await setCheckboxByLabel(page, "立即发布", true);
  await waitForButtonEnabled(page, "^发布$", "bounty question submit enabled");
  expectBrowserHttpFailure(expectedBrowserIssues, `${API_BASE}/topics/${encodeURIComponent(draftTopic.id)}/publish`, 412);
  await clickButton(page, "^发布$");
  await waitForText(page, "悬赏积分不足", "bounty insufficient credit publish error");
  const failedPublishDraft = await latestMyTopicForTitle(fixture, topicTitle);
  const failedPublishStatus = Number(failedPublishDraft.status ?? 0);
  if (failedPublishStatus !== 1) {
    throw new Error(`Insufficient-credit bounty draft status = ${failedPublishStatus}, want 1`);
  }
  const failedPublishBounty = Number(failedPublishDraft.bounty_score ?? failedPublishDraft.bountyScore ?? 0);
  if (failedPublishBounty !== oversizedBountyScore) {
    throw new Error(`Insufficient-credit bounty draft score = ${failedPublishBounty}, want ${oversizedBountyScore}`);
  }
  await fillByLabel(page, "悬赏积分", String(bountyScore));
  await waitForButtonEnabled(page, "^发布$", "bounty question submit enabled after balance-sized bounty");
  const questionerBalanceBeforePublish = await currentCreditBalance(fixture);
  await clickButton(page, "^发布$");
  await waitForText(page, topicTitle, "bounty topic detail");
  await waitForText(page, `${bountyScore} 积分悬赏`, "bounty topic score");
  const bountyText = summarizeMembershipBountyText(await bodyText(page), topicTitle);

  const topic = await latestTopicForTitle(fixture, topicTitle);
  if (!topic?.id) {
    throw new Error(`Membership bounty topic was not returned by topic list API: ${topicTitle}`);
  }
  if (String(topic.id) !== String(draftTopic.id)) {
    throw new Error(`Published bounty topic id = ${topic.id}, want draft topic id ${draftTopic.id}`);
  }
  const actualBounty = Number(topic.bounty_score ?? topic.bountyScore ?? 0);
  if (actualBounty !== bountyScore) {
    throw new Error(`Membership bounty topic score = ${actualBounty}, want ${bountyScore}`);
  }
  const topicType = String(topic.type || topic.topic_type || topic.topicType || "").toLowerCase();
  if (topicType && topicType !== "qa" && topicType !== "question") {
    throw new Error(`Membership bounty topic type = ${topicType}, want qa`);
  }
  const questionerReserveLedger = await waitForCreditLedgerEntry(
    fixture.auth.accessToken,
    (item) =>
      String(item.reason ?? "") === "qa_bounty_reserved" &&
      String(item.source_type ?? item.sourceType ?? "") === "topic" &&
      String(item.source_id ?? item.sourceId ?? "") === String(topic.id) &&
      Number(item.delta ?? 0) === -bountyScore,
    "questioner bounty reserved ledger",
  );
  const questionerBalanceAfterPublish = await currentCreditBalance(fixture);
  if (questionerBalanceAfterPublish !== questionerBalanceBeforePublish - bountyScore) {
    throw new Error(
      `Questioner balance after bounty publish = ${questionerBalanceAfterPublish}, want ${questionerBalanceBeforePublish - bountyScore}`,
    );
  }

  const answerNeedle = `浏览器联调悬赏答案 ${Date.now()}`;
  const answerContent = `${answerNeedle}：采纳后应结算 ${bountyScore} 积分。`;
  const answerResp = await apiRequest(`/topics/${encodeURIComponent(topic.id)}/comments`, {
    method: "POST",
    token: fixture.answererAuth.accessToken,
    body: {
      content: answerContent,
      parent_id: 0
    }
  });
  const answerComment = answerResp?.comment || answerResp;
  if (!answerComment?.id) {
    throw new Error(`Bounty answer comment creation did not return comment.id: ${JSON.stringify(answerResp)}`);
  }
  const questionerBalanceBeforeAccept = await currentCreditBalance(fixture);
  const answererBalanceBeforeAccept = await currentCreditBalance({
    ...fixture,
    auth: fixture.answererAuth
  });

  await navigate(page, `${FRONTEND_BASE}/topic/${encodeURIComponent(topic.id)}?accepted_e2e=${Date.now()}`);
  await waitForText(page, topicTitle, "bounty topic detail before answer acceptance");
  await waitForText(page, answerNeedle, "bounty answer visible before acceptance");
  await clickButtonWhenEnabled(page, "^采纳答案$", "bounty answer accept button enabled");
  await waitForText(page, "已采纳", "accepted answer badge");
  await waitForText(page, "已解决", "bounty topic resolved state");

  const acceptedTopic = await waitForTopicAccepted(
    topic.id,
    answerComment.id,
    "membership bounty accepted topic",
  );
  const answererLedger = await waitForCreditLedgerEntry(
    fixture.answererAuth.accessToken,
    (item) =>
      String(item.reason ?? "") === "qa_answer_accepted" &&
      String(item.source_type ?? item.sourceType ?? "") === "comment" &&
      String(item.source_id ?? item.sourceId ?? "") === String(answerComment.id) &&
      Number(item.delta ?? 0) === bountyScore,
    "answerer accepted answer reward ledger",
  );
  const questionerBalanceAfterAccept = await currentCreditBalance(fixture);
  const answererBalanceAfterAccept = await currentCreditBalance({
    ...fixture,
    auth: fixture.answererAuth
  });
  if (questionerBalanceAfterAccept !== questionerBalanceBeforeAccept) {
    throw new Error(
      `Questioner balance after bounty acceptance = ${questionerBalanceAfterAccept}, want unchanged ${questionerBalanceBeforeAccept}`,
    );
  }
  if (answererBalanceAfterAccept !== answererBalanceBeforeAccept + bountyScore) {
    throw new Error(
      `Answerer balance after bounty acceptance = ${answererBalanceAfterAccept}, want ${answererBalanceBeforeAccept + bountyScore}`,
    );
  }
  await clickButtonWhenEnabled(page, "^撤销采纳$", "bounty answer unaccept button enabled");
  await waitForText(page, "等待采纳答案", "bounty topic reopened after unaccept");
  await waitForTopicUnaccepted(topic.id, answerComment.id, "membership bounty unaccepted topic");
  const answererReversalLedger = await waitForCreditLedgerEntry(
    fixture.answererAuth.accessToken,
    (item) =>
      String(item.reason ?? "") === "qa_answer_unaccepted" &&
      String(item.source_type ?? item.sourceType ?? "") === "comment" &&
      String(item.source_id ?? item.sourceId ?? "") === String(answerComment.id) &&
      Number(item.delta ?? 0) === -bountyScore,
    "answerer accepted answer reversal ledger",
  );
  const questionerBalanceAfterUnaccept = await currentCreditBalance(fixture);
  const answererBalanceAfterUnaccept = await currentCreditBalance({
    ...fixture,
    auth: fixture.answererAuth
  });
  if (questionerBalanceAfterUnaccept !== questionerBalanceAfterAccept) {
    throw new Error(
      `Questioner balance after bounty unaccept = ${questionerBalanceAfterUnaccept}, want unchanged ${questionerBalanceAfterAccept}`,
    );
  }
  if (answererBalanceAfterUnaccept !== answererBalanceBeforeAccept) {
    throw new Error(
      `Answerer balance after bounty unaccept = ${answererBalanceAfterUnaccept}, want ${answererBalanceBeforeAccept}`,
    );
  }
  await clickButtonWhenEnabled(page, "^采纳答案$", "bounty answer reaccept button enabled");
  await waitForText(page, "已采纳", "reaccepted answer badge");
  await waitForText(page, "已解决", "reaccepted question resolved state");
  const reacceptedTopic = await waitForTopicAccepted(topic.id, answerComment.id, "membership bounty reaccepted topic");
  const answererReacceptedLedger = await waitForCreditLedgerEntry(
    fixture.answererAuth.accessToken,
    (item) =>
      String(item.reason ?? "") === "qa_answer_accepted" &&
      String(item.source_type ?? item.sourceType ?? "") === "comment" &&
      String(item.source_id ?? item.sourceId ?? "") === String(answerComment.id) &&
      String(item.source_event_id ?? item.sourceEventId ?? "") === `content.qa.accepted:${topic.id}:${answerComment.id}:1` &&
      Number(item.delta ?? 0) === bountyScore,
    "answerer reaccepted answer reward ledger",
  );
  const answererBalanceAfterReaccept = await currentCreditBalance({
    ...fixture,
    auth: fixture.answererAuth
  });
  if (answererBalanceAfterReaccept !== answererBalanceAfterAccept) {
    throw new Error(
      `Answerer balance after bounty reaccept = ${answererBalanceAfterReaccept}, want ${answererBalanceAfterAccept}`,
    );
  }
  const bountyReleaseResult = await assertMembershipBountyArchiveReleasesReservation(fixture, bountyScore);

  await revokeMallDigitalEntitlement(fixture, entitlement.id, `Browser E2E first membership revoke ${Date.now()}`);
  await waitForDigitalEntitlement(fixture, order.id, fixture.membershipProduct.id, fixture.membershipGrantKey, "REVOKED");
  const renewalEntitlementAfterFirstRevoke = await waitForDigitalEntitlement(
    fixture,
    renewalOrder.id,
    fixture.membershipProduct.id,
    fixture.membershipGrantKey,
    "ACTIVE",
  );
  const renewalExpiresAtAfterFirstRevoke = Number(
    renewalEntitlementAfterFirstRevoke?.expires_at ?? renewalEntitlementAfterFirstRevoke?.expiresAt ?? 0,
  );
  if (renewalExpiresAtAfterFirstRevoke <= Date.now()) {
    throw new Error(`Renewed membership expires_at after first revoke = ${renewalExpiresAtAfterFirstRevoke}, want future timestamp`);
  }
  const profileAfterFirstMembershipRevoke = await apiRequest("/users/me", {
    token: fixture.auth.accessToken
  });
  const renewalBackgroundRetainedAfterFirstRevoke =
    userProfileBackgroundURL(profileAfterFirstMembershipRevoke?.user) === membershipBackgroundUrl;
  if (!renewalBackgroundRetainedAfterFirstRevoke) {
    throw new Error("Revoking one membership grant hid the profile background while a renewal remained active");
  }
  const renewalBountyReleaseResult = await assertMembershipBountyArchiveReleasesReservation(fixture, bountyScore);

  const revocationReason = `Browser E2E membership revoke ${Date.now()}`;
  const revokedEntitlement = await revokeMallDigitalEntitlement(fixture, renewalEntitlement.id, revocationReason);
  if (String(revokedEntitlement?.status || "").toUpperCase() !== "REVOKED") {
    throw new Error(`Admin entitlement revoke status = ${revokedEntitlement?.status ?? "unknown"}, want REVOKED`);
  }
  await waitForDigitalEntitlement(fixture, renewalOrder.id, fixture.membershipProduct.id, fixture.membershipGrantKey, "REVOKED");
  const cachedBackgroundUrl = await evaluate(page, `(() => {
    try {
      const auth = JSON.parse(window.localStorage.getItem(${JSON.stringify(AUTH_STORAGE_KEY)}) || "null");
      return auth?.user?.background_url || auth?.user?.backgroundUrl || "";
    } catch {
      return "";
    }
  })()`);
  if (cachedBackgroundUrl !== membershipBackgroundUrl) {
    throw new Error(`Cached membership background = ${cachedBackgroundUrl || "empty"}, want ${membershipBackgroundUrl}`);
  }
  await navigate(page, `${FRONTEND_BASE}/user/profile?membership_revoked=${Date.now()}`);
  await waitForText(page, "个人中心", "current profile after membership revoke");
  const membershipRevokedCachedProfileBackgroundStyle = await waitForProfileBackgroundCleared(page, "cached profile background hidden after membership revoke");
  const headerProfileNickname = await assertRevokedMembershipHeaderProfileUpdate(page, fixture);
  await navigate(page, `${FRONTEND_BASE}/dashboard/profile?membership_revoked=${Date.now()}`);
  await waitForText(page, "个人资料", "dashboard profile after membership revoke");
  const membershipRevokedDashboardProfileBackgroundStyle = await waitForDashboardProfileBackgroundCleared(page, "dashboard cached profile background hidden after membership revoke");
  await navigate(page, `${publicProfileUrl}?membership_revoked=${Date.now()}`);
  await waitForText(page, "用户空间", "public profile after membership revoke");
  const membershipRevokedProfileBackgroundStyle = await waitForProfileBackgroundCleared(page, "public profile background hidden after membership revoke");
  const revokedBackgroundRejection = await assertRevokedMembershipRejectsProfileBackground(fixture, membershipBackgroundUrl);
  const adminStatusProfile = await assertRevokedMembershipAdminStatusResponseHidesProfileBackground(fixture);
  const revokedUnacceptRejection = await assertRevokedMembershipRejectsBountyUnaccept(fixture, topic, answerComment);

  const revocationNotifications = await waitForMallOrderNotifications(fixture, renewalOrder.id, ["数字权益已撤销"]);
  const revocationNotification = revocationNotifications[0];
  const notificationSourceID = revocationNotification?.source_id ?? revocationNotification?.sourceId;
  if (String(notificationSourceID) !== String(renewalEntitlement.id)) {
    throw new Error(`Membership revoke notification source_id = ${notificationSourceID ?? "unknown"}, want ${renewalEntitlement.id}`);
  }

  await navigate(page, `${FRONTEND_BASE}/dashboard/messages`);
  await waitForText(page, "数字权益已撤销", "membership revocation notification title");
  await waitForText(page, revocationReason, "membership revocation notification reason");
  await clickButtonInArticle(page, renewalOrder.order_no || renewalOrder.orderNo || String(renewalOrder.id), "查看权益");
  await waitForText(page, "个人列表|数字权益|权益", "membership revocation entitlement target");
  await waitForText(page, fixture.membershipProduct.title, "revoked membership entitlement title");
  await waitForText(page, "当前权益", "revoked membership focused marker");
  await waitForText(page, "已撤销|管理员撤销", "revoked membership entitlement state");
  await waitForText(page, revocationReason, "revoked membership entitlement reason");
  const revokedText = summarizeDigitalEntitlementText(await bodyText(page), fixture.membershipGrantKey, entitlementCode);

  await navigate(page, `${FRONTEND_BASE}/question/create`);
  await waitForText(page, "发布求助|创作中心", "question editor after membership revoke");
  await fillByLabel(page, "悬赏积分", String(bountyScore));
  await waitForText(page, "悬赏需会员权益", "question editor membership gate after revoke");
  const revokedBountyRejection = await assertRevokedMembershipRejectsBountyPublish(fixture, bountyScore);
  const revokedBountyDraftPublishRejection = await assertRevokedMembershipRejectsBountyDraftPublish(fixture, bountyScore);
  const revokedBountyEditRejection = await assertRevokedMembershipRejectsBountyEdit(fixture, topic, bountyScore, topicBody);

  return {
    orderId: String(order.id),
    orderNo,
    pendingDuplicateOrderRejected: true,
    duplicatePaymentApiStatus: duplicatePaymentRejection.status,
    duplicatePaymentApiMessage: duplicatePaymentRejection.message,
    entitlementCode,
    expiresAt: entitlementExpiresAt,
    renewalExpiresAt,
    renewalExpiresAtAfterFirstRevoke,
    renewalBackgroundRetainedAfterFirstRevoke,
    renewalBountyReleaseLedgerId: renewalBountyReleaseResult.releaseLedgerId,
    refundApiStatus: membershipRefundRejection.status,
    refundApiMessage: membershipRefundRejection.message,
    refundActionHidden: true,
    useActionText,
    membershipBackgroundUrl,
    membershipProfileBackgroundStyle,
    membershipRevokedCachedProfileBackgroundStyle,
    membershipRevokedDashboardProfileBackgroundStyle,
    membershipRevokedProfileBackgroundStyle,
    headerProfileNickname,
    adminMuteBackgroundUrl: adminStatusProfile.muteBackgroundUrl,
    adminUnmuteBackgroundUrl: adminStatusProfile.unmuteBackgroundUrl,
    membershipText,
    revocationReason,
    revokedBackgroundApiStatus: revokedBackgroundRejection.status,
    revokedBackgroundApiMessage: revokedBackgroundRejection.message,
    revokedApiStatus: revokedBountyRejection.status,
    revokedApiMessage: revokedBountyRejection.message,
    revokedDraftPublishApiStatus: revokedBountyDraftPublishRejection.status,
    revokedDraftPublishApiMessage: revokedBountyDraftPublishRejection.message,
    revokedEditApiStatus: revokedBountyEditRejection.status,
    revokedEditApiMessage: revokedBountyEditRejection.message,
    revokedUnacceptApiStatus: revokedUnacceptRejection.status,
    revokedUnacceptApiMessage: revokedUnacceptRejection.message,
    revokedText,
    draftTopicId: String(draftTopic.id),
    draftTopicTitle: topicTitle,
    topicId: String(topic.id),
    topicTitle,
    bountyAcceptedCommentId: String(answerComment.id),
    bountyAcceptedTopicStatus: reacceptedTopic.qa_status || reacceptedTopic.qaStatus || acceptedTopic.qa_status || acceptedTopic.qaStatus || "",
    bountyQuestionerLedgerId: String(questionerReserveLedger.id ?? questionerReserveLedger.ID ?? ""),
    bountyAnswererLedgerId: String(answererLedger.id ?? answererLedger.ID ?? ""),
    bountyReversalLedgerId: String(answererReversalLedger.id ?? answererReversalLedger.ID ?? ""),
    bountyReacceptedAnswererLedgerId: String(answererReacceptedLedger.id ?? answererReacceptedLedger.ID ?? ""),
    bountyReleasedTopicId: bountyReleaseResult.topicId,
    bountyReleaseLedgerId: bountyReleaseResult.releaseLedgerId,
    bountyInsufficientCreditBalance,
    bountyInsufficientCreditText: "悬赏积分不足，请先补足积分余额。",
    bountyText
  };
}

async function assertMembershipBountyArchiveReleasesReservation(fixture, bountyScore) {
  const stamp = Date.now();
  const title = `E2E Membership Bounty Release ${stamp}`;
  const balanceBeforePublish = await currentCreditBalance(fixture);
  const created = await apiRequest("/topics", {
    method: "POST",
    token: fixture.auth.accessToken,
    body: {
      slug: `e2e-membership-bounty-release-${stamp}`,
      type: "qa",
      title,
      body: "Unaccepted bounty question should return reserved credits when archived.",
      tags: ["membership", "bounty", "e2e"],
      category_id: 1,
      bounty_score: bountyScore,
      publish: true
    }
  });
  const topic = created?.topic || created;
  if (!topic?.id) {
    throw new Error(`Bounty release topic creation did not return topic id: ${JSON.stringify(created)}`);
  }
  const topicId = String(topic.id);
  const reserveLedger = await waitForCreditLedgerEntry(
    fixture.auth.accessToken,
    (item) =>
      String(item.reason ?? "") === "qa_bounty_reserved" &&
      String(item.source_type ?? item.sourceType ?? "") === "topic" &&
      String(item.source_id ?? item.sourceId ?? "") === topicId &&
      Number(item.delta ?? 0) === -bountyScore,
    "questioner bounty release reserve ledger",
  );
  const balanceAfterPublish = await currentCreditBalance(fixture);
  if (balanceAfterPublish !== balanceBeforePublish - bountyScore) {
    throw new Error(
      `Questioner balance after release-topic bounty publish = ${balanceAfterPublish}, want ${balanceBeforePublish - bountyScore}`,
    );
  }

  await apiRequest(`/topics/${encodeURIComponent(topicId)}`, {
    method: "DELETE",
    token: fixture.auth.accessToken
  });
  const releaseLedger = await waitForCreditLedgerEntry(
    fixture.auth.accessToken,
    (item) =>
      String(item.reason ?? "") === "qa_bounty_released" &&
      String(item.source_type ?? item.sourceType ?? "") === "topic" &&
      String(item.source_id ?? item.sourceId ?? "") === topicId &&
      Number(item.delta ?? 0) === bountyScore,
    "questioner bounty release ledger",
  );
  const balanceAfterArchive = await currentCreditBalance(fixture);
  if (balanceAfterArchive !== balanceBeforePublish) {
    throw new Error(
      `Questioner balance after bounty archive release = ${balanceAfterArchive}, want restored ${balanceBeforePublish}`,
    );
  }
  return {
    topicId,
    reserveLedgerId: String(reserveLedger.id ?? reserveLedger.ID ?? ""),
    releaseLedgerId: String(releaseLedger.id ?? releaseLedger.ID ?? "")
  };
}

async function assertMembershipOrderRejectsRefund(fixture, orderId) {
  const failure = await apiRequestFailure(`/mall/orders/${encodeURIComponent(orderId)}/refunds`, {
    method: "POST",
    token: fixture.auth.accessToken,
    expectedStatus: 412,
    label: "membership order refund",
    body: {
      reason: "membership_refund",
      note: `会员订单普通售后应被拒绝 ${Date.now()}`
    }
  });
  const legacyCode = String(failure.meta?.legacy_code || failure.meta?.legacyCode || "");
  const combined = `${failure.message} ${failure.rawBody}`.toLowerCase();
  if (legacyCode !== "FailedPrecondition" || !combined.includes("membership order refund unavailable")) {
    throw new Error(`Membership order refund rejection mismatch: ${failure.rawBody.slice(0, 800)}`);
  }
  return {
    status: failure.status,
    message: failure.message || "membership order refund unavailable"
  };
}

async function assertRevokedMembershipRejectsBountyDraftPublish(fixture, bountyScore) {
  const stamp = Date.now();
  const title = `E2E Revoked Membership Draft Publish ${stamp}`;
  const created = await apiRequest("/topics", {
    method: "POST",
    token: fixture.auth.accessToken,
    body: {
      slug: `e2e-revoked-membership-draft-${stamp}`,
      type: "qa",
      title,
      body: "Draft creation is allowed after membership revocation, but publishing must stay server-gated.",
      tags: ["membership", "e2e"],
      category_id: 1,
      bounty_score: bountyScore,
      publish: false
    }
  });
  const draft = created?.topic || created;
  const topicID = draft?.id ?? draft?.ID;
  if (!topicID) {
    throw new Error(`Revoked membership bounty draft create did not return topic id: ${JSON.stringify(created)}`);
  }

  const failure = await apiRequestFailure(`/topics/${encodeURIComponent(topicID)}/publish`, {
    method: "POST",
    token: fixture.auth.accessToken,
    expectedStatus: 403,
    label: "revoked membership bounty draft publish"
  });
  const combined = `${failure.message} ${failure.rawBody}`.toLowerCase();
  const hasMembershipReason =
    combined.includes("membership entitlement required") ||
    combined.includes("topic_membership_entitlement_required") ||
    combined.includes("topic_membership");
  if (!hasMembershipReason) {
    throw new Error(`Revoked membership bounty draft publish did not return membership entitlement error: ${failure.rawBody.slice(0, 800)}`);
  }

  const draftAfterFailure = await latestMyTopicForTitle(fixture, title);
  const draftStatus = Number(draftAfterFailure.status ?? 0);
  if (draftStatus !== 1) {
    throw new Error(`Revoked membership bounty draft status after failed publish = ${draftStatus}, want 1`);
  }
  return {
    status: failure.status,
    message: failure.message || "membership entitlement required for bounty QA draft publish"
  };
}

async function assertRevokedMembershipHeaderProfileUpdate(page, fixture) {
  await evaluate(page, `(() => {
    const trigger = document.querySelector('button[aria-label="个人中心"]');
    if (!trigger) throw new Error("Header profile trigger not found");
    trigger.click();
    return true;
  })()`);
  await waitFor(page, "document.querySelector('.auth-popover') !== null", "header profile popover after membership revoke");
  await waitForButtonEnabled(page, "^保存资料$", "header profile save after membership revoke");

  const nickname = `Revoked membership profile ${Date.now()}`;
  await fillBySelector(page, ".auth-popover input[placeholder='昵称']", nickname);
  await clickButton(page, "^保存资料$");
  await waitFor(page, "document.querySelector('.auth-popover') === null", "header profile save completion after membership revoke");

  const response = await apiRequest("/users/me", { token: fixture.auth.accessToken });
  const user = response?.user || response;
  if (String(user?.nickname || "") !== nickname) {
    throw new Error(`Header profile nickname = ${user?.nickname || "empty"}, want ${nickname}`);
  }
  if (String(user?.background_url || user?.backgroundUrl || "").trim() !== "") {
    throw new Error("Header profile update restored a revoked membership background");
  }
  return nickname;
}

async function assertRevokedMembershipRejectsProfileBackground(fixture, backgroundUrl) {
  const failure = await apiRequestFailure("/users/me", {
    method: "PUT",
    token: fixture.auth.accessToken,
    expectedStatus: 403,
    label: "revoked membership profile background",
    body: {
      nickname: fixture.auth.user.nickname || fixture.auth.user.username || "E2E User",
      avatar_url: fixture.auth.user.avatar_url || fixture.auth.user.avatarUrl || "",
      background_url: `${backgroundUrl}?revoked=${Date.now()}`,
      bio: fixture.auth.user.bio || ""
    }
  });
  const combined = `${failure.message} ${failure.rawBody}`.toLowerCase();
  const hasMembershipBackgroundReason =
    combined.includes("profile background membership entitlement required") ||
    combined.includes("background membership") ||
    combined.includes("membership entitlement required");
  if (!hasMembershipBackgroundReason) {
    throw new Error(`Revoked membership profile background did not return membership entitlement error: ${failure.rawBody.slice(0, 800)}`);
  }
  return {
    status: failure.status,
    message: failure.message || "profile background membership entitlement required"
  };
}

async function assertRevokedMembershipAdminStatusResponseHidesProfileBackground(fixture) {
  const userId = fixture.auth.user.id;
  const mute = await apiRequest(`/admin/users/${encodeURIComponent(userId)}/mute`, {
    method: "POST",
    token: fixture.adminToken
  });
  const muteUser = mute?.user;
  assertAdminStatusUser(muteUser, userId, 2, "mute");
  const muteBackgroundUrl = userProfileBackgroundURL(muteUser);
  if (muteBackgroundUrl) {
    throw new Error(`Revoked membership admin mute response leaked background_url: ${muteBackgroundUrl}`);
  }

  const unmute = await apiRequest(`/admin/users/${encodeURIComponent(userId)}/unmute`, {
    method: "POST",
    token: fixture.adminToken
  });
  const unmuteUser = unmute?.user;
  assertAdminStatusUser(unmuteUser, userId, 1, "unmute");
  const unmuteBackgroundUrl = userProfileBackgroundURL(unmuteUser);
  if (unmuteBackgroundUrl) {
    throw new Error(`Revoked membership admin unmute response leaked background_url: ${unmuteBackgroundUrl}`);
  }

  return { muteBackgroundUrl, unmuteBackgroundUrl };
}

function assertAdminStatusUser(user, expectedUserId, expectedStatus, action) {
  if (!user?.id || String(user.id) !== String(expectedUserId)) {
    throw new Error(`Admin ${action} response user id = ${user?.id ?? "missing"}, want ${expectedUserId}`);
  }
  const status = Number(user.status ?? 0);
  if (status !== expectedStatus) {
    throw new Error(`Admin ${action} response status = ${user.status ?? "missing"}, want ${expectedStatus}`);
  }
}

function userProfileBackgroundURL(user) {
  return String(user?.background_url ?? user?.backgroundUrl ?? "").trim();
}

async function assertRevokedMembershipRejectsBountyPublish(fixture, bountyScore) {
  const stamp = Date.now();
  const failure = await apiRequestFailure("/topics", {
    method: "POST",
    token: fixture.auth.accessToken,
    expectedStatus: 403,
    label: "revoked membership bounty publish",
    body: {
      slug: `e2e-revoked-membership-bounty-${stamp}`,
      type: "qa",
      title: `E2E Revoked Membership Bounty API ${stamp}`,
      body: "Direct API publish after membership revocation should be rejected by the server.",
      tags: ["membership", "e2e"],
      category_id: 1,
      bounty_score: bountyScore,
      publish: true
    }
  });
  const combined = `${failure.message} ${failure.rawBody}`.toLowerCase();
  const hasMembershipReason =
    combined.includes("membership entitlement required") ||
    combined.includes("topic_membership_entitlement_required") ||
    combined.includes("topic_membership");
  if (!hasMembershipReason) {
    throw new Error(`Revoked membership bounty publish did not return membership entitlement error: ${failure.rawBody.slice(0, 800)}`);
  }
  return {
    status: failure.status,
    message: failure.message || "membership entitlement required for bounty QA topics"
  };
}

async function assertRevokedMembershipRejectsBountyEdit(fixture, topic, bountyScore, originalBody) {
  const topicID = topic?.id;
  if (!topicID) {
    throw new Error("Cannot verify revoked membership bounty edit without a topic id");
  }
  const failure = await apiRequestFailure(`/topics/${encodeURIComponent(topicID)}`, {
    method: "PUT",
    token: fixture.auth.accessToken,
    expectedStatus: 403,
    label: "revoked membership bounty edit",
    body: {
      title: topic.title || `E2E Revoked Membership Bounty Edit ${Date.now()}`,
      body: `${originalBody}\n\nRevoked membership edit attempt ${Date.now()}`,
      tags: ["membership", "e2e"],
      category_id: Number(topic.category_id ?? topic.categoryId ?? 1),
      bounty_score: bountyScore
    }
  });
  const combined = `${failure.message} ${failure.rawBody}`.toLowerCase();
  const hasMembershipReason =
    combined.includes("membership entitlement required") ||
    combined.includes("topic_membership_entitlement_required") ||
    combined.includes("topic_membership");
  if (!hasMembershipReason) {
    throw new Error(`Revoked membership bounty edit did not return membership entitlement error: ${failure.rawBody.slice(0, 800)}`);
  }
  return {
    status: failure.status,
    message: failure.message || "membership entitlement required for bounty QA topics"
  };
}

async function assertRevokedMembershipRejectsBountyUnaccept(fixture, topic, answerComment) {
  const topicID = topic?.id;
  const commentID = answerComment?.id;
  if (!topicID || !commentID) {
    throw new Error("Cannot verify revoked membership bounty unaccept without a topic and accepted comment");
  }

  const answererFixture = { ...fixture, auth: fixture.answererAuth };
  const answererBalanceBefore = await currentCreditBalance(answererFixture);
  const failure = await apiRequestFailure(
    `/topics/${encodeURIComponent(topicID)}/comments/${encodeURIComponent(commentID)}/unaccept`,
    {
      method: "POST",
      token: fixture.auth.accessToken,
      expectedStatus: 403,
      label: "revoked membership bounty unaccept"
    }
  );
  const combined = `${failure.message} ${failure.rawBody}`.toLowerCase();
  const hasMembershipReason =
    combined.includes("membership entitlement required") ||
    combined.includes("topic_membership_entitlement_required") ||
    combined.includes("topic_membership");
  if (!hasMembershipReason) {
    throw new Error(`Revoked membership bounty unaccept did not return membership entitlement error: ${failure.rawBody.slice(0, 800)}`);
  }

  await waitForTopicAccepted(topicID, commentID, "revoked membership bounty stays accepted after rejected unaccept");
  const answererBalanceAfter = await currentCreditBalance(answererFixture);
  if (answererBalanceAfter !== answererBalanceBefore) {
    throw new Error(
      `Rejected revoked-membership unaccept changed answerer balance to ${answererBalanceAfter}, want ${answererBalanceBefore}`
    );
  }

  return {
    status: failure.status,
    message: failure.message || "membership entitlement required for bounty QA topics"
  };
}

async function markOrderPayingForBrowserResume(order, userId, idempotencyKey) {
  const orderId = String(order?.id ?? "").trim();
  const normalizedUserId = String(userId ?? "").trim();
  const key = String(idempotencyKey ?? "").trim();
  if (!/^\d+$/.test(orderId) || !/^\d+$/.test(normalizedUserId) || !key) {
    throw new Error(`Cannot create browser payment resume fixture: ${JSON.stringify({ orderId, userId: normalizedUserId, idempotencyKey: key })}`);
  }
  const stdout = await runMallPsql(`
    SET search_path TO bbs_mall;
    WITH fixture_time AS (
      SELECT NOW() AS at
    ),
    updated_order AS (
      UPDATE mall_orders
      SET status = 'PAYING',
          payment_method = 'credits',
          updated_at = (SELECT at FROM fixture_time)
      WHERE id = ${pgLiteral(orderId)}::BIGINT
        AND user_id = ${pgLiteral(normalizedUserId)}::BIGINT
        AND status = 'PENDING_PAYMENT'
      RETURNING id, user_id, total_credits
    ),
    inserted_payment AS (
      INSERT INTO mall_payments (
        order_id, user_id, amount_credits, provider, idempotency_key, status,
        provider_trade_no, failure_reason, paid_at, created_at, updated_at
      )
      SELECT
        updated_order.id,
        updated_order.user_id,
        updated_order.total_credits,
        'credits',
        ${pgLiteral(key)},
        'PENDING',
        '',
        '',
        NULL,
        fixture_time.at,
        fixture_time.at
      FROM updated_order
      CROSS JOIN fixture_time
      RETURNING id
    ),
    inserted_log AS (
      INSERT INTO mall_order_status_logs (
        order_id, from_status, to_status, reason, operator_type, operator_id,
        note, created_at
      )
      SELECT
        updated_order.id,
        'PENDING_PAYMENT',
        'PAYING',
        'paying',
        'user',
        updated_order.user_id::TEXT,
        'browser e2e pending payment resume fixture',
        fixture_time.at
      FROM updated_order
      CROSS JOIN inserted_payment
      CROSS JOIN fixture_time
      RETURNING order_id
    )
    SELECT order_id FROM inserted_log;
  `);
  if (!stdout.split(/\s+/).includes(orderId)) {
    throw new Error(`Failed to create browser payment resume fixture for ${orderId}. psql output: ${stdout.slice(0, 500)}`);
  }
}

async function createDuplicatePendingMembershipOrderFixture(fixture) {
  const product = fixture.membershipProduct;
  const userId = fixture.auth?.user?.id;
  const stamp = Date.now();
  const orderNo = `ME2EPAY${stamp}${Math.random().toString(36).slice(2, 7).toUpperCase()}`;
  const idempotencyKey = `duplicate-membership-payment-${stamp}-${Math.random().toString(36).slice(2, 8)}`;
  const quantity = 1;
  const unitPrice = Number(product.price_credits ?? product.priceCredits ?? 0);
  if (!userId || !product?.id || !unitPrice) {
    throw new Error(`Cannot create duplicate membership payment fixture: ${JSON.stringify({ userId, productId: product?.id, unitPrice })}`);
  }
  const stdout = await runMallPsql(`
    SET search_path TO bbs_mall;
    WITH fixture AS (
      SELECT
        ${pgLiteral(userId)}::BIGINT AS user_id,
        ${pgLiteral(product.id)}::BIGINT AS product_id,
        ${pgLiteral(orderNo)} AS order_no,
        ${pgLiteral(idempotencyKey)} AS idempotency_key,
        ${pgLiteral(product.sku || product.SKU || "VIP-MONTH")} AS sku,
        ${pgLiteral(product.title || "会员月卡")} AS title,
        ${pgLiteral(product.category || "digital")} AS category,
        ${pgLiteral(product.grant_type || product.grantType || "membership")} AS grant_type,
        ${pgLiteral(product.grant_key || product.grantKey || fixture.membershipGrantKey)} AS grant_key,
        ${quantity}::INT AS quantity,
        ${unitPrice}::BIGINT AS unit_price_credits,
        NOW() AS at
    ),
    product_lock AS (
      UPDATE mall_products p
      SET stock = p.stock - fixture.quantity,
          updated_at = fixture.at
      FROM fixture
      WHERE p.id = fixture.product_id
        AND p.stock >= fixture.quantity
      RETURNING p.id, p.sku, p.title, p.stock + fixture.quantity AS before_stock, p.stock AS after_stock
    ),
    inserted_order AS (
      INSERT INTO mall_orders (
        order_no, idempotency_key, user_id, original_credits, discount_credits, total_credits,
        coupon_id, coupon_code, status, receiver, phone, address, payment_method, paid_at, created_at, updated_at
      )
      SELECT
        fixture.order_no,
        fixture.idempotency_key,
        fixture.user_id,
        fixture.unit_price_credits * fixture.quantity,
        0,
        fixture.unit_price_credits * fixture.quantity,
        NULL,
        '',
        'PENDING_PAYMENT',
        '',
        '',
        '',
        '',
        NULL,
        fixture.at,
        fixture.at
      FROM fixture
      JOIN product_lock ON product_lock.id = fixture.product_id
      RETURNING id, user_id, order_no, created_at
    ),
    inserted_item AS (
      INSERT INTO mall_order_items (
        order_id, product_id, sku, title, category, grant_type, grant_key, quantity, unit_price_credits, subtotal_credits
      )
      SELECT
        inserted_order.id,
        fixture.product_id,
        fixture.sku,
        fixture.title,
        fixture.category,
        fixture.grant_type,
        fixture.grant_key,
        fixture.quantity,
        fixture.unit_price_credits,
        fixture.unit_price_credits * fixture.quantity
      FROM inserted_order
      CROSS JOIN fixture
      RETURNING order_id
    ),
    inserted_status_log AS (
      INSERT INTO mall_order_status_logs (
        order_id, from_status, to_status, reason, operator_type, operator_id, note, created_at
      )
      SELECT
        inserted_order.id,
        '',
        'PENDING_PAYMENT',
        'created',
        'user',
        fixture.user_id::TEXT,
        'browser e2e duplicate membership payment fixture',
        inserted_order.created_at
      FROM inserted_order
      CROSS JOIN fixture
      CROSS JOIN inserted_item
      RETURNING order_id
    ),
    inserted_stock_log AS (
      INSERT INTO mall_product_stock_logs (
        product_id, sku, title, delta, before_stock, after_stock, reason, reference_type, reference_id, operator_type, operator_id, note, created_at
      )
      SELECT
        product_lock.id,
        product_lock.sku,
        product_lock.title,
        -fixture.quantity,
        product_lock.before_stock,
        product_lock.after_stock,
        'order_created',
        'order',
        inserted_order.id,
        'user',
        fixture.user_id::TEXT,
        'browser e2e duplicate membership payment fixture',
        inserted_order.created_at
      FROM inserted_order
      CROSS JOIN fixture
      JOIN product_lock ON true
      CROSS JOIN inserted_status_log
      RETURNING reference_id
    )
    SELECT id FROM inserted_order;
  `);
  const orderId = stdout.split(/\s+/).find((part) => /^\d+$/.test(part));
  if (!orderId) {
    throw new Error(`Failed to create duplicate membership payment fixture. psql output: ${stdout.slice(0, 500)}`);
  }
  return { id: orderId, orderNo, idempotencyKey };
}

async function assertPendingMembershipPaymentRejected(fixture, orderId) {
  const failure = await apiRequestFailure(`/mall/orders/${encodeURIComponent(orderId)}/pay`, {
    method: "POST",
    token: fixture.auth.accessToken,
    expectedStatus: 412,
    label: "duplicate pending membership payment",
    body: {
      payment_method: "credits",
      idempotency_key: `duplicate-membership-pay-${orderId}-${Date.now()}`
    }
  });
  const legacyCode = String(failure.meta?.legacy_code || failure.meta?.legacyCode || "");
  const combined = `${failure.message} ${failure.rawBody}`.toLowerCase();
  if (legacyCode !== "FailedPrecondition" || !combined.includes("pending membership order already exists")) {
    throw new Error(`Duplicate pending membership payment rejection mismatch: ${failure.rawBody.slice(0, 800)}`);
  }
  await waitForMallOrderStatus(fixture, orderId, 1, "duplicate membership payment order remains pending after rejected payment");
  const payments = await apiRequest(`/mall/orders/${encodeURIComponent(orderId)}/payments`, {
    token: fixture.auth.accessToken
  });
  const paymentItems = listItems(payments);
  if (paymentItems.length !== 0) {
    throw new Error(`Duplicate membership payment rejection created payment records: ${JSON.stringify(paymentItems.slice(0, 5))}`);
  }
  return {
    status: failure.status,
    message: failure.message || "pending membership order already exists"
  };
}

async function latestMallOrderForProduct(fixture, productId, timeoutMs = 10000) {
  const deadline = Date.now() + timeoutMs;
  while (Date.now() < deadline) {
    const data = await apiRequest("/mall/orders?limit=20&offset=0", {
      token: fixture.auth.accessToken
    });
    const order = listItems(data)
      .filter((item) => orderContainsProduct(item, productId))
      .sort((left, right) => Number(right?.id || 0) - Number(left?.id || 0))[0];
    if (order?.id) {
      return order;
    }
    await delay(250);
  }
  return undefined;
}

async function waitForMallOrderStatus(fixture, orderId, expectedStatus, label, timeoutMs = 10000) {
  const expectedStatuses = Array.isArray(expectedStatus) ? expectedStatus : [expectedStatus];
  const deadline = Date.now() + timeoutMs;
  let lastOrder = null;
  while (Date.now() < deadline) {
    const data = await apiRequest(`/mall/orders/${encodeURIComponent(orderId)}`, {
      token: fixture.auth.accessToken
    });
    lastOrder = data?.order || data;
    if (expectedStatuses.includes(mallOrderStatusValue(lastOrder?.status))) {
      return lastOrder;
    }
    await delay(250);
  }
  throw new Error(`Timed out waiting for ${label}: status=${lastOrder?.status ?? "unknown"}, want ${expectedStatuses.join(" or ")}`);
}

async function waitForMallOrderPaymentStatus(fixture, orderId, expectedStatus, label, timeoutMs = 10000) {
  const deadline = Date.now() + timeoutMs;
  let lastPayments = [];
  while (Date.now() < deadline) {
    const data = await apiRequest(`/mall/orders/${encodeURIComponent(orderId)}/payments`, {
      token: fixture.auth.accessToken
    });
    lastPayments = listItems(data);
    const payment = lastPayments.find((item) => mallPaymentStatusValue(item?.status ?? item?.Status) === expectedStatus);
    if (payment?.id || payment?.ID) {
      return payment;
    }
    await delay(250);
  }
  throw new Error(`Timed out waiting for ${label}. Last payments: ${JSON.stringify(lastPayments.slice(0, 10), null, 2)}`);
}

async function waitForCouponUsageStatus(fixture, couponId, expectedStatus, orderId = "", label = "coupon usage", timeoutMs = 10000) {
  const deadline = Date.now() + timeoutMs;
  let lastUsages = [];
  while (Date.now() < deadline) {
    const data = await apiRequest(`/mall/coupons/mine?limit=50&offset=0&status=${encodeURIComponent(expectedStatus)}`, {
      token: fixture.auth.accessToken
    });
    lastUsages = listItems(data);
    const usage = lastUsages.find((item) => {
      const source = item?.coupon || item?.Coupon || {};
      const itemCouponId = item?.coupon_id ?? item?.couponId ?? source?.id ?? source?.Id;
      const itemOrderId = item?.order_id ?? item?.orderId;
      return (
        String(itemCouponId) === String(couponId) &&
        couponUsageStatusValue(item?.status ?? item?.Status) === expectedStatus &&
        (!orderId || String(itemOrderId) === String(orderId))
      );
    });
    if (usage?.id || usage?.ID) {
      return usage;
    }
    await delay(250);
  }
  throw new Error(`Timed out waiting for ${label}. Last usages: ${JSON.stringify(lastUsages.slice(0, 10), null, 2)}`);
}

async function currentCreditBalance(fixture) {
  const data = await apiRequest("/credits/balance", {
    token: fixture.auth.accessToken
  });
  const total = Number(data?.total ?? data?.balance?.total ?? data?.credits ?? 0);
  if (!Number.isFinite(total)) {
    throw new Error(`Credit balance response did not include numeric total: ${JSON.stringify(data)}`);
  }
  return total;
}

async function topUpUserCredits(fixture, delta, sourceEventId) {
  await apiRequest(`/admin/credits/users/${encodeURIComponent(fixture.auth.user.id)}/adjust`, {
    method: "POST",
    token: fixture.adminToken,
    body: {
      delta,
      reason: "browser_mall_payment_recovery",
      description: "Browser mall insufficient payment recovery top-up",
      source_event_id: sourceEventId
    }
  });
  return currentCreditBalance(fixture);
}

async function currentMallProductStock(productId) {
  const data = await apiRequest(`/mall/products/${encodeURIComponent(productId)}`);
  const product = data?.product || data;
  const stock = Number(product?.stock ?? product?.Stock);
  if (!Number.isFinite(stock)) {
    throw new Error(`Mall product ${productId} did not return numeric stock: ${JSON.stringify(data)}`);
  }
  return stock;
}

async function waitForMallProductStock(productId, expectedStock, label, timeoutMs = 10000) {
  const deadline = Date.now() + timeoutMs;
  let lastStock = null;
  while (Date.now() < deadline) {
    lastStock = await currentMallProductStock(productId);
    if (lastStock === expectedStock) {
      return lastStock;
    }
    await delay(250);
  }
  throw new Error(`Timed out waiting for ${label}: stock=${lastStock}, want ${expectedStock}`);
}

async function latestTopicForTitle(_fixture, title) {
  const deadline = Date.now() + 10000;
  let lastTopics = [];
  while (Date.now() < deadline) {
    const data = await apiRequest("/topics?limit=50&offset=0&type=qa");
    lastTopics = listItems(data);
    const topic = lastTopics.find((item) => String(item?.title || "") === String(title));
    if (topic?.id) {
      return topic;
    }
    await delay(500);
  }
  throw new Error(`Timed out waiting for topic title ${title}. Last topics: ${JSON.stringify(lastTopics.slice(0, 10), null, 2)}`);
}

async function waitForTopicAccepted(topicId, commentId, label, timeoutMs = 10000) {
  const deadline = Date.now() + timeoutMs;
  let lastTopic = null;
  while (Date.now() < deadline) {
    const data = await apiRequest(`/topics/${encodeURIComponent(topicId)}`);
    lastTopic = data?.topic || data;
    const acceptedCommentId = lastTopic?.accepted_comment_id ?? lastTopic?.acceptedCommentId;
    const qaStatus = String(lastTopic?.qa_status ?? lastTopic?.qaStatus ?? "").toLowerCase();
    if (String(acceptedCommentId) === String(commentId) && qaStatus === "resolved") {
      return lastTopic;
    }
    await delay(300);
  }
  throw new Error(
    `Timed out waiting for ${label}: accepted=${lastTopic?.accepted_comment_id ?? lastTopic?.acceptedCommentId ?? "unknown"}, qa_status=${lastTopic?.qa_status ?? lastTopic?.qaStatus ?? "unknown"}`,
  );
}

async function waitForTopicUnaccepted(topicId, commentId, label, timeoutMs = 10000) {
  const deadline = Date.now() + timeoutMs;
  let lastTopic = null;
  while (Date.now() < deadline) {
    const data = await apiRequest(`/topics/${encodeURIComponent(topicId)}`);
    lastTopic = data?.topic || data;
    const acceptedCommentId = lastTopic?.accepted_comment_id ?? lastTopic?.acceptedCommentId;
    const qaStatus = String(lastTopic?.qa_status ?? lastTopic?.qaStatus ?? "").toLowerCase();
    if (!acceptedCommentId && qaStatus === "open") {
      return lastTopic;
    }
    await delay(300);
  }
  throw new Error(
    `Timed out waiting for ${label}: accepted=${lastTopic?.accepted_comment_id ?? lastTopic?.acceptedCommentId ?? "unknown"}, qa_status=${lastTopic?.qa_status ?? lastTopic?.qaStatus ?? "unknown"}`,
  );
}

async function waitForCreditLedgerEntry(token, predicate, label, timeoutMs = 20000) {
  const deadline = Date.now() + timeoutMs;
  let lastItems = [];
  while (Date.now() < deadline) {
    const data = await apiRequest("/credits/ledger?limit=50&offset=0", {
      token
    });
    lastItems = listItems(data);
    const entry = lastItems.find(predicate);
    if (entry?.id || entry?.ID) {
      return entry;
    }
    await delay(1000);
  }
  throw new Error(
    `Timed out waiting for ${label}. Last ledger entries: ${JSON.stringify(lastItems.slice(0, 10), null, 2)}`,
  );
}

async function assertRefundCreditLedgerIdempotent(fixture, refundId, expectedDelta, adminNote) {
  const sourceEventId = `mall.refund:${refundId}`;
  const reason = "mall_order_refund";
  const token = fixture.auth.accessToken;
  const firstEntry = await waitForCreditLedgerEntry(
    token,
    (entry) =>
      creditLedgerSourceEventId(entry) === sourceEventId &&
      creditLedgerReason(entry) === reason &&
      creditLedgerDelta(entry) === expectedDelta,
    "mall refund credit ledger",
  );
  const entriesBeforeRetry = await creditLedgerEntriesForSource(token, sourceEventId, reason);
  if (entriesBeforeRetry.length !== 1) {
    throw new Error(`Refund credit ledger entries before retry = ${entriesBeforeRetry.length}, want 1 for ${sourceEventId}`);
  }
  const balanceBeforeRetry = await currentCreditBalance(fixture);

  await approveMallRefund(fixture, refundId, `${adminNote} retry`);

  const entriesAfterRetry = await creditLedgerEntriesForSource(token, sourceEventId, reason);
  if (entriesAfterRetry.length !== 1) {
    throw new Error(`Refund credit ledger entries after retry = ${entriesAfterRetry.length}, want 1 for ${sourceEventId}`);
  }
  const balanceAfterRetry = await currentCreditBalance(fixture);
  if (balanceAfterRetry !== balanceBeforeRetry) {
    throw new Error(`Refund retry changed credit balance from ${balanceBeforeRetry} to ${balanceAfterRetry}`);
  }
  return {
    ledgerId: String(firstEntry.id ?? firstEntry.ID ?? ""),
    sourceEventId,
    countAfterRetry: entriesAfterRetry.length,
    balanceAfterRetry
  };
}

async function creditLedgerEntriesForSource(token, sourceEventId, reason) {
  const data = await apiRequest("/credits/ledger?limit=100&offset=0", {
    token
  });
  return listItems(data).filter((entry) => creditLedgerSourceEventId(entry) === sourceEventId && creditLedgerReason(entry) === reason);
}

function creditLedgerSourceEventId(entry) {
  return String(entry?.source_event_id ?? entry?.sourceEventId ?? entry?.SourceEventId ?? "");
}

function creditLedgerReason(entry) {
  return String(entry?.reason ?? entry?.Reason ?? "").trim();
}

function creditLedgerDelta(entry) {
  return Number(entry?.delta ?? entry?.Delta ?? 0);
}

async function latestMyTopicForTitle(fixture, title) {
  const deadline = Date.now() + 10000;
  let lastTopics = [];
  while (Date.now() < deadline) {
    const data = await apiRequest("/users/me/topics?status=0&limit=50&offset=0", {
      token: fixture.auth.accessToken
    });
    lastTopics = listItems(data);
    const topic = lastTopics.find((item) => String(item?.title || "") === String(title));
    if (topic?.id) {
      return topic;
    }
    await delay(500);
  }
  throw new Error(`Timed out waiting for user topic title ${title}. Last topics: ${JSON.stringify(lastTopics.slice(0, 10), null, 2)}`);
}

async function waitForDigitalEntitlement(fixture, orderId, productId, grantKey, status = "ACTIVE") {
  const entitlements = await waitForDigitalEntitlements(fixture, orderId, productId, grantKey, status, 1);
  return entitlements[0];
}

async function waitForDigitalEntitlements(fixture, orderId, productId, grantKey, status = "ACTIVE", expectedCount = 1) {
  const deadline = Date.now() + 15000;
  let lastEntitlements = [];
  while (Date.now() < deadline) {
    const data = await apiRequest(`/mall/digital-entitlements?limit=50&offset=0&status=${encodeURIComponent(status)}`, {
      token: fixture.auth.accessToken
    });
    lastEntitlements = listItems(data);
    const entitlements = lastEntitlements.filter((item) => {
      const itemOrderId = item?.order_id ?? item?.orderId;
      const itemProductId = item?.product_id ?? item?.productId;
      const itemGrantKey = item?.grant_key ?? item?.grantKey;
      return (
        String(itemOrderId) === String(orderId) &&
        String(itemProductId) === String(productId) &&
        String(itemGrantKey) === String(grantKey)
      );
    });
    if (entitlements.length >= expectedCount) {
      return entitlements.slice(0, expectedCount);
    }
    await delay(500);
  }
  throw new Error(`Timed out waiting for ${expectedCount} ${status} digital entitlements order=${orderId} product=${productId} grant=${grantKey}. Last entitlements: ${JSON.stringify(lastEntitlements.slice(0, 10), null, 2)}`);
}

async function latestRefundForOrder(fixture, orderId) {
  const data = await apiRequest("/mall/refunds?limit=50&offset=0", {
    token: fixture.auth.accessToken
  });
  return listItems(data).find((refund) => String(refund?.order_id ?? refund?.orderId ?? "") === String(orderId));
}

async function waitForRefundStatus(fixture, refundId, expectedStatus, label, timeoutMs = 10000) {
  const deadline = Date.now() + timeoutMs;
  let lastRefund;
  while (Date.now() < deadline) {
    const data = await apiRequest("/mall/refunds?limit=50&offset=0", {
      token: fixture.auth.accessToken
    });
    lastRefund = listItems(data).find((refund) => String(refund?.id ?? refund?.ID ?? "") === String(refundId));
    if (lastRefund && refundStatusValue(lastRefund.status) === expectedStatus) {
      return lastRefund;
    }
    await delay(250);
  }
  throw new Error(`Timed out waiting for ${label}: status=${lastRefund?.status ?? "unknown"}, want ${expectedStatus}`);
}

async function waitForNewRefundForOrder(fixture, orderId, previousRefundId, label, timeoutMs = 10000) {
  const deadline = Date.now() + timeoutMs;
  let lastRefunds = [];
  while (Date.now() < deadline) {
    const data = await apiRequest("/mall/refunds?limit=50&offset=0", {
      token: fixture.auth.accessToken
    });
    lastRefunds = listItems(data).filter((refund) => String(refund?.order_id ?? refund?.orderId ?? "") === String(orderId));
    const refund = lastRefunds.find((item) => String(item?.id ?? item?.ID ?? "") !== String(previousRefundId));
    if (refund?.id || refund?.ID) {
      return refund;
    }
    await delay(250);
  }
  throw new Error(`Timed out waiting for ${label}. Last refunds: ${JSON.stringify(lastRefunds.slice(0, 10), null, 2)}`);
}

async function createMallRefund(fixture, orderId, note) {
  const data = await apiRequest(`/mall/orders/${encodeURIComponent(orderId)}/refunds`, {
    method: "POST",
    token: fixture.auth.accessToken,
    body: {
      reason: "digital_entitlement_refund",
      note
    }
  });
  const status = Number(data?.refund?.status);
  if (!data?.refund?.id || status !== 1) {
    throw new Error(`Mall refund request did not create pending refund for order ${orderId}, status=${data?.refund?.status ?? "unknown"}`);
  }
  return data.refund;
}

async function latestProductReviewForOrder(fixture, orderId, productId) {
  const deadline = Date.now() + 10000;
  let lastReviews = [];
  while (Date.now() < deadline) {
    const data = await apiRequest(`/mall/reviews?product_id=${encodeURIComponent(productId)}&limit=20&offset=0`, {
      token: fixture.auth.accessToken
    });
    lastReviews = listItems(data);
    const review = lastReviews.find((item) =>
      String(item?.order_id ?? item?.orderId ?? "") === String(orderId) &&
      String(item?.product_id ?? item?.productId ?? "") === String(productId)
    );
    if (review?.id) {
      return review;
    }
    await delay(500);
  }
  throw new Error(`Timed out waiting for product review for order ${orderId}, product ${productId}. Last reviews: ${JSON.stringify(lastReviews.slice(0, 10), null, 2)}`);
}

async function assertDuplicateProductReviewRejected(fixture, orderId, productId, reviewContent) {
  const failure = await apiRequestFailure(`/mall/products/${encodeURIComponent(productId)}/reviews`, {
    method: "POST",
    token: fixture.auth.accessToken,
    expectedStatus: 409,
    label: "duplicate product review",
    body: {
      order_id: Number(orderId),
      rating: 5,
      content: `${reviewContent}（重复提交校验）`
    }
  });
  const legacyCode = String(failure.meta?.legacy_code || failure.meta?.legacyCode || "");
  if (legacyCode !== "AlreadyExists") {
    throw new Error(`Duplicate product review legacy code = ${legacyCode || "missing"}, want AlreadyExists. Raw: ${failure.rawBody.slice(0, 800)}`);
  }
  const frontendText = friendlyMallReviewError({
    message: failure.message,
    meta: failure.meta,
    httpCode: failure.httpCode,
    status: failure.status
  });
  if (frontendText !== "该订单已评价过该商品，请勿重复提交。") {
    throw new Error(`Duplicate product review frontend text = ${frontendText}, want duplicate review copy`);
  }
  return {
    status: failure.status,
    legacyCode,
    frontendText
  };
}

async function hideMallReview(fixture, reviewId) {
  const data = await apiRequest(`/admin/mall/reviews/${encodeURIComponent(reviewId)}/status`, {
    method: "PUT",
    token: fixture.adminToken,
    body: {
      status: 3
    }
  });
  const status = data?.review?.status;
  if (Number(status) !== 3 && !String(status).includes("HIDDEN")) {
    throw new Error(`Admin review hide did not hide review ${reviewId}, status=${status ?? "unknown"}`);
  }
  return data.review;
}

async function approveMallRefund(fixture, refundId, adminNote) {
  const data = await apiRequest(`/admin/mall/refunds/${encodeURIComponent(refundId)}/review`, {
    method: "POST",
    token: fixture.adminToken,
    body: {
      approved: true,
      admin_note: adminNote,
      restore_stock: true
    }
  });
  const status = Number(data?.refund?.status);
  if (status !== 3) {
    throw new Error(`Admin refund approval did not approve refund ${refundId}, status=${data?.refund?.status ?? "unknown"}`);
  }
  return data.refund;
}

async function rejectMallRefund(fixture, refundId, adminNote) {
  const data = await apiRequest(`/admin/mall/refunds/${encodeURIComponent(refundId)}/review`, {
    method: "POST",
    token: fixture.adminToken,
    body: {
      approved: false,
      admin_note: adminNote,
      restore_stock: false
    }
  });
  const status = Number(data?.refund?.status);
  if (status !== 4) {
    throw new Error(`Admin refund rejection did not reject refund ${refundId}, status=${data?.refund?.status ?? "unknown"}`);
  }
  return data.refund;
}

async function revokeMallDigitalEntitlement(fixture, entitlementId, reason) {
  const data = await apiRequest(`/admin/mall/digital-entitlements/${encodeURIComponent(entitlementId)}/revoke`, {
    method: "POST",
    token: fixture.adminToken,
    body: { reason }
  });
  if (!data?.entitlement?.id) {
    throw new Error(`Admin entitlement revoke did not return entitlement ${entitlementId}`);
  }
  return data.entitlement;
}

async function publishMallReview(fixture, reviewId) {
  const data = await apiRequest(`/admin/mall/reviews/${encodeURIComponent(reviewId)}/status`, {
    method: "PUT",
    token: fixture.adminToken,
    body: {
      status: 2
    }
  });
  const status = data?.review?.status;
  if (Number(status) !== 2 && !String(status).includes("PUBLISHED")) {
    throw new Error(`Admin review publish did not publish review ${reviewId}, status=${status ?? "unknown"}`);
  }
  return data.review;
}

async function waitForMallOrderNotifications(fixture, orderId, expectedTitles) {
  const deadline = Date.now() + 30000;
  const expected = expectedTitles.map((title) => String(title));
  let lastNotifications = [];
  while (Date.now() < deadline) {
    const data = await apiRequest("/notifications?limit=50&offset=0", {
      token: fixture.auth.accessToken
    });
    lastNotifications = listItems(data).filter((item) => notificationBelongsToOrder(item, orderId));
    const presentTitles = new Set(lastNotifications.map((item) => item.title || ""));
    if (expected.every((title) => presentTitles.has(title))) {
      return expected
        .map((title) => lastNotifications.find((item) => item.title === title))
        .filter(Boolean);
    }
    await delay(500);
  }
  throw new Error(`Timed out waiting for mall order notifications ${expected.join(", ")} for order ${orderId}. Last notifications: ${JSON.stringify(lastNotifications.slice(0, 10), null, 2)}`);
}

async function waitForMallReviewNotifications(fixture, reviewId, expectedTitles) {
  const deadline = Date.now() + 30000;
  const expected = expectedTitles.map((title) => String(title));
  let lastNotifications = [];
  while (Date.now() < deadline) {
    const data = await apiRequest("/notifications?limit=50&offset=0", {
      token: fixture.auth.accessToken
    });
    lastNotifications = listItems(data).filter((item) => notificationBelongsToReview(item, reviewId));
    const presentTitles = new Set(lastNotifications.map((item) => item.title || ""));
    if (expected.every((title) => presentTitles.has(title))) {
      return expected
        .map((title) => lastNotifications.find((item) => item.title === title))
        .filter(Boolean);
    }
    await delay(500);
  }
  throw new Error(`Timed out waiting for mall review notifications ${expected.join(", ")} for review ${reviewId}. Last notifications: ${JSON.stringify(lastNotifications.slice(0, 10), null, 2)}`);
}

function notificationBelongsToOrder(item, orderId) {
  const entityType = item?.entity_type ?? item?.entityType;
  const entityId = item?.entity_id ?? item?.entityId;
  return entityType === "mall_order" && String(entityId) === String(orderId);
}

function notificationBelongsToReview(item, reviewId) {
  const entityType = item?.entity_type ?? item?.entityType;
  const sourceId = item?.source_id ?? item?.sourceId;
  return entityType === "mall_product" && String(sourceId) === String(reviewId);
}

async function shipMallOrder(fixture, orderId) {
  const data = await apiRequest(`/admin/mall/orders/${encodeURIComponent(orderId)}/status`, {
    method: "PUT",
    token: fixture.adminToken,
    body: {
      status: 5,
      shipping_carrier: "E2E Express",
      tracking_no: `E2E${Date.now()}`,
      note: "Browser E2E order shipped"
    }
  });
  const status = Number(data?.order?.status);
  if (status !== 5) {
    throw new Error(`Admin ship did not move order to shipped, status=${data?.order?.status ?? "unknown"}`);
  }
  return data.order;
}

function listItems(data) {
  if (Array.isArray(data?.items)) return data.items;
  if (Array.isArray(data?.list)) return data.list;
  return [];
}

function couponUsageCode(usage) {
  const coupon = usage?.coupon || usage?.Coupon || {};
  return String(usage?.code || usage?.Code || coupon?.code || coupon?.Code || "").trim().toUpperCase();
}

function orderContainsProduct(order, productId) {
  const normalizedProductId = String(productId);
  return Array.isArray(order?.items) && order.items.some((item) => String(item?.product_id ?? item?.productId ?? item?.product?.id ?? "") === normalizedProductId);
}

function orderQuantityForProduct(order, productId) {
  const normalizedProductId = String(productId);
  return (Array.isArray(order?.items) ? order.items : [])
    .filter((item) => String(item?.product_id ?? item?.productId ?? item?.product?.id ?? "") === normalizedProductId)
    .reduce((sum, item) => sum + Number(item?.quantity ?? item?.Quantity ?? 0), 0);
}

function mallOrderStatusValue(status) {
  if (status === undefined || status === null || status === "") return 0;
  const numeric = Number(status);
  if (!Number.isNaN(numeric) && numeric > 0) return numeric;
  const labels = {
    PENDING_PAYMENT: 1,
    ORDER_STATUS_PENDING_PAYMENT: 1,
    PAYING: 2,
    ORDER_STATUS_PAYING: 2,
    PAID: 3,
    ORDER_STATUS_PAID: 3,
    CANCELED: 4,
    ORDER_STATUS_CANCELED: 4,
    SHIPPED: 5,
    ORDER_STATUS_SHIPPED: 5,
    COMPLETED: 6,
    ORDER_STATUS_COMPLETED: 6,
    CLOSED: 7,
    ORDER_STATUS_CLOSED: 7,
    REFUNDED: 8,
    ORDER_STATUS_REFUNDED: 8
  };
  return labels[String(status).trim().toUpperCase()] || 0;
}

function mallPaymentStatusValue(status) {
  if (status === undefined || status === null || status === "") return 0;
  const numeric = Number(status);
  if (!Number.isNaN(numeric) && numeric > 0) return numeric;
  const labels = {
    PENDING: 1,
    PAYMENT_STATUS_PENDING: 1,
    SUCCEEDED: 2,
    PAYMENT_STATUS_SUCCEEDED: 2,
    FAILED: 3,
    PAYMENT_STATUS_FAILED: 3
  };
  return labels[String(status).trim().toUpperCase()] || 0;
}

function refundStatusValue(status) {
  if (status === undefined || status === null || status === "") return 0;
  const numeric = Number(status);
  if (!Number.isNaN(numeric) && numeric > 0) return numeric;
  const labels = {
    REQUESTED: 1,
    REFUND_STATUS_REQUESTED: 1,
    PROCESSING: 2,
    REFUND_STATUS_PROCESSING: 2,
    APPROVED: 3,
    REFUND_STATUS_APPROVED: 3,
    REJECTED: 4,
    REFUND_STATUS_REJECTED: 4,
    CANCELED: 5,
    REFUND_STATUS_CANCELED: 5
  };
  return labels[String(status).trim().toUpperCase()] || 0;
}

function couponUsageStatusValue(status) {
  if (status === undefined || status === null || status === "") return 0;
  const numeric = Number(status);
  if (!Number.isNaN(numeric) && numeric > 0) return numeric;
  const labels = {
    RESERVED: 1,
    COUPON_USAGE_STATUS_RESERVED: 1,
    USED: 2,
    COUPON_USAGE_STATUS_USED: 2,
    RELEASED: 3,
    COUPON_USAGE_STATUS_RELEASED: 3,
    CLAIMED: 4,
    COUPON_USAGE_STATUS_CLAIMED: 4
  };
  return labels[String(status).trim().toUpperCase()] || 0;
}

async function apiRequest(pathname, { method = "GET", body, token } = {}) {
  const response = await fetch(`${API_BASE}${pathname}`, {
    method,
    headers: {
      ...(body === undefined ? {} : { "Content-Type": "application/json" }),
      ...(token ? { Authorization: `Bearer ${token}` } : {})
    },
    body: body === undefined ? undefined : JSON.stringify(body)
  });
  const text = await response.text();
  const data = parseResponseBody(text);
  if (!response.ok || (data && typeof data === "object" && "code" in data && data.code !== 0)) {
    throw new Error(`${method} ${pathname} failed (${response.status}): ${text.slice(0, 800)}`);
  }
  return data?.data ?? data;
}

async function apiRequestFailure(pathname, { method = "GET", body, token, expectedStatus, label = "api failure" } = {}) {
  const response = await fetch(`${API_BASE}${pathname}`, {
    method,
    headers: {
      ...(body === undefined ? {} : { "Content-Type": "application/json" }),
      ...(token ? { Authorization: `Bearer ${token}` } : {})
    },
    body: body === undefined ? undefined : JSON.stringify(body)
  });
  const text = await response.text();
  const data = parseResponseBody(text);
  const failed = !response.ok || (data && typeof data === "object" && "code" in data && data.code !== 0);
  if (!failed) {
    throw new Error(`${label} unexpectedly succeeded (${response.status}): ${text.slice(0, 800)}`);
  }
  if (expectedStatus && response.status !== expectedStatus) {
    throw new Error(`${label} failed with HTTP ${response.status}, want ${expectedStatus}: ${text.slice(0, 800)}`);
  }
  return {
    status: response.status,
    httpCode: Number(data?.http_code || response.status),
    message: String(data?.message || data?.reason || data?.error?.message || data?.error || ""),
    meta: data?.meta || {},
    rawBody: text
  };
}

function runMallPsql(sql) {
  return runPsql(mallPsqlDsn(), "mall frontend E2E fixture", sql);
}

function runPsql(dsn, label, sql) {
  return new Promise((resolve, reject) => {
    const child = spawn(
      PSQL_BIN,
      [
        "--dbname",
        dsn,
        "--no-align",
        "--tuples-only",
        "--set",
        "ON_ERROR_STOP=1",
        "--command",
        sql
      ],
      { stdio: ["ignore", "pipe", "pipe"], windowsHide: true }
    );
    const stdout = [];
    const stderr = [];
    child.stdout?.on("data", (chunk) => stdout.push(String(chunk)));
    child.stderr?.on("data", (chunk) => stderr.push(String(chunk)));
    child.on("error", (error) => {
      reject(new Error(`Failed to run psql for ${label}. Set PSQL_BIN if psql is not in PATH. ${error.message}`));
    });
    child.on("close", (code) => {
      if (code === 0) {
        resolve(stdout.join(""));
        return;
      }
      reject(new Error(`psql failed while preparing ${label} (${code}): ${stderr.join("").slice(0, 800)}`));
    });
  });
}

function mallPsqlDsn() {
  return psqlDsnWithoutSearchPath(MALL_POSTGRES_DSN);
}

function psqlDsnWithoutSearchPath(dsn) {
  try {
    const url = new URL(dsn);
    url.searchParams.delete("search_path");
    return url.toString();
  } catch {
    return dsn;
  }
}

function pgLiteral(value) {
  return `'${String(value).replace(/'/g, "''")}'`;
}

function parseResponseBody(text) {
  if (!text) return null;
  try {
    return JSON.parse(text.replace(/(:\s*)(-?\d{16,})(?=[,}\]])/g, '$1"$2"'));
  } catch {
    return null;
  }
}

async function ensureFrontendServer() {
  const shopUrl = `${FRONTEND_BASE}/shop`;
  if (await httpReachable(shopUrl)) {
    return null;
  }
  if (truthyEnv("MALL_E2E_NO_AUTO_FRONTEND")) {
    await assertHttpReachable(shopUrl, "frontend");
  }
  if (!existsSync(VITE_BIN)) {
    throw new Error(`frontend is not reachable at ${shopUrl}, and Vite was not found at ${VITE_BIN}. Run npm install in frontend or start the frontend manually.`);
  }

  const frontendUrl = new URL(FRONTEND_BASE);
  const host = frontendUrl.hostname || "127.0.0.1";
  const port = frontendUrl.port || (frontendUrl.protocol === "https:" ? "443" : "80");
  const logs = [];
  const server = spawn(process.execPath, [VITE_BIN, "--host", host, "--port", port, "--strictPort"], {
    cwd: FRONTEND_DIR,
    env: {
      ...process.env,
      VITE_API_BASE: API_BASE
    },
    stdio: ["ignore", "pipe", "pipe"],
    windowsHide: true
  });

  server.stdout?.on("data", (chunk) => pushLog(logs, chunk));
  server.stderr?.on("data", (chunk) => pushLog(logs, chunk));
  try {
    await waitForHttpReachable(shopUrl, "frontend", 45000, server, logs);
  } catch (error) {
    await stopProcess(server);
    throw error;
  }
  console.log(`Started frontend dev server for mall e2e at ${FRONTEND_BASE}`);
  return {
    stop: () => stopProcess(server)
  };
}

async function assertHttpReachable(url, label) {
  try {
    const response = await fetch(url);
    if (response.status >= 500) {
      throw new Error(`HTTP ${response.status}`);
    }
  } catch (error) {
    throw new Error(`${label} is not reachable at ${url}: ${error.message}`);
  }
}

async function httpReachable(url) {
  try {
    const response = await fetch(url);
    return response.status < 500;
  } catch {
    return false;
  }
}

async function waitForHttpReachable(url, label, timeoutMs, child, logs) {
  const deadline = Date.now() + timeoutMs;
  while (Date.now() < deadline) {
    if (child.exitCode !== null) {
      throw new Error(`${label} dev server exited early with code ${child.exitCode}. Logs:\n${logs.join("").slice(-3000)}`);
    }
    if (await httpReachable(url)) return;
    await delay(250);
  }
  throw new Error(`Timed out waiting for ${label} at ${url}. Logs:\n${logs.join("").slice(-3000)}`);
}

function pushLog(logs, chunk) {
  logs.push(String(chunk));
  while (logs.join("").length > 5000) {
    logs.shift();
  }
}

function truthyEnv(name) {
  return /^(1|true|yes|on)$/i.test(String(process.env[name] || ""));
}

async function findChromeExecutable() {
  const explicit = process.env.CHROME_EXECUTABLE || process.env.MALL_E2E_CHROME || process.env.PLAYWRIGHT_CHROMIUM_EXECUTABLE;
  const candidates = [
    explicit,
    ...(await puppeteerChromeCandidates()),
    "C:\\Program Files\\Google\\Chrome\\Application\\chrome.exe",
    "C:\\Program Files (x86)\\Google\\Chrome\\Application\\chrome.exe",
    path.join(os.homedir(), "AppData", "Local", "Google", "Chrome", "Application", "chrome.exe"),
    "C:\\Program Files\\Microsoft\\Edge\\Application\\msedge.exe",
    "C:\\Program Files (x86)\\Microsoft\\Edge\\Application\\msedge.exe",
    "/Applications/Google Chrome.app/Contents/MacOS/Google Chrome",
    "/Applications/Microsoft Edge.app/Contents/MacOS/Microsoft Edge",
    "/usr/bin/google-chrome",
    "/usr/bin/chromium",
    "/usr/bin/chromium-browser"
  ].filter(Boolean);
  const match = candidates.find((candidate) => existsSync(candidate));
  if (!match) {
    throw new Error("Chrome/Chromium executable not found. Set CHROME_EXECUTABLE to run frontend mall e2e.");
  }
  return match;
}

async function puppeteerChromeCandidates() {
  const base = path.join(os.homedir(), ".cache", "puppeteer", "chrome");
  if (!existsSync(base)) return [];
  const entries = await readdir(base, { withFileTypes: true }).catch(() => []);
  return entries
    .filter((entry) => entry.isDirectory())
    .map((entry) => path.join(base, entry.name, "chrome-win64", "chrome.exe"))
    .filter((candidate) => existsSync(candidate))
    .sort()
    .reverse();
}

async function getFreePort() {
  return new Promise((resolve, reject) => {
    const server = net.createServer();
    server.on("error", reject);
    server.listen(0, "127.0.0.1", () => {
      const address = server.address();
      server.close(() => resolve(address.port));
    });
  });
}

async function waitForBrowserWebSocket(port, chrome, stderr) {
  const deadline = Date.now() + 15000;
  while (Date.now() < deadline) {
    if (chrome.exitCode !== null) {
      throw new Error(`Chrome exited early with code ${chrome.exitCode}: ${stderr.join("").slice(0, 1000)}`);
    }
    try {
      const response = await fetch(`http://127.0.0.1:${port}/json/version`);
      if (response.ok) {
        const data = await response.json();
        if (data.webSocketDebuggerUrl) return data.webSocketDebuggerUrl;
      }
    } catch {
      // Chrome is still starting.
    }
    await delay(100);
  }
  throw new Error(`Timed out waiting for Chrome DevTools endpoint: ${stderr.join("").slice(0, 1000)}`);
}

async function createPageWebSocket(port, browserWs) {
  const response = await fetch(`http://127.0.0.1:${port}/json/new?about:blank`, { method: "PUT" }).catch(() => null);
  if (response?.ok) {
    const data = await response.json();
    if (data.webSocketDebuggerUrl) return data.webSocketDebuggerUrl;
  }

  const browser = new CDPClient(browserWs);
  await browser.connect();
  try {
    const { targetId } = await browser.send("Target.createTarget", { url: "about:blank" });
    const list = await fetch(`http://127.0.0.1:${port}/json/list`).then((item) => item.json());
    const target = list.find((item) => item.id === targetId);
    if (!target?.webSocketDebuggerUrl) {
      throw new Error("Created Chrome target did not expose websocket URL");
    }
    return target.webSocketDebuggerUrl;
  } finally {
    await browser.close();
  }
}

async function navigate(page, url) {
  await page.send("Page.navigate", { url });
  await waitFor(page, `document.readyState === "complete" || document.readyState === "interactive"`, `navigate ${url}`, 20000);
  await delay(500);
}

async function waitForText(page, pattern, label = pattern, timeoutMs = 20000) {
  const source = pattern instanceof RegExp ? pattern.source : String(pattern);
  await waitFor(page, `new RegExp(${JSON.stringify(source)}, "i").test(document.body?.innerText || "")`, label, timeoutMs);
}

async function waitForPublicProductReview(page, reviewContent, label = "public product review", timeoutMs = 20000) {
  await waitFor(page, `Array.from(document.querySelectorAll(".product-review-block > .product-review-item"))
    .some((item) => (item.innerText || "").includes(${JSON.stringify(reviewContent)}))`, label, timeoutMs);
  return evaluate(page, `(() => {
    const items = Array.from(document.querySelectorAll(".product-review-block > .product-review-item"));
    const item = items.find((node) => (node.innerText || "").includes(${JSON.stringify(reviewContent)}));
    return item?.innerText || "";
  })()`);
}

async function waitForPublicProductReviewHidden(page, reviewContent, label = "hidden public product review", timeoutMs = 20000) {
  await waitFor(page, `(() => {
    const block = document.querySelector(".product-review-block");
    if (!block) return false;
    if ((block.innerText || "").includes("加载中")) return false;
    return Array.from(document.querySelectorAll(".product-review-block > .product-review-item"))
      .every((item) => !(item.innerText || "").includes(${JSON.stringify(reviewContent)}));
  })()`, label, timeoutMs);
  const text = await evaluate(page, `(() => {
    const items = Array.from(document.querySelectorAll(".product-review-block > .product-review-item"));
    return items.map((item) => (item.innerText || "").trim()).filter(Boolean).join("\\n");
  })()`);
  return text || "公开评价列表已移除隐藏评价";
}

async function waitForProfileThemeClass(page, expectedClass, label = "profile theme class", timeoutMs = 20000) {
  await waitFor(page, `document.querySelector(".user-profile-card")?.classList.contains(${JSON.stringify(expectedClass)})`, label, timeoutMs);
  return evaluate(page, `document.querySelector(".user-profile-card")?.className || ""`);
}

async function waitForProfileBackgroundStyle(page, expectedUrl, label = "profile background", timeoutMs = 20000) {
  await waitFor(
    page,
    `document.querySelector(".user-profile-cover")?.style.backgroundImage.includes(${JSON.stringify(expectedUrl)})`,
    label,
    timeoutMs
  );
  return evaluate(page, `document.querySelector(".user-profile-cover")?.style.backgroundImage || ""`);
}

async function waitForProfileBackgroundCleared(page, label = "profile background cleared", timeoutMs = 20000) {
  await waitFor(
    page,
    `(() => {
      const cover = document.querySelector(".user-profile-cover");
      if (!cover) return false;
      const value = cover.style.backgroundImage || "";
      return value === "" || value === "none";
    })()`,
    label,
    timeoutMs
  );
  return evaluate(page, `document.querySelector(".user-profile-cover")?.style.backgroundImage || ""`);
}

async function waitForDashboardProfileBackgroundCleared(page, label = "dashboard profile background cleared", timeoutMs = 20000) {
  await waitFor(
    page,
    `(() => {
      const preview = document.querySelector(".profile-background-preview");
      if (!preview) return false;
      const value = preview.style.backgroundImage || "";
      return value === "" || value === "none";
    })()`,
    label,
    timeoutMs
  );
  return evaluate(page, `document.querySelector(".profile-background-preview")?.style.backgroundImage || ""`);
}

async function waitForAddressDeleted(page, receiver, label = "address deleted", timeoutMs = 20000) {
  await waitFor(page, `!Array.from(document.querySelectorAll(".address-manager-panel article"))
    .some((item) => (item.innerText || "").includes(${JSON.stringify(receiver)}))`, label, timeoutMs);
}

async function waitForPublicBadgePanelReady(page, badgeTitle, label = "public badge panel", timeoutMs = 20000) {
  await waitFor(page, `(() => {
    const text = document.body?.innerText || "";
    if (!text.includes("用户空间")) return false;
    if (text.includes("正在加载用户资料") || text.includes("正在加载用户徽章")) return false;
    return text.includes(${JSON.stringify(badgeTitle)}) ||
      text.includes("暂无公开徽章") ||
      document.querySelector(".data-row") !== null;
  })()`, label, timeoutMs);
}

async function waitFor(page, expression, label, timeoutMs = 15000) {
  const deadline = Date.now() + timeoutMs;
  let lastError = "";
  while (Date.now() < deadline) {
    try {
      if (await evaluate(page, `Boolean(${expression})`)) return;
    } catch (error) {
      lastError = error.message;
    }
    await delay(150);
  }
  const text = await bodyText(page).catch(() => "");
  throw new Error(`Timed out waiting for ${label}${lastError ? ` (${lastError})` : ""}. Body: ${text.slice(0, 1200)}`);
}

async function bodyText(page) {
  return evaluate(page, `document.body?.innerText || ""`);
}

async function clickButton(page, buttonPattern) {
  return evaluate(
    page,
    `(() => {
      const pattern = new RegExp(${JSON.stringify(buttonPattern)}, "i");
      const buttons = Array.from(document.querySelectorAll("button"));
      const button = buttons.find((item) => pattern.test((item.innerText || item.textContent || "").trim()));
      if (!button) throw new Error("Button not found: ${escapeForScript(buttonPattern)}");
      if (button.disabled) throw new Error("Button disabled: " + (button.innerText || button.textContent || "").trim());
      button.scrollIntoView({ block: "center", inline: "center" });
      button.click();
      return (button.innerText || button.textContent || "").trim();
    })()`
  );
}

async function clickButtonWhenEnabled(page, buttonPattern, label = buttonPattern, timeoutMs = 20000) {
  const deadline = Date.now() + timeoutMs;
  let lastError = "";
  while (Date.now() < deadline) {
    try {
      return await clickButton(page, buttonPattern);
    } catch (error) {
      lastError = error.message || String(error);
      if (!lastError.includes("Button not found") && !lastError.includes("Button disabled")) {
        throw error;
      }
    }
    await delay(150);
  }
  const text = await bodyText(page).catch(() => "");
  throw new Error(`Timed out waiting to click ${label}${lastError ? ` (${lastError})` : ""}. Body: ${text.slice(0, 1200)}`);
}

async function waitForButtonEnabled(page, buttonPattern, label = buttonPattern, timeoutMs = 20000) {
  await waitFor(
    page,
    `(() => {
      const pattern = new RegExp(${JSON.stringify(buttonPattern)}, "i");
      const button = Array.from(document.querySelectorAll("button")).find((item) => pattern.test((item.innerText || item.textContent || "").trim()));
      return Boolean(button && !button.disabled);
    })()`,
    label,
    timeoutMs
  );
}

async function waitForButtonNearTextEnabled(page, containerText, buttonPattern, label = buttonPattern, timeoutMs = 20000) {
  await waitFor(
    page,
    `(() => {
      const containerNeedle = ${JSON.stringify(containerText)};
      const pattern = new RegExp(${JSON.stringify(buttonPattern)}, "i");
      const containers = Array.from(document.querySelectorAll("article, section.product-detail-panel, section.cart-panel, section.checkout-panel, section.panel"));
      return containers.some((container) => {
        if (!(container.innerText || "").includes(containerNeedle)) return false;
        return Array.from(container.querySelectorAll("button")).some((button) => pattern.test((button.innerText || button.textContent || "").trim()) && !button.disabled);
      });
    })()`,
    label,
    timeoutMs
  );
}

async function clickButtonInArticle(page, articleText, buttonPattern, label = buttonPattern, timeoutMs = 20000) {
  const deadline = Date.now() + timeoutMs;
  let lastError = "";
  while (Date.now() < deadline) {
    try {
      return await evaluate(
        page,
        `(() => {
          const articleNeedle = ${JSON.stringify(String(articleText).toLowerCase())};
          const pattern = new RegExp(${JSON.stringify(buttonPattern)}, "i");
          const article = Array.from(document.querySelectorAll("article")).find((item) => (item.innerText || "").toLowerCase().includes(articleNeedle));
          if (!article) throw new Error("Article not found: ${escapeForScript(articleText)}");
          const button = Array.from(article.querySelectorAll("button")).find((item) => pattern.test((item.innerText || item.textContent || "").trim()));
          if (!button) throw new Error("Button not found in article: ${escapeForScript(buttonPattern)}");
          if (button.disabled) throw new Error("Button disabled in article: " + (button.innerText || button.textContent || "").trim());
          button.scrollIntoView({ block: "center", inline: "center" });
          button.click();
          return (button.innerText || button.textContent || "").trim();
        })()`
      );
    } catch (error) {
      lastError = error.message || String(error);
      if (!lastError.includes("Article not found") && !lastError.includes("Button not found") && !lastError.includes("Button disabled")) {
        throw error;
      }
    }
    await delay(150);
  }
  const text = await bodyText(page).catch(() => "");
  throw new Error(`Timed out waiting to click ${label} in article ${articleText}${lastError ? ` (${lastError})` : ""}. Body: ${text.slice(0, 1200)}`);
}

async function clickButtonInTabList(page, tabListLabel, buttonPattern, label = buttonPattern) {
  await waitFor(
    page,
    `(() => {
      const tabList = document.querySelector('[role="tablist"][aria-label=${JSON.stringify(tabListLabel)}]');
      const pattern = new RegExp(${JSON.stringify(buttonPattern)}, "i");
      return Boolean(tabList && Array.from(tabList.querySelectorAll("button")).some((button) => pattern.test((button.innerText || button.textContent || "").trim()) && !button.disabled));
    })()`,
    label
  );
  return evaluate(
    page,
    `(() => {
      const tabList = document.querySelector('[role="tablist"][aria-label=${JSON.stringify(tabListLabel)}]');
      if (!tabList) throw new Error("Tab list not found: ${escapeForScript(tabListLabel)}");
      const pattern = new RegExp(${JSON.stringify(buttonPattern)}, "i");
      const button = Array.from(tabList.querySelectorAll("button")).find((item) => pattern.test((item.innerText || item.textContent || "").trim()));
      if (!button) throw new Error("Button not found in tab list: ${escapeForScript(buttonPattern)}");
      if (button.disabled) throw new Error("Button disabled in tab list: " + (button.innerText || button.textContent || "").trim());
      button.scrollIntoView({ block: "center", inline: "center" });
      button.click();
      return (button.innerText || button.textContent || "").trim();
    })()`
  );
}

async function assertButtonAbsentInArticle(page, articleText, buttonPattern, label = buttonPattern) {
  await evaluate(
    page,
    `(() => {
      const articleNeedle = ${JSON.stringify(articleText)};
      const pattern = new RegExp(${JSON.stringify(buttonPattern)}, "i");
      const article = Array.from(document.querySelectorAll("article")).find((item) => (item.innerText || "").includes(articleNeedle));
      if (!article) throw new Error("Article not found: ${escapeForScript(articleText)}");
      const button = Array.from(article.querySelectorAll("button")).find((item) => pattern.test((item.innerText || item.textContent || "").trim()));
      if (button) throw new Error("Unexpected button in article for ${escapeForScript(label)}: " + (button.innerText || button.textContent || "").trim());
      return true;
    })()`
  );
}

async function assertButtonAbsentInOrderDetail(page, orderText, buttonPattern, label = buttonPattern) {
  await evaluate(
    page,
    `(() => {
      const orderNeedle = ${JSON.stringify(orderText)};
      const pattern = new RegExp(${JSON.stringify(buttonPattern)}, "i");
      const panel = Array.from(document.querySelectorAll(".order-detail-panel")).find((item) => (item.innerText || "").includes(orderNeedle));
      if (!panel) throw new Error("Order detail panel not found: ${escapeForScript(orderText)}");
      const button = Array.from(panel.querySelectorAll("button")).find((item) => pattern.test((item.innerText || item.textContent || "").trim()));
      if (button) throw new Error("Unexpected button in order detail for ${escapeForScript(label)}: " + (button.innerText || button.textContent || "").trim());
      return true;
    })()`
  );
}

async function clickButtonNearText(page, containerText, buttonPattern) {
  return evaluate(
    page,
    `(() => {
      const containerNeedle = ${JSON.stringify(containerText)};
      const pattern = new RegExp(${JSON.stringify(buttonPattern)}, "i");
      const containers = Array.from(document.querySelectorAll("article, section.product-detail-panel, section.cart-panel, section.checkout-panel, section.panel"));
      const container = containers.find((item) => (item.innerText || "").includes(containerNeedle) && Array.from(item.querySelectorAll("button")).some((button) => pattern.test((button.innerText || button.textContent || "").trim())));
      if (!container) throw new Error("Container not found: ${escapeForScript(containerText)}");
      const button = Array.from(container.querySelectorAll("button")).find((item) => pattern.test((item.innerText || item.textContent || "").trim()));
      if (!button) throw new Error("Button not found near text: ${escapeForScript(buttonPattern)}");
      if (button.disabled) throw new Error("Button disabled near text: " + (button.innerText || button.textContent || "").trim());
      button.scrollIntoView({ block: "center", inline: "center" });
      button.click();
      return (button.innerText || button.textContent || "").trim();
    })()`
  );
}

async function fillByLabel(page, labelText, value) {
  await waitFor(
    page,
    `Array.from(document.querySelectorAll("label")).some((item) =>
      (item.innerText || "").includes(${JSON.stringify(labelText)}) &&
      item.querySelector("input, textarea, select") !== null
    )`,
    `field ${labelText}`,
  );
  return evaluate(
    page,
    `(() => {
      const labels = Array.from(document.querySelectorAll("label"));
      const label = labels.find((item) => (item.innerText || "").includes(${JSON.stringify(labelText)}));
      if (!label) throw new Error("Label not found: ${escapeForScript(labelText)}");
      const field = label.querySelector("input, textarea, select");
      if (!field) throw new Error("Field not found for label: ${escapeForScript(labelText)}");
      const prototype = field instanceof HTMLTextAreaElement
        ? HTMLTextAreaElement.prototype
        : field instanceof HTMLSelectElement
          ? HTMLSelectElement.prototype
          : HTMLInputElement.prototype;
      const descriptor = Object.getOwnPropertyDescriptor(prototype, "value");
      descriptor.set.call(field, ${JSON.stringify(value)});
      field.dispatchEvent(new Event("input", { bubbles: true }));
      field.dispatchEvent(new Event("change", { bubbles: true }));
      return field.value;
    })()`
  );
}

async function fieldValueByLabel(page, labelText) {
  return evaluate(
    page,
    `(() => {
      const labels = Array.from(document.querySelectorAll("label"));
      const label = labels.find((item) => (item.innerText || "").includes(${JSON.stringify(labelText)}));
      if (!label) throw new Error("Label not found: ${escapeForScript(labelText)}");
      const field = label.querySelector("input, textarea, select");
      if (!field) throw new Error("Field not found for label: ${escapeForScript(labelText)}");
      return field.value;
    })()`
  );
}

async function fillBySelector(page, selector, value) {
  await waitForSelector(page, selector, `field ${selector}`);
  return evaluate(
    page,
    `(() => {
      const field = document.querySelector(${JSON.stringify(selector)});
      if (!field) throw new Error("Field not found: ${escapeForScript(selector)}");
      const prototype = field instanceof HTMLTextAreaElement
        ? HTMLTextAreaElement.prototype
        : field instanceof HTMLSelectElement
          ? HTMLSelectElement.prototype
          : HTMLInputElement.prototype;
      const descriptor = Object.getOwnPropertyDescriptor(prototype, "value");
      descriptor.set.call(field, ${JSON.stringify(value)});
      field.dispatchEvent(new Event("input", { bubbles: true }));
      field.dispatchEvent(new Event("change", { bubbles: true }));
      return field.value;
    })()`
  );
}

async function fillInputByAriaLabel(page, label, value) {
  return evaluate(
    page,
    `(() => {
      const field = Array.from(document.querySelectorAll("input")).find((item) => item.getAttribute("aria-label") === ${JSON.stringify(label)});
      if (!field) throw new Error("Input not found: ${escapeForScript(label)}");
      const descriptor = Object.getOwnPropertyDescriptor(HTMLInputElement.prototype, "value");
      descriptor.set.call(field, ${JSON.stringify(value)});
      field.dispatchEvent(new Event("input", { bubbles: true }));
      field.dispatchEvent(new Event("change", { bubbles: true }));
      return field.value;
    })()`
  );
}

async function clickByAriaLabel(page, label) {
  return evaluate(
    page,
    `(() => {
      const button = Array.from(document.querySelectorAll("button")).find((item) => item.getAttribute("aria-label") === ${JSON.stringify(label)});
      if (!button) throw new Error("Button not found: ${escapeForScript(label)}");
      if (button.disabled) throw new Error("Button disabled: ${escapeForScript(label)}");
      button.scrollIntoView({ block: "center", inline: "center" });
      button.click();
      return true;
    })()`
  );
}

async function setBrowserAuth(page, auth) {
  await evaluate(
    page,
    `window.localStorage.setItem(${JSON.stringify(AUTH_STORAGE_KEY)}, ${JSON.stringify(JSON.stringify(auth))})`
  );
}

async function setFileInputFiles(page, selector, filePath) {
  const document = await page.send("DOM.getDocument");
  const nodeId = document.root?.nodeId;
  if (!nodeId) {
    throw new Error("Browser document node is unavailable for file upload");
  }
  const result = await page.send("DOM.querySelector", { nodeId, selector });
  if (!result.nodeId) {
    throw new Error(`File input not found: ${selector}`);
  }
  await page.send("DOM.setFileInputFiles", { files: [filePath], nodeId: result.nodeId });
}

async function fieldValueBySelector(page, selector) {
  await waitForSelector(page, selector, `field ${selector}`);
  return evaluate(
    page,
    `(() => {
      const field = document.querySelector(${JSON.stringify(selector)});
      if (!field) throw new Error("Field not found: ${escapeForScript(selector)}");
      return field.value;
    })()`
  );
}

async function waitForSelector(page, selector, label = selector, timeoutMs = 20000) {
  await waitFor(page, `document.querySelector(${JSON.stringify(selector)}) !== null`, label, timeoutMs);
}

async function setCheckboxByLabel(page, labelText, checked) {
  return evaluate(
    page,
    `(() => {
      const labels = Array.from(document.querySelectorAll("label"));
      const label = labels.find((item) => (item.innerText || "").includes(${JSON.stringify(labelText)}));
      if (!label) throw new Error("Label not found: ${escapeForScript(labelText)}");
      const field = label.querySelector("input[type='checkbox']");
      if (!field) throw new Error("Checkbox not found for label: ${escapeForScript(labelText)}");
      if (field.disabled) throw new Error("Checkbox disabled for label: ${escapeForScript(labelText)}");
      if (field.checked !== Boolean(${JSON.stringify(checked)})) {
        field.click();
      }
      return field.checked;
    })()`
  );
}

async function evaluate(page, expression) {
  const response = await page.send("Runtime.evaluate", {
    expression,
    awaitPromise: true,
    returnByValue: true,
    userGesture: true
  });
  if (response.exceptionDetails) {
    const detail = response.exceptionDetails;
    throw new Error(detail.exception?.description || detail.text || "Runtime.evaluate failed");
  }
  return response.result?.value;
}

function escapeForScript(value) {
  return String(value).replace(/\\/g, "\\\\").replace(/"/g, '\\"').replace(/\n/g, "\\n");
}

function delay(ms) {
  return new Promise((resolve) => setTimeout(resolve, ms));
}

function stopProcess(child) {
  return new Promise((resolve) => {
    if (!child || child.exitCode !== null) {
      resolve();
      return;
    }
    const timeout = setTimeout(() => resolve(), 3000);
    child.once("exit", () => {
      clearTimeout(timeout);
      resolve();
    });
    child.kill();
  });
}

class CDPClient {
  constructor(url) {
    this.url = url;
    this.nextId = 1;
    this.pending = new Map();
    this.listeners = new Map();
  }

  connect() {
    return new Promise((resolve, reject) => {
      this.ws = new WebSocket(this.url);
      this.ws.addEventListener("open", () => resolve());
      this.ws.addEventListener("error", () => reject(new Error(`Could not connect to ${this.url}`)), { once: true });
      this.ws.addEventListener("message", (event) => this.handleMessage(event.data));
      this.ws.addEventListener("close", () => {
        for (const { reject: rejectPending, timeout } of this.pending.values()) {
          clearTimeout(timeout);
          rejectPending(new Error("CDP websocket closed"));
        }
        this.pending.clear();
      });
    });
  }

  send(method, params = {}, timeoutMs = CDP_COMMAND_TIMEOUT_MS) {
    const id = this.nextId++;
    return new Promise((resolve, reject) => {
      const timeout = setTimeout(() => {
        if (!this.pending.delete(id)) return;
        reject(new Error(`CDP ${method} timed out after ${timeoutMs}ms`));
      }, timeoutMs);
      this.pending.set(id, { resolve, reject, method, timeout });
      try {
        this.ws.send(JSON.stringify({ id, method, params }));
      } catch (error) {
        clearTimeout(timeout);
        this.pending.delete(id);
        reject(error);
      }
    });
  }

  on(method, listener) {
    const listeners = this.listeners.get(method) || [];
    listeners.push(listener);
    this.listeners.set(method, listeners);
  }

  handleMessage(raw) {
    const message = JSON.parse(raw);
    if (message.id) {
      const pending = this.pending.get(message.id);
      if (!pending) return;
      this.pending.delete(message.id);
      clearTimeout(pending.timeout);
      if (message.error) {
        pending.reject(new Error(`${pending.method} failed: ${message.error.message}`));
      } else {
        pending.resolve(message.result || {});
      }
      return;
    }
    const listeners = this.listeners.get(message.method) || [];
    for (const listener of listeners) {
      listener(message.params || {});
    }
  }

  close() {
    return new Promise((resolve) => {
      if (!this.ws || this.ws.readyState === WebSocket.CLOSED) {
        resolve();
        return;
      }
      this.ws.addEventListener("close", () => resolve(), { once: true });
      this.ws.close();
    });
  }
}

try {
  await main();
} catch (error) {
  console.error(error.stack || error.message || error);
  process.exitCode = 1;
}
