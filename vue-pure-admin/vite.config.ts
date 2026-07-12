import { getPluginsList } from "./build/plugins";
import { include, exclude } from "./build/optimize";
import { type UserConfigExport, type ConfigEnv, loadEnv } from "vite";
import {
  root,
  alias,
  wrapperEnv,
  pathResolve,
  __APP_INFO__
} from "./build/utils";

export default async ({ mode }: ConfigEnv): Promise<UserConfigExport> => {
  const {
    VITE_CDN,
    VITE_PORT,
    VITE_COMPRESSION,
    VITE_PUBLIC_PATH,
    VITE_API_PROXY_TARGET
  } = wrapperEnv(loadEnv(mode, root));
  return {
    base: VITE_PUBLIC_PATH,
    root,
    resolve: {
      alias
    },
    // 服务端渲染
    server: {
      // 端口号
      port: VITE_PORT,
      host: "0.0.0.0",
      // 本地跨域代理 https://cn.vitejs.dev/config/server-options.html#server-proxy
      proxy: VITE_API_PROXY_TARGET
        ? {
            "/api": {
              target: VITE_API_PROXY_TARGET,
              changeOrigin: true
            }
          }
        : {},
      // 预热文件以提前转换和缓存结果，降低启动期间的初始页面加载时长并防止转换瀑布
      warmup: {
        clientFiles: ["./index.html", "./src/{views,components}/*"]
      }
    },
    plugins: await getPluginsList(VITE_CDN, VITE_COMPRESSION),
    // https://cn.vitejs.dev/config/dep-optimization-options.html#dep-optimization-options
    optimizeDeps: {
      include,
      exclude
    },
    build: {
      // https://cn.vitejs.dev/guide/build.html#browser-compatibility
      target: "es2015",
      sourcemap: false,
      // 运营后台依赖较重，按可缓存 vendor chunk 控制单包体积回归
      chunkSizeWarningLimit: 1200,
      rolldownOptions: {
        input: {
          index: pathResolve("./index.html", import.meta.url)
        },
        // 静态资源分类打包
        output: {
          codeSplitting: {
            groups: [
              {
                name: "vendor-vue",
                test: /node_modules[\\/](vue|vue-router|pinia|@vueuse)[\\/]/,
                priority: 50
              },
              {
                name: "vendor-ui",
                test: /node_modules[\\/](@pureadmin|element-plus|@element-plus|plus-pro-components|@iconify)[\\/]/,
                priority: 40
              },
              {
                name: "vendor-charts",
                test: /node_modules[\\/](echarts|zrender)[\\/]/,
                priority: 30
              },
              {
                name: "vendor-editor",
                test: /node_modules[\\/](codemirror|codemirror-editor-vue3|@wangeditor|vditor|highlight\.js)[\\/]/,
                priority: 30
              },
              {
                name: "vendor-data",
                test: /node_modules[\\/](vxe-table|xe-utils|xlsx|sortablejs)[\\/]/,
                priority: 30
              },
              {
                name: "vendor",
                test: /node_modules[\\/]/,
                priority: 10,
                maxSize: 900 * 1024
              }
            ]
          },
          chunkFileNames: "static/js/[name]-[hash].js",
          entryFileNames: "static/js/[name]-[hash].js",
          assetFileNames: "static/[ext]/[name]-[hash].[ext]"
        },
        checks: {
          pluginTimings: false,
          toleratedTransform: false
        }
      }
    },
    define: {
      __INTLIFY_PROD_DEVTOOLS__: false,
      __APP_INFO__: JSON.stringify(__APP_INFO__)
    }
  };
};
