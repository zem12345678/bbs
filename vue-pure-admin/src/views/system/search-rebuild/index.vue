<script setup lang="ts">
import dayjs from "dayjs";
import { computed, onMounted, onUnmounted, ref } from "vue";
import { ElMessageBox } from "element-plus";
import { message } from "@/utils/message";
import { hasPerms } from "@/utils/auth";
import {
  getAdminSearchRebuildStatus,
  startAdminSearchRebuild,
  type AdminSearchRebuildResult,
  type AdminSearchRebuildStatus
} from "@/api/admin";

defineOptions({
  name: "SystemSearchRebuild"
});

const pollInterval = 2000;
const loading = ref(false);
const starting = ref(false);
const status = ref<AdminSearchRebuildStatus | null>(null);
let pollTimer: ReturnType<typeof setTimeout> | undefined;

const canStart = computed(() => hasPerms("system:rebuild_search"));
const canView = computed(() => hasPerms("system:view_search_rebuild"));
const currentState = computed(
  () => status.value?.state?.toLowerCase() || "idle"
);
const isActive = computed(
  () => currentState.value === "queued" || currentState.value === "running"
);
const articleTotal = computed(() =>
  statusNumber("article_total", "articleTotal")
);
const articleIndexed = computed(() =>
  statusNumber("article_indexed", "articleIndexed")
);
const topicTotal = computed(() => statusNumber("topic_total", "topicTotal"));
const topicIndexed = computed(() =>
  statusNumber("topic_indexed", "topicIndexed")
);
const userTotal = computed(() => statusNumber("user_total", "userTotal"));
const userIndexed = computed(() =>
  statusNumber("user_indexed", "userIndexed")
);
const indexedTotal = computed(
  () => articleIndexed.value + topicIndexed.value + userIndexed.value
);
const contentTotal = computed(
  () => articleTotal.value + topicTotal.value + userTotal.value
);
const progress = computed(() => {
  if (contentTotal.value > 0) {
    return Math.min(
      100,
      Math.round((indexedTotal.value / contentTotal.value) * 100)
    );
  }
  return currentState.value === "completed" ? 100 : 0;
});
const errorMessage = computed(() => String(status.value?.error || "").trim());

function statusNumber(snakeKey: string, camelKey: string) {
  const data = (status.value ?? {}) as Record<string, unknown>;
  return Number(data[snakeKey] ?? data[camelKey] ?? 0);
}

function statusText() {
  switch (currentState.value) {
    case "queued":
      return "排队中";
    case "running":
      return "执行中";
    case "completed":
      return "已完成";
    case "failed":
      return "失败";
    default:
      return "暂无任务";
  }
}

function statusTagType() {
  switch (currentState.value) {
    case "queued":
      return "warning";
    case "running":
      return "primary";
    case "completed":
      return "success";
    case "failed":
      return "danger";
    default:
      return "info";
  }
}

function formatTime(value?: number) {
  if (!value) return "-";
  const timestamp = value > 9999999999 ? value : value * 1000;
  return dayjs(timestamp).format("YYYY-MM-DD HH:mm:ss");
}

function resolveStatus(data?: AdminSearchRebuildResult) {
  return data?.status ?? null;
}

function clearPollTimer() {
  if (pollTimer) {
    clearTimeout(pollTimer);
    pollTimer = undefined;
  }
}

function schedulePoll() {
  clearPollTimer();
  if (!canView.value || !isActive.value) return;
  pollTimer = setTimeout(() => void loadStatus(true), pollInterval);
}

async function loadStatus(silent = false) {
  if (!canView.value) return;
  if (!silent) loading.value = true;
  try {
    const { code, data, message: msg } = await getAdminSearchRebuildStatus();
    if (code !== 0) {
      if (!silent) message(msg || "加载搜索重建状态失败", { type: "error" });
      return;
    }
    status.value = resolveStatus(data);
  } catch (error) {
    if (!silent) {
      message(error instanceof Error ? error.message : "加载搜索重建状态失败", {
        type: "error"
      });
    }
  } finally {
    if (!silent) loading.value = false;
    schedulePoll();
  }
}

async function startRebuild() {
  if (!canStart.value) {
    message("没有重建搜索索引权限", { type: "warning" });
    return;
  }
  if (!canView.value) {
    message("需要查看搜索重建状态权限才能发起任务", { type: "warning" });
    return;
  }
  if (isActive.value) {
    message("已有搜索重建任务正在执行", { type: "warning" });
    return;
  }

  try {
    await ElMessageBox.confirm(
      "将重新索引当前已发布的文章、话题和启用用户。此操作不会删除搜索索引中已有的历史文档，执行期间请勿重复发起。",
      "确认重建搜索索引",
      {
        type: "warning",
        confirmButtonText: "开始重建",
        cancelButtonText: "取消"
      }
    );
  } catch {
    return;
  }

  starting.value = true;
  try {
    const { code, data, message: msg } = await startAdminSearchRebuild();
    if (code !== 0) {
      message(msg || "启动搜索重建失败", { type: "error" });
      return;
    }
    status.value = resolveStatus(data);
    message("搜索重建任务已提交", { type: "success" });
    schedulePoll();
  } catch (error) {
    message(error instanceof Error ? error.message : "启动搜索重建失败", {
      type: "error"
    });
  } finally {
    starting.value = false;
  }
}

onMounted(() => void loadStatus());
onUnmounted(clearPollTimer);
</script>

<template>
  <div class="search-rebuild-page">
    <el-alert
      title="重建会重新写入当前已发布的文章、话题和启用用户，不会删除搜索索引中已有的历史文档。"
      type="warning"
      :closable="false"
      show-icon
    />

    <el-card shadow="never" class="search-rebuild-card">
      <template #header>
        <div class="card-header">
          <div>
            <h2>搜索索引重建</h2>
            <p>用于在索引异常或数据修复后重新同步公开搜索数据。</p>
          </div>
          <div class="header-actions">
            <el-button
              :loading="loading"
              :disabled="!canView"
              @click="loadStatus()"
            >
              刷新状态
            </el-button>
            <el-button
              type="primary"
              :loading="starting"
              :disabled="!canStart || !canView || isActive"
              @click="startRebuild"
            >
              {{ isActive ? "重建进行中" : "开始重建" }}
            </el-button>
          </div>
        </div>
      </template>

      <el-alert
        v-if="!canView"
        title="当前账号没有查看搜索重建状态的权限，因此不能发起重建任务。"
        type="info"
        :closable="false"
        show-icon
        class="permission-alert"
      />

      <div v-else v-loading="loading" class="status-content">
        <div class="status-summary">
          <div>
            <span class="status-label">当前状态</span>
            <el-tag :type="statusTagType()" effect="light">
              {{ statusText() }}
            </el-tag>
          </div>
          <span v-if="status?.job_id || status?.jobId" class="job-id">
            任务 ID：{{ status?.job_id ?? status?.jobId }}
          </span>
        </div>

        <el-progress
          :percentage="progress"
          :status="currentState === 'failed' ? 'exception' : undefined"
          :indeterminate="isActive && contentTotal === 0"
          :duration="4"
          striped
          striped-flow
        />

        <div class="progress-detail">
          <div>
            <strong>{{ articleIndexed }}</strong>
            <span>/ {{ articleTotal }} 篇文章</span>
          </div>
          <div>
            <strong>{{ topicIndexed }}</strong>
            <span>/ {{ topicTotal }} 个话题</span>
          </div>
          <div>
            <strong>{{ userIndexed }}</strong>
            <span>/ {{ userTotal }} 个用户</span>
          </div>
          <div>
            <strong>{{ indexedTotal }}</strong>
            <span>/ {{ contentTotal }} 条索引</span>
          </div>
        </div>

        <el-alert
          v-if="errorMessage"
          :title="errorMessage"
          type="error"
          :closable="false"
          show-icon
          class="error-alert"
        />

        <el-descriptions :column="2" border class="status-meta">
          <el-descriptions-item label="发起人 ID">
            {{ status?.requested_by ?? status?.requestedBy ?? "-" }}
          </el-descriptions-item>
          <el-descriptions-item label="开始时间">
            {{ formatTime(status?.started_at ?? status?.startedAt) }}
          </el-descriptions-item>
          <el-descriptions-item label="完成时间">
            {{ formatTime(status?.completed_at ?? status?.completedAt) }}
          </el-descriptions-item>
          <el-descriptions-item label="最近更新">
            {{ formatTime(status?.updated_at ?? status?.updatedAt) }}
          </el-descriptions-item>
        </el-descriptions>
      </div>
    </el-card>
  </div>
</template>

<style scoped>
.search-rebuild-page {
  display: flex;
  flex-direction: column;
  gap: 16px;
}

.search-rebuild-card {
  max-width: 920px;
}

.card-header,
.header-actions,
.status-summary,
.progress-detail {
  display: flex;
  align-items: center;
}

.card-header,
.status-summary {
  justify-content: space-between;
}

.card-header {
  gap: 16px;
}

.card-header h2 {
  margin: 0;
  font-size: 18px;
  font-weight: 600;
  color: var(--el-text-color-primary);
}

.card-header p {
  margin: 6px 0 0;
  font-size: 13px;
  color: var(--el-text-color-secondary);
}

.header-actions {
  flex-shrink: 0;
  gap: 8px;
}

.permission-alert,
.error-alert {
  margin-bottom: 16px;
}

.status-content {
  min-height: 180px;
}

.status-summary {
  gap: 12px;
  margin-bottom: 18px;
}

.status-summary > div {
  display: flex;
  gap: 8px;
  align-items: center;
}

.status-label,
.job-id,
.progress-detail span {
  font-size: 13px;
  color: var(--el-text-color-secondary);
}

.job-id {
  max-width: 60%;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.progress-detail {
  display: grid;
  grid-template-columns: repeat(4, minmax(0, 1fr));
  gap: 12px;
  margin: 14px 0 18px;
}

.progress-detail > div {
  padding: 12px;
  background: var(--el-fill-color-extra-light);
  border: 1px solid var(--el-border-color-light);
  border-radius: 6px;
}

.progress-detail strong {
  margin-right: 4px;
  color: var(--el-text-color-primary);
}

.status-meta {
  margin-top: 16px;
}

@media (max-width: 768px) {
  .card-header,
  .header-actions,
  .status-summary {
    align-items: stretch;
    flex-direction: column;
  }

  .progress-detail {
    grid-template-columns: 1fr;
  }

  .job-id {
    max-width: 100%;
  }
}
</style>
