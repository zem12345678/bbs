<script setup lang="ts">
import dayjs from "dayjs";
import { computed, onMounted, reactive, ref } from "vue";
import { ElMessageBox } from "element-plus";
import { message } from "@/utils/message";
import { hasPerms } from "@/utils/auth";
import {
  hideAdminComment,
  listAdminComments,
  type AdminComment
} from "@/api/admin";
import { useRenderIcon } from "@/components/ReIcon/src/hooks";

defineOptions({
  name: "GovernanceComments"
});

type CommentRow = Partial<AdminComment> & Record<string, any>;

const loading = ref(false);
const comments = ref<AdminComment[]>([]);
const query = reactive({
  status: -1,
  entityId: undefined as number | undefined,
  authorId: undefined as number | undefined,
  pageSize: 20,
  currentPage: 1,
  total: 0
});

const canHide = computed(() => hasPerms("governance:hide_comment"));
const canList = computed(() => hasPerms("governance:list_comments"));

const statusOptions = [
  { label: "全部", value: -1 },
  { label: "可见", value: 1 },
  { label: "已隐藏", value: 0 }
];

function statusMeta(status?: number) {
  switch (status) {
    case 1:
      return { label: "可见", type: "success" as const };
    case 0:
      return { label: "已隐藏", type: "info" as const };
    default:
      return { label: `未知(${status ?? "-"})`, type: "danger" as const };
  }
}

function entityId(row: CommentRow) {
  return row.entity_id ?? row.entityId ?? 0;
}

function rootId(row: CommentRow) {
  return row.root_id ?? row.rootId ?? 0;
}

function parentId(row: CommentRow) {
  return row.parent_id ?? row.parentId ?? 0;
}

function authorId(row: CommentRow) {
  return row.author_id ?? row.authorId ?? 0;
}

function createdAt(row: CommentRow) {
  return row.created_at ?? row.createdAt;
}

function formatTime(value?: number) {
  if (!value) return "-";
  const timestamp = value > 9999999999 ? value : value * 1000;
  return dayjs(timestamp).format("YYYY-MM-DD HH:mm");
}

async function loadComments() {
  if (!canList.value) {
    comments.value = [];
    query.total = 0;
    return;
  }
  loading.value = true;
  try {
    const {
      code,
      data,
      message: msg
    } = await listAdminComments({
      entity_type: "article",
      entity_id: query.entityId,
      author_id: query.authorId,
      status: query.status,
      page: query.currentPage,
      page_size: query.pageSize
    });
    if (code !== 0) {
      message(msg || "加载评论列表失败", { type: "error" });
      return;
    }
    comments.value = data.items ?? [];
    query.total = data.total ?? 0;
  } finally {
    loading.value = false;
  }
}

function resetQuery() {
  query.status = -1;
  query.entityId = undefined;
  query.authorId = undefined;
  query.currentPage = 1;
  loadComments();
}

async function handleHide(row: CommentRow) {
  if (!canHide.value) {
    message("没有隐藏评论权限", { type: "warning" });
    return;
  }
  const id = Number(row.id);
  if (!id) {
    message("评论 ID 无效", { type: "warning" });
    return;
  }
  await ElMessageBox.confirm(`确认隐藏评论 #${id}？`, "隐藏评论", {
    type: "warning",
    confirmButtonText: "确认",
    cancelButtonText: "取消"
  });
  loading.value = true;
  try {
    const { code, message: msg } = await hideAdminComment(id);
    if (code !== 0) {
      message(msg || "隐藏失败", { type: "error" });
      return;
    }
    message("评论已隐藏", { type: "success" });
    await loadComments();
  } finally {
    loading.value = false;
  }
}

function onPageSizeChange(size: number) {
  query.pageSize = size;
  query.currentPage = 1;
  loadComments();
}

function onCurrentPageChange(page: number) {
  query.currentPage = page;
  loadComments();
}

onMounted(loadComments);
</script>

<template>
  <div class="comment-governance">
    <section class="comment-panel">
      <div class="panel-header">
        <div>
          <h2>评论管理</h2>
          <p>按时间查看社区评论并隐藏违规内容</p>
        </div>
      </div>

      <el-alert
        v-if="!canList"
        title="当前账号没有 governance:list_comments 权限"
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
            @change="loadComments"
          >
            <el-option
              v-for="item in statusOptions"
              :key="item.value"
              :label="item.label"
              :value="item.value"
            />
          </el-select>
        </el-form-item>
        <el-form-item label="文章 ID">
          <el-input-number
            v-model="query.entityId"
            :min="1"
            :step="1"
            :controls="false"
            class="w-40!"
          />
        </el-form-item>
        <el-form-item label="作者 ID">
          <el-input-number
            v-model="query.authorId"
            :min="1"
            :step="1"
            :controls="false"
            class="w-40!"
          />
        </el-form-item>
        <el-form-item>
          <el-button
            type="primary"
            :icon="useRenderIcon('ri/search-line')"
            :disabled="!canList"
            :loading="loading"
            @click="loadComments"
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
        :data="comments"
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
        <el-table-column prop="id" label="评论 ID" width="120" />
        <el-table-column label="文章" width="120">
          <template #default="{ row }">#{{ entityId(row) }}</template>
        </el-table-column>
        <el-table-column label="楼层" width="130">
          <template #default="{ row }">
            {{ rootId(row) ? `回复 #${rootId(row)}` : "主评论" }}
          </template>
        </el-table-column>
        <el-table-column label="父评论" width="120">
          <template #default="{ row }">
            {{ parentId(row) ? `#${parentId(row)}` : "-" }}
          </template>
        </el-table-column>
        <el-table-column label="作者" width="120">
          <template #default="{ row }">#{{ authorId(row) }}</template>
        </el-table-column>
        <el-table-column
          prop="content"
          label="内容"
          min-width="320"
          show-overflow-tooltip
        />
        <el-table-column label="状态" width="110">
          <template #default="{ row }">
            <el-tag :type="statusMeta(row.status).type">
              {{ statusMeta(row.status).label }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column label="时间" width="170">
          <template #default="{ row }">
            {{ formatTime(createdAt(row)) }}
          </template>
        </el-table-column>
        <el-table-column label="操作" fixed="right" width="120">
          <template #default="{ row }">
            <el-button
              v-if="row.status === 1 && canHide"
              link
              type="danger"
              @click="handleHide(row)"
            >
              隐藏
            </el-button>
            <span v-if="row.status !== 1 || !canHide" class="muted-text">
              -
            </span>
          </template>
        </el-table-column>
      </pure-table>
    </section>
  </div>
</template>

<style scoped>
.comment-panel {
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

.search-form {
  padding: 12px 0 4px;
}

.permission-alert {
  margin-bottom: 8px;
}

.muted-text {
  color: var(--el-text-color-placeholder);
}
</style>
