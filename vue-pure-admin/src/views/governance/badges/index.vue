<script setup lang="ts">
import dayjs from "dayjs";
import { computed, onMounted, reactive, ref } from "vue";
import { ElMessageBox, type FormInstance, type FormRules } from "element-plus";
import { message } from "@/utils/message";
import { hasPerms } from "@/utils/auth";
import {
  createAdminBadge,
  deleteAdminBadge,
  listAdminBadges,
  updateAdminBadge,
  type AdminBadge,
  type AdminBadgePayload
} from "@/api/admin";
import { normalizeEntityId, type EntityId } from "@/utils/entityId";
import { useRenderIcon } from "@/components/ReIcon/src/hooks";

defineOptions({
  name: "GovernanceBadges"
});

type BadgeRow = Partial<AdminBadge> & Record<string, any>;
type DialogMode = "create" | "edit";

const loading = ref(false);
const saving = ref(false);
const dialogVisible = ref(false);
const dialogMode = ref<DialogMode>("create");
const formRef = ref<FormInstance>();
const badges = ref<AdminBadge[]>([]);

const query = reactive({
  status: 0,
  pageSize: 20,
  currentPage: 1,
  total: 0
});

const form = reactive({
  id: 0 as EntityId,
  key: "",
  name: "",
  description: "",
  iconUrl: "",
  ruleType: "manual",
  ruleValue: 0,
  status: 2,
  sort: 0
});

const canList = computed(() => hasPerms("governance:list_badges"));
const canCreate = computed(() => hasPerms("governance:create_badge"));
const canUpdate = computed(() => hasPerms("governance:update_badge"));
const canDelete = computed(() => hasPerms("governance:delete_badge"));

const dialogTitle = computed(() =>
  dialogMode.value === "create" ? "新增徽章" : "编辑徽章"
);

const columns: TableColumnList = [
  { prop: "id", label: "ID", width: 90 },
  { label: "图标", width: 90, slot: "icon" },
  { prop: "key", label: "徽章标识", minWidth: 150, showOverflowTooltip: true },
  { prop: "name", label: "徽章名称", minWidth: 160, showOverflowTooltip: true },
  {
    prop: "description",
    label: "说明",
    minWidth: 240,
    showOverflowTooltip: true
  },
  { label: "授予规则", minWidth: 180, slot: "rule" },
  { prop: "sort", label: "排序", width: 90 },
  { label: "状态", width: 100, slot: "status" },
  { label: "更新时间", width: 170, slot: "updatedAt" },
  { label: "操作", fixed: "right", width: 180, slot: "operation" }
];

const statusOptions = [
  { label: "全部", value: 0 },
  { label: "启用", value: 2 },
  { label: "停用", value: 1 }
];

const ruleOptions = [
  { label: "手动授予", value: "manual" },
  { label: "创建账号", value: "account_created" },
  { label: "关注数量", value: "following_count" },
  { label: "粉丝数量", value: "follower_count" },
  { label: "文章数量", value: "article_count" },
  { label: "话题数量", value: "topic_count" },
  { label: "评论数量", value: "comment_count" },
  { label: "积分达到", value: "score" }
];

const rules: FormRules = {
  key: [
    { required: true, message: "请输入徽章标识", trigger: "blur" },
    {
      pattern: /^[a-z0-9_.-]+$/,
      message: "只允许小写字母、数字、下划线、点和短横线",
      trigger: "blur"
    }
  ],
  name: [
    { required: true, message: "请输入徽章名称", trigger: "blur" },
    { min: 1, max: 128, message: "名称长度需在 1-128 个字符", trigger: "blur" }
  ],
  iconUrl: [
    {
      pattern: /^$|^https?:\/\/.+/i,
      message: "图标地址必须以 http:// 或 https:// 开头",
      trigger: "blur"
    }
  ],
  ruleType: [{ required: true, message: "请选择授予规则", trigger: "change" }],
  ruleValue: [{ required: true, message: "请输入规则阈值", trigger: "change" }],
  status: [{ required: true, message: "请选择状态", trigger: "change" }]
};

function statusMeta(status?: number) {
  switch (status) {
    case 2:
      return { label: "启用", type: "success" as const };
    case 1:
      return { label: "停用", type: "info" as const };
    default:
      return { label: `未知(${status ?? "-"})`, type: "danger" as const };
  }
}

function ruleTypeOf(row: BadgeRow) {
  return row.rule_type ?? row.ruleType ?? "manual";
}

function ruleValueOf(row: BadgeRow) {
  return row.rule_value ?? row.ruleValue ?? 0;
}

function iconUrlOf(row: BadgeRow) {
  return row.icon_url ?? row.iconUrl ?? "";
}

function ruleLabel(type?: string) {
  return ruleOptions.find(item => item.value === type)?.label ?? type ?? "-";
}

function updatedAt(row: BadgeRow) {
  return row.updated_at ?? row.updatedAt;
}

function formatTime(value?: number) {
  if (!value) return "-";
  const timestamp = value > 9999999999 ? value : value * 1000;
  return dayjs(timestamp).format("YYYY-MM-DD HH:mm");
}

function resetFormModel() {
  form.id = 0;
  form.key = "";
  form.name = "";
  form.description = "";
  form.iconUrl = "";
  form.ruleType = "manual";
  form.ruleValue = 0;
  form.status = 2;
  form.sort = 0;
  formRef.value?.clearValidate();
}

function buildPayload(): AdminBadgePayload {
  return {
    key: form.key.trim(),
    name: form.name.trim(),
    description: form.description.trim(),
    icon_url: form.iconUrl.trim(),
    rule_type: form.ruleType.trim(),
    rule_value: Number(form.ruleValue || 0),
    status: form.status,
    sort: Number(form.sort || 0)
  };
}

async function loadBadges() {
  if (!canList.value) {
    badges.value = [];
    query.total = 0;
    return;
  }
  loading.value = true;
  try {
    const {
      code,
      data,
      message: msg
    } = await listAdminBadges({
      status: query.status,
      limit: query.pageSize,
      offset: (query.currentPage - 1) * query.pageSize
    });
    if (code !== 0) {
      message(msg || "加载徽章列表失败", { type: "error" });
      return;
    }
    badges.value = data.items ?? [];
    query.total = data.total ?? badges.value.length;
  } finally {
    loading.value = false;
  }
}

function resetQuery() {
  query.status = 0;
  query.currentPage = 1;
  loadBadges();
}

function openCreateDialog() {
  if (!canCreate.value) {
    message("没有新增徽章权限", { type: "warning" });
    return;
  }
  dialogMode.value = "create";
  resetFormModel();
  dialogVisible.value = true;
}

function openEditDialog(row: BadgeRow) {
  if (!canUpdate.value) {
    message("没有修改徽章权限", { type: "warning" });
    return;
  }
  dialogMode.value = "edit";
  form.id = normalizeEntityId(row.id) ?? 0;
  form.key = row.key ?? "";
  form.name = row.name ?? "";
  form.description = row.description ?? "";
  form.iconUrl = iconUrlOf(row);
  form.ruleType = ruleTypeOf(row);
  form.ruleValue = Number(ruleValueOf(row));
  form.status = Number(row.status ?? 2);
  form.sort = Number(row.sort ?? 0);
  formRef.value?.clearValidate();
  dialogVisible.value = true;
}

async function saveBadge() {
  if (dialogMode.value === "create" && !canCreate.value) {
    message("没有新增徽章权限", { type: "warning" });
    return;
  }
  if (dialogMode.value === "edit" && !canUpdate.value) {
    message("没有修改徽章权限", { type: "warning" });
    return;
  }
  const valid = await formRef.value?.validate().catch(() => false);
  if (!valid) return;
  const payload = buildPayload();
  saving.value = true;
  try {
    const { code, message: msg } =
      dialogMode.value === "create"
        ? await createAdminBadge(payload)
        : await updateAdminBadge(form.id, payload);
    if (code !== 0) {
      message(msg || "保存徽章失败", { type: "error" });
      return;
    }
    message("徽章已保存", { type: "success" });
    dialogVisible.value = false;
    await loadBadges();
  } finally {
    saving.value = false;
  }
}

async function handleDelete(row: BadgeRow) {
  if (!canDelete.value) {
    message("没有删除徽章权限", { type: "warning" });
    return;
  }
  const id = normalizeEntityId(row.id);
  if (!id) {
    message("徽章 ID 无效", { type: "warning" });
    return;
  }
  const confirmed = await ElMessageBox.confirm(
    `确认删除徽章「${row.name || id}」？`,
    "删除徽章",
    {
      type: "warning",
      confirmButtonText: "确认",
      cancelButtonText: "取消"
    }
  ).catch(() => false);
  if (!confirmed) return;
  loading.value = true;
  try {
    const { code, message: msg } = await deleteAdminBadge(id);
    if (code !== 0) {
      message(msg || "删除徽章失败", { type: "error" });
      return;
    }
    message("徽章已删除", { type: "success" });
    await loadBadges();
  } finally {
    loading.value = false;
  }
}

function onPageSizeChange(size: number) {
  query.pageSize = size;
  query.currentPage = 1;
  loadBadges();
}

function onCurrentPageChange(page: number) {
  query.currentPage = page;
  loadBadges();
}

onMounted(loadBadges);
</script>

<template>
  <div class="badges-page">
    <section class="governance-panel">
      <div class="panel-header">
        <div>
          <h2>徽章管理</h2>
          <p>配置社区用户徽章、授予规则和成长体系展示状态</p>
        </div>
        <el-button
          type="primary"
          :icon="useRenderIcon('ri/add-line')"
          :disabled="!canCreate"
          @click="openCreateDialog"
        >
          新增徽章
        </el-button>
      </div>

      <el-alert
        v-if="!canList"
        title="当前账号没有 governance:list_badges 权限"
        type="warning"
        show-icon
        :closable="false"
        class="permission-alert"
      />

      <el-form :inline="true" class="search-form">
        <el-form-item label="状态">
          <el-select v-model="query.status" class="w-36!" @change="loadBadges">
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
            @click="loadBadges"
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
        :data="badges"
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
        <template #icon="{ row }">
          <el-avatar
            v-if="iconUrlOf(row)"
            :size="32"
            shape="square"
            :src="iconUrlOf(row)"
          />
          <el-avatar v-else :size="32" shape="square">徽</el-avatar>
        </template>
        <template #rule="{ row }">
          <el-tag effect="plain">
            {{ ruleLabel(ruleTypeOf(row)) }} / {{ ruleValueOf(row) }}
          </el-tag>
        </template>
        <template #status="{ row }">
          <el-tag :type="statusMeta(row.status).type">
            {{ statusMeta(row.status).label }}
          </el-tag>
        </template>
        <template #updatedAt="{ row }">
          {{ formatTime(updatedAt(row)) }}
        </template>
        <template #operation="{ row }">
          <el-button
            link
            type="primary"
            :disabled="!canUpdate"
            :icon="useRenderIcon('ri/edit-line')"
            @click="openEditDialog(row)"
          >
            编辑
          </el-button>
          <el-button
            link
            type="danger"
            :disabled="!canDelete"
            :icon="useRenderIcon('ri/delete-bin-line')"
            @click="handleDelete(row)"
          >
            删除
          </el-button>
        </template>
      </pure-table>
    </section>

    <el-dialog
      v-model="dialogVisible"
      :title="dialogTitle"
      width="620px"
      destroy-on-close
    >
      <el-form
        ref="formRef"
        :model="form"
        :rules="rules"
        label-width="92px"
        class="badge-form"
      >
        <el-form-item label="徽章标识" prop="key">
          <el-input
            v-model="form.key"
            maxlength="64"
            show-word-limit
            placeholder="例如 community-member"
          />
        </el-form-item>
        <el-form-item label="徽章名称" prop="name">
          <el-input
            v-model="form.name"
            maxlength="128"
            show-word-limit
            placeholder="请输入徽章名称"
          />
        </el-form-item>
        <el-form-item label="图标地址" prop="iconUrl">
          <el-input
            v-model="form.iconUrl"
            placeholder="https://example.com/badge.png"
          />
        </el-form-item>
        <el-form-item label="授予规则" prop="ruleType">
          <el-select v-model="form.ruleType" class="w-full!">
            <el-option
              v-for="item in ruleOptions"
              :key="item.value"
              :label="item.label"
              :value="item.value"
            />
          </el-select>
        </el-form-item>
        <el-form-item label="规则阈值" prop="ruleValue">
          <el-input-number v-model="form.ruleValue" :min="0" :max="999999" />
        </el-form-item>
        <el-form-item label="状态" prop="status">
          <el-radio-group v-model="form.status">
            <el-radio-button :value="2">启用</el-radio-button>
            <el-radio-button :value="1">停用</el-radio-button>
          </el-radio-group>
        </el-form-item>
        <el-form-item label="排序">
          <el-input-number v-model="form.sort" :min="0" :max="999999" />
        </el-form-item>
        <el-form-item label="说明">
          <el-input
            v-model="form.description"
            type="textarea"
            :rows="3"
            maxlength="240"
            show-word-limit
            placeholder="徽章含义、授予条件或展示口径"
          />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="dialogVisible = false">取消</el-button>
        <el-button type="primary" :loading="saving" @click="saveBadge">
          保存
        </el-button>
      </template>
    </el-dialog>
  </div>
</template>

<style scoped>
.badges-page {
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

.badge-form {
  padding-right: 10px;
}

@media (width <= 768px) {
  .panel-header {
    flex-direction: column;
  }
}
</style>
