<script setup lang="ts">
import dayjs from "dayjs";
import { computed, onMounted, reactive, ref } from "vue";
import { type FormInstance, type FormRules } from "element-plus";
import { message } from "@/utils/message";
import { hasPerms } from "@/utils/auth";
import {
  listAdminSettings,
  updateAdminSetting,
  type AdminSetting,
  type AdminSettingPayload
} from "@/api/admin";
import { useRenderIcon } from "@/components/ReIcon/src/hooks";

defineOptions({
  name: "GovernanceSettings"
});

type SettingRow = Partial<AdminSetting> & Record<string, any>;
type DialogMode = "create" | "edit";

const loading = ref(false);
const saving = ref(false);
const dialogVisible = ref(false);
const dialogMode = ref<DialogMode>("edit");
const formRef = ref<FormInstance>();
const settings = ref<AdminSetting[]>([]);

const query = reactive({
  group: "",
  status: 0,
  pageSize: 20,
  currentPage: 1,
  total: 0
});

const form = reactive({
  key: "",
  value: "",
  group: "site",
  valueType: "string",
  description: "",
  status: 2
});

const canList = computed(() => hasPerms("governance:list_settings"));
const canUpdate = computed(() => hasPerms("governance:update_setting"));
const dialogTitle = computed(() =>
  dialogMode.value === "create" ? "新增配置" : "编辑配置"
);

const columns: TableColumnList = [
  { prop: "key", label: "配置键", minWidth: 180, showOverflowTooltip: true },
  { label: "分组", width: 110, slot: "group" },
  { label: "类型", width: 100, slot: "valueType" },
  { prop: "value", label: "配置值", minWidth: 260, showOverflowTooltip: true },
  {
    prop: "description",
    label: "说明",
    minWidth: 220,
    showOverflowTooltip: true
  },
  { label: "状态", width: 100, slot: "status" },
  { label: "更新时间", width: 170, slot: "updatedAt" },
  { label: "操作", fixed: "right", width: 110, slot: "operation" }
];

const groupOptions = [
  { label: "全部", value: "" },
  { label: "站点", value: "site" },
  { label: "SEO", value: "seo" },
  { label: "上传", value: "upload" },
  { label: "邮件", value: "email" },
  { label: "安全", value: "security" },
  { label: "运营", value: "operation" }
];

const statusOptions = [
  { label: "全部", value: 0 },
  { label: "启用", value: 2 },
  { label: "停用", value: 1 }
];

const valueTypeOptions = [
  { label: "字符串", value: "string" },
  { label: "数字", value: "int" },
  { label: "布尔", value: "bool" },
  { label: "JSON", value: "json" },
  { label: "文本", value: "text" }
];

const rules: FormRules = {
  key: [
    { required: true, message: "请输入配置键", trigger: "blur" },
    {
      pattern: /^[a-z0-9_.-]+$/,
      message: "只允许小写字母、数字、下划线、点和短横线",
      trigger: "blur"
    }
  ],
  group: [{ required: true, message: "请选择分组", trigger: "change" }],
  valueType: [{ required: true, message: "请选择类型", trigger: "change" }],
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

function groupLabel(group?: string) {
  return groupOptions.find(item => item.value === group)?.label ?? group ?? "-";
}

function valueTypeOf(row: SettingRow) {
  return row.value_type ?? row.valueType ?? "string";
}

function valueTypeLabel(type?: string) {
  return (
    valueTypeOptions.find(item => item.value === type)?.label ?? type ?? "-"
  );
}

function updatedAt(row: SettingRow) {
  return row.updated_at ?? row.updatedAt;
}

function formatTime(value?: number) {
  if (!value) return "-";
  const timestamp = value > 9999999999 ? value : value * 1000;
  return dayjs(timestamp).format("YYYY-MM-DD HH:mm");
}

function resetFormModel() {
  form.key = "";
  form.value = "";
  form.group = "site";
  form.valueType = "string";
  form.description = "";
  form.status = 2;
  formRef.value?.clearValidate();
}

function buildPayload(): AdminSettingPayload {
  return {
    key: form.key.trim(),
    value: form.value.trim(),
    group: form.group.trim(),
    value_type: form.valueType.trim(),
    description: form.description.trim(),
    status: form.status
  };
}

async function loadSettings() {
  if (!canList.value) {
    settings.value = [];
    query.total = 0;
    return;
  }
  loading.value = true;
  try {
    const {
      code,
      data,
      message: msg
    } = await listAdminSettings({
      group: query.group,
      status: query.status,
      limit: query.pageSize,
      offset: (query.currentPage - 1) * query.pageSize
    });
    if (code !== 0) {
      message(msg || "加载站点设置失败", { type: "error" });
      return;
    }
    settings.value = data.items ?? [];
    query.total = data.total ?? settings.value.length;
  } finally {
    loading.value = false;
  }
}

function resetQuery() {
  query.group = "";
  query.status = 0;
  query.currentPage = 1;
  loadSettings();
}

function openCreateDialog() {
  if (!canUpdate.value) {
    message("没有维护站点设置权限", { type: "warning" });
    return;
  }
  dialogMode.value = "create";
  resetFormModel();
  dialogVisible.value = true;
}

function openEditDialog(row: SettingRow) {
  if (!canUpdate.value) {
    message("没有维护站点设置权限", { type: "warning" });
    return;
  }
  dialogMode.value = "edit";
  form.key = row.key ?? "";
  form.value = row.value ?? "";
  form.group = row.group ?? "site";
  form.valueType = valueTypeOf(row);
  form.description = row.description ?? "";
  form.status = Number(row.status ?? 2);
  formRef.value?.clearValidate();
  dialogVisible.value = true;
}

async function saveSetting() {
  if (!canUpdate.value) {
    message("没有维护站点设置权限", { type: "warning" });
    return;
  }
  const valid = await formRef.value?.validate().catch(() => false);
  if (!valid) return;
  const payload = buildPayload();
  if (!payload.key) {
    message("请输入配置键", { type: "warning" });
    return;
  }
  saving.value = true;
  try {
    const { code, message: msg } = await updateAdminSetting(
      payload.key,
      payload
    );
    if (code !== 0) {
      message(msg || "保存站点设置失败", { type: "error" });
      return;
    }
    message("站点设置已保存", { type: "success" });
    dialogVisible.value = false;
    await loadSettings();
  } finally {
    saving.value = false;
  }
}

function onPageSizeChange(size: number) {
  query.pageSize = size;
  query.currentPage = 1;
  loadSettings();
}

function onCurrentPageChange(page: number) {
  query.currentPage = page;
  loadSettings();
}

onMounted(loadSettings);
</script>

<template>
  <div class="settings-page">
    <section class="governance-panel">
      <div class="panel-header">
        <div>
          <h2>站点设置</h2>
          <p>维护站点基础信息、SEO、上传和邮件等运行配置</p>
        </div>
        <el-button
          type="primary"
          :icon="useRenderIcon('ri/add-line')"
          :disabled="!canUpdate"
          @click="openCreateDialog"
        >
          新增配置
        </el-button>
      </div>

      <el-alert
        v-if="!canList"
        title="当前账号没有 governance:list_settings 权限"
        type="warning"
        show-icon
        :closable="false"
        class="permission-alert"
      />

      <el-form :inline="true" class="search-form">
        <el-form-item label="分组">
          <el-select v-model="query.group" class="w-36!" @change="loadSettings">
            <el-option
              v-for="item in groupOptions"
              :key="item.value"
              :label="item.label"
              :value="item.value"
            />
          </el-select>
        </el-form-item>
        <el-form-item label="状态">
          <el-select
            v-model="query.status"
            class="w-36!"
            @change="loadSettings"
          >
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
            @click="loadSettings"
          >
            查询
          </el-button>
          <el-button :icon="useRenderIcon('ep/refresh')" @click="resetQuery">
            重置
          </el-button>
        </el-form-item>
      </el-form>

      <pure-table
        row-key="key"
        adaptive
        :adaptiveConfig="{ offsetBottom: 156 }"
        align-whole="center"
        table-layout="auto"
        :loading="loading"
        :data="settings"
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
        <template #group="{ row }">
          <el-tag effect="plain">{{ groupLabel(row.group) }}</el-tag>
        </template>
        <template #valueType="{ row }">
          {{ valueTypeLabel(valueTypeOf(row)) }}
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
        class="setting-form"
      >
        <el-form-item label="配置键" prop="key">
          <el-input
            v-model="form.key"
            :disabled="dialogMode === 'edit'"
            placeholder="例如 site_name"
          />
        </el-form-item>
        <el-form-item label="分组" prop="group">
          <el-select v-model="form.group" class="w-full!">
            <el-option
              v-for="item in groupOptions.filter(item => item.value)"
              :key="item.value"
              :label="item.label"
              :value="item.value"
            />
          </el-select>
        </el-form-item>
        <el-form-item label="类型" prop="valueType">
          <el-select v-model="form.valueType" class="w-full!">
            <el-option
              v-for="item in valueTypeOptions"
              :key="item.value"
              :label="item.label"
              :value="item.value"
            />
          </el-select>
        </el-form-item>
        <el-form-item label="配置值">
          <el-input
            v-model="form.value"
            type="textarea"
            :rows="
              form.valueType === 'json' || form.valueType === 'text' ? 6 : 3
            "
            placeholder="请输入配置值"
          />
        </el-form-item>
        <el-form-item label="状态" prop="status">
          <el-radio-group v-model="form.status">
            <el-radio-button :value="2">启用</el-radio-button>
            <el-radio-button :value="1">停用</el-radio-button>
          </el-radio-group>
        </el-form-item>
        <el-form-item label="说明">
          <el-input
            v-model="form.description"
            type="textarea"
            :rows="3"
            maxlength="240"
            show-word-limit
            placeholder="配置用途、取值说明或影响范围"
          />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="dialogVisible = false">取消</el-button>
        <el-button type="primary" :loading="saving" @click="saveSetting">
          保存
        </el-button>
      </template>
    </el-dialog>
  </div>
</template>

<style scoped>
.settings-page {
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

.setting-form {
  padding-right: 10px;
}

@media (width <= 768px) {
  .panel-header {
    flex-direction: column;
  }
}
</style>
