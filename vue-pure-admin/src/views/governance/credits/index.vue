<script setup lang="ts">
import dayjs from "dayjs";
import { computed, onMounted, reactive, ref } from "vue";
import { ElMessageBox, type FormInstance, type FormRules } from "element-plus";
import { message } from "@/utils/message";
import { hasPerms } from "@/utils/auth";
import {
  adjustAdminUserCredits,
  getAdminUserCreditBalance,
  listAdminUserCreditLedger,
  listGovernanceUsers,
  type AdminCreditBalance,
  type AdminCreditLedger,
  type AdminUser
} from "@/api/admin";
import { entityIdText, normalizeEntityId, type EntityId } from "@/utils/entityId";
import { useRenderIcon } from "@/components/ReIcon/src/hooks";

defineOptions({
  name: "GovernanceCredits"
});

type UserRow = Partial<AdminUser> & Record<string, any>;
type LedgerRow = Partial<AdminCreditLedger> & Record<string, any>;

const loadingUsers = ref(false);
const loadingCredits = ref(false);
const saving = ref(false);
const adjustVisible = ref(false);
const users = ref<AdminUser[]>([]);
const ledger = ref<AdminCreditLedger[]>([]);
const balance = ref<AdminCreditBalance | null>(null);
const selectedUser = ref<UserRow | null>(null);
const manualUserId = ref("");
const formRef = ref<FormInstance>();

const userQuery = reactive({
  keyword: "",
  status: 0,
  pageSize: 10,
  currentPage: 1,
  total: 0
});

const creditQuery = reactive({
  pageSize: 20,
  currentPage: 1,
  total: 0
});

const form = reactive({
  delta: 0,
  reason: "admin_adjustment",
  description: ""
});

const canListUsers = computed(() => hasPerms("governance:list_users"));
const canListCredits = computed(() => hasPerms("governance:list_user_credits"));
const canAdjustCredits = computed(() => hasPerms("governance:adjust_user_credits"));
const currentUserId = computed<EntityId | undefined>(() =>
  normalizeEntityId(selectedUser.value?.id ?? manualUserId.value)
);
const currentUserTitle = computed(() => {
  if (selectedUser.value) {
    return `${selectedUser.value.username || selectedUser.value.nickname || "用户"} #${entityIdText(selectedUser.value.id)}`;
  }
  return currentUserId.value ? `用户 #${currentUserId.value}` : "未选择用户";
});

const userColumns: TableColumnList = [
  { prop: "id", label: "用户 ID", width: 110 },
  { label: "账号", minWidth: 170, slot: "account" },
  { prop: "nickname", label: "昵称", minWidth: 130, showOverflowTooltip: true },
  { label: "状态", width: 90, slot: "userStatus" },
  { label: "注册时间", width: 170, slot: "createdAt" },
  { label: "操作", fixed: "right", width: 120, slot: "userOperation" }
];

const ledgerColumns: TableColumnList = [
  { prop: "id", label: "流水 ID", width: 100 },
  { label: "变动", width: 110, slot: "delta" },
  { label: "变动后", width: 110, slot: "balanceAfter" },
  { label: "原因", minWidth: 150, slot: "reason" },
  { prop: "description", label: "说明", minWidth: 240, showOverflowTooltip: true },
  { label: "来源", minWidth: 180, slot: "source" },
  { label: "创建时间", width: 170, slot: "createdAt" }
];

const statusOptions = [
  { label: "全部", value: 0 },
  { label: "正常", value: 1 },
  { label: "禁言", value: 2 }
];

const reasonOptions = [
  { label: "人工调整", value: "admin_adjustment" },
  { label: "客服补偿", value: "customer_compensation" },
  { label: "活动发放", value: "activity_reward" },
  { label: "纠错冲正", value: "correction" }
];

const rules: FormRules = {
  delta: [
    { required: true, message: "请输入调整积分", trigger: "change" },
    {
      validator: (_rule, value, callback) => {
        if (Number(value) === 0) {
          callback(new Error("调整积分不能为 0"));
          return;
        }
        callback();
      },
      trigger: "change"
    }
  ],
  reason: [{ required: true, message: "请选择调整原因", trigger: "change" }],
  description: [
    { required: true, message: "请输入调整说明", trigger: "blur" },
    { min: 2, max: 200, message: "说明长度需在 2-200 个字符", trigger: "blur" }
  ]
};

function userStatusMeta(status?: number) {
  switch (Number(status || 0)) {
    case 1:
      return { label: "正常", type: "success" as const };
    case 2:
      return { label: "禁言", type: "danger" as const };
    default:
      return { label: "未知", type: "info" as const };
  }
}

function valueOf(row: LedgerRow | UserRow, snakeKey: string, camelKey: string) {
  return row[snakeKey] ?? row[camelKey];
}

function formatTime(value?: number) {
  if (!value) return "-";
  const timestamp = value > 9999999999 ? value : value * 1000;
  return dayjs(timestamp).format("YYYY-MM-DD HH:mm");
}

function reasonLabel(reason?: string) {
  const matched = reasonOptions.find(item => item.value === reason);
  if (matched) return matched.label;
  switch (reason) {
    case "welcome":
      return "注册奖励";
    case "mall_order_payment":
      return "商城支付";
    case "article_published":
      return "发布文章";
    default:
      return reason || "-";
  }
}

function resetUserQuery() {
  userQuery.keyword = "";
  userQuery.status = 0;
  userQuery.currentPage = 1;
  loadUsers();
}

async function loadUsers() {
  if (!canListUsers.value) {
    users.value = [];
    userQuery.total = 0;
    return;
  }
  loadingUsers.value = true;
  try {
    const { code, data, message: msg } = await listGovernanceUsers({
      query: userQuery.keyword,
      status: userQuery.status,
      page: userQuery.currentPage,
      page_size: userQuery.pageSize
    });
    if (code !== 0) {
      message(msg || "加载用户列表失败", { type: "error" });
      return;
    }
    users.value = data.items ?? [];
    userQuery.total = data.total ?? users.value.length;
  } finally {
    loadingUsers.value = false;
  }
}

async function selectUser(row: UserRow) {
  selectedUser.value = row;
  manualUserId.value = String(row.id ?? "");
  creditQuery.currentPage = 1;
  await loadCredits();
}

async function loadManualCredits() {
  selectedUser.value = null;
  creditQuery.currentPage = 1;
  await loadCredits();
}

async function loadCredits() {
  if (!canListCredits.value) {
    ledger.value = [];
    balance.value = null;
    creditQuery.total = 0;
    return;
  }
  const userId = currentUserId.value;
  if (!userId) {
    message("请输入或选择用户 ID", { type: "warning" });
    return;
  }
  loadingCredits.value = true;
  try {
    const [balanceResp, ledgerResp] = await Promise.all([
      getAdminUserCreditBalance(userId),
      listAdminUserCreditLedger(userId, {
        limit: creditQuery.pageSize,
        offset: (creditQuery.currentPage - 1) * creditQuery.pageSize
      })
    ]);
    if (balanceResp.code !== 0) {
      message(balanceResp.message || "加载积分余额失败", { type: "error" });
      return;
    }
    if (ledgerResp.code !== 0) {
      message(ledgerResp.message || "加载积分流水失败", { type: "error" });
      return;
    }
    balance.value = balanceResp.data.balance ?? ledgerResp.data.balance ?? null;
    ledger.value = ledgerResp.data.items ?? [];
    creditQuery.total = ledgerResp.data.total ?? ledger.value.length;
  } finally {
    loadingCredits.value = false;
  }
}

function openAdjustDialog() {
  if (!canAdjustCredits.value) {
    message("没有调整积分权限", { type: "warning" });
    return;
  }
  if (!currentUserId.value) {
    message("请先选择用户", { type: "warning" });
    return;
  }
  form.delta = 0;
  form.reason = "admin_adjustment";
  form.description = "";
  formRef.value?.clearValidate();
  adjustVisible.value = true;
}

async function submitAdjust() {
  if (!canAdjustCredits.value || !currentUserId.value) return;
  const valid = await formRef.value?.validate().catch(() => false);
  if (!valid) return;
  const delta = Number(form.delta || 0);
  const confirmed = await ElMessageBox.confirm(
    `确认给 ${currentUserTitle.value} ${delta > 0 ? "增加" : "扣减"} ${Math.abs(delta)} 积分？`,
    "积分调整确认",
    {
      type: delta > 0 ? "warning" : "error",
      confirmButtonText: "确认调整",
      cancelButtonText: "取消"
    }
  ).catch(() => false);
  if (!confirmed) return;
  saving.value = true;
  try {
    const { code, message: msg } = await adjustAdminUserCredits(currentUserId.value, {
      delta,
      reason: form.reason,
      description: form.description.trim()
    });
    if (code !== 0) {
      message(msg || "调整积分失败", { type: "error" });
      return;
    }
    message("积分已调整", { type: "success" });
    adjustVisible.value = false;
    await loadCredits();
  } finally {
    saving.value = false;
  }
}

function onUserPageSizeChange(size: number) {
  userQuery.pageSize = size;
  userQuery.currentPage = 1;
  loadUsers();
}

function onUserCurrentPageChange(page: number) {
  userQuery.currentPage = page;
  loadUsers();
}

function onLedgerPageSizeChange(size: number) {
  creditQuery.pageSize = size;
  creditQuery.currentPage = 1;
  loadCredits();
}

function onLedgerCurrentPageChange(page: number) {
  creditQuery.currentPage = page;
  loadCredits();
}

onMounted(loadUsers);
</script>

<template>
  <div class="credits-page">
    <section class="governance-panel">
      <div class="panel-header">
        <div>
          <h2>积分管理</h2>
          <p>查询用户积分余额、流水，并处理客服补偿、活动发放和纠错冲正</p>
        </div>
        <el-button
          type="primary"
          :icon="useRenderIcon('ri/coins-line')"
          :disabled="!canAdjustCredits || !currentUserId"
          @click="openAdjustDialog"
        >
          调整积分
        </el-button>
      </div>

      <el-alert
        v-if="!canListCredits"
        title="当前账号没有 governance:list_user_credits 权限"
        type="warning"
        show-icon
        :closable="false"
        class="permission-alert"
      />

      <section class="credit-summary">
        <div>
          <span>当前用户</span>
          <strong>{{ currentUserTitle }}</strong>
        </div>
        <div>
          <span>可用积分</span>
          <strong>{{ balance?.total ?? "-" }}</strong>
        </div>
        <div>
          <span>更新时间</span>
          <strong>{{ formatTime(balance?.updated_at ?? balance?.updatedAt) }}</strong>
        </div>
      </section>

      <el-form :inline="true" class="search-form">
        <el-form-item label="用户 ID">
          <el-input
            v-model="manualUserId"
            class="w-56!"
            clearable
            placeholder="输入用户 ID 直接查询"
          />
        </el-form-item>
        <el-form-item>
          <el-button
            type="primary"
            :icon="useRenderIcon('ri/search-line')"
            :disabled="!canListCredits"
            :loading="loadingCredits"
            @click="loadManualCredits"
          >
            查询积分
          </el-button>
        </el-form-item>
        <el-form-item label="用户检索">
          <el-input
            v-model="userQuery.keyword"
            class="w-56!"
            clearable
            placeholder="账号/昵称/邮箱"
            @keyup.enter="loadUsers"
          />
        </el-form-item>
        <el-form-item label="状态">
          <el-select v-model="userQuery.status" class="w-32!" @change="loadUsers">
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
            :icon="useRenderIcon('ri-user-search-line')"
            :disabled="!canListUsers"
            :loading="loadingUsers"
            @click="loadUsers"
          >
            搜索用户
          </el-button>
          <el-button :icon="useRenderIcon('ep/refresh')" @click="resetUserQuery">
            重置
          </el-button>
        </el-form-item>
      </el-form>

      <pure-table
        row-key="id"
        adaptive
        :adaptiveConfig="{ offsetBottom: 460 }"
        align-whole="center"
        table-layout="auto"
        :loading="loadingUsers"
        :data="users"
        :columns="userColumns"
        :pagination="{
          total: userQuery.total,
          pageSize: userQuery.pageSize,
          currentPage: userQuery.currentPage,
          background: true
        }"
        :header-cell-style="{
          background: 'var(--el-fill-color-light)',
          color: 'var(--el-text-color-primary)'
        }"
        @page-size-change="onUserPageSizeChange"
        @page-current-change="onUserCurrentPageChange"
      >
        <template #account="{ row }">
          <div class="account-cell">
            <strong>{{ row.username || "-" }}</strong>
            <span>{{ row.email || "-" }}</span>
          </div>
        </template>
        <template #userStatus="{ row }">
          <el-tag :type="userStatusMeta(row.status).type">
            {{ userStatusMeta(row.status).label }}
          </el-tag>
        </template>
        <template #createdAt="{ row }">
          {{ formatTime(valueOf(row, "created_at", "createdAt")) }}
        </template>
        <template #userOperation="{ row }">
          <el-button
            link
            type="primary"
            :icon="useRenderIcon('ri-wallet-3-line')"
            :disabled="!canListCredits"
            @click="selectUser(row)"
          >
            查看积分
          </el-button>
        </template>
      </pure-table>
    </section>

    <section class="governance-panel ledger-panel">
      <div class="panel-header compact">
        <div>
          <h2>积分流水</h2>
          <p>{{ currentUserTitle }} 的积分变动记录</p>
        </div>
        <el-button
          :icon="useRenderIcon('ep/refresh')"
          :disabled="!currentUserId || !canListCredits"
          :loading="loadingCredits"
          @click="loadCredits"
        >
          刷新
        </el-button>
      </div>

      <pure-table
        row-key="id"
        adaptive
        :adaptiveConfig="{ offsetBottom: 156 }"
        align-whole="center"
        table-layout="auto"
        :loading="loadingCredits"
        :data="ledger"
        :columns="ledgerColumns"
        :pagination="{
          total: creditQuery.total,
          pageSize: creditQuery.pageSize,
          currentPage: creditQuery.currentPage,
          background: true
        }"
        :header-cell-style="{
          background: 'var(--el-fill-color-light)',
          color: 'var(--el-text-color-primary)'
        }"
        @page-size-change="onLedgerPageSizeChange"
        @page-current-change="onLedgerCurrentPageChange"
      >
        <template #delta="{ row }">
          <el-tag :type="Number(row.delta) >= 0 ? 'success' : 'danger'" effect="plain">
            {{ Number(row.delta) >= 0 ? "+" : "" }}{{ row.delta }}
          </el-tag>
        </template>
        <template #balanceAfter="{ row }">
          {{ valueOf(row, "balance_after", "balanceAfter") ?? "-" }}
        </template>
        <template #reason="{ row }">
          {{ reasonLabel(row.reason) }}
        </template>
        <template #source="{ row }">
          <div class="source-cell">
            <span>{{ valueOf(row, "source_type", "sourceType") || "-" }}</span>
            <small>{{ valueOf(row, "source_event_id", "sourceEventId") || "-" }}</small>
          </div>
        </template>
        <template #createdAt="{ row }">
          {{ formatTime(valueOf(row, "created_at", "createdAt")) }}
        </template>
      </pure-table>
    </section>

    <el-dialog
      v-model="adjustVisible"
      title="调整积分"
      width="560px"
      destroy-on-close
    >
      <el-form
        ref="formRef"
        :model="form"
        :rules="rules"
        label-width="92px"
        class="credit-form"
      >
        <el-form-item label="用户">
          <el-input :model-value="currentUserTitle" disabled />
        </el-form-item>
        <el-form-item label="调整值" prop="delta">
          <el-input-number
            v-model="form.delta"
            :min="-999999"
            :max="999999"
            :step="10"
            class="w-full!"
          />
        </el-form-item>
        <el-form-item label="原因" prop="reason">
          <el-select v-model="form.reason" class="w-full!">
            <el-option
              v-for="item in reasonOptions"
              :key="item.value"
              :label="item.label"
              :value="item.value"
            />
          </el-select>
        </el-form-item>
        <el-form-item label="说明" prop="description">
          <el-input
            v-model="form.description"
            type="textarea"
            maxlength="200"
            show-word-limit
            :rows="4"
            placeholder="说明调整背景，便于运营审计"
          />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="adjustVisible = false">取消</el-button>
        <el-button type="primary" :loading="saving" @click="submitAdjust">
          确认调整
        </el-button>
      </template>
    </el-dialog>
  </div>
</template>

<style scoped>
.credits-page {
  display: grid;
  gap: 16px;
}

.governance-panel {
  padding: 18px;
  background: var(--el-bg-color);
  border: 1px solid var(--el-border-color-light);
  border-radius: 8px;
}

.panel-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 16px;
  margin-bottom: 16px;
}

.panel-header.compact {
  margin-bottom: 12px;
}

.panel-header h2 {
  margin: 0;
  font-size: 18px;
  font-weight: 650;
}

.panel-header p {
  margin: 4px 0 0;
  color: var(--el-text-color-secondary);
}

.permission-alert {
  margin-bottom: 16px;
}

.credit-summary {
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  gap: 12px;
  margin-bottom: 16px;
}

.credit-summary > div {
  min-height: 74px;
  padding: 14px 16px;
  background: var(--el-fill-color-lighter);
  border: 1px solid var(--el-border-color-light);
  border-radius: 8px;
}

.credit-summary span,
.source-cell small,
.account-cell span {
  display: block;
  color: var(--el-text-color-secondary);
}

.credit-summary strong {
  display: block;
  margin-top: 8px;
  font-size: 20px;
}

.search-form {
  margin-bottom: 12px;
}

.account-cell,
.source-cell {
  display: grid;
  gap: 2px;
  text-align: left;
}

.source-cell small {
  font-size: 12px;
}

.ledger-panel {
  min-height: 360px;
}

.credit-form :deep(.el-input-number .el-input__inner) {
  text-align: left;
}

@media (max-width: 980px) {
  .panel-header {
    align-items: flex-start;
    flex-direction: column;
  }

  .credit-summary {
    grid-template-columns: 1fr;
  }
}
</style>
