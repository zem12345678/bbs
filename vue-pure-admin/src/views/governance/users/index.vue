<script setup lang="ts">
import dayjs from "dayjs";
import { computed, onMounted, reactive, ref } from "vue";
import { ElMessageBox } from "element-plus";
import { message } from "@/utils/message";
import { hasPerms } from "@/utils/auth";
import {
  listGovernanceUsers,
  muteAdminUser,
  unmuteAdminUser,
  type AdminUser
} from "@/api/admin";
import { normalizeEntityId } from "@/utils/entityId";
import { useRenderIcon } from "@/components/ReIcon/src/hooks";
import GovernanceDetailDrawer from "../components/GovernanceDetailDrawer.vue";

type UserRow = Partial<AdminUser> & Record<string, any>;

defineOptions({
  name: "GovernanceUsers"
});

const loading = ref(false);
const users = ref<AdminUser[]>([]);
const detailVisible = ref(false);
const selectedUser = ref<UserRow | null>(null);
const query = reactive({
  keyword: "",
  status: 0,
  pageSize: 20,
  currentPage: 1,
  total: 0
});

const canList = computed(() => hasPerms("governance:list_users"));
const canMute = computed(() => hasPerms("governance:mute_user"));
const canUnmute = computed(() => hasPerms("governance:unmute_user"));

const columns: TableColumnList = [
  { prop: "id", label: "用户 ID", width: 96 },
  { label: "账号", minWidth: 190, slot: "account" },
  { prop: "nickname", label: "昵称", minWidth: 140 },
  { label: "状态", width: 100, slot: "status" },
  { label: "关注数据", minWidth: 150, slot: "counts" },
  { label: "注册时间", width: 170, slot: "createdAt" },
  { label: "最后登录", width: 170, slot: "lastLoginAt" },
  { label: "操作", fixed: "right", width: 210, slot: "operation" }
];

const userDetailFields = computed(() => {
  const user = selectedUser.value;
  if (!user) return [];
  return [
    { label: "用户 ID", value: `#${user.id ?? "-"}` },
    { label: "状态", status: statusMeta(user.status) },
    { label: "主页背景", imageUrl: userBackgroundUrl(user) },
    { label: "头像", imageUrl: userAvatarUrl(user) },
    { label: "用户名", value: user.username },
    { label: "昵称", value: user.nickname },
    { label: "邮箱", value: user.email },
    { label: "角色", tags: user.roles ?? [] },
    {
      label: "粉丝数",
      value: formatCount(valueOf(user, "follower_count", "followerCount"))
    },
    {
      label: "关注数",
      value: formatCount(valueOf(user, "following_count", "followingCount"))
    },
    {
      label: "锁定状态",
      value: valueOf(user, "locked_flag", "lockedFlag") ? "已锁定" : "未锁定"
    },
    {
      label: "注册时间",
      value: formatTime(valueOf(user, "created_at", "createdAt"))
    },
    {
      label: "更新时间",
      value: formatTime(valueOf(user, "updated_at", "updatedAt"))
    },
    {
      label: "最后登录",
      value: formatTime(valueOf(user, "last_login_at", "lastLoginAt"))
    }
  ];
});

const userDetailSections = computed(() => {
  const user = selectedUser.value;
  if (!user) return [];
  return [{ title: "个人简介", content: user.bio }];
});

const statusOptions = [
  { label: "全部", value: 0 },
  { label: "正常", value: 1 },
  { label: "禁言", value: 2 }
];

function statusMeta(status?: number) {
  switch (status) {
    case 1:
      return { label: "正常", type: "success" as const };
    case 2:
      return { label: "禁言", type: "danger" as const };
    default:
      return { label: `未知(${status ?? "-"})`, type: "info" as const };
  }
}

function valueOf(row: UserRow, snakeKey: string, camelKey: string) {
  return row[snakeKey] ?? row[camelKey];
}

function userAvatarUrl(row: UserRow) {
  return row.avatar_url ?? row.avatarUrl ?? "";
}

function userBackgroundUrl(row: UserRow) {
  return row.background_url ?? row.backgroundUrl ?? "";
}

function formatCount(value?: number) {
  return Number(value ?? 0).toLocaleString();
}

function formatTime(value?: number) {
  if (!value) return "-";
  const timestamp = value > 9999999999 ? value : value * 1000;
  return dayjs(timestamp).format("YYYY-MM-DD HH:mm");
}

async function loadUsers() {
  if (!canList.value) {
    return;
  }
  loading.value = true;
  try {
    const {
      code,
      data,
      message: msg
    } = await listGovernanceUsers({
      query: query.keyword,
      status: query.status,
      page: query.currentPage,
      page_size: query.pageSize
    });
    if (code !== 0) {
      message(msg || "加载用户列表失败", { type: "error" });
      return;
    }
    users.value = data.items ?? [];
    query.total = data.total ?? 0;
  } finally {
    loading.value = false;
  }
}

function resetQuery() {
  query.keyword = "";
  query.status = 0;
  query.currentPage = 1;
  loadUsers();
}

function openDetail(row: UserRow) {
  selectedUser.value = row;
  detailVisible.value = true;
}

async function updateUserStatus(row: UserRow, muted: boolean) {
  if (muted && !canMute.value) {
    message("没有禁言用户权限", { type: "warning" });
    return;
  }
  if (!muted && !canUnmute.value) {
    message("没有解禁用户权限", { type: "warning" });
    return;
  }
  const userId = normalizeEntityId(row.id);
  if (!userId) {
    message("用户 ID 无效", { type: "warning" });
    return;
  }
  const actionText = muted ? "禁言" : "解禁";
  await ElMessageBox.confirm(
    `确认${actionText}用户 ${row.username || `#${userId}`}？`,
    {
      title: "用户状态确认",
      type: "warning",
      confirmButtonText: "确认",
      cancelButtonText: "取消"
    }
  );
  loading.value = true;
  try {
    const { code, message: msg } = muted
      ? await muteAdminUser(userId)
      : await unmuteAdminUser(userId);
    if (code !== 0) {
      message(msg || `${actionText}失败`, { type: "error" });
      return;
    }
    message(`${actionText}已生效`, { type: "success" });
    await loadUsers();
  } finally {
    loading.value = false;
  }
}

function onPageSizeChange(size: number) {
  query.pageSize = size;
  query.currentPage = 1;
  loadUsers();
}

function onCurrentPageChange(page: number) {
  query.currentPage = page;
  loadUsers();
}

onMounted(loadUsers);
</script>

<template>
  <div class="governance-users-page">
    <section class="governance-panel">
      <div class="panel-header">
        <div>
          <h2>用户管理</h2>
          <p>管理社区用户状态和基础信息</p>
        </div>
      </div>

      <el-alert
        v-if="!canList"
        title="当前账号没有 governance:list_users 权限"
        type="warning"
        show-icon
        :closable="false"
        class="permission-alert"
      />

      <el-form :inline="true" class="search-form">
        <el-form-item label="关键词">
          <el-input
            v-model="query.keyword"
            clearable
            class="w-64!"
            placeholder="用户名 / 邮箱 / 昵称"
            @keyup.enter="loadUsers"
          />
        </el-form-item>
        <el-form-item label="状态">
          <el-select v-model="query.status" class="w-36!" @change="loadUsers">
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
            @click="loadUsers"
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
        :data="users"
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
        <template #account="{ row }">
          <div class="user-cell">
            <span class="username">{{ row.username || "-" }}</span>
            <span class="email">{{ row.email || "-" }}</span>
          </div>
        </template>
        <template #status="{ row }">
          <el-tag :type="statusMeta(row.status).type">
            {{ statusMeta(row.status).label }}
          </el-tag>
        </template>
        <template #counts="{ row }">
          <div class="count-cell">
            <span>
              粉丝
              {{ formatCount(valueOf(row, "follower_count", "followerCount")) }}
            </span>
            <span>
              关注
              {{
                formatCount(valueOf(row, "following_count", "followingCount"))
              }}
            </span>
          </div>
        </template>
        <template #createdAt="{ row }">
          {{ formatTime(valueOf(row, "created_at", "createdAt")) }}
        </template>
        <template #lastLoginAt="{ row }">
          {{ formatTime(valueOf(row, "last_login_at", "lastLoginAt")) }}
        </template>
        <template #operation="{ row }">
          <el-button
            link
            type="primary"
            :icon="useRenderIcon('ri/eye-line')"
            @click="openDetail(row)"
          >
            查看
          </el-button>
          <el-button
            v-if="row.status !== 2"
            link
            type="danger"
            :disabled="!canMute"
            @click="updateUserStatus(row, true)"
          >
            禁言
          </el-button>
          <el-button
            v-if="row.status === 2"
            link
            type="primary"
            :disabled="!canUnmute"
            @click="updateUserStatus(row, false)"
          >
            解禁
          </el-button>
        </template>
      </pure-table>

      <GovernanceDetailDrawer
        v-model="detailVisible"
        title="用户详情"
        :fields="userDetailFields"
        :sections="userDetailSections"
      />
    </section>
  </div>
</template>

<style scoped>
.governance-users-page {
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

.permission-alert {
  margin-bottom: 8px;
}

.search-form {
  padding: 12px 0 4px;
}

.user-cell,
.count-cell {
  display: flex;
  flex-direction: column;
  gap: 3px;
  align-items: flex-start;
}

.username {
  font-weight: 600;
  color: var(--el-text-color-primary);
}

.email,
.count-cell {
  font-size: 12px;
  color: var(--el-text-color-secondary);
}

@media (width <= 768px) {
  .panel-header {
    flex-direction: column;
  }
}
</style>
