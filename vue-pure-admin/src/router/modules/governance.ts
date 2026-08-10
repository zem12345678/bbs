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
      path: "/governance/credits",
      name: "GovernanceCredits",
      component: () => import("@/views/governance/credits/index.vue"),
      meta: {
        title: "积分管理",
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
      path: "/governance/channels",
      name: "GovernanceChannels",
      component: () => import("@/views/governance/channels/index.vue"),
      meta: {
        title: "圈子治理",
        roles: ["admin", "moderator", "superadmin"]
      }
    },
    {
      path: "/governance/categories",
      name: "GovernanceCategories",
      component: () => import("@/views/governance/categories/index.vue"),
      meta: {
        title: "分类管理",
        roles: ["admin", "superadmin"]
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
    },
    {
      path: "/governance/forbidden-words",
      name: "GovernanceForbiddenWords",
      component: () => import("@/views/governance/forbidden-words/index.vue"),
      meta: {
        title: "敏感词管理",
        roles: ["admin", "superadmin"]
      }
    },
    {
      path: "/governance/settings",
      name: "GovernanceSettings",
      component: () => import("@/views/governance/settings/index.vue"),
      meta: {
        title: "站点设置",
        roles: ["admin", "superadmin"]
      }
    },
    {
      path: "/governance/announcements",
      name: "GovernanceAnnouncements",
      component: () => import("@/views/governance/announcements/index.vue"),
      meta: {
        title: "公告管理",
        roles: ["admin", "superadmin"]
      }
    },
    {
      path: "/governance/links",
      name: "GovernanceLinks",
      component: () => import("@/views/governance/links/index.vue"),
      meta: {
        title: "友情链接",
        roles: ["admin", "superadmin"]
      }
    },
    {
      path: "/governance/ads",
      name: "GovernanceAds",
      component: () => import("@/views/governance/ads/index.vue"),
      meta: {
        title: "广告管理",
        roles: ["admin", "superadmin"]
      }
    },
    {
      path: "/governance/tasks",
      name: "GovernanceTasks",
      component: () => import("@/views/governance/tasks/index.vue"),
      meta: {
        title: "任务管理",
        roles: ["admin", "superadmin"]
      }
    },
    {
      path: "/governance/badges",
      name: "GovernanceBadges",
      component: () => import("@/views/governance/badges/index.vue"),
      meta: {
        title: "徽章管理",
        roles: ["admin", "superadmin"]
      }
    },
    {
      path: "/governance/levels",
      name: "GovernanceLevels",
      component: () => import("@/views/governance/levels/index.vue"),
      meta: {
        title: "等级管理",
        roles: ["admin", "superadmin"]
      }
    }
  ]
} satisfies RouteConfigsTable;
