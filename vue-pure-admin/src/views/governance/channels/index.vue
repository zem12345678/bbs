<script setup lang="ts">
import dayjs from "dayjs";
import { computed, onMounted, reactive, ref } from "vue";
import { ElMessageBox } from "element-plus";
import { message } from "@/utils/message";
import { hasPerms } from "@/utils/auth";
import {
  listAdminChannels,
  setAdminChannelArchived,
  setAdminChannelFeatured,
  type AdminChannel
} from "@/api/admin";
import {
  entityIdText,
  normalizeDecimalEntityId,
  normalizeEntityId
} from "@/utils/entityId";
import { useRenderIcon } from "@/components/ReIcon/src/hooks";

defineOptions({
  name: "GovernanceChannels"
});

type ChannelRow = Partial<AdminChannel> & Record<string, any>;

const loading = ref(false);
const channels = ref<AdminChannel[]>([]);
const query = reactive({
  keyword: "",
  categoryId: "",
  archivedStatus: 0,
  pageSize: 20,
  currentPage: 1,
  total: 0
});

const canList = computed(() => hasPerms("governance:list_channels"));
const canFeature = computed(() => hasPerms("governance:feature_channel"));
const canArchive = computed(() => hasPerms("governance:archive_channel"));
const canRestore = computed(() => hasPerms("governance:restore_channel"));

const columns: TableColumnList = [
  { prop: "id", label: "圈子 ID", width: 100 },
  { prop: "name", label: "名称", minWidth: 180, showOverflowTooltip: true },
  {
    prop: "description",
    label: "简介",
    minWidth: 240,
    showOverflowTooltip: true
  },
  { label: "所有者", width: 110, slot: "owner" },
  { label: "分类", width: 110, slot: "category" },
  { label: "内容数据", minWidth: 160, slot: "counts" },
  { label: "精选", width: 90, slot: "featured" },
  { label: "状态", width: 100, slot: "status" },
  { label: "最后发布", width: 170, slot: "lastPostedAt" },
  { label: "更新时间", width: 170, slot: "updatedAt" },
  { label: "操作", fixed: "right", width: 250, slot: "operation" }
];

const archivedOptions = [
  { label: "全部", value: 0 },
  { label: "正常", value: 1 },
  { label: "已归档", value: 2 }
];

function ownerId(row: ChannelRow) {
  return row.owner_id ?? row.ownerId;
}

function categoryId(row: ChannelRow) {
  return row.category_id ?? row.categoryId;
}

function isArchived(row: ChannelRow) {
  return Boolean(row.is_archived ?? row.isArchived);
}

function isFeatured(row: ChannelRow) {
  return Boolean(row.is_featured ?? row.isFeatured);
}

function followersCount(row: ChannelRow) {
  return Number(row.followers_count ?? row.followersCount ?? 0);
}

function topicsCount(row: ChannelRow) {
  return Number(row.topics_count ?? row.topicsCount ?? 0);
}

function lastPostedAt(row: ChannelRow) {
  return row.last_posted_at ?? row.lastPostedAt;
}

function updatedAt(row: ChannelRow) {
  return row.updated_at ?? row.updatedAt;
}

function formatTime(value?: number) {
  if (!value) return "-";
  const timestamp = value > 9999999999 ? value : value * 1000;
  return dayjs(timestamp).format("YYYY-MM-DD HH:mm");
}

async function loadChannels() {
  if (!canList.value) {
    channels.value = [];
    query.total = 0;
    return;
  }
  const normalizedCategoryId = normalizeDecimalEntityId(query.categoryId);
  if (query.categoryId.trim() && !normalizedCategoryId) {
    message("分类 ID 必须是有效的正整数", { type: "warning" });
    return;
  }
  loading.value = true;
  try {
    const {
      code,
      data,
      message: msg
    } = await listAdminChannels({
      q: query.keyword.trim(),
      category_id: normalizedCategoryId,
      archived_status: query.archivedStatus,
      limit: query.pageSize,
      offset: (query.currentPage - 1) * query.pageSize
    });
    if (code !== 0) {
      message(msg || "加载圈子列表失败", { type: "error" });
      return;
    }
    channels.value = data.items ?? [];
    query.total = data.total ?? channels.value.length;
  } finally {
    loading.value = false;
  }
}

function resetQuery() {
  query.keyword = "";
  query.categoryId = "";
  query.archivedStatus = 0;
  query.currentPage = 1;
  loadChannels();
}

async function handleFeatured(row: ChannelRow) {
  if (!canFeature.value) {
    message("没有设置精选圈子权限", { type: "warning" });
    return;
  }
  if (isArchived(row)) {
    message("已归档圈子不能设为精选", { type: "warning" });
    return;
  }
  const id = normalizeEntityId(row.id);
  if (!id) {
    message("圈子 ID 无效", { type: "warning" });
    return;
  }
  const featured = !isFeatured(row);
  loading.value = true;
  try {
    const { code, message: msg } = await setAdminChannelFeatured(id, featured);
    if (code !== 0) {
      message(msg || "更新精选状态失败", { type: "error" });
      return;
    }
    message(featured ? "已设为精选圈子" : "已取消精选", {
      type: "success"
    });
    await loadChannels();
  } finally {
    loading.value = false;
  }
}

async function handleArchived(row: ChannelRow) {
  const archived = isArchived(row);
  const allowed = archived ? canRestore.value : canArchive.value;
  if (!allowed) {
    message(archived ? "没有恢复圈子权限" : "没有归档圈子权限", {
      type: "warning"
    });
    return;
  }
  const id = normalizeEntityId(row.id);
  if (!id) {
    message("圈子 ID 无效", { type: "warning" });
    return;
  }
  const confirmed = await ElMessageBox.confirm(
    archived
      ? `确认恢复圈子「${row.name || id}」？恢复后不会自动设为精选。`
      : `确认归档圈子「${row.name || id}」？归档会同时取消精选。`,
    archived ? "恢复圈子" : "归档圈子",
    {
      type: "warning",
      confirmButtonText: "确认",
      cancelButtonText: "取消"
    }
  ).catch(() => false);
  if (!confirmed) return;
  loading.value = true;
  try {
    const { code, message: msg } = await setAdminChannelArchived(id, !archived);
    if (code !== 0) {
      message(msg || (archived ? "恢复圈子失败" : "归档圈子失败"), {
        type: "error"
      });
      return;
    }
    message(archived ? "圈子已恢复" : "圈子已归档", { type: "success" });
    await loadChannels();
  } finally {
    loading.value = false;
  }
}

function onPageSizeChange(size: number) {
  query.pageSize = size;
  query.currentPage = 1;
  loadChannels();
}

function onCurrentPageChange(page: number) {
  query.currentPage = page;
  loadChannels();
}

onMounted(loadChannels);
</script>

<template>
  <div class="channels-page">
    <section class="governance-panel">
      <div class="panel-header">
        <div>
          <h2>圈子治理</h2>
          <p>维护精选入口和圈子可用状态</p>
        </div>
      </div>

      <el-alert
        v-if="!canList"
        title="当前账号没有 governance:list_channels 权限"
        type="warning"
        show-icon
        :closable="false"
        class="permission-alert"
      />

      <el-form :inline="true" class="search-form">
        <el-form-item label="关键词">
          <el-input
            v-model="query.keyword"
            placeholder="名称或简介"
            clearable
            class="w-48!"
            @keyup.enter="loadChannels"
          />
        </el-form-item>
        <el-form-item label="分类 ID">
          <el-input
            v-model="query.categoryId"
            placeholder="全部分类"
            clearable
            inputmode="numeric"
            class="w-40!"
            @keyup.enter="loadChannels"
          />
        </el-form-item>
        <el-form-item label="归档状态">
          <el-select
            v-model="query.archivedStatus"
            class="w-36!"
            @change="loadChannels"
          >
            <el-option
              v-for="item in archivedOptions"
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
            @click="loadChannels"
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
        :data="channels"
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
        <template #owner="{ row }">#{{ entityIdText(ownerId(row)) }}</template>
        <template #category="{ row }">
          {{ categoryId(row) ? `#${entityIdText(categoryId(row))}` : "-" }}
        </template>
        <template #counts="{ row }">
          <span>主题 {{ topicsCount(row) }}</span>
          <span class="count-divider">关注 {{ followersCount(row) }}</span>
        </template>
        <template #featured="{ row }">
          <el-tag :type="isFeatured(row) ? 'warning' : 'info'" effect="plain">
            {{ isFeatured(row) ? "精选" : "普通" }}
          </el-tag>
        </template>
        <template #status="{ row }">
          <el-tag :type="isArchived(row) ? 'info' : 'success'">
            {{ isArchived(row) ? "已归档" : "正常" }}
          </el-tag>
        </template>
        <template #lastPostedAt="{ row }">
          {{ formatTime(lastPostedAt(row)) }}
        </template>
        <template #updatedAt="{ row }">
          {{ formatTime(updatedAt(row)) }}
        </template>
        <template #operation="{ row }">
          <el-button
            link
            :type="isFeatured(row) ? 'warning' : 'primary'"
            :disabled="!canFeature || isArchived(row)"
            :icon="
              useRenderIcon(isFeatured(row) ? 'ri/star-fill' : 'ri/star-line')
            "
            @click="handleFeatured(row)"
          >
            {{ isFeatured(row) ? "取消精选" : "设为精选" }}
          </el-button>
          <el-button
            link
            :type="isArchived(row) ? 'success' : 'danger'"
            :disabled="isArchived(row) ? !canRestore : !canArchive"
            :icon="
              useRenderIcon(
                isArchived(row) ? 'ri/restart-line' : 'ri/archive-line'
              )
            "
            @click="handleArchived(row)"
          >
            {{ isArchived(row) ? "恢复" : "归档" }}
          </el-button>
        </template>
      </pure-table>
    </section>
  </div>
</template>

<style scoped>
.channels-page {
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

.count-divider {
  margin-left: 12px;
}
</style>
