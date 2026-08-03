<script setup lang="ts">
import dayjs from "dayjs";
import { copyTextToClipboard } from "@pureadmin/utils";
import { computed, onMounted, reactive, ref } from "vue";
import { ElMessageBox } from "element-plus";
import { message } from "@/utils/message";
import { hasPerms } from "@/utils/auth";
import {
  createAdminInviteCodes,
  listAdminInviteCodes,
  revokeAdminInviteCode,
  type AdminInviteCode
} from "@/api/admin";
import { normalizeEntityId } from "@/utils/entityId";
import { useRenderIcon } from "@/components/ReIcon/src/hooks";

defineOptions({ name: "GovernanceInvites" });

type InviteRow = Partial<AdminInviteCode> & Record<string, any>;

const loading = ref(false);
const creating = ref(false);
const dialogVisible = ref(false);
const invites = ref<AdminInviteCode[]>([]);
const generated = ref<AdminInviteCode[]>([]);
const query = reactive({
  status: "",
  pageSize: 20,
  currentPage: 1,
  total: 0
});
const form = reactive({
  count: 10,
  expiresAt: "" as string | Date
});

const canList = computed(() => hasPerms("governance:list_invite_codes"));
const canCreate = computed(() => hasPerms("governance:create_invite_codes"));
const canRevoke = computed(() => hasPerms("governance:revoke_invite_code"));

const columns: TableColumnList = [
  { prop: "code", label: "邀请码", minWidth: 180, slot: "code" },
  { label: "状态", width: 100, slot: "status" },
  { label: "创建管理员", width: 120, slot: "creator" },
  { label: "使用用户", width: 120, slot: "user" },
  { label: "有效期", width: 170, slot: "expiresAt" },
  { label: "使用时间", width: 170, slot: "usedAt" },
  { label: "创建时间", width: 170, slot: "createdAt" },
  { label: "操作", fixed: "right", width: 120, slot: "operation" }
];

const statusOptions = [
  { label: "全部", value: "" },
  { label: "未使用", value: "unused" },
  { label: "已使用", value: "used" },
  { label: "已过期", value: "expired" },
  { label: "已撤销", value: "revoked" }
];

function field(row: InviteRow, snake: string, camel: string) {
  return row[snake] ?? row[camel];
}

function statusMeta(status?: string) {
  switch (String(status || "").toLowerCase()) {
    case "unused":
      return { label: "未使用", type: "success" as const };
    case "used":
      return { label: "已使用", type: "info" as const };
    case "expired":
      return { label: "已过期", type: "warning" as const };
    case "revoked":
      return { label: "已撤销", type: "danger" as const };
    default:
      return { label: status || "未知", type: "info" as const };
  }
}

function formatTime(value?: number) {
  if (!value) return "-";
  const timestamp = value > 9999999999 ? value : value * 1000;
  return dayjs(timestamp).format("YYYY-MM-DD HH:mm");
}

async function loadInvites() {
  if (!canList.value) {
    invites.value = [];
    query.total = 0;
    return;
  }
  loading.value = true;
  try {
    const { code, data, message: msg } = await listAdminInviteCodes({
      status: query.status,
      page: query.currentPage,
      page_size: query.pageSize
    });
    if (code !== 0) {
      message(msg || "加载邀请码失败", { type: "error" });
      return;
    }
    invites.value = data.items ?? [];
    query.total = data.total ?? invites.value.length;
  } finally {
    loading.value = false;
  }
}

function resetQuery() {
  query.status = "";
  query.currentPage = 1;
  void loadInvites();
}

function openCreateDialog() {
  if (!canCreate.value) {
    message("没有生成邀请码权限", { type: "warning" });
    return;
  }
  form.count = 10;
  form.expiresAt = "";
  generated.value = [];
  dialogVisible.value = true;
}

async function createCodes() {
  const count = Math.floor(Number(form.count));
  if (count < 1 || count > 100) {
    message("单次生成数量必须为 1–100", { type: "warning" });
    return;
  }
  const expiresAt = form.expiresAt ? dayjs(form.expiresAt).unix() : 0;
  if (expiresAt && expiresAt <= dayjs().unix()) {
    message("过期时间必须晚于当前时间", { type: "warning" });
    return;
  }
  creating.value = true;
  try {
    const { code, data, message: msg } = await createAdminInviteCodes({
      count,
      ...(expiresAt ? { expires_at: expiresAt } : {})
    });
    if (code !== 0) {
      message(msg || "生成邀请码失败", { type: "error" });
      return;
    }
    generated.value = data.items ?? [];
    message(`已生成 ${generated.value.length} 个邀请码`, { type: "success" });
    query.currentPage = 1;
    await loadInvites();
  } finally {
    creating.value = false;
  }
}

async function copyCodes(rows: InviteRow[]) {
  const value = rows.map(row => String(row.code || "").trim()).filter(Boolean).join("\n");
  if (!value) return;
  const copied = await copyTextToClipboard(value);
  message(copied ? "邀请码已复制" : "复制失败，请手动复制", {
    type: copied ? "success" : "error"
  });
}

async function revokeCode(row: InviteRow) {
  if (!canRevoke.value) {
    message("没有撤销邀请码权限", { type: "warning" });
    return;
  }
  const id = normalizeEntityId(row.id);
  if (!id || row.status !== "unused") return;
  const confirmed = await ElMessageBox.confirm(
    `确认撤销邀请码「${row.code || id}」？撤销后不可恢复。`,
    "撤销邀请码",
    { type: "warning", confirmButtonText: "确认撤销", cancelButtonText: "取消" }
  ).catch(() => false);
  if (!confirmed) return;
  const { code, message: msg } = await revokeAdminInviteCode(id);
  if (code !== 0) {
    message(msg || "撤销邀请码失败", { type: "error" });
    return;
  }
  message("邀请码已撤销", { type: "success" });
  await loadInvites();
}

function onPageSizeChange(size: number) {
  query.pageSize = size;
  query.currentPage = 1;
  void loadInvites();
}

function onCurrentPageChange(page: number) {
  query.currentPage = page;
  void loadInvites();
}

onMounted(loadInvites);
</script>

<template>
  <div class="invites-page">
    <section class="governance-panel">
      <div class="panel-header">
        <div>
          <h2>注册邀请码</h2>
          <p>批量生成、审计使用情况并撤销尚未使用的邀请码</p>
        </div>
        <el-button
          type="primary"
          :icon="useRenderIcon('ri/add-line')"
          :disabled="!canCreate"
          @click="openCreateDialog"
        >
          生成邀请码
        </el-button>
      </div>

      <el-alert
        v-if="!canList"
        title="当前账号没有 governance:list_invite_codes 权限"
        type="warning"
        show-icon
        :closable="false"
        class="permission-alert"
      />

      <el-form :inline="true" class="search-form">
        <el-form-item label="状态">
          <el-select v-model="query.status" class="w-36!" @change="loadInvites">
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
            @click="loadInvites"
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
        :data="invites"
        :columns="columns"
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
        <template #code="{ row }">
          <el-button link type="primary" @click="copyCodes([row])">
            {{ row.code }}
          </el-button>
        </template>
        <template #status="{ row }">
          <el-tag :type="statusMeta(row.status).type">
            {{ statusMeta(row.status).label }}
          </el-tag>
        </template>
        <template #creator="{ row }">
          {{ field(row, "created_by_admin_id", "createdByAdminId") || "-" }}
        </template>
        <template #user="{ row }">
          {{ field(row, "used_by_user_id", "usedByUserId") || "-" }}
        </template>
        <template #expiresAt="{ row }">
          {{ formatTime(field(row, "expires_at", "expiresAt")) }}
        </template>
        <template #usedAt="{ row }">
          {{ formatTime(field(row, "used_at", "usedAt")) }}
        </template>
        <template #createdAt="{ row }">
          {{ formatTime(field(row, "created_at", "createdAt")) }}
        </template>
        <template #operation="{ row }">
          <el-button
            link
            type="danger"
            :disabled="!canRevoke || row.status !== 'unused'"
            :icon="useRenderIcon('ri/close-circle-line')"
            @click="revokeCode(row)"
          >
            撤销
          </el-button>
        </template>
      </pure-table>
    </section>

    <el-dialog v-model="dialogVisible" title="生成邀请码" width="620px" destroy-on-close>
      <el-form label-width="92px">
        <el-form-item label="生成数量" required>
          <el-input-number v-model="form.count" :min="1" :max="100" />
        </el-form-item>
        <el-form-item label="过期时间">
          <el-date-picker
            v-model="form.expiresAt"
            type="datetime"
            placeholder="不选择则长期有效"
            class="w-full!"
          />
        </el-form-item>
      </el-form>
      <div v-if="generated.length" class="generated-codes">
        <div class="generated-header">
          <strong>本次生成结果</strong>
          <el-button link type="primary" @click="copyCodes(generated)">
            复制全部
          </el-button>
        </div>
        <el-input
          :model-value="generated.map(item => item.code).join('\n')"
          type="textarea"
          :rows="Math.min(10, generated.length + 1)"
          readonly
        />
      </div>
      <template #footer>
        <el-button @click="dialogVisible = false">关闭</el-button>
        <el-button type="primary" :loading="creating" @click="createCodes">
          生成
        </el-button>
      </template>
    </el-dialog>
  </div>
</template>

<style scoped>
.invites-page {
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

.panel-header,
.generated-header {
  display: flex;
  gap: 16px;
  align-items: flex-start;
  justify-content: space-between;
}

.panel-header {
  margin-bottom: 14px;
}

.panel-header h2 {
  margin: 0;
  font-size: 18px;
  font-weight: 600;
}

.panel-header p {
  margin: 6px 0 0;
  font-size: 13px;
  color: var(--el-text-color-secondary);
}

.permission-alert {
  margin-bottom: 8px;
}

.search-form {
  padding: 12px 0 4px;
}

.generated-codes {
  display: flex;
  flex-direction: column;
  gap: 8px;
  margin-top: 8px;
}

@media (width <= 768px) {
  .panel-header {
    flex-direction: column;
  }
}
</style>
