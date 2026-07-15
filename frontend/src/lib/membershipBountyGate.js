export function membershipBountyGateState(needsMembership, gate = {}) {
  if (!needsMembership) {
    return { blocked: false, reason: "" };
  }
  if (gate.loading) {
    return { blocked: true, reason: "checking" };
  }
  if (gate.error) {
    return { blocked: false, reason: "precheck_failed" };
  }
  if (!gate.checked) {
    return { blocked: true, reason: "checking" };
  }
  if (!gate.active) {
    return { blocked: true, reason: "missing" };
  }
  return { blocked: false, reason: "active" };
}
