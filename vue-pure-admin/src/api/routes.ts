import { http } from "@/utils/http";
import {
  toPureResult,
  unwrapGatewayData,
  type GatewayEnvelope
} from "./client";

type SystemMenuNode = {
  id: number;
  parent_id?: number;
  parentId?: number;
  name?: string;
  title?: string;
  icon?: string;
  path?: string;
  component?: string;
  type?: string;
  permission?: string;
  auths?: string;
  menuType?: number;
  sort?: number;
  rank?: number;
  visible?: string;
  is_hide?: string;
  isHide?: string;
  showLink?: boolean;
  children?: SystemMenuNode[];
};

type CurrentMenuPayload = {
  items?: SystemMenuNode[];
  list?: SystemMenuNode[];
};

export const getAsyncRoutes = async () => {
  const response = await http.request<GatewayEnvelope<CurrentMenuPayload>>(
    "get",
    "/api/v1/admin/auth/menus"
  );
  const payload = unwrapGatewayData(response);
  const menus = payload?.items ?? payload?.list ?? [];
  return toPureResult(menus.filter(isRouteMenu).map(toRoute), "success");
};

function toRoute(menu: SystemMenuNode): Record<string, any> {
  const children = (menu.children ?? []).filter(isRouteMenu).map(toRoute);
  const path = normalizePath(menu.path, menu.name);
  const permission = menu.permission ?? menu.auths ?? "";
  const component =
    normalizeComponent(menu.component) ??
    (children.length > 0 ? "empty/index" : componentFromPath(path));

  return {
    path,
    name: routeName(menu, path),
    component,
    redirect: children.length > 0 ? children[0].path : undefined,
    meta: {
      title: menu.title || menu.name || path,
      icon: menu.icon || undefined,
      rank: Number(menu.rank ?? menu.sort ?? 99),
      showLink: resolveShowLink(menu),
      auths: permission ? [permission] : []
    },
    children: children.length > 0 ? children : undefined
  };
}

function isRouteMenu(menu: SystemMenuNode) {
  const type = String(menu.type ?? "").toUpperCase();
  const path = String(menu.path ?? "");
  const name = String(menu.name ?? "");
  return (
    type !== "F" &&
    type !== "B" &&
    menu.menuType !== 3 &&
    !/(^|[/?#&])perm=/i.test(path) &&
    !/(^|[/?#&])perm=/i.test(name)
  );
}

function normalizePath(path?: string, fallback?: string) {
  const value = (path || fallback || "/").trim();
  return value.startsWith("/") ? value : `/${value}`;
}

function normalizeComponent(component?: string) {
  const value = component?.trim();
  return value || undefined;
}

function componentFromPath(path: string) {
  return `${path.replace(/^\/+/, "").replace(/\/+$/, "")}/index`;
}

function routeName(menu: SystemMenuNode, path: string) {
  const value = menu.name || path.replace(/^\/+/, "") || "root";
  if (/^https?:\/\//.test(value)) return value;
  const name = value
    .split(/[^A-Za-z0-9]+/)
    .filter(Boolean)
    .map(item => item.charAt(0).toUpperCase() + item.slice(1))
    .join("");
  return name || "Route";
}

function resolveShowLink(menu: SystemMenuNode) {
  if (typeof menu.showLink === "boolean") return menu.showLink;
  return menu.visible !== "1" && menu.is_hide !== "1" && menu.isHide !== "1";
}
