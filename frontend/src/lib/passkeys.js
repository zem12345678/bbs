export function passkeysSupported(scope = globalThis) {
  return Boolean(scope?.PublicKeyCredential && scope?.navigator?.credentials?.create && scope?.navigator?.credentials?.get);
}

export function creationOptionsFromResponse(data) {
  const publicKey = data?.options?.publicKey || data?.publicKey;
  if (!publicKey || typeof publicKey !== "object") throw new Error("Passkey 注册参数无效。");
  return {
    ...publicKey,
    challenge: base64URLToBytes(publicKey.challenge),
    user: { ...publicKey.user, id: base64URLToBytes(publicKey.user?.id) },
    excludeCredentials: credentialDescriptors(publicKey.excludeCredentials)
  };
}

export function requestOptionsFromResponse(data) {
  const publicKey = data?.options?.publicKey || data?.publicKey;
  if (!publicKey || typeof publicKey !== "object") throw new Error("Passkey 验证参数无效。");
  return {
    ...publicKey,
    challenge: base64URLToBytes(publicKey.challenge),
    allowCredentials: credentialDescriptors(publicKey.allowCredentials)
  };
}

export async function createPasskey(data, credentials = globalThis?.navigator?.credentials) {
  if (!credentials?.create) throw new Error("当前浏览器不支持 Passkey。");
  const credential = await credentials.create({ publicKey: creationOptionsFromResponse(data) });
  if (!credential) throw new Error("Passkey 注册已取消。");
  return publicKeyCredentialJSON(credential);
}

export async function getPasskey(data, credentials = globalThis?.navigator?.credentials) {
  if (!credentials?.get) throw new Error("当前浏览器不支持 Passkey。");
  const credential = await credentials.get({ publicKey: requestOptionsFromResponse(data) });
  if (!credential) throw new Error("Passkey 验证已取消。");
  return publicKeyCredentialJSON(credential);
}

export function publicKeyCredentialJSON(credential) {
  if (typeof credential?.toJSON === "function") return credential.toJSON();
  if (!credential?.response) throw new Error("Passkey 凭据无效。");
  const response = credential.response;
  const json = {
    id: credential.id,
    rawId: bytesToBase64URL(credential.rawId),
    type: credential.type,
    authenticatorAttachment: credential.authenticatorAttachment || undefined,
    clientExtensionResults: typeof credential.getClientExtensionResults === "function" ? credential.getClientExtensionResults() : {},
    response: {
      clientDataJSON: bytesToBase64URL(response.clientDataJSON),
      authenticatorData: bytesToBase64URL(response.authenticatorData),
      signature: bytesToBase64URL(response.signature),
      userHandle: response.userHandle ? bytesToBase64URL(response.userHandle) : null,
      attestationObject: bytesToBase64URL(response.attestationObject),
      transports: typeof response.getTransports === "function" ? response.getTransports() : undefined
    }
  };
  return removeUndefined(json);
}

export function normalizePasskeyList(data) {
  return {
    items: Array.isArray(data?.items)
      ? data.items.map((item) => ({
          credentialId: String(item?.credential_id || ""),
          name: String(item?.name || ""),
          backupEligible: item?.backup_eligible === true,
          backupState: item?.backup_state === true,
          createdAt: finiteNumber(item?.created_at),
          updatedAt: finiteNumber(item?.updated_at),
          lastUsedAt: finiteNumber(item?.last_used_at)
        })).filter((item) => item.credentialId)
      : [],
    passwordlessEnabled: data?.passwordless_enabled === true,
    limit: Math.max(0, finiteNumber(data?.limit)) || 20
  };
}

export function friendlyPasskeyError(error, fallback = "Passkey 操作失败") {
  const name = String(error?.name || "");
  if (name === "NotAllowedError") return "Passkey 操作已取消或超时。";
  if (name === "InvalidStateError") return "这枚 Passkey 已绑定到当前账号。";
  if (name === "NotSupportedError" || name === "SecurityError") return "当前浏览器或站点地址不支持 Passkey。";
  return error?.message || fallback;
}

export function base64URLToBytes(value) {
  const text = String(value || "").trim();
  if (!text) return new Uint8Array();
  const base64 = text.replace(/-/g, "+").replace(/_/g, "/").padEnd(Math.ceil(text.length / 4) * 4, "=");
  const binary = globalThis.atob(base64);
  return Uint8Array.from(binary, (character) => character.charCodeAt(0));
}

export function bytesToBase64URL(value) {
  if (value === undefined || value === null) return undefined;
  const bytes = value instanceof Uint8Array ? value : new Uint8Array(value);
  let binary = "";
  for (const byte of bytes) binary += String.fromCharCode(byte);
  return globalThis.btoa(binary).replace(/\+/g, "-").replace(/\//g, "_").replace(/=+$/g, "");
}

function credentialDescriptors(value) {
  if (!Array.isArray(value)) return value;
  return value.map((descriptor) => ({ ...descriptor, id: base64URLToBytes(descriptor?.id) }));
}

function removeUndefined(value) {
  if (Array.isArray(value)) return value.map(removeUndefined);
  if (!value || typeof value !== "object") return value;
  return Object.fromEntries(Object.entries(value).filter(([, entry]) => entry !== undefined).map(([key, entry]) => [key, removeUndefined(entry)]));
}

function finiteNumber(value) {
  const number = Number(value);
  return Number.isFinite(number) && number > 0 ? number : 0;
}
