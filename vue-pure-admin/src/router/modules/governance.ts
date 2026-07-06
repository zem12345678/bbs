export default {
  path: "/governance",
  redirect: "/governance/users",
  meta: {
    icon: "ri/community-line",
    title: "社区管理",
    rank: 1,
    showLink: false
  },
  children: [
    {
      path: "/governance/users",
      name: "GovernanceUsers",
      component: () => import("@/views/governance/users/index.vue"),
      meta: {
        title: "用户管理",
        roles: ["admin", "superadmin"]
      }
    },
    {
      path: "/governance/articles",
      name: "GovernanceArticles",
      component: () => import("@/views/governance/articles/index.vue"),
      meta: {
        title: "文章管理",
        roles: ["admin", "moderator", "superadmin"]
      }
    },
    {
      path: "/governance/topics",
      name: "GovernanceTopics",
      component: () => import("@/views/governance/topics/index.vue"),
      meta: {
        title: "话题管理",
        roles: ["admin", "moderator", "superadmin"]
      }
    },
    {
      path: "/governance/comments",
      name: "GovernanceComments",
      component: () => import("@/views/governance/comments/index.vue"),
      meta: {
        title: "评论管理",
        roles: ["admin", "moderator", "superadmin"]
      }
    },
    {
      path: "/governance/reports",
      name: "GovernanceReports",
      component: () => import("@/views/governance/reports/index.vue"),
      meta: {
        title: "举报管理",
        roles: ["admin", "moderator", "superadmin"]
      }
    }
  ]
} satisfies RouteConfigsTable;
