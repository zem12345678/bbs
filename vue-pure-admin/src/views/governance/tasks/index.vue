<script setup lang="ts">
import dayjs from "dayjs";
import { computed, onMounted, reactive, ref } from "vue";
import { ElMessageBox, type FormInstance, type FormRules } from "element-plus";
import { message } from "@/utils/message";
import { hasPerms } from "@/utils/auth";
import {
  createAdminTask,
  deleteAdminTask,
  listAdminTasks,
  updateAdminTask,
  type AdminTask,
  type AdminTaskPayload
} from "@/api/admin";
import { normalizeEntityId } from "@/utils/entityId";
import { useRenderIcon } from "@/components/ReIcon/src/hooks";

defineOptions({
  name: "GovernanceTasks"
});

type TaskRow = Partial<AdminTask> & Record<string, any>;
type DialogMode = "create" | "edit";

const taskKeyOptions = [
  { label: "每日签到", value: "daily_check_in" },
  { label: "发布第一条话题", value: "first_topic" }
];

const loading = ref(false);
const saving = ref(false);
const dialogVisible = ref(false);
const dialogMode = ref<DialogMode>("create");
const formRef = ref<FormInstance>();
const tasks = ref<AdminTask[]>([]);

const query = reactive({
  status: 0,
  pageSize: 20,
  currentPage: 1,
  total: 0
});

const form = reactive({
  id: 0,
  key: "daily_check_in",
  title: "",
  description: "",
  rewardPoints: 0,
  status: 2,
  sort: 0
});

const canList = computed(() => hasPerms("governance:list_tasks"));
const canCreate = computed(() => hasPerms("governance:create_task"));
const canUpdate = computed(() => hasPerms("governance:update_task"));
const canDelete = computed(() => hasPerms("governance:delete_task"));

const dialogTitle = computed(() =>
  dialogMode.value === "create" ? "新增任务" : "编辑任务"
);

const visibleTaskKeyOptions = computed(() => {
  if (!form.key || taskKeyOptions.some(item => item.value === form.key)) {
    return taskKeyOptions;
  }
  return [
    { label: `未接入：${form.key}`, value: form.key },
    ...taskKeyOptions
  ];
});

const columns: TableColumnList = [
  { prop: "id", label: "ID", width: 90 },
  { prop: "key", label: "任务标识", minWidth: 150, showOverflowTooltip: true },
  {
    prop: "title",
    label: "任务名称",
    minWidth: 180,
    showOverflowTooltip: true
  },
  {
    prop: "description",
    label: "说明",
    minWidth: 260,
    showOverflowTooltip: true
  },
  { label: "奖励积分", width: 110, slot: "rewardPoints" },
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

const rules: FormRules = {
  key: [
    { required: true, message: "请选择任务类型", trigger: "change" }
  ],
  title: [
    { required: true, message: "请输入任务名称", trigger: "blur" },
    { min: 1, max: 128, message: "名称长度需在 1-128 个字符", trigger: "blur" }
  ],
  rewardPoints: [
    { required: true, message: "请输入奖励积分", trigger: "change" }
  ],
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

function rewardPointsOf(row: TaskRow) {
  return row.reward_points ?? row.rewardPoints ?? 0;
}

function updatedAt(row: TaskRow) {
  return row.updated_at ?? row.updatedAt;
}

function formatTime(value?: number) {
  if (!value) return "-";
  const timestamp = value > 9999999999 ? value : value * 1000;
  return dayjs(timestamp).format("YYYY-MM-DD HH:mm");
}

function resetFormModel() {
  form.id = 0;
  form.key = "daily_check_in";
  form.title = "";
  form.description = "";
  form.rewardPoints = 0;
  form.status = 2;
  form.sort = 0;
  formRef.value?.clearValidate();
}

function buildPayload(): AdminTaskPayload {
  return {
    key: form.key.trim(),
    title: form.title.trim(),
    description: form.description.trim(),
    reward_points: Number(form.rewardPoints || 0),
    status: form.status,
    sort: Number(form.sort || 0)
  };
}

async function loadTasks() {
  if (!canList.value) {
    tasks.value = [];
    query.total = 0;
    return;
  }
  loading.value = true;
  try {
    const {
      code,
      data,
      message: msg
    } = await listAdminTasks({
      status: query.status,
      limit: query.pageSize,
      offset: (query.currentPage - 1) * query.pageSize
    });
    if (code !== 0) {
      message(msg || "加载任务列表失败", { type: "error" });
      return;
    }
    tasks.value = data.items ?? [];
    query.total = data.total ?? tasks.value.length;
  } finally {
    loading.value = false;
  }
}

function resetQuery() {
  query.status = 0;
  query.currentPage = 1;
  loadTasks();
}

function openCreateDialog() {
  if (!canCreate.value) {
    message("没有新增任务权限", { type: "warning" });
    return;
  }
  dialogMode.value = "create";
  resetFormModel();
  dialogVisible.value = true;
}

function openEditDialog(row: TaskRow) {
  if (!canUpdate.value) {
    message("没有修改任务权限", { type: "warning" });
    return;
  }
  dialogMode.value = "edit";
  form.id = Number(row.id ?? 0);
  form.key = row.key ?? "";
  form.title = row.title ?? "";
  form.description = row.description ?? "";
  form.rewardPoints = Number(rewardPointsOf(row));
  form.status = Number(row.status ?? 2);
  form.sort = Number(row.sort ?? 0);
  formRef.value?.clearValidate();
  dialogVisible.value = true;
}

async function saveTask() {
  if (dialogMode.value === "create" && !canCreate.value) {
    message("没有新增任务权限", { type: "warning" });
    return;
  }
  if (dialogMode.value === "edit" && !canUpdate.value) {
    message("没有修改任务权限", { type: "warning" });
    return;
  }
  const valid = await formRef.value?.validate().catch(() => false);
  if (!valid) return;
  const payload = buildPayload();
  saving.value = true;
  try {
    const { code, message: msg } =
      dialogMode.value === "create"
        ? await createAdminTask(payload)
        : await updateAdminTask(form.id, payload);
    if (code !== 0) {
      message(msg || "保存任务失败", { type: "error" });
      return;
    }
    message("任务已保存", { type: "success" });
    dialogVisible.value = false;
    await loadTasks();
  } finally {
    saving.value = false;
  }
}

async function handleDelete(row: TaskRow) {
  if (!canDelete.value) {
    message("没有删除任务权限", { type: "warning" });
    return;
  }
  const id = normalizeEntityId(row.id);
  if (!id) {
    message("任务 ID 无效", { type: "warning" });
    return;
  }
  const confirmed = await ElMessageBox.confirm(
    `确认删除任务「${row.title || id}」？`,
    "删除任务",
    {
      type: "warning",
      confirmButtonText: "确认",
      cancelButtonText: "取消"
    }
  ).catch(() => false);
  if (!confirmed) return;
  loading.value = true;
  try {
    const { code, message: msg } = await deleteAdminTask(id);
    if (code !== 0) {
      message(msg || "删除任务失败", { type: "error" });
      return;
    }
    message("任务已删除", { type: "success" });
    await loadTasks();
  } finally {
    loading.value = false;
  }
}

function onPageSizeChange(size: number) {
  query.pageSize = size;
  query.currentPage = 1;
  loadTasks();
}

function onCurrentPageChange(page: number) {
  query.currentPage = page;
  loadTasks();
}

onMounted(loadTasks);
</script>

<template>
  <div class="tasks-page">
    <section class="governance-panel">
      <div class="panel-header">
        <div>
          <h2>任务管理</h2>
          <p>配置已接入任务的积分奖励和前台展示状态</p>
        </div>
        <el-button
          type="primary"
          :icon="useRenderIcon('ri/add-line')"
          :disabled="!canCreate"
          @click="openCreateDialog"
        >
          新增任务
        </el-button>
      </div>

      <el-alert
        v-if="!canList"
        title="当前账号没有 governance:list_tasks 权限"
        type="warning"
        show-icon
        :closable="false"
        class="permission-alert"
      />

      <el-form :inline="true" class="search-form">
        <el-form-item label="状态">
          <el-select v-model="query.status" class="w-36!" @change="loadTasks">
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
            @click="loadTasks"
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
        :data="tasks"
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
        <template #rewardPoints="{ row }">
          <el-tag type="warning" effect="plain">
            +{{ rewardPointsOf(row) }}
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
        class="task-form"
      >
        <el-form-item label="任务标识" prop="key">
          <el-select
            v-model="form.key"
            placeholder="请选择任务类型"
          >
            <el-option
              v-for="item in visibleTaskKeyOptions"
              :key="item.value"
              :label="item.label"
              :value="item.value"
            />
          </el-select>
        </el-form-item>
        <el-form-item label="任务名称" prop="title">
          <el-input
            v-model="form.title"
            maxlength="128"
            show-word-limit
            placeholder="请输入任务名称"
          />
        </el-form-item>
        <el-form-item label="奖励积分" prop="rewardPoints">
          <el-input-number v-model="form.rewardPoints" :min="1" :max="999999" />
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
            placeholder="任务完成条件、触发来源或展示说明"
          />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="dialogVisible = false">取消</el-button>
        <el-button type="primary" :loading="saving" @click="saveTask">
          保存
        </el-button>
      </template>
    </el-dialog>
  </div>
</template>

<style scoped>
.tasks-page {
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

.task-form {
  padding-right: 10px;
}

@media (width <= 768px) {
  .panel-header {
    flex-direction: column;
  }
}
</style>
