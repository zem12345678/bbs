import { system } from "@/router/enums";

export default {
  path: "/system",
  redirect: "/system/user",
  meta: {
    icon: "ri/settings-3-line",
    title: "系统管理",
    rank: system,
    showLink: false
  },
  children: [
    {
      path: "/system/user",
      name: "SystemUser",
      component: () => import("@/views/system/user/index.vue"),
      meta: {
        title: "用户管理",
        roles: ["admin", "superadmin"]
      }
    },
    {
      path: "/system/role",
      name: "SystemRole",
      component: () => import("@/views/system/role/index.vue"),
      meta: {
        title: "角色管理",
        roles: ["admin", "superadmin"]
      }
    },
    {
      path: "/system/menu",
      name: "SystemMenu",
      component: () => import("@/views/system/menu/index.vue"),
      meta: {
        title: "菜单管理",
        roles: ["admin", "superadmin"]
      }
    },
    {
      path: "/system/dept",
      name: "SystemDept",
      component: () => import("@/views/system/dept/index.vue"),
      meta: {
        title: "部门管理",
        roles: ["admin", "superadmin"]
      }
    }
  ]
} satisfies RouteConfigsTable;
