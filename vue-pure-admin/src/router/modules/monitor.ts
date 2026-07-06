import { monitor } from "@/router/enums";

export default {
  path: "/monitor",
  redirect: "/monitor/logs/login",
  meta: {
    icon: "ri:dashboard-line",
    title: "系统监控",
    rank: monitor,
    showLink: false
  },
  children: [
    {
      path: "/monitor/logs/login",
      name: "LoginLog",
      component: () => import("@/views/monitor/logs/login/index.vue"),
      meta: {
        title: "登录日志",
        roles: ["admin", "superadmin"]
      }
    },
    {
      path: "/monitor/logs/operation",
      name: "OperationLog",
      component: () => import("@/views/monitor/logs/operation/index.vue"),
      meta: {
        title: "操作日志",
        roles: ["admin", "superadmin"]
      }
    }
  ]
} satisfies RouteConfigsTable;
