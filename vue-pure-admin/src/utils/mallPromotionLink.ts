import { copyTextToClipboard } from "@pureadmin/utils";
import { buildFrontendUrl } from "@/utils/frontendUrl";

type PromotionParams = Record<string, string | number | undefined | null>;

export function buildMallPromotionUrl(params: PromotionParams) {
  const query = new URLSearchParams();
  for (const [key, value] of Object.entries(params)) {
    const text = String(value ?? "").trim();
    if (text) query.set(key, text);
  }
  const search = query.toString();
  const path = `/shop${search ? `?${search}` : ""}`;
  return buildFrontendUrl(path);
}

export function copyMallPromotionUrl(url: string) {
  return copyTextToClipboard(url);
}

export function openMallPromotionUrl(url: string) {
  window.open(url, "_blank", "noopener,noreferrer");
}
