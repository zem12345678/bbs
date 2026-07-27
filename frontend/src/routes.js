export const defaultPage = "广场";

export const pageRoutes = [
  { key: "home", label: "首页", path: "/" },
  { key: "plaza", label: "广场", path: "/plaza" },
  { key: "circles", label: "圈子", path: "/circles" },
  { key: "chat", label: "聊天室", path: "/chat" },
  { key: "help", label: "求助", path: "/help" },
  { key: "resources", label: "资源", path: "/resources" },
  { key: "shop", label: "商城", path: "/shop" },
  { key: "member", label: "会员", path: "/member" },
  { key: "more", label: "更多", path: "/more" }
];

export const navItems = pageRoutes.map((route) => route.label);

const pathActivePageRules = [
  { prefix: "/chat", label: "聊天室" },
  { prefix: "/room", label: "聊天室" },
  { prefix: "/topic", label: "广场" },
  { prefix: "/topics", label: "广场" },
  { prefix: "/article", label: "广场" },
  { prefix: "/articles", label: "广场" },
  { prefix: "/search", label: "广场" },
  { prefix: "/auth", label: "会员" },
  { prefix: "/u", label: "会员" },
  { prefix: "/user", label: "会员" },
  { prefix: "/dashboard", label: "会员" },
  { prefix: "/links", label: "更多" },
  { prefix: "/tasks", label: "更多" },
  { prefix: "/about", label: "更多" },
  { prefix: "/install", label: "更多" },
  { prefix: "/redirect", label: "更多" }
];

const pagePathMap = new Map(pageRoutes.map((route) => [route.label, route.path]));
const pathPageMap = new Map(pageRoutes.map((route) => [route.path, route.label]));

export function pageToPath(page) {
  return pagePathMap.get(page) || pagePathMap.get(defaultPage);
}

export function pathToPage(pathname) {
  const normalizedPath = normalizePath(pathname);
  const staticPage = pathPageMap.get(normalizedPath);
  if (staticPage) {
    return staticPage;
  }
  const rule = pathActivePageRules.find(({ prefix }) => normalizedPath === prefix || normalizedPath.startsWith(`${prefix}/`));
  return rule?.label || defaultPage;
}

function normalizePath(pathname) {
  if (!pathname || pathname === "/") {
    return "/";
  }
  return pathname.replace(/\/+$/, "");
}
