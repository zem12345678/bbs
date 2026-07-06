<script setup lang="ts">
import dayjs from "dayjs";
import { computed, onMounted, reactive, ref } from "vue";
import { ElMessageBox } from "element-plus";
import { message } from "@/utils/message";
import { hasPerms } from "@/utils/auth";
import {
  archiveAdminArticle,
  hideAdminArticle,
  listAdminArticles,
  type AdminArticle
} from "@/api/admin";
import { useRenderIcon } from "@/components/ReIcon/src/hooks";

defineOptions({
  name: "GovernanceArticles"
});

type ArticleRow = Partial<AdminArticle> & Record<string, any>;

const loading = ref(false);
const articles = ref<AdminArticle[]>([]);
const query = reactive({
  status: 2,
  tag: "",
  authorId: undefined as number | undefined,
  pageSize: 20,
  currentPage: 1,
  total: 0
});

const canHide = computed(() => hasPerms("governance:hide_article"));
const canArchive = computed(() => hasPerms("governance:archive_article"));
const canList = computed(() => hasPerms("governance:list_articles"));

const statusOptions = [
  { label: "全部", value: 0 },
  { label: "草稿", value: 1 },
  { label: "已发布", value: 2 },
  { label: "已隐藏", value: 3 },
  { label: "已归档", value: 4 }
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

function articleAuthorId(article: ArticleRow) {
  return article.author_id ?? article.authorId ?? 0;
}

function articleCreatedAt(article: ArticleRow) {
  return article.created_at ?? article.createdAt;
}

function articlePublishedAt(article: ArticleRow) {
  return article.published_at ?? article.publishedAt;
}

function formatTime(value?: number) {
  if (!value) return "-";
  const timestamp = value > 9999999999 ? value : value * 1000;
  return dayjs(timestamp).format("YYYY-MM-DD HH:mm");
}

async function loadArticles() {
  if (!canList.value) {
    articles.value = [];
    query.total = 0;
    return;
  }
  loading.value = true;
  try {
    const {
      code,
      data,
      message: msg
    } = await listAdminArticles({
      status: query.status,
      tag: query.tag.trim(),
      author_id: query.authorId,
      limit: query.pageSize,
      offset: (query.currentPage - 1) * query.pageSize
    });
    if (code !== 0) {
      message(msg || "加载文章列表失败", { type: "error" });
      return;
    }
    articles.value = data.items ?? [];
    query.total = data.total ?? articles.value.length;
  } finally {
    loading.value = false;
  }
}

function resetQuery() {
  query.status = 2;
  query.tag = "";
  query.authorId = undefined;
  query.currentPage = 1;
  loadArticles();
}

async function handleHide(row: ArticleRow) {
  if (!canHide.value) {
    message("没有隐藏文章权限", { type: "warning" });
    return;
  }
  const id = Number(row.id);
  if (!id) {
    message("文章 ID 无效", { type: "warning" });
    return;
  }
  await ElMessageBox.confirm(`确认隐藏文章 #${id}？`, "隐藏文章", {
    type: "warning",
    confirmButtonText: "确认",
    cancelButtonText: "取消"
  });
  loading.value = true;
  try {
    const { code, message: msg } = await hideAdminArticle(id);
    if (code !== 0) {
      message(msg || "隐藏失败", { type: "error" });
      return;
    }
    message("文章已隐藏", { type: "success" });
    await loadArticles();
  } finally {
    loading.value = false;
  }
}

async function handleArchive(row: ArticleRow) {
  if (!canArchive.value) {
    message("没有归档文章权限", { type: "warning" });
    return;
  }
  const id = Number(row.id);
  if (!id) {
    message("文章 ID 无效", { type: "warning" });
    return;
  }
  await ElMessageBox.confirm(`确认归档文章 #${id}？`, "归档文章", {
    type: "warning",
    confirmButtonText: "确认",
    cancelButtonText: "取消"
  });
  loading.value = true;
  try {
    const { code, message: msg } = await archiveAdminArticle(id);
    if (code !== 0) {
      message(msg || "归档失败", { type: "error" });
      return;
    }
    message("文章已归档", { type: "success" });
    await loadArticles();
  } finally {
    loading.value = false;
  }
}

function onPageSizeChange(size: number) {
  query.pageSize = size;
  query.currentPage = 1;
  loadArticles();
}

function onCurrentPageChange(page: number) {
  query.currentPage = page;
  loadArticles();
}

onMounted(loadArticles);
</script>

<template>
  <div class="article-governance">
    <section class="article-panel">
      <div class="panel-header">
        <div>
          <h2>文章管理</h2>
          <p>查看社区文章并处理违规内容</p>
        </div>
      </div>

      <el-alert
        v-if="!canList"
        title="当前账号没有 governance:list_articles 权限"
        type="warning"
        show-icon
        :closable="false"
        class="permission-alert"
      />

      <el-form :inline="true" class="search-form">
        <el-form-item label="状态">
          <el-select
            v-model="query.status"
            class="w-40!"
            @change="loadArticles"
          >
            <el-option
              v-for="item in statusOptions"
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
            @click="loadArticles"
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
        :data="articles"
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
        <el-table-column prop="id" label="文章 ID" width="100" />
        <el-table-column
          prop="title"
          label="标题"
          min-width="220"
          show-overflow-tooltip
        />
        <el-table-column
          prop="summary"
          label="摘要"
          min-width="240"
          show-overflow-tooltip
        />
        <el-table-column label="作者" width="110">
          <template #default="{ row }">#{{ articleAuthorId(row) }}</template>
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
            {{ formatTime(articleCreatedAt(row)) }}
          </template>
        </el-table-column>
        <el-table-column label="发布时间" width="170">
          <template #default="{ row }">
            {{ formatTime(articlePublishedAt(row)) }}
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
.article-panel {
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
