<script setup lang="ts">
import dayjs from "dayjs";
import { computed, onMounted, reactive, ref } from "vue";
import { type FormInstance, type FormRules } from "element-plus";
import { message } from "@/utils/message";
import { hasPerms } from "@/utils/auth";
import { normalizeEntityId } from "@/utils/entityId";
import { useRenderIcon } from "@/components/ReIcon/src/hooks";
import {
  createAdminMallProductCategory,
  listAdminMallProductCategories,
  updateAdminMallProductCategory,
  type AdminMallProductCategory,
  type AdminMallProductCategoryPayload
} from "@/api/admin";

defineOptions({
  name: "MallCategories"
});

type CategoryRow = Partial<AdminMallProductCategory> & Record<string, any>;
type DialogMode = "create" | "edit";

const loading = ref(false);
const saving = ref(false);
const dialogVisible = ref(false);
const dialogMode = ref<DialogMode>("create");
const formRef = ref<FormInstance>();
const categories = ref<AdminMallProductCategory[]>([]);

const query = reactive({
  keyword: "",
  status: 0,
  pageSize: 20,
  currentPage: 1,
  total: 0
});

const form = reactive({
  id: 0,
  slug: "",
  name: "",
  description: "",
  status: 2,
  sort: 0
});

const canList = computed(() => hasPerms("mall:list_product_categories"));
const canCreate = computed(() => hasPerms("mall:create_product_category"));
const canUpdate = computed(() => hasPerms("mall:update_product_category"));

const dialogTitle = computed(() =>
  dialogMode.value === "create" ? "新增商品分类" : "编辑商品分类"
);

const statusOptions = [
  { label: "全部", value: 0 },
  { label: "草稿", value: 1 },
  { label: "启用", value: 2 },
  { label: "归档", value: 3 }
];

const editableStatusOptions = statusOptions.filter(item => item.value > 0);

const columns: TableColumnList = [
  { prop: "id", label: "ID", width: 90 },
  { prop: "name", label: "分类名称", minWidth: 160, showOverflowTooltip: true },
  { prop: "slug", label: "标识", minWidth: 150, showOverflowTooltip: true },
  {
    prop: "description",
    label: "说明",
    minWidth: 240,
    showOverflowTooltip: true
  },
  { label: "商品数", width: 100, slot: "productCount" },
  { label: "状态", width: 100, slot: "status" },
  { prop: "sort", label: "排序", width: 90 },
  { label: "更新时间", width: 170, slot: "updatedAt" },
  { label: "操作", fixed: "right", width: 110, slot: "operation" }
];

const rules: FormRules = {
  slug: [
    { required: true, message: "请输入分类标识", trigger: "blur" },
    {
      pattern: /^[a-z0-9_.-]+$/,
      message: "标识只允许小写字母、数字、下划线、点和短横线",
      trigger: "blur"
    },
    { min: 1, max: 64, message: "标识长度需在 1-64 个字符", trigger: "blur" }
  ],
  name: [
    { required: true, message: "请输入分类名称", trigger: "blur" },
    { min: 1, max: 80, message: "名称长度需在 1-80 个字符", trigger: "blur" }
  ],
  status: [{ required: true, message: "请选择状态", trigger: "change" }]
};

function statusCode(value?: number | string) {
  if (typeof value === "number") return value;
  switch (String(value || "").toUpperCase()) {
    case "PRODUCT_CATEGORY_STATUS_DRAFT":
    case "DRAFT":
      return 1;
    case "PRODUCT_CATEGORY_STATUS_ACTIVE":
    case "ACTIVE":
      return 2;
    case "PRODUCT_CATEGORY_STATUS_ARCHIVED":
    case "ARCHIVED":
      return 3;
    default:
      return 0;
  }
}

function statusMeta(value?: number | string) {
  switch (statusCode(value)) {
    case 1:
      return { label: "草稿", type: "info" as const };
    case 2:
      return { label: "启用", type: "success" as const };
    case 3:
      return { label: "归档", type: "warning" as const };
    default:
      return { label: "未知", type: "danger" as const };
  }
}

function productCountOf(row: CategoryRow) {
  return Number(row.product_count ?? row.productCount ?? 0);
}

function updatedAt(row: CategoryRow) {
  return row.updated_at ?? row.updatedAt;
}

function formatTime(value?: number) {
  if (!value) return "-";
  const timestamp = value > 9999999999 ? value : value * 1000;
  return dayjs(timestamp).format("YYYY-MM-DD HH:mm");
}

function normalizeSlugInput(value: string) {
  return value.trim().toLowerCase();
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

function buildPayload(): AdminMallProductCategoryPayload {
  return {
    slug: normalizeSlugInput(form.slug),
    name: form.name.trim(),
    description: form.description.trim(),
    status: Number(form.status || 2),
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
    } = await listAdminMallProductCategories({
      keyword: query.keyword.trim(),
      status: query.status,
      limit: query.pageSize,
      offset: (query.currentPage - 1) * query.pageSize
    });
    if (code !== 0) {
      message(msg || "加载商品分类失败", { type: "error" });
      return;
    }
    categories.value = data.items ?? [];
    query.total = data.total ?? categories.value.length;
  } finally {
    loading.value = false;
  }
}

function resetQuery() {
  query.keyword = "";
  query.status = 0;
  query.currentPage = 1;
  loadCategories();
}

function openCreateDialog() {
  if (!canCreate.value) {
    message("没有新增商品分类权限", { type: "warning" });
    return;
  }
  dialogMode.value = "create";
  resetFormModel();
  dialogVisible.value = true;
}

function openEditDialog(row: CategoryRow) {
  if (!canUpdate.value) {
    message("没有修改商品分类权限", { type: "warning" });
    return;
  }
  dialogMode.value = "edit";
  form.id = Number(row.id ?? 0);
  form.slug = row.slug ?? "";
  form.name = row.name ?? "";
  form.description = row.description ?? "";
  form.status = statusCode(row.status) || 2;
  form.sort = Number(row.sort ?? 0);
  formRef.value?.clearValidate();
  dialogVisible.value = true;
}

async function saveCategory() {
  if (dialogMode.value === "create" && !canCreate.value) {
    message("没有新增商品分类权限", { type: "warning" });
    return;
  }
  if (dialogMode.value === "edit" && !canUpdate.value) {
    message("没有修改商品分类权限", { type: "warning" });
    return;
  }
  form.slug = normalizeSlugInput(form.slug);
  const valid = await formRef.value?.validate().catch(() => false);
  if (!valid) return;
  const payload = buildPayload();
  saving.value = true;
  try {
    const id = normalizeEntityId(form.id);
    const { code, message: msg } =
      dialogMode.value === "create"
        ? await createAdminMallProductCategory(payload)
        : await updateAdminMallProductCategory(id, payload);
    if (code !== 0) {
      message(msg || "保存商品分类失败", { type: "error" });
      return;
    }
    message("商品分类已保存", { type: "success" });
    dialogVisible.value = false;
    await loadCategories();
  } finally {
    saving.value = false;
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
  <div class="mall-categories-page">
    <section class="mall-panel">
      <div class="panel-header">
        <div>
          <h2>商品分类</h2>
          <p>维护积分商城分类、排序和上下架状态</p>
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
        title="当前账号没有 mall:list_product_categories 权限"
        type="warning"
        show-icon
        :closable="false"
        class="permission-alert"
      />

      <el-form :inline="true" class="search-form">
        <el-form-item label="关键词">
          <el-input
            v-model="query.keyword"
            class="w-56!"
            clearable
            placeholder="分类名称 / 标识"
            @keyup.enter="loadCategories"
          />
        </el-form-item>
        <el-form-item label="状态">
          <el-select
            v-model="query.status"
            class="w-32!"
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
        <template #productCount="{ row }">
          <el-tag effect="plain">{{ productCountOf(row) }}</el-tag>
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
      width="640px"
      destroy-on-close
    >
      <el-form
        ref="formRef"
        :model="form"
        :rules="rules"
        label-width="96px"
        class="category-form"
      >
        <el-row :gutter="16">
          <el-col :span="12">
            <el-form-item label="分类标识" prop="slug">
              <el-input
                v-model="form.slug"
                maxlength="64"
                show-word-limit
                placeholder="例如 digital"
                @blur="form.slug = normalizeSlugInput(form.slug)"
              />
            </el-form-item>
          </el-col>
          <el-col :span="12">
            <el-form-item label="分类名称" prop="name">
              <el-input
                v-model="form.name"
                maxlength="80"
                show-word-limit
                placeholder="例如 数字权益"
              />
            </el-form-item>
          </el-col>
        </el-row>
        <el-row :gutter="16">
          <el-col :span="12">
            <el-form-item label="状态" prop="status">
              <el-radio-group v-model="form.status">
                <el-radio-button
                  v-for="item in editableStatusOptions"
                  :key="item.value"
                  :value="item.value"
                >
                  {{ item.label }}
                </el-radio-button>
              </el-radio-group>
            </el-form-item>
          </el-col>
          <el-col :span="12">
            <el-form-item label="排序">
              <el-input-number
                v-model="form.sort"
                :min="0"
                :max="999999"
                controls-position="right"
              />
            </el-form-item>
          </el-col>
        </el-row>
        <el-form-item label="分类说明">
          <el-input
            v-model="form.description"
            type="textarea"
            :rows="4"
            maxlength="500"
            show-word-limit
            placeholder="分类用途、商品范围或运营说明"
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
.mall-categories-page {
  display: flex;
  flex-direction: column;
  gap: 16px;
}

.mall-panel {
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

.panel-header h2 {
  margin: 0;
  font-size: 18px;
  font-weight: 700;
  color: var(--el-text-color-primary);
}

.panel-header p {
  margin: 6px 0 0;
  color: var(--el-text-color-secondary);
}

.search-form {
  padding: 14px 14px 2px;
  margin-bottom: 14px;
  background: var(--el-fill-color-lighter);
  border-radius: 8px;
}

.permission-alert {
  margin-bottom: 14px;
}

.category-form :deep(.el-input-number) {
  width: 100%;
}
</style>
