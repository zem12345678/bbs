<script setup lang="ts">
import { computed, ref, markRaw, onMounted } from "vue";
import ReCol from "@/components/ReCol";
import { useDark, randomGradient } from "./utils";
import WelcomeTable from "./components/table/index.vue";
import { ReNormalCountTo } from "@/components/ReCountTo";
import { useRenderFlicker } from "@/components/ReFlicker";
import { ChartBar, ChartLine, ChartRound } from "./components/charts";
import Segmented, { type OptionsType } from "@/components/ReSegmented";
import { message } from "@/utils/message";
import {
  getAdminOverview,
  type AdminOverview,
  type AdminOverviewActivity
} from "@/api/admin";
import { chartData, cardVisuals } from "./data";

defineOptions({
  name: "Welcome"
});

const { isDark } = useDark();

const overview = ref<AdminOverview>();
const loading = ref(false);
const curWeek = ref(1); // 0上期、1本期
const optionsBasis: Array<OptionsType> = [
  {
    label: "上期"
  },
  {
    label: "本期"
  }
];

const fallbackChart = {
  labels: ["-", "-", "-", "-", "-", "-", "-"],
  previous: {
    contentData: [0, 0, 0, 0, 0, 0, 0],
    governanceData: [0, 0, 0, 0, 0, 0, 0]
  },
  current: {
    contentData: [0, 0, 0, 0, 0, 0, 0],
    governanceData: [0, 0, 0, 0, 0, 0, 0]
  }
};

const metricCards = computed(() => {
  const items = overview.value?.metrics?.length
    ? overview.value.metrics
    : chartData;
  return items.map((item, index) => {
    const fallback = chartData.find(card => card.key === item.key);
    const visual =
      cardVisuals[item.key as keyof typeof cardVisuals] ??
      cardVisuals[
        (chartData[index]?.key ?? chartData[0].key) as keyof typeof cardVisuals
      ];
    return {
      ...fallback,
      ...item,
      ...visual,
      duration: fallback?.duration ?? 1200,
      data: item.data?.length ? item.data : fallback?.data ?? []
    };
  });
});

const activeChart = computed(() => overview.value?.chart ?? fallbackChart);
const activeBarData = computed(() =>
  curWeek.value === 0 ? activeChart.value.previous : activeChart.value.current
);
const progressItems = computed(() => overview.value?.progress ?? []);
const dailyRows = computed(() => overview.value?.daily ?? []);
const latestActivities = computed(() => overview.value?.latest ?? []);

function activityBackground(item: AdminOverviewActivity) {
  if (item.type === "login") return "#41b6ff";
  if (item.type === "operation") return "#26ce83";
  return randomGradient({ randomizeHue: true });
}

async function loadOverview() {
  loading.value = true;
  try {
    const {
      code,
      data,
      message: msg
    } = await getAdminOverview();
    if (code !== 0) {
      message(msg || "加载运营概览失败", { type: "error" });
      return;
    }
    overview.value = data;
  } catch (error) {
    message("加载运营概览失败", { type: "error" });
  } finally {
    loading.value = false;
  }
}

onMounted(loadOverview);
</script>

<template>
  <div>
    <el-row :gutter="24" justify="space-around">
      <re-col
        v-for="(item, index) in metricCards"
        :key="item.key"
        v-motion
        class="mb-4.5"
        :value="6"
        :md="12"
        :sm="12"
        :xs="24"
        :initial="{
          opacity: 0,
          y: 100
        }"
        :enter="{
          opacity: 1,
          y: 0,
          transition: {
            delay: 80 * (index + 1)
          }
        }"
      >
        <el-card class="line-card" shadow="never">
          <div class="flex justify-between">
            <span class="text-md font-medium">
              {{ item.name }}
            </span>
            <div
              class="size-8 flex-c rounded-md"
              :style="{
                backgroundColor: isDark ? 'transparent' : item.bgColor
              }"
            >
              <IconifyIconOffline
                :icon="item.icon"
                :color="item.color"
                width="18"
                height="18"
              />
            </div>
          </div>
          <div class="flex justify-between items-start mt-3">
            <div class="w-1/2">
              <ReNormalCountTo
                :duration="item.duration"
                :fontSize="'1.6em'"
                :startVal="0"
                :endVal="item.value"
              />
              <p class="font-medium text-green-500">{{ item.percent }}</p>
            </div>
            <ChartLine
              v-if="item.data.length > 1"
              :key="`${item.key}-${item.value}-${item.data.join('-')}`"
              class="w-1/2!"
              :color="item.color"
              :data="item.data"
            />
            <ChartRound v-else class="w-1/2!" />
          </div>
        </el-card>
      </re-col>

      <re-col
        v-motion
        class="mb-4.5"
        :value="18"
        :xs="24"
        :initial="{
          opacity: 0,
          y: 100
        }"
        :enter="{
          opacity: 1,
          y: 0,
          transition: {
            delay: 400
          }
        }"
      >
        <el-card class="bar-card" shadow="never">
          <div class="flex justify-between">
            <span class="text-md font-medium">运营概览</span>
            <Segmented v-model="curWeek" :options="optionsBasis" />
          </div>
          <div class="flex justify-between items-start mt-3">
            <ChartBar
              :xLabels="activeChart.labels"
              :requireData="activeBarData.contentData"
              :questionData="activeBarData.governanceData"
              primaryName="内容新增"
              secondaryName="治理事件"
            />
          </div>
        </el-card>
      </re-col>

      <re-col
        v-motion
        class="mb-4.5"
        :value="6"
        :xs="24"
        :initial="{
          opacity: 0,
          y: 100
        }"
        :enter="{
          opacity: 1,
          y: 0,
          transition: {
            delay: 480
          }
        }"
      >
        <el-card shadow="never">
          <div class="flex justify-between">
            <span class="text-md font-medium">治理健康度</span>
          </div>
          <div
            v-for="(item, index) in progressItems"
            :key="index"
            :class="[
              'flex',
              'justify-between',
              'items-start',
              index === 0 ? 'mt-8' : 'mt-[2.15rem]'
            ]"
          >
            <el-progress
              :text-inside="true"
              :percentage="item.percentage"
              :stroke-width="21"
              :color="item.color"
              striped
              striped-flow
              :duration="item.duration"
            />
            <span class="text-nowrap ml-2 text-text_color_regular text-sm">
              {{ item.week || item.label }}
            </span>
          </div>
          <el-empty
            v-if="!loading && progressItems.length === 0"
            description="暂无治理数据"
            :image-size="80"
          />
        </el-card>
      </re-col>

      <re-col
        v-motion
        class="mb-4.5"
        :value="18"
        :xs="24"
        :initial="{
          opacity: 0,
          y: 100
        }"
        :enter="{
          opacity: 1,
          y: 0,
          transition: {
            delay: 560
          }
        }"
      >
        <el-card shadow="never">
          <div class="flex justify-between">
            <span class="text-md font-medium">每日统计</span>
          </div>
          <el-scrollbar max-height="504" class="mt-3">
            <WelcomeTable :data="dailyRows" :loading="loading" />
          </el-scrollbar>
        </el-card>
      </re-col>

      <re-col
        v-motion
        class="mb-4.5"
        :value="6"
        :xs="24"
        :initial="{
          opacity: 0,
          y: 100
        }"
        :enter="{
          opacity: 1,
          y: 0,
          transition: {
            delay: 640
          }
        }"
      >
        <el-card shadow="never">
          <div class="flex justify-between">
            <span class="text-md font-medium">最新动态</span>
          </div>
          <el-scrollbar max-height="504" class="mt-3">
            <el-timeline>
              <el-timeline-item
                v-for="(item, index) in latestActivities"
                :key="index"
                center
                placement="top"
                :icon="
                  markRaw(
                    useRenderFlicker({
                      background: activityBackground(item)
                    })
                  )
                "
                :timestamp="item.date"
              >
                <p class="text-text_color_regular text-sm">
                  {{ item.summary || "系统动态" }}
                </p>
                <p
                  v-if="item.detail"
                  class="mt-1 text-text_color_secondary text-xs"
                >
                  {{ item.detail }}
                </p>
              </el-timeline-item>
            </el-timeline>
            <el-empty
              v-if="!loading && latestActivities.length === 0"
              description="暂无动态"
              :image-size="80"
            />
          </el-scrollbar>
        </el-card>
      </re-col>
    </el-row>
  </div>
</template>

<style lang="scss" scoped>
:deep(.el-card) {
  --el-card-border-color: none;

  /* 解决概率进度条宽度 */
  .el-progress--line {
    width: 85%;
  }

  /* 解决概率进度条字体大小 */
  .el-progress-bar__innerText {
    font-size: 15px;
  }

  /* 隐藏 el-scrollbar 滚动条 */
  .el-scrollbar__bar {
    display: none;
  }

  /* el-timeline 每一项上下、左右边距 */
  .el-timeline-item {
    margin: 0 6px;
  }
}

:deep(.el-timeline.is-start) {
  padding-left: 0;
}

.main-content {
  margin: 20px 20px 0 !important;
}
</style>
