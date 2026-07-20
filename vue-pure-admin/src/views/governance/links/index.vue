<script setup lang="ts">
import dayjs from "dayjs";
import { computed, onMounted, reactive, ref } from "vue";
import { ElMessageBox, type FormInstance, type FormRules } from "element-plus";
import { message } from "@/utils/message";
import { hasPerms } from "@/utils/auth";
import {
  createAdminLink,
  deleteAdminLink,
  listAdminLinks,
  updateAdminLink,
  type AdminLink,
  type AdminLinkPayload
} from "@/api/admin";
import { normalizeEntityId } from "@/utils/entityId";
import { useRenderIcon } from "@/components/ReIcon/src/hooks";

defineOptions({
  name: "GovernanceLinks"
});

type LinkRow = Partial<AdminLink> & Record<string, any>;
type DialogMode = "create" | "edit";

const loading = ref(false);
const saving = ref(false);
const dialogVisible = ref(false);
const dialogMode = ref<DialogMode>("create");
const formRef = ref<FormInstance>();
const links = ref<AdminLink[]>([]);

const query = reactive({
  status: 0,
  pageSize: 20,
  currentPage: 1,
  total: 0
});

const form = reactive({
  id: 0,
  key: "",
  title: "",
  url: "",
  description: "",
  status: 2,
  sort: 0
});

const canList = computed(() => hasPerms("governance:list_links"));
const canCreate = computed(() => hasPerms("governance:create_link"));
const canUpdate = computed(() => hasPerms("governance:update_link"));
const canDelete = computed(() => hasPerms("governance:delete_link"));

const dialogTitle = computed(() =>
  dialogMode.value === "create" ? "新增友情链接" : "编辑友情链接"
);

const columns: TableColumnList = [
  { prop: "id", label: "ID", width: 90 },
  { prop: "key", label: "标识", minWidth: 140, showOverflowTooltip: true },
  { prop: "title", label: "名称", minWidth: 180, showOverflowTooltip: true },
  { label: "链接地址", minWidth: 260, slot: "url" },
  {
    prop: "description",
    label: "说明",
    minWidth: 220,
    showOverflowTooltip: true
  },
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
    { required: true, message: "请输入链接标识", trigger: "blur" },
    {
      pattern: /^[a-z0-9_.-]+$/,
      message: "只允许小写字母、数字、下划线、点和短横线",
      trigger: "blur"
    }
  ],
  title: [
    { required: true, message: "请输入链接名称", trigger: "blur" },
    { min: 1, max: 128, message: "名称长度需在 1-128 个字符", trigger: "blur" }
  ],
  url: [
    { required: true, message: "请输入链接地址", trigger: "blur" },
    {
      validator: (_rule, value, callback) => {
        callback(
          safeExternalURL(value)
            ? undefined
            : new Error("请输入不含账号密码的有效 http:// 或 https:// 链接")
        );
      },
      trigger: "blur"
    }
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

function urlOf(row: LinkRow) {
  return String(row.url ?? row.URL ?? "");
}

function safeExternalURL(value: unknown) {
  const raw = typeof value === "string" ? value.trim() : "";
  if (!/^https?:\/\/[^/?#\\\s]+/i.test(raw)) return "";
  try {
    const parsed = new URL(raw);
    if (
      (parsed.protocol !== "http:" && parsed.protocol !== "https:") ||
      !parsed.hostname ||
      parsed.username ||
      parsed.password
    ) {
      return "";
    }
    return raw;
  } catch {
    return "";
  }
}

function safeURLOf(row: LinkRow) {
  return safeExternalURL(urlOf(row));
}

function updatedAt(row: LinkRow) {
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
  form.title = "";
  form.url = "";
  form.description = "";
  form.status = 2;
  form.sort = 0;
  formRef.value?.clearValidate();
}

function buildPayload(): AdminLinkPayload {
  return {
    key: form.key.trim(),
    title: form.title.trim(),
    url: form.url.trim(),
    description: form.description.trim(),
    status: form.status,
    sort: Number(form.sort || 0)
  };
}

async function loadLinks() {
  if (!canList.value) {
    links.value = [];
    query.total = 0;
    return;
  }
  loading.value = true;
  try {
    const {
      code,
      data,
      message: msg
    } = await listAdminLinks({
      status: query.status,
      limit: query.pageSize,
      offset: (query.currentPage - 1) * query.pageSize
    });
    if (code !== 0) {
      message(msg || "加载友情链接失败", { type: "error" });
      return;
    }
    links.value = data.items ?? [];
    query.total = data.total ?? links.value.length;
  } finally {
    loading.value = false;
  }
}

function resetQuery() {
  query.status = 0;
  query.currentPage = 1;
  loadLinks();
}

function openCreateDialog() {
  if (!canCreate.value) {
    message("没有新增友情链接权限", { type: "warning" });
    return;
  }
  dialogMode.value = "create";
  resetFormModel();
  dialogVisible.value = true;
}

function openEditDialog(row: LinkRow) {
  if (!canUpdate.value) {
    message("没有修改友情链接权限", { type: "warning" });
    return;
  }
  dialogMode.value = "edit";
  form.id = Number(row.id ?? 0);
  form.key = row.key ?? "";
  form.title = row.title ?? "";
  form.url = urlOf(row);
  form.description = row.description ?? "";
  form.status = Number(row.status ?? 2);
  form.sort = Number(row.sort ?? 0);
  formRef.value?.clearValidate();
  dialogVisible.value = true;
}

async function saveLink() {
  if (dialogMode.value === "create" && !canCreate.value) {
    message("没有新增友情链接权限", { type: "warning" });
    return;
  }
  if (dialogMode.value === "edit" && !canUpdate.value) {
    message("没有修改友情链接权限", { type: "warning" });
    return;
  }
  const valid = await formRef.value?.validate().catch(() => false);
  if (!valid) return;
  const payload = buildPayload();
  saving.value = true;
  try {
    const { code, message: msg } =
      dialogMode.value === "create"
        ? await createAdminLink(payload)
        : await updateAdminLink(form.id, payload);
    if (code !== 0) {
      message(msg || "保存友情链接失败", { type: "error" });
      return;
    }
    message("友情链接已保存", { type: "success" });
    dialogVisible.value = false;
    await loadLinks();
  } finally {
    saving.value = false;
  }
}

async function handleDelete(row: LinkRow) {
  if (!canDelete.value) {
    message("没有删除友情链接权限", { type: "warning" });
    return;
  }
  const id = normalizeEntityId(row.id);
  if (!id) {
    message("友情链接 ID 无效", { type: "warning" });
    return;
  }
  const confirmed = await ElMessageBox.confirm(
    `确认删除友情链接「${row.title || id}」？`,
    "删除友情链接",
    {
      type: "warning",
      confirmButtonText: "确认",
      cancelButtonText: "取消"
    }
  ).catch(() => false);
  if (!confirmed) return;
  loading.value = true;
  try {
    const { code, message: msg } = await deleteAdminLink(id);
    if (code !== 0) {
      message(msg || "删除友情链接失败", { type: "error" });
      return;
    }
    message("友情链接已删除", { type: "success" });
    await loadLinks();
  } finally {
    loading.value = false;
  }
}

function onPageSizeChange(size: number) {
  query.pageSize = size;
  query.currentPage = 1;
  loadLinks();
}

function onCurrentPageChange(page: number) {
  query.currentPage = page;
  loadLinks();
}

onMounted(loadLinks);
</script>

<template>
  <div class="links-page">
    <section class="governance-panel">
      <div class="panel-header">
        <div>
          <h2>友情链接</h2>
          <p>维护社区前台展示的外部资源、合作站点和文档入口</p>
        </div>
        <el-button
          type="primary"
          :icon="useRenderIcon('ri/add-line')"
          :disabled="!canCreate"
          @click="openCreateDialog"
        >
          新增链接
        </el-button>
      </div>

      <el-alert
        v-if="!canList"
        title="当前账号没有 governance:list_links 权限"
        type="warning"
        show-icon
        :closable="false"
        class="permission-alert"
      />

      <el-form :inline="true" class="search-form">
        <el-form-item label="状态">
          <el-select v-model="query.status" class="w-36!" @change="loadLinks">
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
            @click="loadLinks"
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
        :data="links"
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
        <template #url="{ row }">
          <el-link
            v-if="safeURLOf(row)"
            type="primary"
            :href="safeURLOf(row)"
            target="_blank"
            rel="noreferrer noopener"
            :underline="false"
          >
            {{ urlOf(row) }}
          </el-link>
          <span v-else>{{ urlOf(row) || "-" }}</span>
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
        class="link-form"
      >
        <el-form-item label="链接标识" prop="key">
          <el-input
            v-model="form.key"
            maxlength="64"
            show-word-limit
            placeholder="例如 go-docs"
          />
        </el-form-item>
        <el-form-item label="链接名称" prop="title">
          <el-input
            v-model="form.title"
            maxlength="128"
            show-word-limit
            placeholder="请输入链接名称"
          />
        </el-form-item>
        <el-form-item label="链接地址" prop="url">
          <el-input v-model="form.url" placeholder="https://example.com" />
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
            placeholder="链接用途、展示位置或合作说明"
          />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="dialogVisible = false">取消</el-button>
        <el-button type="primary" :loading="saving" @click="saveLink">
          保存
        </el-button>
      </template>
    </el-dialog>
  </div>
</template>

<style scoped>
.links-page {
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

.link-form {
  padding-right: 10px;
}

@media (width <= 768px) {
  .panel-header {
    flex-direction: column;
  }
}
</style>
