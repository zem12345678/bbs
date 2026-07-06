<script setup lang="ts">
import dayjs from "dayjs";
import { computed, onMounted, reactive, ref } from "vue";
import { ElMessageBox } from "element-plus";
import { message } from "@/utils/message";
import { hasPerms } from "@/utils/auth";
import {
  archiveAdminTopic,
  hideAdminTopic,
  listAdminTopics,
  type AdminTopic
} from "@/api/admin";
import { useRenderIcon } from "@/components/ReIcon/src/hooks";

defineOptions({
  name: "GovernanceTopics"
});

type TopicRow = Partial<AdminTopic> & Record<string, any>;

const loading = ref(false);
const topics = ref<AdminTopic[]>([]);
const query = reactive({
  status: 2,
  type: "",
  tag: "",
  authorId: undefined as number | undefined,
  pageSize: 20,
  currentPage: 1,
  total: 0
});

const canHide = computed(() => hasPerms("governance:hide_topic"));
const canArchive = computed(() => hasPerms("governance:archive_topic"));
const canList = computed(() => hasPerms("governance:list_topics"));

const statusOptions = [
  { label: "全部", value: 0 },
  { label: "草稿", value: 1 },
  { label: "已发布", value: 2 },
  { label: "已隐藏", value: 3 },
  { label: "已归档", value: 4 }
];

const typeOptions = [
  { label: "全部", value: "" },
  { label: "话题", value: "topic" },
  { label: "动态", value: "tweet" }
];

function statusMeta(status?: number) {
  switch (status) {
    case 1:
      return { label: "草稿", type: "info" as const };
    case 2:
      return { label: "已发布", type: "success" as const };
    case 3:
      return { label: "已隐藏", type: "warning" as const };
    case 4:
      return { label: "已归档", type: "info" as const };
    default:
      return { label: `未知(${status ?? "-"})`, type: "danger" as const };
  }
}

function topicAuthorId(topic: TopicRow) {
  return topic.author_id ?? topic.authorId ?? 0;
}

function topicCreatedAt(topic: TopicRow) {
  return topic.created_at ?? topic.createdAt;
}

function topicPublishedAt(topic: TopicRow) {
  return topic.published_at ?? topic.publishedAt;
}

function formatTime(value?: number) {
  if (!value) return "-";
  const timestamp = value > 9999999999 ? value : value * 1000;
  return dayjs(timestamp).format("YYYY-MM-DD HH:mm");
}

async function loadTopics() {
  if (!canList.value) {
    topics.value = [];
    query.total = 0;
    return;
  }
  loading.value = true;
  try {
    const {
      code,
      data,
      message: msg
    } = await listAdminTopics({
      status: query.status,
      type: query.type,
      tag: query.tag.trim(),
      author_id: query.authorId,
      limit: query.pageSize,
      offset: (query.currentPage - 1) * query.pageSize
    });
    if (code !== 0) {
      message(msg || "加载话题列表失败", { type: "error" });
      return;
    }
    topics.value = data.items ?? [];
    query.total = data.total ?? topics.value.length;
  } finally {
    loading.value = false;
  }
}

function resetQuery() {
  query.status = 2;
  query.type = "";
  query.tag = "";
  query.authorId = undefined;
  query.currentPage = 1;
  loadTopics();
}

async function handleHide(row: TopicRow) {
  if (!canHide.value) {
    message("没有隐藏话题权限", { type: "warning" });
    return;
  }
  const id = Number(row.id);
  if (!id) {
    message("话题 ID 无效", { type: "warning" });
    return;
  }
  await ElMessageBox.confirm(`确认隐藏话题 #${id}？`, "隐藏话题", {
    type: "warning",
    confirmButtonText: "确认",
    cancelButtonText: "取消"
  });
  loading.value = true;
  try {
    const { code, message: msg } = await hideAdminTopic(id);
    if (code !== 0) {
      message(msg || "隐藏失败", { type: "error" });
      return;
    }
    message("话题已隐藏", { type: "success" });
    await loadTopics();
  } finally {
    loading.value = false;
  }
}

async function handleArchive(row: TopicRow) {
  if (!canArchive.value) {
    message("没有归档话题权限", { type: "warning" });
    return;
  }
  const id = Number(row.id);
  if (!id) {
    message("话题 ID 无效", { type: "warning" });
    return;
  }
  await ElMessageBox.confirm(`确认归档话题 #${id}？`, "归档话题", {
    type: "warning",
    confirmButtonText: "确认",
    cancelButtonText: "取消"
  });
  loading.value = true;
  try {
    const { code, message: msg } = await archiveAdminTopic(id);
    if (code !== 0) {
      message(msg || "归档失败", { type: "error" });
      return;
    }
    message("话题已归档", { type: "success" });
    await loadTopics();
  } finally {
    loading.value = false;
  }
}

function onPageSizeChange(size: number) {
  query.pageSize = size;
  query.currentPage = 1;
  loadTopics();
}

function onCurrentPageChange(page: number) {
  query.currentPage = page;
  loadTopics();
}

onMounted(loadTopics);
</script>

<template>
  <div class="topic-governance">
    <section class="topic-panel">
      <div class="panel-header">
        <div>
          <h2>话题管理</h2>
          <p>查看社区话题和动态并处理违规内容</p>
        </div>
      </div>

      <el-alert
        v-if="!canList"
        title="当前账号没有 governance:list_topics 权限"
        type="warning"
        show-icon
        :closable="false"
        class="permission-alert"
      />

      <el-form :inline="true" class="search-form">
        <el-form-item label="状态">
          <el-select v-model="query.status" class="w-40!" @change="loadTopics">
            <el-option
              v-for="item in statusOptions"
              :key="item.value"
              :label="item.label"
              :value="item.value"
            />
          </el-select>
        </el-form-item>
        <el-form-item label="类型">
          <el-select v-model="query.type" class="w-40!" @change="loadTopics">
            <el-option
              v-for="item in typeOptions"
              :key="item.value"
              :label="item.label"
              :value="item.value"
            />
          </el-select>
        </el-form-item>
        <el-form-item label="标签">
          <el-input
            v-model="query.tag"
            placeholder="按标签筛选"
            clearable
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
            @click="loadTopics"
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
        :data="topics"
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
        <el-table-column prop="id" label="话题 ID" width="100" />
        <el-table-column label="类型" width="90">
          <template #default="{ row }">
            <el-tag effect="plain">
              {{ row.type === "tweet" ? "动态" : "话题" }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column
          prop="title"
          label="标题"
          min-width="220"
          show-overflow-tooltip
        />
        <el-table-column
          prop="body"
          label="正文"
          min-width="260"
          show-overflow-tooltip
        />
        <el-table-column label="作者" width="110">
          <template #default="{ row }">#{{ topicAuthorId(row) }}</template>
        </el-table-column>
        <el-table-column label="标签" min-width="180">
          <template #default="{ row }">
            <div class="tag-list">
              <el-tag
                v-for="tag in row.tags || []"
                :key="tag"
                size="small"
                effect="plain"
              >
                {{ tag }}
              </el-tag>
              <span v-if="!row.tags?.length" class="muted-text">-</span>
            </div>
          </template>
        </el-table-column>
        <el-table-column label="状态" width="110">
          <template #default="{ row }">
            <el-tag :type="statusMeta(row.status).type">
              {{ statusMeta(row.status).label }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column label="创建时间" width="170">
          <template #default="{ row }">
            {{ formatTime(topicCreatedAt(row)) }}
          </template>
        </el-table-column>
        <el-table-column label="发布时间" width="170">
          <template #default="{ row }">
            {{ formatTime(topicPublishedAt(row)) }}
          </template>
        </el-table-column>
        <el-table-column label="操作" fixed="right" width="180">
          <template #default="{ row }">
            <el-button
              v-if="row.status === 2 && canHide"
              link
              type="warning"
              @click="handleHide(row)"
            >
              隐藏
            </el-button>
            <el-button
              v-if="row.status !== 4 && canArchive"
              link
              type="danger"
              @click="handleArchive(row)"
            >
              归档
            </el-button>
            <span
              v-if="
                (row.status !== 2 || !canHide) &&
                (row.status === 4 || !canArchive)
              "
              class="muted-text"
            >
              -
            </span>
          </template>
        </el-table-column>
      </pure-table>
    </section>
  </div>
</template>

<style scoped>
.topic-panel {
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

.tag-list {
  display: flex;
  flex-wrap: wrap;
  gap: 4px;
  justify-content: center;
}

.muted-text {
  color: var(--el-text-color-placeholder);
}
</style>
