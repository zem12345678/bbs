export default {
  path: "/mall",
  redirect: "/mall/products",
  meta: {
    icon: "ri/store-2-line",
    title: "商城管理",
    rank: 2,
    showLink: false
  },
  children: [
    {
      path: "/mall/categories",
      name: "MallCategories",
      component: () => import("@/views/mall/categories/index.vue"),
      meta: {
        title: "商品分类",
        roles: ["admin", "superadmin"]
      }
    },
    {
      path: "/mall/products",
      name: "MallProducts",
      component: () => import("@/views/mall/products/index.vue"),
      meta: {
        title: "商品管理",
        roles: ["admin", "superadmin"]
      }
    },
    {
      path: "/mall/coupons",
      name: "MallCoupons",
      component: () => import("@/views/mall/coupons/index.vue"),
      meta: {
        title: "优惠券管理",
        roles: ["admin", "superadmin"]
      }
    },
    {
      path: "/mall/orders",
      name: "MallOrders",
      component: () => import("@/views/mall/orders/index.vue"),
      meta: {
        title: "订单管理",
        roles: ["admin", "superadmin"]
      }
    },
    {
      path: "/mall/refunds",
      name: "MallRefunds",
      component: () => import("@/views/mall/refunds/index.vue"),
      meta: {
        title: "售后管理",
        roles: ["admin", "superadmin"]
      }
    }
  ]
} satisfies RouteConfigsTable;
