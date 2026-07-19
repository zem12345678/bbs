const MEMBERSHIP_BOUNTY_DOMAIN_CODE = "topic_membership_entitlement_required";
const BOUNTY_CREDIT_DOMAIN_CODE = "topic_bounty_credit_insufficient";
const BOUNTY_CREDIT_MESSAGE = "insufficient credit balance for bounty qa topic";
const PAID_ATTACHMENT_MEMBERSHIP_MESSAGE = "membership entitlement required for paid attachments";

function errorStatus(error) {
  return Number(error?.status || error?.httpCode || error?.responseStatus || 0);
}

function errorText(error) {
  return [
    error?.message,
    error?.reason,
    error?.rawBody,
    error?.code,
    error?.meta?.legacy_code,
    error?.meta?.legacyCode
  ]
    .filter(Boolean)
    .join(" ")
    .toLowerCase();
}

export function isMembershipBountyError(error) {
  if (errorStatus(error) !== 403) return false;
  const text = errorText(error);
  return text.includes(MEMBERSHIP_BOUNTY_DOMAIN_CODE) || (text.includes("membership") && text.includes("bounty"));
}

export function isMembershipPaidAttachmentError(error) {
  if (errorStatus(error) !== 403) return false;
  return errorText(error).includes(PAID_ATTACHMENT_MEMBERSHIP_MESSAGE);
}

export function isBountyCreditInsufficientError(error) {
  if (errorStatus(error) !== 412) return false;
  const text = errorText(error);
  return text.includes(BOUNTY_CREDIT_DOMAIN_CODE) || text.includes(BOUNTY_CREDIT_MESSAGE) || (text.includes("insufficient") && text.includes("credit") && text.includes("bounty"));
}
