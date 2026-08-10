<script setup lang="ts">
import dayjs from "dayjs";
import { computed, onMounted, reactive, ref } from "vue";
import { ElMessageBox, type FormInstance, type FormRules } from "element-plus";
import { message } from "@/utils/message";
import { hasPerms } from "@/utils/auth";
import {
  createAdminAd,
  deleteAdminAd,
  listAdminAds,
  updateAdminAd,
  type AdminAd,
  type AdminAdPayload
} from "@/api/admin";
import { normalizeEntityId, type EntityId } from "@/utils/entityId";
import { useRenderIcon } from "@/components/ReIcon/src/hooks";

defineOptions({
  name: "GovernanceAds"
});

type DialogMode = "create" | "edit";
type PublishingFilter = "all" | "active" | "inactive";

type AdFormModel = {
  id: EntityId | "";
  url: string;
  memo: string;
  place: string;
  priority: string;
  ratio: number;
  startsAt: Date | null;
  expiresAt: Date | null;
  imageUrl: string;
  weekdays: number[];
};

const allWeekdayBits = [1, 2, 4, 8, 16, 32, 64];
const weekdayOptions = [
  "周日",
  "周一",
  "周二",
  "周三",
  "周四",
  "周五",
  "周六"
].map((label, index) => ({ label, value: 1 << index }));
const placeOptions = [
  { label: "横幅", value: "horizontal" },
  { label: "大横幅", value: "horizontal-big" },
  { label: "右侧栏", value: "vertical" },
  { label: "内容流", value: "inline" }
];
const priorityOptions = [
  { label: "高", value: "high" },
  { label: "中", value: "middle" },
  { label: "低", value: "low" }
];
const publishingOptions: Array<{
  label: string;
  value: PublishingFilter;
}> = [
  { label: "全部", value: "all" },
  { label: "生效中", value: "active" },
  { label: "未生效", value: "inactive" }
];

const loading = ref(false);
const saving = ref(false);
const dialogVisible = ref(false);
const dialogMode = ref<DialogMode>("create");
const formRef = ref<FormInstance>();
const ads = ref<AdminAd[]>([]);
const untilId = ref<EntityId>();
const cursorHistory = ref<Array<EntityId | undefined>>([]);
let listRequestVersion = 0;

const query = reactive({
  publishing: "all" as PublishingFilter,
  pageSize: 20
});

const form = reactive<AdFormModel>({
  id: "",
  url: "",
  memo: "",
  place: "horizontal",
  priority: "middle",
  ratio: 1,
  startsAt: null,
  expiresAt: null,
  imageUrl: "",
  weekdays: [...allWeekdayBits]
});

const canList = computed(() => hasPerms("governance:list_ads"));
const canCreate = computed(() => hasPerms("governance:create_ad"));
const canUpdate = computed(() => hasPerms("governance:update_ad"));
const canDelete = computed(() => hasPerms("governance:delete_ad"));
const currentPage = computed(() => cursorHistory.value.length + 1);
const canGoPrevious = computed(() => cursorHistory.value.length > 0);
const canGoNext = computed(
  () =>
    ads.value.length === query.pageSize &&
    normalizeEntityId(ads.value.at(-1)?.id) !== undefined
);
const dialogTitle = computed(() =>
  dialogMode.value === "create" ? "新增广告" : "编辑广告"
);
const formImageUrl = computed(() => safeExternalURL(form.imageUrl));

const columns: TableColumnList = [
  { prop: "id", label: "ID", width: 130, showOverflowTooltip: true },
  { label: "素材", width: 112, slot: "image" },
  { prop: "memo", label: "备注", minWidth: 160, showOverflowTooltip: true },
  { label: "跳转地址", minWidth: 220, slot: "url" },
  { label: "位置", width: 100, slot: "place" },
  { label: "优先级", width: 90, slot: "priority" },
  { prop: "ratio", label: "权重", width: 80 },
  { label: "投放日", minWidth: 190, slot: "weekdays" },
  { label: "投放周期", minWidth: 180, slot: "schedule" },
  { label: "状态", width: 90, slot: "status" },
  { label: "操作", fixed: "right", width: 150, slot: "operation" }
];

function validateURL(
  _rule: unknown,
  value: unknown,
  callback: (error?: Error) => void
) {
  callback(
    safeExternalURL(value)
      ? undefined
      : new Error("请输入不含账号密码的有效 http:// 或 https:// 地址")
  );
}

const rules: FormRules = {
  url: [
    { required: true, message: "请输入跳转地址", trigger: "blur" },
    { validator: validateURL, trigger: "blur" }
  ],
  imageUrl: [
    { required: true, message: "请输入图片地址", trigger: "blur" },
    { validator: validateURL, trigger: "blur" }
  ],
  place: [{ required: true, message: "请选择广告位置", trigger: "change" }],
  priority: [{ required: true, message: "请选择优先级", trigger: "change" }],
  ratio: [
    { required: true, message: "请输入权重", trigger: "change" },
    {
      validator: (_rule, value, callback) => {
        callback(
          Number.isInteger(value) && value > 0
            ? undefined
            : new Error("权重必须是正整数")
        );
      },
      trigger: "change"
    }
  ],
  startsAt: [{ required: true, message: "请选择开始时间", trigger: "change" }],
  expiresAt: [
    { required: true, message: "请选择结束时间", trigger: "change" },
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
  ],
  weekdays: [
    {
      type: "array",
      required: true,
      min: 1,
      message: "请至少选择一个投放日",
      trigger: "change"
    }
  ]
};

function safeExternalURL(value: unknown) {
  const raw = typeof value === "string" ? value.trim() : "";
  if (!/^https?:\/\/[^/?#\\\s]+/i.test(raw)) return "";
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

function placeLabel(value: string) {
  return placeOptions.find(item => item.value === value)?.label ?? value;
}

function priorityMeta(value: string) {
  switch (value) {
    case "high":
      return { label: "高", type: "danger" as const };
    case "middle":
      return { label: "中", type: "warning" as const };
    case "low":
      return { label: "低", type: "info" as const };
    default:
      return { label: value || "-", type: "info" as const };
  }
}

function scheduleMeta(row: AdminAd) {
  const startsAt = dayjs(row.startsAt).valueOf();
  const expiresAt = dayjs(row.expiresAt).valueOf();
  const now = Date.now();
  if (Number.isNaN(startsAt) || Number.isNaN(expiresAt)) {
    return { label: "时间异常", type: "danger" as const };
  }
  if (startsAt > now) return { label: "待生效", type: "warning" as const };
  if (expiresAt <= now) return { label: "已结束", type: "info" as const };
  return { label: "生效中", type: "success" as const };
}

function formatTime(value: string) {
  const parsed = dayjs(value);
  return parsed.isValid() ? parsed.format("YYYY-MM-DD HH:mm") : "-";
}

function weekdayText(mask: number) {
  return (
    weekdayOptions
      .filter(item => (Number(mask) & item.value) !== 0)
      .map(item => item.label.replace("周", ""))
      .join("、") || "-"
  );
}

function maskToWeekdays(mask: number) {
  return weekdayOptions
    .filter(item => (Number(mask) & item.value) !== 0)
    .map(item => item.value);
}

function weekdaysToMask(weekdays: number[]) {
  return weekdays.reduce((mask, bit) => mask | bit, 0);
}

function resetCursor() {
  untilId.value = undefined;
  cursorHistory.value = [];
}

function resetFormModel() {
  const now = new Date();
  form.id = "";
  form.url = "";
  form.memo = "";
  form.place = "horizontal";
  form.priority = "middle";
  form.ratio = 1;
  form.startsAt = now;
  form.expiresAt = dayjs(now).add(30, "day").toDate();
  form.imageUrl = "";
  form.weekdays = [...allWeekdayBits];
  formRef.value?.clearValidate();
}

function dateValue(value: string) {
  const parsed = dayjs(value);
  return parsed.isValid() ? parsed.toDate() : null;
}

function buildPayload(): AdminAdPayload {
  return {
    url: form.url.trim(),
    memo: form.memo.trim(),
    place: form.place,
    priority: form.priority,
    ratio: form.ratio,
    startsAt: form.startsAt?.getTime() ?? 0,
    expiresAt: form.expiresAt?.getTime() ?? 0,
    imageUrl: form.imageUrl.trim(),
    dayOfWeek: weekdaysToMask(form.weekdays)
  };
}

async function loadAds() {
  if (!canList.value) {
    ads.value = [];
    return;
  }
  const requestVersion = ++listRequestVersion;
  const publishingValue =
    query.publishing === "all" ? null : query.publishing === "active";
  loading.value = true;
  try {
    const data = await listAdminAds({
      limit: query.pageSize,
      untilId: untilId.value,
      publishing: publishingValue
    });
    if (requestVersion !== listRequestVersion) return;
    ads.value = Array.isArray(data) ? data : [];
  } catch {
    if (requestVersion === listRequestVersion) {
      ads.value = [];
      message("加载广告失败", { type: "error" });
    }
  } finally {
    if (requestVersion === listRequestVersion) loading.value = false;
  }
}

function searchAds() {
  resetCursor();
  loadAds();
}

function resetQuery() {
  query.publishing = "all";
  query.pageSize = 20;
  searchAds();
}

function goToPreviousPage() {
  if (!canGoPrevious.value) return;
  untilId.value = cursorHistory.value.pop();
  loadAds();
}

function goToNextPage() {
  const nextCursor = normalizeEntityId(ads.value.at(-1)?.id);
  if (!canGoNext.value || nextCursor === undefined) return;
  cursorHistory.value.push(untilId.value);
  untilId.value = nextCursor;
  loadAds();
}

function openCreateDialog() {
  if (!canCreate.value) {
    message("没有新增广告权限", { type: "warning" });
    return;
  }
  dialogMode.value = "create";
  resetFormModel();
  dialogVisible.value = true;
}

function openEditDialog(row: AdminAd) {
  if (!canUpdate.value) {
    message("没有修改广告权限", { type: "warning" });
    return;
  }
  const id = normalizeEntityId(row.id);
  if (id === undefined) {
    message("广告 ID 无效", { type: "warning" });
    return;
  }
  dialogMode.value = "edit";
  form.id = id;
  form.url = row.url;
  form.memo = row.memo;
  form.place = row.place;
  form.priority = row.priority;
  form.ratio = Number(row.ratio || 1);
  form.startsAt = dateValue(row.startsAt);
  form.expiresAt = dateValue(row.expiresAt);
  form.imageUrl = row.imageUrl;
  form.weekdays = maskToWeekdays(row.dayOfWeek);
  formRef.value?.clearValidate();
  dialogVisible.value = true;
}

async function saveAd() {
  const requiredPermission =
    dialogMode.value === "create" ? canCreate.value : canUpdate.value;
  if (!requiredPermission) {
    message("没有保存广告权限", { type: "warning" });
    return;
  }
  const valid = await formRef.value?.validate().catch(() => false);
  if (!valid) return;
  const payload = buildPayload();
  saving.value = true;
  try {
    if (dialogMode.value === "create") {
      await createAdminAd(payload);
    } else {
      await updateAdminAd(form.id, payload);
    }
    message("广告已保存", { type: "success" });
    dialogVisible.value = false;
    resetCursor();
    await loadAds();
  } catch {
    message("保存广告失败", { type: "error" });
  } finally {
    saving.value = false;
  }
}

async function handleDelete(row: AdminAd) {
  if (!canDelete.value) {
    message("没有删除广告权限", { type: "warning" });
    return;
  }
  const id = normalizeEntityId(row.id);
  if (id === undefined) {
    message("广告 ID 无效", { type: "warning" });
    return;
  }
  const confirmed = await ElMessageBox.confirm(
    `确认删除广告「${row.memo || id}」？`,
    "删除广告",
    {
      type: "warning",
      confirmButtonText: "确认",
      cancelButtonText: "取消"
    }
  ).catch(() => false);
  if (!confirmed) return;
  loading.value = true;
  try {
    await deleteAdminAd(id);
    message("广告已删除", { type: "success" });
    resetCursor();
    await loadAds();
  } catch {
    message("删除广告失败", { type: "error" });
  } finally {
    loading.value = false;
  }
}

onMounted(loadAds);
</script>

<template>
  <div class="ads-page">
    <section class="governance-panel">
      <div class="panel-header">
        <div>
          <h2>广告管理</h2>
          <p>维护广告素材、投放位置、时段和星期计划</p>
        </div>
        <el-button
          type="primary"
          :icon="useRenderIcon('ri/add-line')"
          :disabled="!canCreate"
          @click="openCreateDialog"
        >
          新增广告
        </el-button>
      </div>

      <el-alert
        v-if="!canList"
        title="当前账号没有 governance:list_ads 权限"
        type="warning"
        show-icon
        :closable="false"
        class="permission-alert"
      />

      <el-form :inline="true" class="search-form">
        <el-form-item label="投放状态">
          <el-select v-model="query.publishing" class="w-36!">
            <el-option
              v-for="item in publishingOptions"
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
            <el-option :value="100" label="100 条" />
          </el-select>
        </el-form-item>
        <el-form-item>
          <el-button
            type="primary"
            :icon="useRenderIcon('ri/search-line')"
            :disabled="!canList"
            :loading="loading"
            @click="searchAds"
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
        :data="ads"
        :columns="columns"
        :header-cell-style="{
          background: 'var(--el-fill-color-light)',
          color: 'var(--el-text-color-primary)'
        }"
      >
        <template #image="{ row }">
          <el-image
            v-if="safeExternalURL(row.imageUrl)"
            class="ad-thumbnail"
            :src="safeExternalURL(row.imageUrl)"
            :preview-src-list="[safeExternalURL(row.imageUrl)]"
            :alt="row.memo || '广告图片'"
            fit="cover"
            preview-teleported
          >
            <template #error>
              <span class="image-fallback">加载失败</span>
            </template>
          </el-image>
          <span v-else>-</span>
        </template>
        <template #url="{ row }">
          <el-link
            v-if="safeExternalURL(row.url)"
            type="primary"
            :href="safeExternalURL(row.url)"
            target="_blank"
            rel="noreferrer noopener"
            :underline="false"
          >
            {{ row.url }}
          </el-link>
          <span v-else>{{ row.url || "-" }}</span>
        </template>
        <template #place="{ row }">
          {{ placeLabel(row.place) }}
        </template>
        <template #priority="{ row }">
          <el-tag :type="priorityMeta(row.priority).type" effect="plain">
            {{ priorityMeta(row.priority).label }}
          </el-tag>
        </template>
        <template #weekdays="{ row }">
          {{ weekdayText(row.dayOfWeek) }}
        </template>
        <template #schedule="{ row }">
          <div class="schedule-cell">
            <span>{{ formatTime(row.startsAt) }}</span>
            <span>至 {{ formatTime(row.expiresAt) }}</span>
          </div>
        </template>
        <template #status="{ row }">
          <el-tag :type="scheduleMeta(row).type">
            {{ scheduleMeta(row).label }}
          </el-tag>
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
        <span>第 {{ currentPage }} 页，本页 {{ ads.length }} 条</span>
        <el-button-group>
          <el-button
            :icon="useRenderIcon('ri/arrow-left-s-line')"
            :disabled="loading || !canGoPrevious"
            @click="goToPreviousPage"
          >
            上一页
          </el-button>
          <el-button :disabled="loading || !canGoNext" @click="goToNextPage">
            下一页
            <component :is="useRenderIcon('ri/arrow-right-s-line')" />
          </el-button>
        </el-button-group>
      </div>
    </section>

    <el-dialog
      v-model="dialogVisible"
      :title="dialogTitle"
      width="min(760px, calc(100vw - 32px))"
      destroy-on-close
    >
      <el-form
        ref="formRef"
        :model="form"
        :rules="rules"
        label-width="88px"
        class="ad-form"
      >
        <el-form-item label="广告图片" prop="imageUrl">
          <div class="image-url-field">
            <el-input
              v-model="form.imageUrl"
              placeholder="https://example.com/ad.png"
            />
            <el-image
              v-if="formImageUrl"
              class="form-thumbnail"
              :src="formImageUrl"
              :preview-src-list="[formImageUrl]"
              alt="广告图片预览"
              fit="cover"
              preview-teleported
            />
          </div>
        </el-form-item>
        <el-form-item label="跳转地址" prop="url">
          <el-input v-model="form.url" placeholder="https://example.com" />
        </el-form-item>
        <el-form-item label="运营备注">
          <el-input
            v-model="form.memo"
            type="textarea"
            :rows="3"
            maxlength="240"
            show-word-limit
            placeholder="广告名称、活动或投放说明"
          />
        </el-form-item>

        <el-row :gutter="16">
          <el-col :xs="24" :sm="8">
            <el-form-item label="投放位置" prop="place">
              <el-select v-model="form.place" class="field-control">
                <el-option
                  v-for="item in placeOptions"
                  :key="item.value"
                  :label="item.label"
                  :value="item.value"
                />
              </el-select>
            </el-form-item>
          </el-col>
          <el-col :xs="24" :sm="8">
            <el-form-item label="优先级" prop="priority">
              <el-select v-model="form.priority" class="field-control">
                <el-option
                  v-for="item in priorityOptions"
                  :key="item.value"
                  :label="item.label"
                  :value="item.value"
                />
              </el-select>
            </el-form-item>
          </el-col>
          <el-col :xs="24" :sm="8">
            <el-form-item label="权重" prop="ratio">
              <el-input-number
                v-model="form.ratio"
                :min="1"
                :precision="0"
                controls-position="right"
                class="field-control"
              />
            </el-form-item>
          </el-col>
        </el-row>

        <el-row :gutter="16">
          <el-col :xs="24" :sm="12">
            <el-form-item label="开始时间" prop="startsAt">
              <el-date-picker
                v-model="form.startsAt"
                type="datetime"
                placeholder="选择开始时间"
                class="field-control"
                @change="formRef?.validateField('expiresAt')"
              />
            </el-form-item>
          </el-col>
          <el-col :xs="24" :sm="12">
            <el-form-item label="结束时间" prop="expiresAt">
              <el-date-picker
                v-model="form.expiresAt"
                type="datetime"
                placeholder="选择结束时间"
                class="field-control"
              />
            </el-form-item>
          </el-col>
        </el-row>

        <el-form-item label="投放日" prop="weekdays">
          <el-checkbox-group v-model="form.weekdays" class="weekday-editor">
            <el-checkbox
              v-for="item in weekdayOptions"
              :key="item.value"
              :value="item.value"
            >
              {{ item.label }}
            </el-checkbox>
          </el-checkbox-group>
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="dialogVisible = false">取消</el-button>
        <el-button type="primary" :loading="saving" @click="saveAd">
          保存
        </el-button>
      </template>
    </el-dialog>
  </div>
</template>

<style scoped>
.ads-page {
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

.ad-thumbnail {
  width: 80px;
  height: 48px;
  cursor: zoom-in;
  border: 1px solid var(--el-border-color-light);
  border-radius: 4px;
}

.image-fallback {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 100%;
  height: 100%;
  font-size: 12px;
  color: var(--el-text-color-secondary);
  background: var(--el-fill-color-light);
}

.schedule-cell {
  display: flex;
  flex-direction: column;
  gap: 2px;
  line-height: 1.4;
}

.cursor-pagination {
  display: flex;
  gap: 16px;
  align-items: center;
  justify-content: flex-end;
  padding-top: 14px;
  font-size: 13px;
  color: var(--el-text-color-secondary);
}

.ad-form {
  padding-right: 10px;
}

.image-url-field {
  display: grid;
  grid-template-columns: minmax(0, 1fr) 80px;
  gap: 12px;
  align-items: center;
  width: 100%;
}

.form-thumbnail {
  width: 80px;
  height: 48px;
  cursor: zoom-in;
  border: 1px solid var(--el-border-color-light);
  border-radius: 4px;
}

.field-control {
  width: 100%;
}

.weekday-editor {
  display: flex;
  flex-wrap: wrap;
  gap: 0 18px;
}

.weekday-editor :deep(.el-checkbox) {
  margin-right: 0;
}

@media (width <= 768px) {
  .panel-header {
    flex-direction: column;
  }

  .cursor-pagination {
    flex-direction: column;
    align-items: flex-end;
  }

  .ad-form {
    padding-right: 0;
  }
}
</style>
