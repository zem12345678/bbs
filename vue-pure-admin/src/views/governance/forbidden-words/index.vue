<script setup lang="ts">
import dayjs from "dayjs";
import { computed, onMounted, reactive, ref } from "vue";
import { ElMessageBox, type FormInstance, type FormRules } from "element-plus";
import { message } from "@/utils/message";
import { hasPerms } from "@/utils/auth";
import {
  createAdminForbiddenWord,
  deleteAdminForbiddenWord,
  listAdminForbiddenWords,
  updateAdminForbiddenWord,
  type AdminForbiddenWord,
  type AdminForbiddenWordPayload
} from "@/api/admin";
import { normalizeEntityId } from "@/utils/entityId";
import { useRenderIcon } from "@/components/ReIcon/src/hooks";

defineOptions({
  name: "GovernanceForbiddenWords"
});

type ForbiddenWordRow = Partial<AdminForbiddenWord> & Record<string, any>;
type DialogMode = "create" | "edit";

const loading = ref(false);
const saving = ref(false);
const dialogVisible = ref(false);
const dialogMode = ref<DialogMode>("create");
const formRef = ref<FormInstance>();
const words = ref<AdminForbiddenWord[]>([]);

const query = reactive({
  status: 0,
  keyword: "",
  pageSize: 20,
  currentPage: 1,
  total: 0
});

const form = reactive({
  id: 0,
  word: "",
  scene: "content",
  action: "reject",
  replacement: "",
  description: "",
  status: 2
});

const canList = computed(() => hasPerms("governance:list_forbidden_words"));
const canCreate = computed(() => hasPerms("governance:create_forbidden_word"));
const canUpdate = computed(() => hasPerms("governance:update_forbidden_word"));
const canDelete = computed(() => hasPerms("governance:delete_forbidden_word"));

const dialogTitle = computed(() =>
  dialogMode.value === "create" ? "新增敏感词" : "编辑敏感词"
);

const columns: TableColumnList = [
  { prop: "id", label: "ID", width: 90 },
  { prop: "word", label: "敏感词", minWidth: 160, showOverflowTooltip: true },
  { label: "场景", width: 120, slot: "scene" },
  { label: "动作", width: 120, slot: "action" },
  {
    prop: "replacement",
    label: "替换文本",
    minWidth: 140,
    showOverflowTooltip: true
  },
  {
    prop: "description",
    label: "说明",
    minWidth: 220,
    showOverflowTooltip: true
  },
  { label: "状态", width: 100, slot: "status" },
  { label: "更新时间", width: 170, slot: "updatedAt" },
  { label: "操作", fixed: "right", width: 180, slot: "operation" }
];

const statusOptions = [
  { label: "全部", value: 0 },
  { label: "启用", value: 2 },
  { label: "停用", value: 1 }
];

const sceneOptions = [
  { label: "内容", value: "content" },
  { label: "评论", value: "comment" },
  { label: "资料", value: "profile" },
  { label: "账号", value: "account" }
];

const actionOptions = [
  { label: "拒绝发布", value: "reject" },
  { label: "进入审核", value: "review" },
  { label: "替换文本", value: "replace" }
];

const rules: FormRules = {
  word: [
    { required: true, message: "请输入敏感词", trigger: "blur" },
    {
      min: 1,
      max: 128,
      message: "敏感词长度需在 1-128 个字符",
      trigger: "blur"
    }
  ],
  scene: [{ required: true, message: "请选择场景", trigger: "change" }],
  action: [{ required: true, message: "请选择处理动作", trigger: "change" }],
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

function sceneLabel(scene?: string) {
  return sceneOptions.find(item => item.value === scene)?.label ?? scene ?? "-";
}

function actionMeta(action?: string) {
  switch (action) {
    case "reject":
      return { label: "拒绝发布", type: "danger" as const };
    case "review":
      return { label: "进入审核", type: "warning" as const };
    case "replace":
      return { label: "替换文本", type: "primary" as const };
    default:
      return { label: action || "-", type: "info" as const };
  }
}

function updatedAt(row: ForbiddenWordRow) {
  return row.updated_at ?? row.updatedAt;
}

function formatTime(value?: number) {
  if (!value) return "-";
  const timestamp = value > 9999999999 ? value : value * 1000;
  return dayjs(timestamp).format("YYYY-MM-DD HH:mm");
}

function resetFormModel() {
  form.id = 0;
  form.word = "";
  form.scene = "content";
  form.action = "reject";
  form.replacement = "";
  form.description = "";
  form.status = 2;
  formRef.value?.clearValidate();
}

function buildPayload(): AdminForbiddenWordPayload {
  return {
    word: form.word.trim(),
    scene: form.scene.trim(),
    action: form.action.trim(),
    replacement: form.action === "replace" ? form.replacement.trim() : "",
    description: form.description.trim(),
    status: form.status
  };
}

async function loadWords() {
  if (!canList.value) {
    words.value = [];
    query.total = 0;
    return;
  }
  loading.value = true;
  try {
    const {
      code,
      data,
      message: msg
    } = await listAdminForbiddenWords({
      status: query.status,
      query: query.keyword.trim(),
      limit: query.pageSize,
      offset: (query.currentPage - 1) * query.pageSize
    });
    if (code !== 0) {
      message(msg || "加载敏感词列表失败", { type: "error" });
      return;
    }
    words.value = data.items ?? [];
    query.total = data.total ?? words.value.length;
  } finally {
    loading.value = false;
  }
}

function resetQuery() {
  query.status = 0;
  query.keyword = "";
  query.currentPage = 1;
  loadWords();
}

function openCreateDialog() {
  if (!canCreate.value) {
    message("没有新增敏感词权限", { type: "warning" });
    return;
  }
  dialogMode.value = "create";
  resetFormModel();
  dialogVisible.value = true;
}

function openEditDialog(row: ForbiddenWordRow) {
  if (!canUpdate.value) {
    message("没有修改敏感词权限", { type: "warning" });
    return;
  }
  dialogMode.value = "edit";
  form.id = Number(row.id ?? 0);
  form.word = row.word ?? "";
  form.scene = row.scene ?? "content";
  form.action = row.action ?? "reject";
  form.replacement = row.replacement ?? "";
  form.description = row.description ?? "";
  form.status = Number(row.status ?? 2);
  formRef.value?.clearValidate();
  dialogVisible.value = true;
}

async function saveWord() {
  if (dialogMode.value === "create" && !canCreate.value) {
    message("没有新增敏感词权限", { type: "warning" });
    return;
  }
  if (dialogMode.value === "edit" && !canUpdate.value) {
    message("没有修改敏感词权限", { type: "warning" });
    return;
  }
  const valid = await formRef.value?.validate().catch(() => false);
  if (!valid) return;
  const payload = buildPayload();
  if (!payload.word) {
    message("请输入敏感词", { type: "warning" });
    return;
  }
  saving.value = true;
  try {
    const { code, message: msg } =
      dialogMode.value === "create"
        ? await createAdminForbiddenWord(payload)
        : await updateAdminForbiddenWord(form.id, payload);
    if (code !== 0) {
      message(msg || "保存敏感词失败", { type: "error" });
      return;
    }
    message("敏感词已保存", { type: "success" });
    dialogVisible.value = false;
    await loadWords();
  } finally {
    saving.value = false;
  }
}

async function handleDelete(row: ForbiddenWordRow) {
  if (!canDelete.value) {
    message("没有删除敏感词权限", { type: "warning" });
    return;
  }
  const id = normalizeEntityId(row.id);
  if (!id) {
    message("敏感词 ID 无效", { type: "warning" });
    return;
  }
  await ElMessageBox.confirm(
    `确认删除敏感词「${row.word || id}」？`,
    "删除敏感词",
    {
      type: "warning",
      confirmButtonText: "确认",
      cancelButtonText: "取消"
    }
  );
  loading.value = true;
  try {
    const { code, message: msg } = await deleteAdminForbiddenWord(id);
    if (code !== 0) {
      message(msg || "删除敏感词失败", { type: "error" });
      return;
    }
    message("敏感词已删除", { type: "success" });
    await loadWords();
  } finally {
    loading.value = false;
  }
}

function onPageSizeChange(size: number) {
  query.pageSize = size;
  query.currentPage = 1;
  loadWords();
}

function onCurrentPageChange(page: number) {
  query.currentPage = page;
  loadWords();
}

onMounted(loadWords);
</script>

<template>
  <div class="forbidden-words-page">
    <section class="governance-panel">
      <div class="panel-header">
        <div>
          <h2>敏感词管理</h2>
          <p>配置内容、评论和资料场景的敏感词处理规则</p>
        </div>
        <el-button
          type="primary"
          :icon="useRenderIcon('ri/add-line')"
          :disabled="!canCreate"
          @click="openCreateDialog"
        >
          新增敏感词
        </el-button>
      </div>

      <el-alert
        v-if="!canList"
        title="当前账号没有 governance:list_forbidden_words 权限"
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
            class="w-52!"
            placeholder="敏感词 / 场景 / 动作"
            @keyup.enter="loadWords"
          />
        </el-form-item>
        <el-form-item label="状态">
          <el-select v-model="query.status" class="w-36!" @change="loadWords">
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
            @click="loadWords"
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
        :data="words"
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
        <template #scene="{ row }">
          <el-tag effect="plain">{{ sceneLabel(row.scene) }}</el-tag>
        </template>
        <template #action="{ row }">
          <el-tag :type="actionMeta(row.action).type">
            {{ actionMeta(row.action).label }}
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
      width="560px"
      destroy-on-close
    >
      <el-form
        ref="formRef"
        :model="form"
        :rules="rules"
        label-width="92px"
        class="word-form"
      >
        <el-form-item label="敏感词" prop="word">
          <el-input
            v-model="form.word"
            maxlength="128"
            show-word-limit
            placeholder="请输入需要拦截的词"
          />
        </el-form-item>
        <el-form-item label="场景" prop="scene">
          <el-select v-model="form.scene" class="w-full!">
            <el-option
              v-for="item in sceneOptions"
              :key="item.value"
              :label="item.label"
              :value="item.value"
            />
          </el-select>
        </el-form-item>
        <el-form-item label="处理动作" prop="action">
          <el-select v-model="form.action" class="w-full!">
            <el-option
              v-for="item in actionOptions"
              :key="item.value"
              :label="item.label"
              :value="item.value"
            />
          </el-select>
        </el-form-item>
        <el-form-item v-if="form.action === 'replace'" label="替换文本">
          <el-input v-model="form.replacement" placeholder="例如 ***" />
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
            placeholder="规则用途、处理口径或来源"
          />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="dialogVisible = false">取消</el-button>
        <el-button type="primary" :loading="saving" @click="saveWord">
          保存
        </el-button>
      </template>
    </el-dialog>
  </div>
</template>

<style scoped>
.forbidden-words-page {
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

.word-form {
  padding-right: 10px;
}

@media (width <= 768px) {
  .panel-header {
    flex-direction: column;
  }
}
</style>
