<script setup lang="ts">
import dayjs from "dayjs";
import { computed, onMounted, reactive, ref } from "vue";
import { ElMessageBox, type FormInstance, type FormRules } from "element-plus";
import { message } from "@/utils/message";
import { hasPerms } from "@/utils/auth";
import { useRenderIcon } from "@/components/ReIcon/src/hooks";
import {
  createAdminAnnouncement,
  deleteAdminAnnouncement,
  listAdminAnnouncements,
  updateAdminAnnouncement,
  type AdminAnnouncement,
  type AdminAnnouncementDisplay,
  type AdminAnnouncementIcon,
  type AdminAnnouncementPayload
} from "@/api/admin";

defineOptions({
  name: "GovernanceAnnouncements"
});

type DialogMode = "create" | "edit";
type StatusFilter = "all" | "active" | "archived";

type AnnouncementForm = {
  id: string;
  title: string;
  text: string;
  imageUrl: string;
  icon: AdminAnnouncementIcon;
  display: AdminAnnouncementDisplay;
  forExistingUsers: boolean;
  silence: boolean;
  needConfirmationToRead: boolean;
  confetti: boolean;
  userId: string;
  active: boolean;
  startsAt: Date | null;
  endsAt: Date | null;
};

const iconOptions: Array<{ label: string; value: AdminAnnouncementIcon }> = [
  { label: "信息", value: "info" },
  { label: "提醒", value: "warning" },
  { label: "错误", value: "error" },
  { label: "成功", value: "success" }
];
const displayOptions: Array<{
  label: string;
  value: AdminAnnouncementDisplay;
}> = [
  { label: "普通", value: "normal" },
  { label: "横幅", value: "banner" },
  { label: "对话框", value: "dialog" }
];
const statusOptions: Array<{ label: string; value: StatusFilter }> = [
  { label: "全部", value: "all" },
  { label: "启用", value: "active" },
  { label: "已归档", value: "archived" }
];

const loading = ref(false);
const saving = ref(false);
const dialogVisible = ref(false);
const dialogMode = ref<DialogMode>("create");
const formRef = ref<FormInstance>();
const announcements = ref<AdminAnnouncement[]>([]);
const untilId = ref<string>();
const cursorHistory = ref<Array<string | undefined>>([]);
let listRequestVersion = 0;

const query = reactive({
  status: "all" as StatusFilter,
  pageSize: 20
});

const form = reactive<AnnouncementForm>({
  id: "",
  title: "",
  text: "",
  imageUrl: "",
  icon: "info",
  display: "banner",
  forExistingUsers: false,
  silence: false,
  needConfirmationToRead: false,
  confetti: false,
  userId: "",
  active: true,
  startsAt: null,
  endsAt: null
});

const canList = computed(() => hasPerms("governance:list_announcements"));
const canCreate = computed(() =>
  hasPerms("governance:create_announcement")
);
const canUpdate = computed(() =>
  hasPerms("governance:update_announcement")
);
const canDelete = computed(() =>
  hasPerms("governance:delete_announcement")
);
const currentPage = computed(() => cursorHistory.value.length + 1);
const canGoPrevious = computed(() => cursorHistory.value.length > 0);
const canGoNext = computed(
  () =>
    announcements.value.length === query.pageSize &&
    Boolean(announcements.value.at(-1)?.id)
);
const dialogTitle = computed(() =>
  dialogMode.value === "create" ? "新增公告" : "编辑公告"
);
const previewImageUrl = computed(() => safeExternalURL(form.imageUrl));

const columns: TableColumnList = [
  { label: "公告", minWidth: 240, slot: "content" },
  { label: "图片", width: 96, slot: "image" },
  { label: "展示", width: 100, slot: "display" },
  { label: "级别", width: 90, slot: "icon" },
  { label: "排期", minWidth: 180, slot: "schedule" },
  { label: "状态", width: 90, slot: "status" },
  { prop: "reads", label: "已读", width: 76 },
  { label: "更新时间", width: 150, slot: "updatedAt" },
  { label: "操作", fixed: "right", width: 150, slot: "operation" }
];

const rules: FormRules = {
  title: [
    { required: true, message: "请输入公告标题", trigger: "blur" },
    { max: 256, message: "标题不能超过 256 个字符", trigger: "blur" }
  ],
  text: [{ required: true, message: "请输入公告正文", trigger: "blur" }],
  imageUrl: [
    {
      validator: (_rule, value, callback) => {
        callback(
          !value || safeExternalURL(value)
            ? undefined
            : new Error("请输入不含账号密码的有效 http:// 或 https:// 地址")
        );
      },
      trigger: "blur"
    }
  ],
  userId: [
    {
      validator: (_rule, value, callback) => {
        callback(
          !value || /^[1-9]\d*$/.test(value)
            ? undefined
            : new Error("请输入有效的正整数用户 ID")
        );
      },
      trigger: "blur"
    }
  ],
  endsAt: [
    {
      validator: (_rule, value, callback) => {
        if (!(value instanceof Date) || !(form.startsAt instanceof Date)) {
          callback();
          return;
        }
        callback(
          value.getTime() > form.startsAt.getTime()
            ? undefined
            : new Error("结束时间必须晚于开始时间")
        );
      },
      trigger: "change"
    }
  ]
};

function safeExternalURL(value: unknown) {
  const raw = typeof value === "string" ? value.trim() : "";
  if (!raw || !/^https?:\/\/[^/?#\\\s]+/i.test(raw)) return "";
  try {
    const parsed = new URL(raw);
    if (
      !["http:", "https:"].includes(parsed.protocol) ||
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

function formatTime(value?: string | null) {
  if (!value) return "-";
  const parsed = dayjs(value);
  return parsed.isValid() ? parsed.format("YYYY-MM-DD HH:mm") : "-";
}

function parseDate(value?: string | null) {
  if (!value) return null;
  const parsed = dayjs(value);
  return parsed.isValid() ? parsed.toDate() : null;
}

function displayLabel(value: AdminAnnouncementDisplay) {
  return displayOptions.find(item => item.value === value)?.label ?? value;
}

function iconMeta(value: AdminAnnouncementIcon) {
  switch (value) {
    case "success":
      return { label: "成功", type: "success" as const };
    case "warning":
      return { label: "提醒", type: "warning" as const };
    case "error":
      return { label: "错误", type: "danger" as const };
    default:
      return { label: "信息", type: "info" as const };
  }
}

function scheduleMeta(row: AdminAnnouncement) {
  if (!row.isActive) return { label: "已归档", type: "info" as const };
  const now = Date.now();
  const startsAt = row.startsAt ? dayjs(row.startsAt).valueOf() : 0;
  const endsAt = row.endsAt ? dayjs(row.endsAt).valueOf() : 0;
  if (startsAt > now) return { label: "待生效", type: "warning" as const };
  if (endsAt > 0 && endsAt <= now) {
    return { label: "已结束", type: "info" as const };
  }
  return { label: "生效中", type: "success" as const };
}

function resetCursor() {
  untilId.value = undefined;
  cursorHistory.value = [];
}

function resetForm() {
  form.id = "";
  form.title = "";
  form.text = "";
  form.imageUrl = "";
  form.icon = "info";
  form.display = "banner";
  form.forExistingUsers = false;
  form.silence = false;
  form.needConfirmationToRead = false;
  form.confetti = false;
  form.userId = "";
  form.active = true;
  form.startsAt = null;
  form.endsAt = null;
  formRef.value?.clearValidate();
}

function buildPayload(includeUserId: boolean): AdminAnnouncementPayload {
  const payload: AdminAnnouncementPayload = {
    title: form.title.trim(),
    text: form.text.trim(),
    imageUrl: form.imageUrl.trim() || null,
    icon: form.icon,
    display: form.display,
    forExistingUsers: form.forExistingUsers,
    silence: form.silence,
    needConfirmationToRead: form.needConfirmationToRead,
    confetti: form.confetti,
    isActive: form.active,
    startsAt: form.startsAt?.getTime() ?? 0,
    endsAt: form.endsAt?.getTime() ?? 0
  };
  if (includeUserId) payload.userId = form.userId.trim() || null;
  return payload;
}

async function loadAnnouncements() {
  if (!canList.value) {
    announcements.value = [];
    return;
  }
  const requestVersion = ++listRequestVersion;
  loading.value = true;
  try {
    const data = await listAdminAnnouncements({
      limit: query.pageSize,
      untilId: untilId.value,
      status: query.status
    });
    if (requestVersion !== listRequestVersion) return;
    announcements.value = Array.isArray(data) ? data : [];
  } catch {
    if (requestVersion === listRequestVersion) {
      announcements.value = [];
      message("加载公告失败", { type: "error" });
    }
  } finally {
    if (requestVersion === listRequestVersion) loading.value = false;
  }
}

function searchAnnouncements() {
  resetCursor();
  loadAnnouncements();
}

function resetQuery() {
  query.status = "all";
  query.pageSize = 20;
  searchAnnouncements();
}

function goToPreviousPage() {
  if (!canGoPrevious.value) return;
  untilId.value = cursorHistory.value.pop();
  loadAnnouncements();
}

function goToNextPage() {
  const nextCursor = announcements.value.at(-1)?.id;
  if (!canGoNext.value || !nextCursor) return;
  cursorHistory.value.push(untilId.value);
  untilId.value = nextCursor;
  loadAnnouncements();
}

function openCreateDialog() {
  if (!canCreate.value) {
    message("没有新增公告权限", { type: "warning" });
    return;
  }
  dialogMode.value = "create";
  resetForm();
  dialogVisible.value = true;
}

function openEditDialog(row: AdminAnnouncement) {
  if (!canUpdate.value) {
    message("没有修改公告权限", { type: "warning" });
    return;
  }
  dialogMode.value = "edit";
  form.id = row.id;
  form.title = row.title;
  form.text = row.text;
  form.imageUrl = row.imageUrl ?? "";
  form.icon = row.icon;
  form.display = row.display;
  form.forExistingUsers = row.forExistingUsers;
  form.silence = row.silence;
  form.needConfirmationToRead = row.needConfirmationToRead;
  form.confetti = row.confetti;
  form.userId = row.userId ?? "";
  form.active = row.isActive;
  form.startsAt = parseDate(row.startsAt);
  form.endsAt = parseDate(row.endsAt);
  formRef.value?.clearValidate();
  dialogVisible.value = true;
}

async function saveAnnouncement() {
  if (dialogMode.value === "create" ? !canCreate.value : !canUpdate.value) {
    message("没有保存公告权限", { type: "warning" });
    return;
  }
  const valid = await formRef.value?.validate().catch(() => false);
  if (!valid) return;
  saving.value = true;
  try {
    const payload = buildPayload(dialogMode.value === "create");
    if (dialogMode.value === "create") {
      await createAdminAnnouncement(payload);
    } else {
      await updateAdminAnnouncement(form.id, payload);
    }
    message("公告已保存", { type: "success" });
    dialogVisible.value = false;
    resetCursor();
    await loadAnnouncements();
  } catch {
    message("保存公告失败", { type: "error" });
  } finally {
    saving.value = false;
  }
}

async function handleDelete(row: AdminAnnouncement) {
  if (!canDelete.value) {
    message("没有删除公告权限", { type: "warning" });
    return;
  }
  const confirmed = await ElMessageBox.confirm(
    `确认删除公告「${row.title}」？`,
    "删除公告",
    {
      type: "warning",
      confirmButtonText: "确认",
      cancelButtonText: "取消"
    }
  ).catch(() => false);
  if (!confirmed) return;
  loading.value = true;
  try {
    await deleteAdminAnnouncement(row.id);
    message("公告已删除", { type: "success" });
    resetCursor();
    await loadAnnouncements();
  } catch {
    message("删除公告失败", { type: "error" });
  } finally {
    loading.value = false;
  }
}

onMounted(loadAnnouncements);
</script>

<template>
  <div class="announcements-page">
    <section class="governance-panel">
      <div class="panel-header">
        <div>
          <h2>公告管理</h2>
          <p>维护社区公告内容、展示方式和生效时间</p>
        </div>
        <el-button
          type="primary"
          :icon="useRenderIcon('ri/add-line')"
          :disabled="!canCreate"
          @click="openCreateDialog"
        >
          新增公告
        </el-button>
      </div>

      <el-alert
        v-if="!canList"
        title="当前账号没有 governance:list_announcements 权限"
        type="warning"
        show-icon
        :closable="false"
        class="permission-alert"
      />

      <el-form :inline="true" class="search-form">
        <el-form-item label="状态">
          <el-select v-model="query.status" class="w-32!">
            <el-option
              v-for="item in statusOptions"
              :key="item.value"
              :label="item.label"
              :value="item.value"
            />
          </el-select>
        </el-form-item>
        <el-form-item label="每页">
          <el-select v-model="query.pageSize" class="w-24!">
            <el-option :value="10" label="10 条" />
            <el-option :value="20" label="20 条" />
            <el-option :value="50" label="50 条" />
          </el-select>
        </el-form-item>
        <el-form-item>
          <el-button
            type="primary"
            :icon="useRenderIcon('ri/search-line')"
            :disabled="!canList"
            :loading="loading"
            @click="searchAnnouncements"
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
        :adaptiveConfig="{ offsetBottom: 178 }"
        align-whole="center"
        table-layout="auto"
        :loading="loading"
        :data="announcements"
        :columns="columns"
        :header-cell-style="{
          background: 'var(--el-fill-color-light)',
          color: 'var(--el-text-color-primary)'
        }"
      >
        <template #content="{ row }">
          <div class="announcement-content">
            <strong>{{ row.title }}</strong>
            <span>{{ row.text }}</span>
          </div>
        </template>
        <template #image="{ row }">
          <el-image
            v-if="safeExternalURL(row.imageUrl)"
            class="announcement-thumbnail"
            :src="safeExternalURL(row.imageUrl)"
            :preview-src-list="[safeExternalURL(row.imageUrl)]"
            :alt="row.title"
            fit="cover"
            preview-teleported
          />
          <span v-else>-</span>
        </template>
        <template #display="{ row }">
          {{ displayLabel(row.display) }}
        </template>
        <template #icon="{ row }">
          <el-tag :type="iconMeta(row.icon).type" effect="plain">
            {{ iconMeta(row.icon).label }}
          </el-tag>
        </template>
        <template #schedule="{ row }">
          <div class="schedule-cell">
            <span>{{ formatTime(row.startsAt) }}</span>
            <span>至 {{ formatTime(row.endsAt) }}</span>
          </div>
        </template>
        <template #status="{ row }">
          <el-tag :type="scheduleMeta(row).type">
            {{ scheduleMeta(row).label }}
          </el-tag>
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

      <div class="cursor-pagination">
        <span>第 {{ currentPage }} 页，本页 {{ announcements.length }} 条</span>
        <el-button-group>
          <el-button
            :icon="useRenderIcon('ri/arrow-left-s-line')"
            :disabled="loading || !canGoPrevious"
            @click="goToPreviousPage"
          >
            上一页
          </el-button>
          <el-button
            :disabled="loading || !canGoNext"
            @click="goToNextPage"
          >
            下一页
            <component :is="useRenderIcon('ri/arrow-right-s-line')" />
          </el-button>
        </el-button-group>
      </div>
    </section>

    <el-dialog
      v-model="dialogVisible"
      :title="dialogTitle"
      width="min(680px, calc(100vw - 32px))"
      destroy-on-close
    >
      <el-form ref="formRef" :model="form" :rules="rules" label-width="88px">
        <el-form-item label="标题" prop="title">
          <el-input v-model="form.title" maxlength="256" show-word-limit />
        </el-form-item>
        <el-form-item label="正文" prop="text">
          <el-input
            v-model="form.text"
            type="textarea"
            :rows="6"
            maxlength="20000"
            show-word-limit
          />
        </el-form-item>
        <el-form-item label="图片地址" prop="imageUrl">
          <el-input v-model="form.imageUrl" clearable />
        </el-form-item>
        <el-form-item v-if="previewImageUrl" label="图片预览">
          <el-image
            class="form-image-preview"
            :src="previewImageUrl"
            :preview-src-list="[previewImageUrl]"
            fit="cover"
            preview-teleported
          />
        </el-form-item>
        <el-form-item
          v-if="dialogMode === 'create'"
          label="定向用户 ID"
          prop="userId"
        >
          <el-input v-model="form.userId" clearable />
        </el-form-item>
        <div class="form-grid">
          <el-form-item label="展示方式" prop="display">
            <el-select v-model="form.display" class="w-full!">
              <el-option
                v-for="item in displayOptions"
                :key="item.value"
                :label="item.label"
                :value="item.value"
              />
            </el-select>
          </el-form-item>
          <el-form-item label="级别" prop="icon">
            <el-select v-model="form.icon" class="w-full!">
              <el-option
                v-for="item in iconOptions"
                :key="item.value"
                :label="item.label"
                :value="item.value"
              />
            </el-select>
          </el-form-item>
        </div>
        <div class="form-grid">
          <el-form-item label="开始时间" prop="startsAt">
            <el-date-picker
              v-model="form.startsAt"
              type="datetime"
              class="w-full!"
              placeholder="立即生效"
              clearable
            />
          </el-form-item>
          <el-form-item label="结束时间" prop="endsAt">
            <el-date-picker
              v-model="form.endsAt"
              type="datetime"
              class="w-full!"
              placeholder="长期有效"
              clearable
            />
          </el-form-item>
        </div>
        <div class="form-grid form-switches">
          <el-form-item label="启用">
            <el-switch v-model="form.active" />
          </el-form-item>
          <el-form-item label="仅已有用户">
            <el-switch v-model="form.forExistingUsers" />
          </el-form-item>
          <el-form-item label="静默">
            <el-switch v-model="form.silence" />
          </el-form-item>
          <el-form-item label="确认已读">
            <el-switch v-model="form.needConfirmationToRead" />
          </el-form-item>
          <el-form-item label="彩带效果">
            <el-switch v-model="form.confetti" />
          </el-form-item>
        </div>
      </el-form>
      <template #footer>
        <el-button @click="dialogVisible = false">取消</el-button>
        <el-button type="primary" :loading="saving" @click="saveAnnouncement">
          保存
        </el-button>
      </template>
    </el-dialog>
  </div>
</template>

<style scoped>
.announcements-page {
  padding: 16px;
}

.governance-panel {
  padding: 20px;
  background: var(--el-bg-color);
  border: 1px solid var(--el-border-color-light);
  border-radius: 6px;
}

.panel-header {
  display: flex;
  gap: 16px;
  align-items: flex-start;
  justify-content: space-between;
  margin-bottom: 18px;
}

.panel-header h2 {
  margin: 0 0 4px;
  font-size: 18px;
}

.panel-header p {
  margin: 0;
  color: var(--el-text-color-secondary);
}

.permission-alert {
  margin-bottom: 16px;
}

.search-form {
  margin-bottom: 8px;
}

.announcement-content,
.schedule-cell {
  display: flex;
  flex-direction: column;
  gap: 4px;
  min-width: 0;
}

.announcement-content {
  text-align: left;
}

.announcement-content span {
  overflow: hidden;
  color: var(--el-text-color-secondary);
  text-overflow: ellipsis;
  white-space: nowrap;
}

.announcement-content strong {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.announcement-thumbnail {
  width: 64px;
  height: 48px;
  border-radius: 4px;
}

.cursor-pagination {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-top: 16px;
  color: var(--el-text-color-secondary);
}

.form-grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 16px;
}

.form-switches :deep(.el-form-item) {
  margin-bottom: 12px;
}

.form-image-preview {
  width: 180px;
  height: 100px;
  border-radius: 4px;
}

@media (width <= 720px) {
  .announcements-page {
    padding: 8px;
  }

  .governance-panel {
    padding: 14px;
  }

  .panel-header,
  .cursor-pagination {
    align-items: stretch;
    flex-direction: column;
  }

  .form-grid {
    grid-template-columns: minmax(0, 1fr);
    gap: 0;
  }
}
</style>
