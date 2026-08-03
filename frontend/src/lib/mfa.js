export function mfaChallengeFromResponse(data) {
  if (!data || data.mfa_required !== true) return null;
  const challenge = String(data.mfa_challenge || "").trim();
  if (!challenge) return null;
  return {
    challenge,
    expiresAt: finiteNumber(data.mfa_expires_at),
    user: data.user || null
  };
}

export function normalizeMFAStatus(data) {
  return {
    enabled: data?.enabled === true,
    recoveryCodesRemaining: Math.max(0, finiteNumber(data?.recovery_codes_remaining)),
    enabledAt: finiteNumber(data?.enabled_at)
  };
}

export function recoveryCodesFromResponse(data) {
  if (!Array.isArray(data?.recovery_codes)) return [];
  return data.recovery_codes.map((code) => String(code || "").trim()).filter(Boolean);
}

export function recoveryCodesText(codes) {
  return recoveryCodesFromResponse({ recovery_codes: codes }).join("\n");
}

function finiteNumber(value) {
  const number = Number(value);
  return Number.isFinite(number) && number > 0 ? number : 0;
}
