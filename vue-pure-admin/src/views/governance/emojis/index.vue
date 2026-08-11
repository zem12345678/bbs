<script setup lang="ts">
import dayjs from "dayjs";
import { computed, onMounted, reactive, ref } from "vue";
import {
  ElMessageBox,
  type FormInstance,
  type FormRules,
  type UploadProps,
  type UploadRequestOptions
} from "element-plus";
import { message } from "@/utils/message";
import { hasPerms } from "@/utils/auth";
import {
  createAdminEmoji,
  deleteAdminEmoji,
  listAdminEmojis,
  updateAdminEmoji,
  uploadAdminEmoji,
  type AdminEmoji,
  type AdminEmojiPayload
} from "@/api/admin";
import { normalizeEntityId, type EntityId } from "@/utils/entityId";
import { useRenderIcon } from "@/components/ReIcon/src/hooks";

defineOptions({
  name: "GovernanceEmojis"
});

type DialogMode = "create" | "edit";

const loading = ref(false);
const saving = ref(false);
const uploading = ref(false);
const dialogVisible = ref(false);
const dialogMode = ref<DialogMode>("create");
const formRef = ref<FormInstance>();
const emojis = ref<AdminEmoji[]>([]);
let listRequestVersion = 0;

const query = reactive({
  keyword: "",
  pageSize: 20,
  currentPage: 1,
  total: 0
});

const form = reactive({
  id: "" as EntityId,
  name: "",
  category: "",
  aliasesText: "",
  license: "",
  isSensitive: false,
  localOnly: false,
  imageUrl: "",
  fileId: ""
});

const canList = computed(() => hasPerms("governance:list_emojis"));
const canCreate = computed(() => hasPerms("governance:create_emoji"));
const canUpdate = computed(() => hasPerms("governance:update_emoji"));
const canDelete = computed(() => hasPerms("governance:delete_emoji"));
const canUpload = computed(() => canCreate.value || canUpdate.value);
const dialogTitle = computed(() =>
  dialogMode.value === "create" ? "新增 Emoji" : "编辑 Emoji"
);
const previewImageUrl = computed(() => safeImageURL(form.imageUrl));

const columns: TableColumnList = [
  { prop: "name", label: "名称", minWidth: 150, showOverflowTooltip: true },
  { prop: "id", label: "ID", width: 100 },
  { label: "图片", width: 82, slot: "image" },
  { label: "分类", minWidth: 130, slot: "category" },
  { label: "别名", minWidth: 220, slot: "aliases" },
  { label: "属性", minWidth: 160, slot: "flags" },
  { label: "更新时间", width: 170, slot: "updatedAt" },
  { label: "操作", fixed: "right", width: 180, slot: "operation" }
];

const rules: FormRules = {
  name: [
    { required: true, message: "请输入 Emoji 名称", trigger: "blur" },
    {
      pattern: /^[\p{L}\p{N}\p{M}_+\-]+$/u,
      message: "只允许文字、数字、下划线、加号和短横线",
      trigger: "blur"
    },
    { min: 1, max: 128, message: "名称长度需在 1-128 个字符", trigger: "blur" }
  ],
  category: [{ max: 128, message: "分类最多 128 个字符", trigger: "blur" }],
  license: [
    { max: 1024, message: "许可说明最多 1024 个字符", trigger: "blur" }
  ],
  aliasesText: [
    {
      validator: (_rule, _value, callback) => {
        const aliases = parseAliases(form.aliasesText);
        if (aliases.length > 64) {
          callback(new Error("别名最多 64 个"));
          return;
        }
        if (aliases.some(alias => alias.length > 128)) {
          callback(new Error("单个别名最多 128 个字符"));
          return;
        }
        callback();
      },
      trigger: "blur"
    }
  ],
  fileId: [
    {
      validator: (_rule, _value, callback) => {
        if (dialogMode.value === "create" && !form.fileId) {
          callback(new Error("请上传 Emoji 图片"));
          return;
        }
        callback();
      },
      trigger: "change"
    }
  ]
};

function safeImageURL(value: unknown) {
  const raw = typeof value === "string" ? value.trim() : "";
  if (!/^https?:\/\/[^/?#\\\s]+/i.test(raw)) return "";
  try {
    const parsed = new URL(raw);
    return ["http:", "https:"].includes(parsed.protocol) &&
      parsed.hostname &&
      !parsed.username &&
      !parsed.password
      ? raw
      : "";
  } catch {
    return "";
  }
}

function imageUrlOf(row: AdminEmoji) {
  return (
    [row.publicUrl, row.url, row.originalUrl].map(safeImageURL).find(Boolean) ??
    ""
  );
}

function nullableText(value: string) {
  const normalized = value.trim();
  return normalized || null;
}

function parseAliases(value: string) {
  const seen = new Set<string>();
  return value
    .split(/[,，\n]/)
    .map(item => item.trim())
    .filter(item => {
      const key = item.toLowerCase();
      if (!item || seen.has(key)) return false;
      seen.add(key);
      return true;
    });
}

function formatTime(value?: string | null) {
  const parsed = dayjs(value);
  return value && parsed.isValid() ? parsed.format("YYYY-MM-DD HH:mm") : "-";
}

function resetFormModel() {
  form.id = "";
  form.name = "";
  form.category = "";
  form.aliasesText = "";
  form.license = "";
  form.isSensitive = false;
  form.localOnly = false;
  form.imageUrl = "";
  form.fileId = "";
  formRef.value?.clearValidate();
}

function buildPayload(): AdminEmojiPayload {
  const payload: AdminEmojiPayload = {
    name: form.name.trim(),
    category: nullableText(form.category),
    aliases: parseAliases(form.aliasesText),
    license: nullableText(form.license),
    isSensitive: form.isSensitive,
    localOnly: form.localOnly
  };
  if (form.fileId) payload.fileId = form.fileId;
  return payload;
}

async function loadEmojis() {
  if (!canList.value) {
    emojis.value = [];
    query.total = 0;
    return;
  }
  const requestVersion = ++listRequestVersion;
  loading.value = true;
  try {
    const {
      code,
      data,
      message: msg
    } = await listAdminEmojis({
      query: query.keyword.trim(),
      limit: query.pageSize,
      offset: (query.currentPage - 1) * query.pageSize
    });
    if (requestVersion !== listRequestVersion) return;
    if (code !== 0) {
      emojis.value = [];
      query.total = 0;
      message(msg || "加载 Emoji 列表失败", { type: "error" });
      return;
    }
    emojis.value = data.items ?? [];
    query.total = data.total ?? emojis.value.length;
  } catch {
    if (requestVersion === listRequestVersion) {
      emojis.value = [];
      query.total = 0;
      message("加载 Emoji 列表失败", { type: "error" });
    }
  } finally {
    if (requestVersion === listRequestVersion) loading.value = false;
  }
}

function searchEmojis() {
  query.currentPage = 1;
  loadEmojis();
}

function resetQuery() {
  query.keyword = "";
  query.currentPage = 1;
  loadEmojis();
}

function openCreateDialog() {
  if (!canCreate.value) {
    message("没有新增 Emoji 权限", { type: "warning" });
    return;
  }
  dialogMode.value = "create";
  resetFormModel();
  dialogVisible.value = true;
}

function openEditDialog(row: AdminEmoji) {
  if (!canUpdate.value) {
    message("没有修改 Emoji 权限", { type: "warning" });
    return;
  }
  const id = normalizeEntityId(row.id);
  if (id === undefined) {
    message("Emoji ID 无效", { type: "warning" });
    return;
  }
  dialogMode.value = "edit";
  form.id = id;
  form.name = row.name ?? "";
  form.category = row.category ?? "";
  form.aliasesText = (row.aliases ?? []).join(", ");
  form.license = row.license ?? "";
  form.isSensitive = Boolean(row.isSensitive);
  form.localOnly = Boolean(row.localOnly);
  form.imageUrl = imageUrlOf(row);
  form.fileId = "";
  formRef.value?.clearValidate();
  dialogVisible.value = true;
}

const beforeImageUpload: UploadProps["beforeUpload"] = file => {
  const supportedTypes = ["image/png", "image/jpeg", "image/gif", "image/webp"];
  if (!supportedTypes.includes(file.type)) {
    message("仅支持 PNG、JPEG、GIF 或 WebP 图片", { type: "warning" });
    return false;
  }
  if (file.size <= 0 || file.size > 5 * 1024 * 1024) {
    message("图片大小需在 1 字节至 5 MiB 之间", { type: "warning" });
    return false;
  }
  return true;
};

async function handleImageUpload(options: UploadRequestOptions) {
  const data = new FormData();
  data.append("file", options.file);
  uploading.value = true;
  try {
    const { code, data: uploaded, message: msg } = await uploadAdminEmoji(data);
    if (code !== 0) throw new Error(msg || "上传 Emoji 图片失败");
    const fileId = uploaded.fileId || uploaded.file_id || "";
    if (!fileId || !safeImageURL(uploaded.url)) {
      throw new Error("上传响应缺少图片信息");
    }
    form.fileId = fileId;
    form.imageUrl = uploaded.url;
    await formRef.value?.validateField("fileId").catch(() => undefined);
    message("Emoji 图片已上传", { type: "success" });
    return uploaded;
  } catch (error) {
    message(error instanceof Error ? error.message : "上传 Emoji 图片失败", {
      type: "error"
    });
    throw error;
  } finally {
    uploading.value = false;
  }
}

async function saveEmoji() {
  const allowed =
    dialogMode.value === "create" ? canCreate.value : canUpdate.value;
  if (!allowed) {
    message("没有保存 Emoji 权限", { type: "warning" });
    return;
  }
  const valid = await formRef.value?.validate().catch(() => false);
  if (!valid) return;
  const payload = buildPayload();
  saving.value = true;
  try {
    const { code, message: msg } =
      dialogMode.value === "create"
        ? await createAdminEmoji(payload)
        : await updateAdminEmoji(form.id, payload);
    if (code !== 0) {
      message(msg || "保存 Emoji 失败", { type: "error" });
      return;
    }
    message("Emoji 已保存", { type: "success" });
    dialogVisible.value = false;
    await loadEmojis();
  } catch {
    message("保存 Emoji 失败", { type: "error" });
  } finally {
    saving.value = false;
  }
}

async function handleDelete(row: AdminEmoji) {
  if (!canDelete.value) {
    message("没有删除 Emoji 权限", { type: "warning" });
    return;
  }
  const id = normalizeEntityId(row.id);
  if (id === undefined) {
    message("Emoji ID 无效", { type: "warning" });
    return;
  }
  const confirmed = await ElMessageBox.confirm(
    `确认删除 Emoji「:${row.name}:」？`,
    "删除 Emoji",
    {
      type: "warning",
      confirmButtonText: "确认",
      cancelButtonText: "取消"
    }
  ).catch(() => false);
  if (!confirmed) return;
  loading.value = true;
  try {
    const { code, message: msg } = await deleteAdminEmoji(id);
    if (code !== 0) {
      message(msg || "删除 Emoji 失败", { type: "error" });
      return;
    }
    message("Emoji 已删除", { type: "success" });
    if (emojis.value.length === 1 && query.currentPage > 1) {
      query.currentPage -= 1;
    }
    await loadEmojis();
  } catch {
    message("删除 Emoji 失败", { type: "error" });
  } finally {
    loading.value = false;
  }
}

function onPageSizeChange(size: number) {
  query.pageSize = size;
  query.currentPage = 1;
  loadEmojis();
}

function onCurrentPageChange(page: number) {
  query.currentPage = page;
  loadEmojis();
}

onMounted(loadEmojis);
</script>

<template>
  <div class="emojis-page">
    <section class="governance-panel">
      <div class="panel-header">
        <div>
          <h2>Emoji 管理</h2>
          <p>维护站点自定义 Emoji 图片、别名和使用属性</p>
        </div>
        <el-button
          type="primary"
          :icon="useRenderIcon('ri/add-line')"
          :disabled="!canCreate"
          @click="openCreateDialog"
        >
          新增 Emoji
        </el-button>
      </div>

      <el-alert
        v-if="!canList"
        title="当前账号没有 governance:list_emojis 权限"
        type="warning"
        show-icon
        :closable="false"
        class="permission-alert"
      />

      <el-form :inline="true" class="search-form" @submit.prevent>
        <el-form-item label="名称">
          <el-input
            v-model="query.keyword"
            clearable
            maxlength="128"
            placeholder="搜索名称或别名"
            @keyup.enter="searchEmojis"
          />
        </el-form-item>
        <el-form-item>
          <el-button
            type="primary"
            :icon="useRenderIcon('ri/search-line')"
            :disabled="!canList"
            :loading="loading"
            @click="searchEmojis"
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
        :data="emojis"
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
        <template #image="{ row }">
          <el-image
            v-if="imageUrlOf(row)"
            class="emoji-thumbnail"
            :src="imageUrlOf(row)"
            :preview-src-list="[imageUrlOf(row)]"
            :alt="`${row.name} Emoji`"
            fit="contain"
            preview-teleported
          >
            <template #error>
              <span class="image-fallback">失败</span>
            </template>
          </el-image>
          <span v-else>-</span>
        </template>
        <template #category="{ row }">
          {{ row.category || "未分类" }}
        </template>
        <template #aliases="{ row }">
          <span class="aliases-cell">{{ row.aliases?.join("、") || "-" }}</span>
        </template>
        <template #flags="{ row }">
          <div class="flag-list">
            <el-tag v-if="row.isSensitive" type="warning" effect="plain">
              敏感
            </el-tag>
            <el-tag v-if="row.localOnly" type="info" effect="plain">
              仅本地
            </el-tag>
            <span v-if="!row.isSensitive && !row.localOnly">-</span>
          </div>
        </template>
        <template #updatedAt="{ row }">
          {{ formatTime(row.updatedAt) }}
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
      width="min(680px, calc(100vw - 32px))"
      destroy-on-close
    >
      <el-form
        ref="formRef"
        :model="form"
        :rules="rules"
        label-width="94px"
        class="emoji-form"
      >
        <el-form-item label="图片" prop="fileId">
          <div class="image-editor">
            <el-image
              v-if="previewImageUrl"
              class="form-thumbnail"
              :src="previewImageUrl"
              :preview-src-list="[previewImageUrl]"
              :alt="form.name ? `${form.name} Emoji 预览` : 'Emoji 图片预览'"
              fit="contain"
              preview-teleported
            />
            <div v-else class="form-thumbnail image-placeholder">暂无图片</div>
            <div class="upload-actions">
              <el-upload
                accept=".png,.jpg,.jpeg,.gif,.webp"
                :show-file-list="false"
                :before-upload="beforeImageUpload"
                :http-request="handleImageUpload"
                :disabled="uploading || !canUpload"
              >
                <el-button
                  :icon="useRenderIcon('ri/upload-2-line')"
                  :loading="uploading"
                  :disabled="!canUpload"
                >
                  {{ previewImageUrl ? "替换图片" : "上传图片" }}
                </el-button>
              </el-upload>
              <span class="upload-tip">PNG/JPEG/GIF/WebP，最大 5 MiB</span>
            </div>
          </div>
        </el-form-item>

        <el-form-item label="名称" prop="name">
          <el-input
            v-model="form.name"
            maxlength="128"
            show-word-limit
            placeholder="例如 party_parrot"
          >
            <template #prepend>:</template>
            <template #append>:</template>
          </el-input>
        </el-form-item>
        <el-form-item label="分类" prop="category">
          <el-input
            v-model="form.category"
            maxlength="128"
            show-word-limit
            placeholder="留空表示未分类"
          />
        </el-form-item>
        <el-form-item label="别名" prop="aliasesText">
          <el-input
            v-model="form.aliasesText"
            type="textarea"
            :rows="2"
            placeholder="多个别名用逗号或换行分隔"
          />
        </el-form-item>
        <el-form-item label="许可说明" prop="license">
          <el-input
            v-model="form.license"
            type="textarea"
            :rows="3"
            maxlength="1024"
            show-word-limit
            placeholder="版权或使用许可，留空表示未填写"
          />
        </el-form-item>
        <el-form-item label="使用属性">
          <div class="switch-list">
            <div class="switch-item">
              <div>
                <strong>敏感内容</strong>
                <span>在展示时标记该 Emoji 为敏感内容</span>
              </div>
              <el-switch v-model="form.isSensitive" />
            </div>
            <div class="switch-item">
              <div>
                <strong>仅本地使用</strong>
                <span>限制该 Emoji 仅在本站范围内使用</span>
              </div>
              <el-switch v-model="form.localOnly" />
            </div>
          </div>
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="dialogVisible = false">取消</el-button>
        <el-button
          type="primary"
          :loading="saving"
          :disabled="
            uploading || (dialogMode === 'create' ? !canCreate : !canUpdate)
          "
          @click="saveEmoji"
        >
          保存
        </el-button>
      </template>
    </el-dialog>
  </div>
</template>

<style scoped>
.emojis-page {
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

.emoji-thumbnail,
.form-thumbnail {
  width: 48px;
  height: 48px;
  cursor: zoom-in;
  background: var(--el-fill-color-light);
  border: 1px solid var(--el-border-color-light);
  border-radius: 4px;
}

.image-fallback,
.image-placeholder {
  display: flex;
  align-items: center;
  justify-content: center;
  font-size: 12px;
  color: var(--el-text-color-secondary);
}

.aliases-cell {
  display: block;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.flag-list {
  display: flex;
  flex-wrap: wrap;
  gap: 6px;
  justify-content: center;
}

.emoji-form {
  padding-right: 10px;
}

.image-editor {
  display: flex;
  gap: 14px;
  align-items: center;
  width: 100%;
}

.upload-actions {
  display: flex;
  flex-direction: column;
  gap: 6px;
  align-items: flex-start;
}

.upload-tip,
.switch-item span {
  font-size: 12px;
  color: var(--el-text-color-secondary);
}

.switch-list {
  display: flex;
  flex-direction: column;
  gap: 12px;
  width: 100%;
}

.switch-item {
  display: flex;
  gap: 16px;
  align-items: center;
  justify-content: space-between;
}

.switch-item div {
  display: flex;
  flex-direction: column;
  line-height: 1.5;
}

@media (width <= 768px) {
  .panel-header {
    flex-direction: column;
  }

  .emoji-form {
    padding-right: 0;
  }
}
</style>
