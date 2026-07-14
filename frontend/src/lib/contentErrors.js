const MEMBERSHIP_BOUNTY_DOMAIN_CODE = "topic_membership_entitlement_required";

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
