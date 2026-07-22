<script setup lang="ts">
import dayjs from "dayjs";
import { computed, onMounted, reactive, ref, watch } from "vue";
import { useRoute } from "vue-router";
import { message } from "@/utils/message";
import { hasPerms } from "@/utils/auth";
import { normalizeEntityId } from "@/utils/entityId";
import { downloadCsv, type CsvColumn } from "@/utils/csvExport";
import { loadAllOffsetPages } from "@/utils/offsetPages";
import { useRenderIcon } from "@/components/ReIcon/src/hooks";
import {
  listAdminMallProductReviews,
  updateAdminMallProductReviewStatus,
  type AdminMallProductReview
} from "@/api/admin";

defineOptions({
  name: "MallReviews"
});

type ReviewRow = Partial<AdminMallProductReview> & Record<string, any>;

const route = useRoute();
const loading = ref(false);
const actionId = ref("");
const exporting = ref(false);
const reviews = ref<AdminMallProductReview[]>([]);

const query = reactive({
  productId: "",
  userId: "",
  status: 0,
  pageSize: 20,
  currentPage: 1,
  total: 0
});

const canList = computed(() => hasPerms("mall:list_product_reviews"));
const canUpdate = computed(() => hasPerms("mall:update_product_review"));

const statusOptions = [
  { label: "全部", value: 0 },
  { label: "待审核", value: 1 },
  { label: "已公开", value: 2 },
  { label: "已隐藏", value: 3 }
];

function routeQueryText(...keys: string[]) {
  for (const key of keys) {
    const value = route.query[key];
    const firstValue = Array.isArray(value) ? value[0] : value;
    const text = String(firstValue ?? "").trim();
    if (text) return text;
  }
  return "";
}

function applyRouteQuery() {
  const routeStatus = Number(routeQueryText("status"));
  query.productId = routeQueryText("product_id", "productId");
  query.userId = routeQueryText("user_id", "userId");
  query.status = statusOptions.some(item => item.value === routeStatus)
    ? routeStatus
    : 0;
  query.currentPage = 1;
}

const reviewExportColumns: CsvColumn<ReviewRow>[] = [
  { header: "评价ID", value: row => row.id ?? "" },
  { header: "商品ID", value: productIdOf },
  { header: "商品SKU", value: productSkuOf },
  { header: "商品名称", value: productTitleOf },
  { header: "订单ID", value: orderIdOf },
  { header: "用户ID", value: userIdOf },
  { header: "评分", value: row => Number(row.rating || 0) },
  { header: "状态", value: row => statusMeta(row.status).label },
  { header: "评价内容", value: reviewText },
  { header: "晒单图片数", value: row => reviewImages(row).length },
  { header: "创建时间", value: row => formatTime(createdAt(row)) },
  { header: "更新时间", value: row => formatTime(updatedAt(row)) }
];

const columns: TableColumnList = [
  { prop: "id", label: "ID", width: 90 },
  { label: "商品", minWidth: 220, slot: "product" },
  { label: "用户/订单", minWidth: 170, slot: "userOrder" },
  { label: "评分", width: 130, slot: "rating" },
  { label: "评价内容", minWidth: 340, slot: "content" },
  { label: "状态", width: 100, slot: "status" },
  { label: "更新时间", width: 170, slot: "updatedAt" },
  { label: "操作", fixed: "right", width: 170, slot: "operation" }
];

function statusCode(value?: number | string) {
  if (typeof value === "number") return value;
  switch (String(value || "").toUpperCase()) {
    case "PRODUCT_REVIEW_STATUS_PENDING":
    case "PENDING":
      return 1;
    case "PRODUCT_REVIEW_STATUS_PUBLISHED":
    case "PUBLISHED":
      return 2;
    case "PRODUCT_REVIEW_STATUS_HIDDEN":
    case "HIDDEN":
      return 3;
    default:
      return 0;
  }
}

function statusMeta(value?: number | string) {
  switch (statusCode(value)) {
    case 1:
      return { label: "待审核", type: "warning" as const };
    case 2:
      return { label: "已公开", type: "success" as const };
    case 3:
      return { label: "已隐藏", type: "info" as const };
    default:
      return { label: "未知", type: "danger" as const };
  }
}

function productIdOf(row: ReviewRow) {
  return row.product_id ?? row.productId;
}

function productSkuOf(row: ReviewRow) {
  return row.product_sku ?? row.productSku ?? "";
}

function productTitleOf(row: ReviewRow) {
  return row.product_title ?? row.productTitle ?? "-";
}

function orderIdOf(row: ReviewRow) {
  return row.order_id ?? row.orderId;
}

function userIdOf(row: ReviewRow) {
  return row.user_id ?? row.userId;
}

function updatedAt(row: ReviewRow) {
  return row.updated_at ?? row.updatedAt;
}

function createdAt(row: ReviewRow) {
  return row.created_at ?? row.createdAt;
}

function contentOf(row: ReviewRow) {
  return String(row.content ?? "");
}

function reviewImages(row: ReviewRow) {
  const content = contentOf(row);
  const pattern = /!\[[^\]]*\]\(([^)\s]+)(?:\s+"[^"]*")?\)/g;
  const images: string[] = [];
  let match = pattern.exec(content);
  while (match) {
    images.push(match[1]);
    match = pattern.exec(content);
  }
  return images;
}

function reviewText(row: ReviewRow) {
  return (
    contentOf(row)
      .replace(/!\[[^\]]*\]\(([^)\s]+)(?:\s+"[^"]*")?\)/g, "")
      .replace(/\n{3,}/g, "\n\n")
      .trim() || "未填写评价内容"
  );
}

function formatTime(value?: number) {
  if (!value) return "-";
  const timestamp = value > 9999999999 ? value : value * 1000;
  return dayjs(timestamp).format("YYYY-MM-DD HH:mm");
}

function toEntityId(value: string) {
  const text = String(value || "").trim();
  if (!text) return undefined;
  const n = Number(text);
  return Number.isFinite(n) && n > 0 ? n : undefined;
}

function currentReviewListParams(
  limit = query.pageSize,
  offset = (query.currentPage - 1) * query.pageSize
) {
  return {
    product_id: toEntityId(query.productId),
    user_id: toEntityId(query.userId),
    status: query.status,
    limit,
    offset
  };
}

async function loadReviews() {
  if (!canList.value) {
    reviews.value = [];
    query.total = 0;
    return;
  }
  loading.value = true;
  try {
    const {
      code,
      data,
      message: msg
    } = await listAdminMallProductReviews(currentReviewListParams());
    if (code !== 0) {
      message(msg || "加载评价列表失败", { type: "error" });
      return;
    }
    reviews.value = data.items ?? [];
    query.total = data.total ?? reviews.value.length;
  } finally {
    loading.value = false;
  }
}

async function exportReviews() {
  if (!canList.value) {
    message("没有导出评价权限", { type: "warning" });
    return;
  }
  exporting.value = true;
  try {
    const { code, items, message: msg } = await loadAllOffsetPages(
      ({ limit, offset }) =>
        listAdminMallProductReviews(currentReviewListParams(limit, offset))
    );
    if (code !== 0) {
      message(msg || "导出评价失败", { type: "error" });
      return;
    }
    if (items.length === 0) {
      message("当前筛选条件下没有可导出的评价", { type: "warning" });
      return;
    }
    downloadCsv(
      `mall-product-reviews-${dayjs().format("YYYYMMDDHHmmss")}.csv`,
      reviewExportColumns,
      items
    );
    message(`已导出 ${items.length} 条评价`, { type: "success" });
  } catch (error: any) {
    message(error?.message || "导出评价失败", { type: "error" });
  } finally {
    exporting.value = false;
  }
}

function resetQuery() {
  query.productId = "";
  query.userId = "";
  query.status = 0;
  query.currentPage = 1;
  loadReviews();
}

async function updateReviewStatus(row: ReviewRow, status: number) {
  if (!canUpdate.value) {
    message("没有修改评价状态权限", { type: "warning" });
    return;
  }
  const id = normalizeEntityId(row.id);
  actionId.value = `${id}:${status}`;
  try {
    const { code, message: msg } = await updateAdminMallProductReviewStatus(id, {
      status
    });
    if (code !== 0) {
      message(msg || "评价状态更新失败", { type: "error" });
      return;
    }
    message(status === 2 ? "评价已公开" : "评价已隐藏", { type: "success" });
    await loadReviews();
  } finally {
    actionId.value = "";
  }
}

function onPageSizeChange(size: number) {
  query.pageSize = size;
  query.currentPage = 1;
  loadReviews();
}

function onCurrentPageChange(page: number) {
  query.currentPage = page;
  loadReviews();
}

watch(
  () => [
    route.query.product_id,
    route.query.productId,
    route.query.user_id,
    route.query.userId,
    route.query.status
  ],
  () => {
    applyRouteQuery();
    loadReviews();
  }
);

onMounted(() => {
  applyRouteQuery();
  loadReviews();
});
</script>

<template>
  <div class="mall-reviews-page">
    <section class="mall-panel">
      <div class="panel-header">
        <div>
          <h2>评价管理</h2>
          <p>治理商品评价、晒单内容和前台展示状态</p>
        </div>
        <div class="panel-actions">
          <el-button
            :icon="useRenderIcon('ep/refresh')"
            :loading="loading"
            :disabled="!canList"
            @click="loadReviews"
          >
            刷新
          </el-button>
          <el-button
            type="success"
            plain
            :icon="useRenderIcon('ri/download-2-line')"
            :loading="exporting"
            :disabled="!canList"
            @click="exportReviews"
          >
            导出评价
          </el-button>
        </div>
      </div>

      <el-alert
        v-if="!canList"
        title="当前账号没有 mall:list_product_reviews 权限"
        type="warning"
        show-icon
        :closable="false"
        class="permission-alert"
      />

      <el-form :inline="true" class="search-form">
        <el-form-item label="商品ID">
          <el-input
            v-model="query.productId"
            class="w-36!"
            clearable
            placeholder="商品ID"
            @keyup.enter="loadReviews"
          />
        </el-form-item>
        <el-form-item label="用户ID">
          <el-input
            v-model="query.userId"
            class="w-36!"
            clearable
            placeholder="用户ID"
            @keyup.enter="loadReviews"
          />
        </el-form-item>
        <el-form-item label="状态">
          <el-select
            v-model="query.status"
            class="w-32!"
            @change="loadReviews"
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
            @click="loadReviews"
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
        :data="reviews"
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
        <template #product="{ row }">
          <div class="product-cell">
            <strong>{{ productTitleOf(row) }}</strong>
            <span>#{{ productIdOf(row) }} · {{ productSkuOf(row) || "无SKU" }}</span>
          </div>
        </template>
        <template #userOrder="{ row }">
          <div class="product-cell">
            <strong>用户 #{{ userIdOf(row) }}</strong>
            <span>订单 #{{ orderIdOf(row) }}</span>
          </div>
        </template>
        <template #rating="{ row }">
          <el-rate
            :model-value="Number(row.rating || 0)"
            disabled
            show-score
            score-template="{value}"
          />
        </template>
        <template #content="{ row }">
          <div class="review-content-cell">
            <p>{{ reviewText(row) }}</p>
            <div v-if="reviewImages(row).length > 0" class="review-image-list">
              <el-image
                v-for="(url, index) in reviewImages(row).slice(0, 6)"
                :key="`${url}-${index}`"
                :src="url"
                :preview-src-list="reviewImages(row)"
                :initial-index="index"
                fit="cover"
                preview-teleported
                class="review-image"
              />
            </div>
          </div>
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
            v-if="statusCode(row.status) !== 2"
            link
            type="primary"
            :disabled="!canUpdate"
            :loading="actionId === `${row.id}:2`"
            :icon="useRenderIcon('ri/eye-line')"
            @click="updateReviewStatus(row, 2)"
          >
            公开
          </el-button>
          <el-button
            v-if="statusCode(row.status) !== 3"
            link
            type="warning"
            :disabled="!canUpdate"
            :loading="actionId === `${row.id}:3`"
            :icon="useRenderIcon('ri/eye-off-line')"
            @click="updateReviewStatus(row, 3)"
          >
            隐藏
          </el-button>
        </template>
      </pure-table>
    </section>
  </div>
</template>

<style scoped>
.mall-reviews-page {
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

.panel-actions {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
  justify-content: flex-end;
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

.product-cell {
  display: flex;
  flex-direction: column;
  gap: 4px;
  min-width: 0;
  text-align: left;
}

.product-cell strong {
  overflow: hidden;
  font-weight: 600;
  color: var(--el-text-color-primary);
  text-overflow: ellipsis;
  white-space: nowrap;
}

.product-cell span {
  font-size: 12px;
  color: var(--el-text-color-secondary);
}

.review-content-cell {
  display: flex;
  flex-direction: column;
  gap: 8px;
  min-width: 0;
  text-align: left;
}

.review-content-cell p {
  display: -webkit-box;
  margin: 0;
  overflow: hidden;
  color: var(--el-text-color-primary);
  text-overflow: ellipsis;
  white-space: pre-line;
  -webkit-box-orient: vertical;
  -webkit-line-clamp: 3;
}

.review-image-list {
  display: flex;
  flex-wrap: wrap;
  gap: 6px;
}

.review-image {
  width: 56px;
  height: 56px;
  overflow: hidden;
  background: var(--el-fill-color-light);
  border: 1px solid var(--el-border-color-light);
  border-radius: 6px;
}
</style>
