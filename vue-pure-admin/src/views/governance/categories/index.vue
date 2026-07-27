<script setup lang="ts">
import dayjs from "dayjs";
import { computed, onMounted, reactive, ref } from "vue";
import { ElMessageBox, type FormInstance, type FormRules } from "element-plus";
import { message } from "@/utils/message";
import { hasPerms } from "@/utils/auth";
import {
  createAdminCategory,
  deleteAdminCategory,
  listAdminCategories,
  updateAdminCategory,
  type AdminCategory,
  type AdminCategoryPayload
} from "@/api/admin";
import { normalizeEntityId, type EntityId } from "@/utils/entityId";
import { useRenderIcon } from "@/components/ReIcon/src/hooks";

defineOptions({
  name: "GovernanceCategories"
});

type CategoryRow = Partial<AdminCategory> & Record<string, any>;
type DialogMode = "create" | "edit";

const loading = ref(false);
const saving = ref(false);
const dialogVisible = ref(false);
const dialogMode = ref<DialogMode>("create");
const formRef = ref<FormInstance>();
const categories = ref<AdminCategory[]>([]);

const query = reactive({
  status: 0,
  pageSize: 20,
  currentPage: 1,
  total: 0
});

const form = reactive({
  id: 0 as EntityId,
  slug: "",
  name: "",
  description: "",
  status: 2,
  sort: 0
});

const canList = computed(() => hasPerms("governance:list_categories"));
const canCreate = computed(() => hasPerms("governance:create_category"));
const canUpdate = computed(() => hasPerms("governance:update_category"));
const canDelete = computed(() => hasPerms("governance:delete_category"));

const dialogTitle = computed(() =>
  dialogMode.value === "create" ? "新增分类" : "编辑分类"
);

const columns: TableColumnList = [
  { prop: "id", label: "ID", width: 90 },
  { prop: "slug", label: "分类标识", minWidth: 150, showOverflowTooltip: true },
  { prop: "name", label: "分类名称", minWidth: 180, showOverflowTooltip: true },
  {
    prop: "description",
    label: "说明",
    minWidth: 260,
    showOverflowTooltip: true
  },
  { label: "话题数", width: 100, slot: "topicCount" },
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
  slug: [
    { required: true, message: "请输入分类标识", trigger: "blur" },
    {
      pattern: /^[a-z0-9_.-]+$/,
      message: "只允许小写字母、数字、下划线、点和短横线",
      trigger: "blur"
    }
  ],
  name: [
    { required: true, message: "请输入分类名称", trigger: "blur" },
    { min: 1, max: 80, message: "名称长度需在 1-80 个字符", trigger: "blur" }
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

function topicCountOf(row: CategoryRow) {
  return Number(row.topic_count ?? row.topicCount ?? 0);
}

function updatedAt(row: CategoryRow) {
  return row.updated_at ?? row.updatedAt;
}

function formatTime(value?: number) {
  if (!value) return "-";
  const timestamp = value > 9999999999 ? value : value * 1000;
  return dayjs(timestamp).format("YYYY-MM-DD HH:mm");
}

function resetFormModel() {
  form.id = 0;
  form.slug = "";
  form.name = "";
  form.description = "";
  form.status = 2;
  form.sort = 0;
  formRef.value?.clearValidate();
}

function buildPayload(): AdminCategoryPayload {
  return {
    slug: form.slug.trim(),
    name: form.name.trim(),
    description: form.description.trim(),
    status: form.status,
    sort: Number(form.sort || 0)
  };
}

async function loadCategories() {
  if (!canList.value) {
    categories.value = [];
    query.total = 0;
    return;
  }
  loading.value = true;
  try {
    const {
      code,
      data,
      message: msg
    } = await listAdminCategories({
      status: query.status,
      limit: query.pageSize,
      offset: (query.currentPage - 1) * query.pageSize
    });
    if (code !== 0) {
      message(msg || "加载分类列表失败", { type: "error" });
      return;
    }
    categories.value = data.items ?? [];
    query.total = data.total ?? categories.value.length;
  } finally {
    loading.value = false;
  }
}

function resetQuery() {
  query.status = 0;
  query.currentPage = 1;
  loadCategories();
}

function openCreateDialog() {
  if (!canCreate.value) {
    message("没有新增分类权限", { type: "warning" });
    return;
  }
  dialogMode.value = "create";
  resetFormModel();
  dialogVisible.value = true;
}

function openEditDialog(row: CategoryRow) {
  if (!canUpdate.value) {
    message("没有修改分类权限", { type: "warning" });
    return;
  }
  dialogMode.value = "edit";
  form.id = normalizeEntityId(row.id) ?? 0;
  form.slug = row.slug ?? "";
  form.name = row.name ?? "";
  form.description = row.description ?? "";
  form.status = Number(row.status ?? 2);
  form.sort = Number(row.sort ?? 0);
  formRef.value?.clearValidate();
  dialogVisible.value = true;
}

async function saveCategory() {
  if (dialogMode.value === "create" && !canCreate.value) {
    message("没有新增分类权限", { type: "warning" });
    return;
  }
  if (dialogMode.value === "edit" && !canUpdate.value) {
    message("没有修改分类权限", { type: "warning" });
    return;
  }
  const valid = await formRef.value?.validate().catch(() => false);
  if (!valid) return;
  const payload = buildPayload();
  saving.value = true;
  try {
    const { code, message: msg } =
      dialogMode.value === "create"
        ? await createAdminCategory(payload)
        : await updateAdminCategory(form.id, payload);
    if (code !== 0) {
      message(msg || "保存分类失败", { type: "error" });
      return;
    }
    message("分类已保存", { type: "success" });
    dialogVisible.value = false;
    await loadCategories();
  } finally {
    saving.value = false;
  }
}

async function handleDelete(row: CategoryRow) {
  if (!canDelete.value) {
    message("没有删除分类权限", { type: "warning" });
    return;
  }
  if (topicCountOf(row) > 0) {
    message("已有话题绑定的分类不能删除，可先停用", { type: "warning" });
    return;
  }
  const id = normalizeEntityId(row.id);
  if (!id) {
    message("分类 ID 无效", { type: "warning" });
    return;
  }
  const confirmed = await ElMessageBox.confirm(
    `确认删除分类「${row.name || id}」？`,
    "删除分类",
    {
      type: "warning",
      confirmButtonText: "确认",
      cancelButtonText: "取消"
    }
  ).catch(() => false);
  if (!confirmed) return;
  loading.value = true;
  try {
    const { code, message: msg } = await deleteAdminCategory(id);
    if (code !== 0) {
      message(msg || "删除分类失败", { type: "error" });
      return;
    }
    message("分类已删除", { type: "success" });
    await loadCategories();
  } finally {
    loading.value = false;
  }
}

function onPageSizeChange(size: number) {
  query.pageSize = size;
  query.currentPage = 1;
  loadCategories();
}

function onCurrentPageChange(page: number) {
  query.currentPage = page;
  loadCategories();
}

onMounted(loadCategories);
</script>

<template>
  <div class="categories-page">
    <section class="governance-panel">
      <div class="panel-header">
        <div>
          <h2>分类管理</h2>
          <p>维护社区话题分类、展示排序和发布入口状态</p>
        </div>
        <el-button
          type="primary"
          :icon="useRenderIcon('ri/add-line')"
          :disabled="!canCreate"
          @click="openCreateDialog"
        >
          新增分类
        </el-button>
      </div>

      <el-alert
        v-if="!canList"
        title="当前账号没有 governance:list_categories 权限"
        type="warning"
        show-icon
        :closable="false"
        class="permission-alert"
      />

      <el-form :inline="true" class="search-form">
        <el-form-item label="状态">
          <el-select
            v-model="query.status"
            class="w-36!"
            @change="loadCategories"
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
            @click="loadCategories"
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
        :data="categories"
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
        <template #topicCount="{ row }">
          <el-tag effect="plain">{{ topicCountOf(row) }}</el-tag>
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
            :disabled="!canDelete || topicCountOf(row) > 0"
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
        class="category-form"
      >
        <el-form-item label="分类标识" prop="slug">
          <el-input
            v-model="form.slug"
            maxlength="64"
            show-word-limit
            placeholder="例如 engineering"
          />
        </el-form-item>
        <el-form-item label="分类名称" prop="name">
          <el-input
            v-model="form.name"
            maxlength="80"
            show-word-limit
            placeholder="请输入分类名称"
          />
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
            placeholder="分类用途、内容边界或运营说明"
          />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="dialogVisible = false">取消</el-button>
        <el-button type="primary" :loading="saving" @click="saveCategory">
          保存
        </el-button>
      </template>
    </el-dialog>
  </div>
</template>

<style scoped>
.categories-page {
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

.category-form {
  padding-right: 10px;
}

@media (width <= 768px) {
  .panel-header {
    flex-direction: column;
  }
}
</style>
