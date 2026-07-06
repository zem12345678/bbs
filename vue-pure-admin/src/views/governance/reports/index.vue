<script setup lang="ts">
import dayjs from "dayjs";
import { computed, onMounted, reactive, ref } from "vue";
import { ElMessageBox } from "element-plus";
import { message } from "@/utils/message";
import { hasPerms } from "@/utils/auth";
import {
  auditAdminReport,
  listAdminReports,
  muteAdminUser,
  unmuteAdminUser,
  type AdminReport
} from "@/api/admin";
import { useRenderIcon } from "@/components/ReIcon/src/hooks";

type ReportRow = Partial<AdminReport> & Record<string, any>;

defineOptions({
  name: "GovernanceReports"
});

const loading = ref(false);
const reports = ref<AdminReport[]>([]);
const userActionLoading = ref(false);
const userForm = reactive({
  userId: undefined as number | undefined
});
const query = reactive({
  status: 1,
  pageSize: 20,
  currentPage: 1,
  total: 0
});

const canList = computed(() => hasPerms("governance:list_reports"));
const canAudit = computed(() => hasPerms("governance:audit_report"));
const canMute = computed(() => hasPerms("governance:mute_user"));
const canUnmute = computed(() => hasPerms("governance:unmute_user"));

const statusOptions = [
  { label: "全部", value: 0 },
  { label: "待处理", value: 1 },
  { label: "已处理", value: 2 },
  { label: "已驳回", value: 3 }
];

function statusMeta(status: number) {
  switch (status) {
    case 1:
      return { label: "待处理", type: "warning" as const };
    case 2:
      return { label: "已处理", type: "success" as const };
    case 3:
      return { label: "已驳回", type: "info" as const };
    default:
      return { label: `未知(${status})`, type: "danger" as const };
  }
}

function reportEntity(report: ReportRow) {
  const entity = report.entity ?? {};
  const type = entity.entity_type ?? entity.entityType ?? "-";
  const id = entity.entity_id ?? entity.entityId ?? "-";
  return `${type} #${id}`;
}

function reporterId(report: ReportRow) {
  return report.reporter_id ?? report.reporterId ?? 0;
}

function handledBy(report: ReportRow) {
  return report.handled_by ?? report.handledBy ?? 0;
}

function formatTime(value?: number) {
  if (!value) return "-";
  const timestamp = value > 9999999999 ? value : value * 1000;
  return dayjs(timestamp).format("YYYY-MM-DD HH:mm");
}

function reportCreatedAt(report: ReportRow) {
  return report.created_at ?? report.createdAt;
}

function reportHandledAt(report: ReportRow) {
  return report.handled_at ?? report.handledAt;
}

async function loadReports() {
  if (!canList.value) {
    reports.value = [];
    query.total = 0;
    return;
  }
  loading.value = true;
  try {
    const {
      code,
      data,
      message: msg
    } = await listAdminReports({
      status: query.status,
      limit: query.pageSize,
      offset: (query.currentPage - 1) * query.pageSize
    });
    if (code !== 0) {
      message(msg || "加载举报列表失败", { type: "error" });
      return;
    }
    reports.value = data.items ?? [];
    query.total = data.total ?? 0;
  } finally {
    loading.value = false;
  }
}

function resetQuery() {
  query.status = 1;
  query.currentPage = 1;
  loadReports();
}

async function handleAudit(row: ReportRow, nextStatus: number) {
  if (!canAudit.value) {
    message("没有审核举报权限", { type: "warning" });
    return;
  }
  const reportId = Number(row.id);
  if (!reportId) {
    message("举报 ID 无效", { type: "warning" });
    return;
  }
  const meta = statusMeta(nextStatus);
  await ElMessageBox.confirm(`确认将举报 #${reportId} 标记为${meta.label}？`, {
    title: "审核确认",
    type: "warning",
    confirmButtonText: "确认",
    cancelButtonText: "取消"
  });
  loading.value = true;
  try {
    const { code, message: msg } = await auditAdminReport(reportId, nextStatus);
    if (code !== 0) {
      message(msg || "审核失败", { type: "error" });
      return;
    }
    message("审核已提交", { type: "success" });
    await loadReports();
  } finally {
    loading.value = false;
  }
}

async function updateUserStatus(muted: boolean) {
  if (muted && !canMute.value) {
    message("没有禁言用户权限", { type: "warning" });
    return;
  }
  if (!muted && !canUnmute.value) {
    message("没有解禁用户权限", { type: "warning" });
    return;
  }
  if (!userForm.userId || userForm.userId <= 0) {
    message("请输入有效用户 ID", { type: "warning" });
    return;
  }
  const actionText = muted ? "禁言" : "解禁";
  await ElMessageBox.confirm(
    `确认${actionText}用户 #${userForm.userId}？`,
    "用户状态确认",
    {
      type: "warning",
      confirmButtonText: "确认",
      cancelButtonText: "取消"
    }
  );
  userActionLoading.value = true;
  try {
    const {
      code,
      data,
      message: msg
    } = muted
      ? await muteAdminUser(userForm.userId)
      : await unmuteAdminUser(userForm.userId);
    if (code !== 0) {
      message(msg || `${actionText}失败`, { type: "error" });
      return;
    }
    const statusLabel = data.user.status === 2 ? "已禁言" : "正常";
    message(
      `用户 ${data.user.username || data.user.id} 当前状态：${statusLabel}`,
      {
        type: "success"
      }
    );
  } finally {
    userActionLoading.value = false;
  }
}

function onPageSizeChange(size: number) {
  query.pageSize = size;
  query.currentPage = 1;
  loadReports();
}

function onCurrentPageChange(page: number) {
  query.currentPage = page;
  loadReports();
}

onMounted(loadReports);
</script>

<template>
  <div class="governance-page">
    <section class="governance-panel">
      <div class="panel-header">
        <div>
          <h2>举报审核</h2>
          <p>处理用户提交的内容举报</p>
        </div>
      </div>

      <el-alert
        v-if="!canList"
        title="当前账号没有举报列表查询权限"
        type="warning"
        show-icon
        :closable="false"
        class="permission-alert"
      />

      <el-form :inline="true" class="search-form">
        <el-form-item label="状态">
          <el-select v-model="query.status" class="w-40!" @change="loadReports">
            <el-option
              v-for="item in statusOptions"
              :key="item.value"
              :label="item.label"
              :value="item.value"
            />
          </el-select>
        </el-form-item>
        <el-form-item>
          <el-button
            type="primary"
            :icon="useRenderIcon('ri/search-line')"
            :disabled="!canList"
            :loading="loading"
            @click="loadReports"
          >
            查询
          </el-button>
          <el-button :icon="useRenderIcon('ep/refresh')" @click="resetQuery">
            重置
          </el-button>
        </el-form-item>
      </el-form>

      <pure-table
        row-key="id"
        adaptive
        :adaptiveConfig="{ offsetBottom: 156 }"
        align-whole="center"
        table-layout="auto"
        :loading="loading"
        :data="reports"
        :pagination="{
          total: query.total,
          pageSize: query.pageSize,
          currentPage: query.currentPage,
          background: true
        }"
        :header-cell-style="{
          background: 'var(--el-fill-color-light)',
          color: 'var(--el-text-color-primary)'
        }"
        @page-size-change="onPageSizeChange"
        @page-current-change="onCurrentPageChange"
      >
        <el-table-column prop="id" label="举报 ID" width="96" />
        <el-table-column label="对象" min-width="130">
          <template #default="{ row }">
            <el-tag effect="plain">{{ reportEntity(row) }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column label="举报人" min-width="110">
          <template #default="{ row }">#{{ reporterId(row) }}</template>
        </el-table-column>
        <el-table-column prop="reason" label="原因" min-width="150" />
        <el-table-column
          prop="description"
          label="说明"
          min-width="220"
          show-overflow-tooltip
        />
        <el-table-column label="状态" width="110">
          <template #default="{ row }">
            <el-tag :type="statusMeta(row.status).type">
              {{ statusMeta(row.status).label }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column label="处理人" width="110">
          <template #default="{ row }">
            {{ handledBy(row) ? `#${handledBy(row)}` : "-" }}
          </template>
        </el-table-column>
        <el-table-column label="提交时间" width="170">
          <template #default="{ row }">
            {{ formatTime(reportCreatedAt(row)) }}
          </template>
        </el-table-column>
        <el-table-column label="处理时间" width="170">
          <template #default="{ row }">
            {{ formatTime(reportHandledAt(row)) }}
          </template>
        </el-table-column>
        <el-table-column label="操作" fixed="right" width="180">
          <template #default="{ row }">
            <el-button
              v-if="row.status === 1 && canAudit"
              link
              type="primary"
              @click="handleAudit(row, 2)"
            >
              通过
            </el-button>
            <el-button
              v-if="row.status === 1 && canAudit"
              link
              type="danger"
              @click="handleAudit(row, 3)"
            >
              驳回
            </el-button>
            <span v-if="row.status !== 1 || !canAudit" class="muted-text">
              -
            </span>
          </template>
        </el-table-column>
      </pure-table>
    </section>

    <section class="governance-panel user-panel">
      <div class="panel-header compact">
        <div>
          <h2>用户状态</h2>
          <p>对社区用户执行禁言或解禁</p>
        </div>
      </div>
      <el-form :inline="true" class="search-form">
        <el-form-item label="用户 ID">
          <el-input-number
            v-model="userForm.userId"
            :min="1"
            :step="1"
            :controls="false"
            class="w-48!"
          />
        </el-form-item>
        <el-form-item>
          <el-button
            type="danger"
            :disabled="!canMute"
            :loading="userActionLoading"
            @click="updateUserStatus(true)"
          >
            禁言
          </el-button>
          <el-button
            type="primary"
            :disabled="!canUnmute"
            :loading="userActionLoading"
            @click="updateUserStatus(false)"
          >
            解禁
          </el-button>
        </el-form-item>
      </el-form>
    </section>
  </div>
</template>

<style scoped>
.governance-page {
  display: flex;
  flex-direction: column;
  gap: 16px;
}

.governance-panel {
  width: 100%;
  padding: 16px;
  background: var(--el-bg-color);
  border: 1px solid var(--el-border-color-light);
  border-radius: 6px;
}

.panel-header {
  display: flex;
  gap: 16px;
  align-items: flex-start;
  justify-content: space-between;
  margin-bottom: 14px;
}

.panel-header h2 {
  margin: 0;
  font-size: 18px;
  font-weight: 600;
  color: var(--el-text-color-primary);
}

.panel-header p {
  margin: 6px 0 0;
  font-size: 13px;
  color: var(--el-text-color-secondary);
}

.panel-header.compact {
  margin-bottom: 8px;
}

.search-form {
  padding: 12px 0 4px;
}

.permission-alert {
  margin-bottom: 8px;
}

.muted-text {
  color: var(--el-text-color-placeholder);
}

@media (width <= 768px) {
  .panel-header {
    flex-direction: column;
  }
}
</style>
