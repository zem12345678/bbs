<script setup lang="ts">
import dayjs from "dayjs";
import { computed, onMounted, reactive, ref } from "vue";
import { type FormInstance, type FormRules } from "element-plus";
import { message } from "@/utils/message";
import { hasPerms } from "@/utils/auth";
import { normalizeEntityId } from "@/utils/entityId";
import { useRenderIcon } from "@/components/ReIcon/src/hooks";
import {
  createAdminMallProduct,
  listAdminMallProductCategories,
  listAdminMallProductStockLogs,
  listAdminMallProducts,
  updateAdminMallProduct,
  type AdminMallProduct,
  type AdminMallProductCategory,
  type AdminMallProductPayload,
  type AdminMallProductStockLog
} from "@/api/admin";

defineOptions({
  name: "MallProducts"
});

type ProductRow = Partial<AdminMallProduct> & Record<string, any>;
type StockLogRow = Partial<AdminMallProductStockLog> & Record<string, any>;
type DialogMode = "create" | "edit";

const loading = ref(false);
const saving = ref(false);
const dialogVisible = ref(false);
const dialogMode = ref<DialogMode>("create");
const formRef = ref<FormInstance>();
const products = ref<AdminMallProduct[]>([]);
const productCategories = ref<AdminMallProductCategory[]>([]);
const stockLogDrawerVisible = ref(false);
const stockLogLoading = ref(false);
const stockLogProduct = ref<ProductRow>();
const stockLogs = ref<AdminMallProductStockLog[]>([]);

const query = reactive({
  keyword: "",
  category: "",
  status: 0,
  pageSize: 20,
  currentPage: 1,
  total: 0
});

const stockLogQuery = reactive({
  reason: "",
  pageSize: 20,
  currentPage: 1,
  total: 0
});

const form = reactive({
  id: 0,
  sku: "",
  title: "",
  description: "",
  category: "",
  cover_url: "",
  price_credits: 0,
  stock: 0,
  status: 2,
  sort: 0
});

const canList = computed(() => hasPerms("mall:list_products"));
const canCreate = computed(() => hasPerms("mall:create_product"));
const canUpdate = computed(() => hasPerms("mall:update_product"));
const canListCategories = computed(() => hasPerms("mall:list_product_categories"));

const dialogTitle = computed(() =>
  dialogMode.value === "create" ? "新增商品" : "编辑商品"
);

const categoryOptions = computed(() => {
  const options = new Map<string, string>();
  for (const item of productCategories.value) {
    if (item.slug) {
      options.set(item.slug, item.name || item.slug);
    }
  }
  for (const item of products.value) {
    const category = String(item.category || "").trim();
    if (category && !options.has(category)) {
      options.set(category, category);
    }
  }
  for (const category of [query.category, form.category]) {
    const value = String(category || "").trim();
    if (value && !options.has(value)) {
      options.set(value, value);
    }
  }
  return Array.from(options.entries()).map(([value, label]) => ({
    value,
    label
  }));
});

const columns: TableColumnList = [
  { prop: "id", label: "ID", width: 90 },
  { label: "封面", width: 90, slot: "cover" },
  { prop: "sku", label: "SKU", minWidth: 140, showOverflowTooltip: true },
  { prop: "title", label: "商品名称", minWidth: 220, showOverflowTooltip: true },
  { label: "分类", minWidth: 140, slot: "category" },
  { label: "售价", width: 110, slot: "price" },
  { prop: "stock", label: "库存", width: 90 },
  { label: "销量", width: 90, slot: "sales" },
  { label: "状态", width: 100, slot: "status" },
  { prop: "sort", label: "排序", width: 90 },
  { label: "更新时间", width: 170, slot: "updatedAt" },
  { label: "操作", fixed: "right", width: 190, slot: "operation" }
];

const statusOptions = [
  { label: "全部", value: 0 },
  { label: "草稿", value: 1 },
  { label: "上架", value: 2 },
  { label: "归档", value: 3 }
];

const editableStatusOptions = statusOptions.filter(item => item.value > 0);

const stockReasonOptions = [
  { label: "全部", value: "" },
  { label: "初始库存", value: "product_created" },
  { label: "人工调整", value: "manual_adjustment" },
  { label: "下单锁定", value: "order_created" },
  { label: "取消释放", value: "order_canceled" },
  { label: "超时释放", value: "order_expired" },
  { label: "售后恢复", value: "refund_restored" }
];

const rules: FormRules = {
  sku: [
    { required: true, message: "请输入 SKU", trigger: "blur" },
    {
      pattern: /^[A-Za-z0-9_.-]+$/,
      message: "SKU 只允许字母、数字、下划线、点和短横线",
      trigger: "blur"
    }
  ],
  title: [
    { required: true, message: "请输入商品名称", trigger: "blur" },
    { min: 1, max: 120, message: "名称长度需在 1-120 个字符", trigger: "blur" }
  ],
  price_credits: [
    { required: true, message: "请输入积分售价", trigger: "change" }
  ],
  stock: [{ required: true, message: "请输入库存", trigger: "change" }],
  status: [{ required: true, message: "请选择状态", trigger: "change" }]
};

function statusMeta(status?: number) {
  switch (status) {
    case 1:
      return { label: "草稿", type: "info" as const };
    case 2:
      return { label: "上架", type: "success" as const };
    case 3:
      return { label: "归档", type: "warning" as const };
    default:
      return { label: `未知(${status ?? "-"})`, type: "danger" as const };
  }
}

function coverOf(row: ProductRow) {
  return row.cover_url ?? row.coverUrl ?? "";
}

function priceOf(row: ProductRow) {
  return Number(row.price_credits ?? row.priceCredits ?? 0);
}

function salesOf(row: ProductRow) {
  return Number(row.sales_count ?? row.salesCount ?? 0);
}

function categoryText(value?: string) {
  const category = String(value || "").trim();
  if (!category) return "未分类";
  const match = productCategories.value.find(item => item.slug === category);
  return match?.name ? `${match.name} (${category})` : category;
}

function deltaOf(row: StockLogRow) {
  return Number(row.delta ?? 0);
}

function beforeStockOf(row: StockLogRow) {
  return Number(row.before_stock ?? row.beforeStock ?? 0);
}

function afterStockOf(row: StockLogRow) {
  return Number(row.after_stock ?? row.afterStock ?? 0);
}

function referenceText(row: StockLogRow) {
  const type = row.reference_type ?? row.referenceType ?? "";
  const id = row.reference_id ?? row.referenceId;
  if (!type || !id) return "-";
  const typeLabel =
    type === "order" ? "订单" : type === "refund" ? "售后" : "商品";
  return `${typeLabel} #${id}`;
}

function operatorText(row: StockLogRow) {
  const type = row.operator_type ?? row.operatorType ?? "";
  const id = row.operator_id ?? row.operatorId ?? "";
  if (!type && !id) return "-";
  const typeLabel = type === "user" ? "用户" : type === "admin" ? "管理员" : type;
  return id ? `${typeLabel} ${id}` : typeLabel || "-";
}

function reasonMeta(reason?: string) {
  switch (reason) {
    case "product_created":
      return { label: "初始库存", type: "info" as const };
    case "manual_adjustment":
      return { label: "人工调整", type: "warning" as const };
    case "order_created":
      return { label: "下单锁定", type: "primary" as const };
    case "order_canceled":
      return { label: "取消释放", type: "success" as const };
    case "order_expired":
      return { label: "超时释放", type: "danger" as const };
    case "refund_restored":
      return { label: "售后恢复", type: "success" as const };
    default:
      return { label: reason || "-", type: "info" as const };
  }
}

function updatedAt(row: ProductRow) {
  return row.updated_at ?? row.updatedAt;
}

function createdAt(row: StockLogRow) {
  return row.created_at ?? row.createdAt;
}

function formatTime(value?: number) {
  if (!value) return "-";
  const timestamp = value > 9999999999 ? value : value * 1000;
  return dayjs(timestamp).format("YYYY-MM-DD HH:mm");
}

function resetFormModel() {
  form.id = 0;
  form.sku = "";
  form.title = "";
  form.description = "";
  form.category = "";
  form.cover_url = "";
  form.price_credits = 0;
  form.stock = 0;
  form.status = 2;
  form.sort = 0;
  formRef.value?.clearValidate();
}

function buildPayload(): AdminMallProductPayload {
  return {
    sku: form.sku.trim(),
    title: form.title.trim(),
    description: form.description.trim(),
    category: form.category.trim(),
    cover_url: form.cover_url.trim(),
    price_credits: Number(form.price_credits || 0),
    stock: Number(form.stock || 0),
    status: Number(form.status || 2),
    sort: Number(form.sort || 0)
  };
}

async function loadProducts() {
  if (!canList.value) {
    products.value = [];
    query.total = 0;
    return;
  }
  loading.value = true;
  try {
    const {
      code,
      data,
      message: msg
    } = await listAdminMallProducts({
      keyword: query.keyword.trim(),
      category: query.category.trim(),
      status: query.status,
      limit: query.pageSize,
      offset: (query.currentPage - 1) * query.pageSize
    });
    if (code !== 0) {
      message(msg || "加载商品列表失败", { type: "error" });
      return;
    }
    products.value = data.items ?? [];
    query.total = data.total ?? products.value.length;
  } finally {
    loading.value = false;
  }
}

async function loadProductCategories() {
  if (!canListCategories.value) {
    productCategories.value = [];
    return;
  }
  const { code, data } = await listAdminMallProductCategories({
    status: 0,
    limit: 100,
    offset: 0
  });
  if (code === 0) {
    productCategories.value = data.items ?? [];
  }
}

async function loadStockLogs() {
  const productId = normalizeEntityId(stockLogProduct.value?.id);
  if (!productId || !canList.value) {
    stockLogs.value = [];
    stockLogQuery.total = 0;
    return;
  }
  stockLogLoading.value = true;
  try {
    const {
      code,
      data,
      message: msg
    } = await listAdminMallProductStockLogs(productId, {
      reason: stockLogQuery.reason,
      limit: stockLogQuery.pageSize,
      offset: (stockLogQuery.currentPage - 1) * stockLogQuery.pageSize
    });
    if (code !== 0) {
      message(msg || "加载库存流水失败", { type: "error" });
      return;
    }
    stockLogs.value = data.items ?? [];
    stockLogQuery.total = data.total ?? stockLogs.value.length;
  } finally {
    stockLogLoading.value = false;
  }
}

function resetQuery() {
  query.keyword = "";
  query.category = "";
  query.status = 0;
  query.currentPage = 1;
  loadProducts();
}

function openCreateDialog() {
  if (!canCreate.value) {
    message("没有新增商品权限", { type: "warning" });
    return;
  }
  dialogMode.value = "create";
  resetFormModel();
  dialogVisible.value = true;
}

function openEditDialog(row: ProductRow) {
  if (!canUpdate.value) {
    message("没有修改商品权限", { type: "warning" });
    return;
  }
  dialogMode.value = "edit";
  form.id = Number(row.id ?? 0);
  form.sku = row.sku ?? "";
  form.title = row.title ?? "";
  form.description = row.description ?? "";
  form.category = row.category ?? "";
  form.cover_url = coverOf(row);
  form.price_credits = priceOf(row);
  form.stock = Number(row.stock ?? 0);
  form.status = Number(row.status ?? 2);
  form.sort = Number(row.sort ?? 0);
  formRef.value?.clearValidate();
  dialogVisible.value = true;
}

function openStockLogDrawer(row: ProductRow) {
  if (!canList.value) {
    message("没有查看商品权限", { type: "warning" });
    return;
  }
  stockLogProduct.value = row;
  stockLogQuery.reason = "";
  stockLogQuery.currentPage = 1;
  stockLogDrawerVisible.value = true;
  loadStockLogs();
}

async function saveProduct() {
  if (dialogMode.value === "create" && !canCreate.value) {
    message("没有新增商品权限", { type: "warning" });
    return;
  }
  if (dialogMode.value === "edit" && !canUpdate.value) {
    message("没有修改商品权限", { type: "warning" });
    return;
  }
  const valid = await formRef.value?.validate().catch(() => false);
  if (!valid) return;
  const payload = buildPayload();
  saving.value = true;
  try {
    const id = normalizeEntityId(form.id);
    const { code, message: msg } =
      dialogMode.value === "create"
        ? await createAdminMallProduct(payload)
        : await updateAdminMallProduct(id, payload);
    if (code !== 0) {
      message(msg || "保存商品失败", { type: "error" });
      return;
    }
    message("商品已保存", { type: "success" });
    dialogVisible.value = false;
    await loadProducts();
  } finally {
    saving.value = false;
  }
}

function onPageSizeChange(size: number) {
  query.pageSize = size;
  query.currentPage = 1;
  loadProducts();
}

function onCurrentPageChange(page: number) {
  query.currentPage = page;
  loadProducts();
}

function onStockLogPageSizeChange(size: number) {
  stockLogQuery.pageSize = size;
  stockLogQuery.currentPage = 1;
  loadStockLogs();
}

function onStockLogReasonChange() {
  stockLogQuery.currentPage = 1;
  loadStockLogs();
}

function onStockLogCurrentPageChange(page: number) {
  stockLogQuery.currentPage = page;
  loadStockLogs();
}

onMounted(() => {
  loadProductCategories();
  loadProducts();
});
</script>

<template>
  <div class="mall-products-page">
    <section class="mall-panel">
      <div class="panel-header">
        <div>
          <h2>商品管理</h2>
          <p>维护积分商城商品、库存、售价和上下架状态</p>
        </div>
        <el-button
          type="primary"
          :icon="useRenderIcon('ri/add-line')"
          :disabled="!canCreate"
          @click="openCreateDialog"
        >
          新增商品
        </el-button>
      </div>

      <el-alert
        v-if="!canList"
        title="当前账号没有 mall:list_products 权限"
        type="warning"
        show-icon
        :closable="false"
        class="permission-alert"
      />

      <el-form :inline="true" class="search-form">
        <el-form-item label="关键词">
          <el-input
            v-model="query.keyword"
            class="w-52!"
            clearable
            placeholder="SKU / 商品名称"
            @keyup.enter="loadProducts"
          />
        </el-form-item>
        <el-form-item label="分类">
          <el-select
            v-model="query.category"
            class="w-40!"
            clearable
            filterable
            allow-create
            default-first-option
            placeholder="商品分类"
            @change="loadProducts"
          >
            <el-option
              v-for="item in categoryOptions"
              :key="item.value"
              :label="item.label"
              :value="item.value"
            />
          </el-select>
        </el-form-item>
        <el-form-item label="状态">
          <el-select
            v-model="query.status"
            class="w-32!"
            @change="loadProducts"
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
            @click="loadProducts"
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
        :data="products"
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
        <template #cover="{ row }">
          <el-image
            v-if="coverOf(row)"
            :src="coverOf(row)"
            fit="cover"
            class="product-cover"
          />
          <el-tag v-else effect="plain" type="info">无</el-tag>
        </template>
        <template #price="{ row }">
          <span class="credit-price">{{ priceOf(row) }}</span>
        </template>
        <template #sales="{ row }">
          <el-tag effect="plain">{{ salesOf(row) }}</el-tag>
        </template>
        <template #category="{ row }">
          <el-tag effect="plain">{{ categoryText(row.category) }}</el-tag>
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
            type="primary"
            :disabled="!canList"
            :icon="useRenderIcon('ri/file-list-3-line')"
            @click="openStockLogDrawer(row)"
          >
            库存流水
          </el-button>
        </template>
      </pure-table>
    </section>

    <el-dialog
      v-model="dialogVisible"
      :title="dialogTitle"
      width="720px"
      destroy-on-close
    >
      <el-form
        ref="formRef"
        :model="form"
        :rules="rules"
        label-width="96px"
        class="product-form"
      >
        <el-row :gutter="16">
          <el-col :span="12">
            <el-form-item label="SKU" prop="sku">
              <el-input
                v-model="form.sku"
                maxlength="64"
                show-word-limit
                placeholder="例如 VIP-MONTH"
              />
            </el-form-item>
          </el-col>
          <el-col :span="12">
            <el-form-item label="分类">
              <el-select
                v-model="form.category"
                filterable
                allow-create
                default-first-option
                clearable
                placeholder="例如 会员权益"
              >
                <el-option
                  v-for="item in categoryOptions"
                  :key="item.value"
                  :label="item.label"
                  :value="item.value"
                />
              </el-select>
            </el-form-item>
          </el-col>
        </el-row>
        <el-form-item label="商品名称" prop="title">
          <el-input
            v-model="form.title"
            maxlength="120"
            show-word-limit
            placeholder="请输入商品名称"
          />
        </el-form-item>
        <el-form-item label="封面地址">
          <el-input
            v-model="form.cover_url"
            maxlength="500"
            show-word-limit
            placeholder="https://..."
          />
        </el-form-item>
        <el-row :gutter="16">
          <el-col :span="8">
            <el-form-item label="积分售价" prop="price_credits">
              <el-input-number
                v-model="form.price_credits"
                :min="0"
                :max="999999999"
                controls-position="right"
              />
            </el-form-item>
          </el-col>
          <el-col :span="8">
            <el-form-item label="库存" prop="stock">
              <el-input-number
                v-model="form.stock"
                :min="0"
                :max="999999999"
                controls-position="right"
              />
            </el-form-item>
          </el-col>
          <el-col :span="8">
            <el-form-item label="排序">
              <el-input-number
                v-model="form.sort"
                :min="0"
                :max="999999"
                controls-position="right"
              />
            </el-form-item>
          </el-col>
        </el-row>
        <el-form-item label="状态" prop="status">
          <el-radio-group v-model="form.status">
            <el-radio-button
              v-for="item in editableStatusOptions"
              :key="item.value"
              :value="item.value"
            >
              {{ item.label }}
            </el-radio-button>
          </el-radio-group>
        </el-form-item>
        <el-form-item label="商品说明">
          <el-input
            v-model="form.description"
            type="textarea"
            :rows="4"
            maxlength="1000"
            show-word-limit
            placeholder="商品权益、兑换说明、发放方式或运营备注"
          />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="dialogVisible = false">取消</el-button>
        <el-button type="primary" :loading="saving" @click="saveProduct">
          保存
        </el-button>
      </template>
    </el-dialog>

    <el-drawer
      v-model="stockLogDrawerVisible"
      size="760px"
      destroy-on-close
      class="stock-log-drawer"
    >
      <template #header>
        <div class="drawer-title">
          <h3>库存流水</h3>
          <span>
            {{ stockLogProduct?.title || "-" }}
            <em v-if="stockLogProduct?.sku">SKU {{ stockLogProduct?.sku }}</em>
          </span>
        </div>
      </template>

      <div class="stock-log-toolbar">
        <el-select
          v-model="stockLogQuery.reason"
          class="w-40!"
          @change="onStockLogReasonChange"
        >
          <el-option
            v-for="item in stockReasonOptions"
            :key="item.value"
            :label="item.label"
            :value="item.value"
          />
        </el-select>
        <el-button
          :icon="useRenderIcon('ep/refresh')"
          :loading="stockLogLoading"
          @click="loadStockLogs"
        >
          刷新
        </el-button>
      </div>

      <el-table
        v-loading="stockLogLoading"
        :data="stockLogs"
        border
        class="stock-log-table"
        empty-text="暂无库存流水"
      >
        <el-table-column label="变动" width="96" align="center">
          <template #default="{ row }">
            <span
              :class="[
                'stock-delta',
                deltaOf(row) >= 0 ? 'stock-delta-up' : 'stock-delta-down'
              ]"
            >
              {{ deltaOf(row) >= 0 ? "+" : "" }}{{ deltaOf(row) }}
            </span>
          </template>
        </el-table-column>
        <el-table-column label="库存" min-width="130" align="center">
          <template #default="{ row }">
            {{ beforeStockOf(row) }} -> {{ afterStockOf(row) }}
          </template>
        </el-table-column>
        <el-table-column label="原因" width="110" align="center">
          <template #default="{ row }">
            <el-tag :type="reasonMeta(row.reason).type" effect="plain">
              {{ reasonMeta(row.reason).label }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column label="关联对象" min-width="130">
          <template #default="{ row }">{{ referenceText(row) }}</template>
        </el-table-column>
        <el-table-column label="操作人" min-width="130">
          <template #default="{ row }">{{ operatorText(row) }}</template>
        </el-table-column>
        <el-table-column
          prop="note"
          label="备注"
          min-width="180"
          show-overflow-tooltip
        />
        <el-table-column label="时间" width="170">
          <template #default="{ row }">{{ formatTime(createdAt(row)) }}</template>
        </el-table-column>
      </el-table>

      <div class="stock-log-pagination">
        <el-pagination
          v-model:current-page="stockLogQuery.currentPage"
          v-model:page-size="stockLogQuery.pageSize"
          background
          layout="total, sizes, prev, pager, next"
          :page-sizes="[10, 20, 50, 100]"
          :total="stockLogQuery.total"
          @size-change="onStockLogPageSizeChange"
          @current-change="onStockLogCurrentPageChange"
        />
      </div>
    </el-drawer>
  </div>
</template>

<style scoped>
.mall-products-page {
  display: flex;
  flex-direction: column;
  gap: 16px;
}

.mall-panel {
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

.product-cover {
  width: 48px;
  height: 48px;
  border-radius: 6px;
  border: 1px solid var(--el-border-color-light);
}

.credit-price {
  font-weight: 600;
  color: var(--el-color-warning);
}

.product-form {
  padding-right: 10px;
}

.product-form :deep(.el-select),
.product-form :deep(.el-input-number) {
  width: 100%;
}

.drawer-title {
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.drawer-title h3 {
  margin: 0;
  font-size: 18px;
  font-weight: 600;
  color: var(--el-text-color-primary);
}

.drawer-title span {
  font-size: 13px;
  color: var(--el-text-color-secondary);
}

.drawer-title em {
  margin-left: 8px;
  font-style: normal;
  color: var(--el-text-color-placeholder);
}

.stock-log-toolbar {
  display: flex;
  gap: 10px;
  align-items: center;
  margin-bottom: 12px;
}

.stock-log-table {
  width: 100%;
}

.stock-delta {
  font-weight: 700;
}

.stock-delta-up {
  color: var(--el-color-success);
}

.stock-delta-down {
  color: var(--el-color-danger);
}

.stock-log-pagination {
  display: flex;
  justify-content: flex-end;
  margin-top: 14px;
}

@media (width <= 768px) {
  .panel-header {
    flex-direction: column;
  }
}
</style>
