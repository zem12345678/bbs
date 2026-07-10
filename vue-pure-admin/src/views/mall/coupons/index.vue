<script setup lang="ts">
import dayjs from "dayjs";
import { computed, onMounted, reactive, ref } from "vue";
import { type FormInstance, type FormRules } from "element-plus";
import { message } from "@/utils/message";
import { hasPerms } from "@/utils/auth";
import { normalizeEntityId } from "@/utils/entityId";
import { useRenderIcon } from "@/components/ReIcon/src/hooks";
import {
  createAdminMallCoupon,
  listAdminMallCouponUsages,
  listAdminMallCoupons,
  updateAdminMallCoupon,
  type AdminMallCoupon,
  type AdminMallCouponUsage,
  type AdminMallCouponPayload
} from "@/api/admin";

defineOptions({
  name: "MallCoupons"
});

type CouponRow = Partial<AdminMallCoupon> & Record<string, any>;
type CouponUsageRow = Partial<AdminMallCouponUsage> & Record<string, any>;
type DialogMode = "create" | "edit";

const loading = ref(false);
const saving = ref(false);
const dialogVisible = ref(false);
const dialogMode = ref<DialogMode>("create");
const formRef = ref<FormInstance>();
const coupons = ref<AdminMallCoupon[]>([]);
const usageDrawerVisible = ref(false);
const usageLoading = ref(false);
const usageCoupon = ref<CouponRow>();
const usages = ref<AdminMallCouponUsage[]>([]);

const query = reactive({
  keyword: "",
  status: 0,
  pageSize: 20,
  currentPage: 1,
  total: 0
});

const usageQuery = reactive({
  userId: "",
  status: 0,
  pageSize: 20,
  currentPage: 1,
  total: 0
});

const form = reactive({
  id: 0,
  code: "",
  name: "",
  description: "",
  discount_credits: 0,
  min_order_credits: 0,
  total_quota: 0,
  per_user_limit: 1,
  status: 2,
  starts_at: "" as number | string,
  ends_at: "" as number | string
});

const canList = computed(() => hasPerms("mall:list_coupons"));
const canCreate = computed(() => hasPerms("mall:create_coupon"));
const canUpdate = computed(() => hasPerms("mall:update_coupon"));
const canListUsages = computed(() => hasPerms("mall:list_coupon_usages"));

const dialogTitle = computed(() =>
  dialogMode.value === "create" ? "新增优惠券" : "编辑优惠券"
);

const statusOptions = [
  { label: "全部", value: 0 },
  { label: "草稿", value: 1 },
  { label: "投放中", value: 2 },
  { label: "已归档", value: 3 }
];

const editableStatusOptions = statusOptions.filter(item => item.value > 0);

const columns: TableColumnList = [
  { prop: "id", label: "ID", width: 90 },
  { label: "优惠券", minWidth: 220, slot: "coupon" },
  { label: "优惠", width: 120, slot: "discount" },
  { label: "门槛", width: 120, slot: "threshold" },
  { label: "领取/使用", width: 150, slot: "usage" },
  { label: "单用户", width: 100, slot: "perUser" },
  { label: "状态", width: 110, slot: "status" },
  { label: "投放时间", minWidth: 230, slot: "window" },
  { label: "更新时间", width: 170, slot: "updatedAt" },
  { label: "操作", fixed: "right", width: 200, slot: "operation" }
];

const usageStatusOptions = [
  { label: "全部", value: 0 },
  { label: "待支付", value: 1 },
  { label: "已使用", value: 2 },
  { label: "已释放", value: 3 }
];

const rules: FormRules = {
  code: [
    { required: true, message: "请输入优惠码", trigger: "blur" },
    {
      pattern: /^[A-Za-z0-9_-]+$/,
      message: "优惠码只允许字母、数字、下划线和短横线",
      trigger: "blur"
    }
  ],
  name: [
    { required: true, message: "请输入优惠券名称", trigger: "blur" },
    { min: 1, max: 80, message: "名称长度需在 1-80 个字符", trigger: "blur" }
  ],
  discount_credits: [
    { required: true, message: "请输入优惠积分", trigger: "change" }
  ],
  min_order_credits: [
    { required: true, message: "请输入使用门槛", trigger: "change" }
  ],
  total_quota: [
    { required: true, message: "请输入发放总量", trigger: "change" }
  ],
  per_user_limit: [
    { required: true, message: "请输入单用户限制", trigger: "change" }
  ],
  status: [{ required: true, message: "请选择状态", trigger: "change" }]
};

function statusCode(value?: number | string) {
  if (typeof value === "number") return value;
  switch (String(value || "").toUpperCase()) {
    case "COUPON_STATUS_DRAFT":
    case "DRAFT":
      return 1;
    case "COUPON_STATUS_ACTIVE":
    case "ACTIVE":
      return 2;
    case "COUPON_STATUS_ARCHIVED":
    case "ARCHIVED":
      return 3;
    default:
      return 0;
  }
}

function statusMeta(value?: number | string) {
  switch (statusCode(value)) {
    case 1:
      return { label: "草稿", type: "info" as const };
    case 2:
      return { label: "投放中", type: "success" as const };
    case 3:
      return { label: "已归档", type: "warning" as const };
    default:
      return { label: "未知", type: "danger" as const };
  }
}

function usageStatusCode(value?: number | string) {
  if (typeof value === "number") return value;
  switch (String(value || "").toUpperCase()) {
    case "COUPON_USAGE_STATUS_RESERVED":
    case "RESERVED":
      return 1;
    case "COUPON_USAGE_STATUS_USED":
    case "USED":
      return 2;
    case "COUPON_USAGE_STATUS_RELEASED":
    case "RELEASED":
      return 3;
    default:
      return 0;
  }
}

function usageStatusMeta(value?: number | string) {
  switch (usageStatusCode(value)) {
    case 1:
      return { label: "待支付", type: "warning" as const };
    case 2:
      return { label: "已使用", type: "success" as const };
    case 3:
      return { label: "已释放", type: "info" as const };
    default:
      return { label: "未知", type: "danger" as const };
  }
}

function discountOf(row: CouponRow) {
  return Number(row.discount_credits ?? row.discountCredits ?? 0);
}

function minOrderOf(row: CouponRow) {
  return Number(row.min_order_credits ?? row.minOrderCredits ?? 0);
}

function totalQuotaOf(row: CouponRow) {
  return Number(row.total_quota ?? row.totalQuota ?? 0);
}

function perUserLimitOf(row: CouponRow) {
  return Number(row.per_user_limit ?? row.perUserLimit ?? 0);
}

function claimedOf(row: CouponRow) {
  return Number(row.claimed_count ?? row.claimedCount ?? 0);
}

function usedOf(row: CouponRow) {
  return Number(row.used_count ?? row.usedCount ?? 0);
}

function startsAtOf(row: CouponRow) {
  return Number(row.starts_at ?? row.startsAt ?? 0);
}

function endsAtOf(row: CouponRow) {
  return Number(row.ends_at ?? row.endsAt ?? 0);
}

function updatedAtOf(row: CouponRow) {
  return Number(row.updated_at ?? row.updatedAt ?? 0);
}

function usageUserId(row: CouponUsageRow) {
  return row.user_id ?? row.userId ?? "-";
}

function usageOrderId(row: CouponUsageRow) {
  return row.order_id ?? row.orderId ?? "-";
}

function usageDiscount(row: CouponUsageRow) {
  return Number(row.discount_credits ?? row.discountCredits ?? 0);
}

function usageCreatedAt(row: CouponUsageRow) {
  return Number(row.created_at ?? row.createdAt ?? 0);
}

function usageUsedAt(row: CouponUsageRow) {
  return Number(row.used_at ?? row.usedAt ?? 0);
}

function usageReleasedAt(row: CouponUsageRow) {
  return Number(row.released_at ?? row.releasedAt ?? 0);
}

function usageUpdatedAt(row: CouponUsageRow) {
  return Number(row.updated_at ?? row.updatedAt ?? 0);
}

function formatTime(value?: number) {
  if (!value) return "不限";
  const timestamp = value > 9999999999 ? value : value * 1000;
  return dayjs(timestamp).format("YYYY-MM-DD HH:mm");
}

function usageText(row: CouponRow) {
  const total = totalQuotaOf(row);
  const claimed = claimedOf(row);
  const used = usedOf(row);
  return `${claimed}/${total || "不限"} 领取 · ${used} 使用`;
}

function timestampValue(value: number | string) {
  const parsed = Number(value || 0);
  return Number.isFinite(parsed) && parsed > 0 ? parsed : 0;
}

function resetFormModel() {
  form.id = 0;
  form.code = "";
  form.name = "";
  form.description = "";
  form.discount_credits = 0;
  form.min_order_credits = 0;
  form.total_quota = 0;
  form.per_user_limit = 1;
  form.status = 2;
  form.starts_at = "";
  form.ends_at = "";
  formRef.value?.clearValidate();
}

function buildPayload(): AdminMallCouponPayload {
  return {
    code: form.code.trim().toUpperCase(),
    name: form.name.trim(),
    description: form.description.trim(),
    discount_credits: Number(form.discount_credits || 0),
    min_order_credits: Number(form.min_order_credits || 0),
    total_quota: Number(form.total_quota || 0),
    per_user_limit: Number(form.per_user_limit || 0),
    status: Number(form.status || 2),
    starts_at: timestampValue(form.starts_at),
    ends_at: timestampValue(form.ends_at)
  };
}

async function loadCoupons() {
  if (!canList.value) {
    coupons.value = [];
    query.total = 0;
    return;
  }
  loading.value = true;
  try {
    const {
      code,
      data,
      message: msg
    } = await listAdminMallCoupons({
      keyword: query.keyword.trim(),
      status: query.status,
      limit: query.pageSize,
      offset: (query.currentPage - 1) * query.pageSize
    });
    if (code !== 0) {
      throw new Error(msg || "优惠券列表加载失败");
    }
    coupons.value = data.items ?? [];
    query.total = data.total ?? coupons.value.length;
  } catch (error: any) {
    coupons.value = [];
    query.total = 0;
    message(error?.message || "优惠券列表加载失败", { type: "error" });
  } finally {
    loading.value = false;
  }
}

async function loadUsages() {
  if (!canListUsages.value || !usageCoupon.value) {
    usages.value = [];
    usageQuery.total = 0;
    return;
  }
  usageLoading.value = true;
  try {
    const {
      code,
      data,
      message: msg
    } = await listAdminMallCouponUsages(normalizeEntityId(usageCoupon.value.id), {
      user_id: usageQuery.userId.trim() || undefined,
      status: usageQuery.status,
      limit: usageQuery.pageSize,
      offset: (usageQuery.currentPage - 1) * usageQuery.pageSize
    });
    if (code !== 0) {
      throw new Error(msg || "优惠券使用记录加载失败");
    }
    usages.value = data.items ?? [];
    usageQuery.total = data.total ?? usages.value.length;
  } catch (error: any) {
    usages.value = [];
    usageQuery.total = 0;
    message(error?.message || "优惠券使用记录加载失败", { type: "error" });
  } finally {
    usageLoading.value = false;
  }
}

function resetQuery() {
  query.keyword = "";
  query.status = 0;
  query.currentPage = 1;
  loadCoupons();
}

function openCreateDialog() {
  resetFormModel();
  dialogMode.value = "create";
  dialogVisible.value = true;
}

function openEditDialog(row: CouponRow) {
  resetFormModel();
  form.id = Number(normalizeEntityId(row.id));
  form.code = row.code || "";
  form.name = row.name || "";
  form.description = row.description || "";
  form.discount_credits = discountOf(row);
  form.min_order_credits = minOrderOf(row);
  form.total_quota = totalQuotaOf(row);
  form.per_user_limit = perUserLimitOf(row);
  form.status = statusCode(row.status) || 2;
  form.starts_at = startsAtOf(row) || "";
  form.ends_at = endsAtOf(row) || "";
  dialogMode.value = "edit";
  dialogVisible.value = true;
}

function openUsageDrawer(row: CouponRow) {
  if (!canListUsages.value) {
    message("没有查看优惠券使用记录权限", { type: "warning" });
    return;
  }
  usageCoupon.value = row;
  usageQuery.userId = "";
  usageQuery.status = 0;
  usageQuery.currentPage = 1;
  usageDrawerVisible.value = true;
  loadUsages();
}

async function saveCoupon() {
  await formRef.value?.validate();
  const payload = buildPayload();
  if (payload.discount_credits <= 0) {
    message("优惠积分必须大于 0", { type: "warning" });
    return;
  }
  if (payload.total_quota < 0 || payload.per_user_limit < 0) {
    message("发放限制不能小于 0", { type: "warning" });
    return;
  }
  if (payload.ends_at && payload.starts_at && payload.ends_at <= payload.starts_at) {
    message("结束时间必须晚于开始时间", { type: "warning" });
    return;
  }
  saving.value = true;
  try {
    const id = normalizeEntityId(form.id);
    const {
      code,
      message: msg
    } =
      dialogMode.value === "create"
        ? await createAdminMallCoupon(payload)
        : await updateAdminMallCoupon(id, payload);
    if (code !== 0) {
      throw new Error(msg || "优惠券保存失败");
    }
    message("优惠券已保存", { type: "success" });
    dialogVisible.value = false;
    await loadCoupons();
  } catch (error: any) {
    message(error?.message || "优惠券保存失败", { type: "error" });
  } finally {
    saving.value = false;
  }
}

function onPageSizeChange(pageSize: number) {
  query.pageSize = pageSize;
  query.currentPage = 1;
  loadCoupons();
}

function onCurrentPageChange(currentPage: number) {
  query.currentPage = currentPage;
  loadCoupons();
}

function onUsagePageSizeChange(pageSize: number) {
  usageQuery.pageSize = pageSize;
  usageQuery.currentPage = 1;
  loadUsages();
}

function onUsageCurrentPageChange(currentPage: number) {
  usageQuery.currentPage = currentPage;
  loadUsages();
}

function onUsageFilterChange() {
  usageQuery.currentPage = 1;
  loadUsages();
}

onMounted(loadCoupons);
</script>

<template>
  <div class="mall-coupons-page">
    <section class="mall-panel">
      <div class="panel-header">
        <div>
          <h2>优惠券管理</h2>
          <p>配置积分商城优惠码、投放时间、使用门槛和领取限制</p>
        </div>
        <el-button
          type="primary"
          :icon="useRenderIcon('ri/add-line')"
          :disabled="!canCreate"
          @click="openCreateDialog"
        >
          新增优惠券
        </el-button>
      </div>

      <el-alert
        v-if="!canList"
        title="当前账号没有 mall:list_coupons 权限"
        type="warning"
        show-icon
        :closable="false"
        class="permission-alert"
      />

      <el-form :inline="true" class="search-form">
        <el-form-item label="关键词">
          <el-input
            v-model="query.keyword"
            class="w-56!"
            clearable
            placeholder="优惠码 / 名称"
            @keyup.enter="loadCoupons"
          />
        </el-form-item>
        <el-form-item label="状态">
          <el-select
            v-model="query.status"
            class="w-32!"
            @change="loadCoupons"
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
            @click="loadCoupons"
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
        :data="coupons"
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
        <template #coupon="{ row }">
          <div class="coupon-cell">
            <strong>{{ row.name || "-" }}</strong>
            <el-tag effect="plain" type="primary">{{ row.code || "-" }}</el-tag>
            <small>{{ row.description || "暂无说明" }}</small>
          </div>
        </template>
        <template #discount="{ row }">
          <span class="credit-price">-{{ discountOf(row) }}</span>
          <small class="unit-text">积分</small>
        </template>
        <template #threshold="{ row }">
          {{ minOrderOf(row) > 0 ? `满 ${minOrderOf(row)} 可用` : "无门槛" }}
        </template>
        <template #usage="{ row }">
          <div class="usage-cell">
            <span>{{ usageText(row) }}</span>
            <el-progress
              :percentage="
                totalQuotaOf(row) > 0
                  ? Math.min(100, Math.round((claimedOf(row) / totalQuotaOf(row)) * 100))
                  : 0
              "
              :show-text="false"
              :stroke-width="5"
            />
          </div>
        </template>
        <template #perUser="{ row }">
          {{ perUserLimitOf(row) > 0 ? `${perUserLimitOf(row)} 张` : "不限" }}
        </template>
        <template #status="{ row }">
          <el-tag :type="statusMeta(row.status).type">
            {{ statusMeta(row.status).label }}
          </el-tag>
        </template>
        <template #window="{ row }">
          <div class="window-cell">
            <span>{{ formatTime(startsAtOf(row)) }}</span>
            <span>至 {{ formatTime(endsAtOf(row)) }}</span>
          </div>
        </template>
        <template #updatedAt="{ row }">
          {{ formatTime(updatedAtOf(row)) }}
        </template>
        <template #operation="{ row }">
          <div class="operation-cell">
            <el-button
              link
              type="primary"
              :disabled="!canListUsages"
              :icon="useRenderIcon('ri/file-list-3-line')"
              @click="openUsageDrawer(row)"
            >
              使用记录
            </el-button>
            <el-button
              link
              type="primary"
              :disabled="!canUpdate"
              :icon="useRenderIcon('ri/edit-line')"
              @click="openEditDialog(row)"
            >
              编辑
            </el-button>
          </div>
        </template>
      </pure-table>
    </section>

    <el-dialog
      v-model="dialogVisible"
      :title="dialogTitle"
      width="760px"
      destroy-on-close
    >
      <el-form
        ref="formRef"
        :model="form"
        :rules="rules"
        label-width="108px"
        class="coupon-form"
      >
        <el-row :gutter="16">
          <el-col :span="12">
            <el-form-item label="优惠码" prop="code">
              <el-input
                v-model="form.code"
                maxlength="48"
                show-word-limit
                placeholder="例如 SUMMER20"
              />
            </el-form-item>
          </el-col>
          <el-col :span="12">
            <el-form-item label="名称" prop="name">
              <el-input
                v-model="form.name"
                maxlength="80"
                show-word-limit
                placeholder="请输入优惠券名称"
              />
            </el-form-item>
          </el-col>
        </el-row>

        <el-row :gutter="16">
          <el-col :span="8">
            <el-form-item label="优惠积分" prop="discount_credits">
              <el-input-number
                v-model="form.discount_credits"
                :min="1"
                :max="999999999"
                controls-position="right"
              />
            </el-form-item>
          </el-col>
          <el-col :span="8">
            <el-form-item label="使用门槛" prop="min_order_credits">
              <el-input-number
                v-model="form.min_order_credits"
                :min="0"
                :max="999999999"
                controls-position="right"
              />
            </el-form-item>
          </el-col>
          <el-col :span="8">
            <el-form-item label="发放总量" prop="total_quota">
              <el-input-number
                v-model="form.total_quota"
                :min="0"
                :max="999999999"
                controls-position="right"
              />
            </el-form-item>
          </el-col>
        </el-row>

        <el-row :gutter="16">
          <el-col :span="8">
            <el-form-item label="单用户限制" prop="per_user_limit">
              <el-input-number
                v-model="form.per_user_limit"
                :min="0"
                :max="999999"
                controls-position="right"
              />
            </el-form-item>
          </el-col>
          <el-col :span="16">
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
          </el-col>
        </el-row>

        <el-row :gutter="16">
          <el-col :span="12">
            <el-form-item label="开始时间">
              <el-date-picker
                v-model="form.starts_at"
                type="datetime"
                value-format="x"
                clearable
                placeholder="不限开始时间"
              />
            </el-form-item>
          </el-col>
          <el-col :span="12">
            <el-form-item label="结束时间">
              <el-date-picker
                v-model="form.ends_at"
                type="datetime"
                value-format="x"
                clearable
                placeholder="不限结束时间"
              />
            </el-form-item>
          </el-col>
        </el-row>

        <el-form-item label="使用说明">
          <el-input
            v-model="form.description"
            type="textarea"
            :rows="4"
            maxlength="500"
            show-word-limit
            placeholder="面向用户展示的优惠说明、限制条件或运营备注"
          />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="dialogVisible = false">取消</el-button>
        <el-button type="primary" :loading="saving" @click="saveCoupon">
          保存
        </el-button>
      </template>
    </el-dialog>

    <el-drawer
      v-model="usageDrawerVisible"
      size="860px"
      destroy-on-close
      class="coupon-usage-drawer"
    >
      <template #header>
        <div class="drawer-title">
          <h3>优惠券使用记录</h3>
          <span>
            {{ usageCoupon?.name || "-" }}
            <em v-if="usageCoupon?.code">{{ usageCoupon?.code }}</em>
          </span>
        </div>
      </template>

      <div class="usage-toolbar">
        <el-input
          v-model="usageQuery.userId"
          class="w-40!"
          clearable
          placeholder="用户 ID"
          @keyup.enter="onUsageFilterChange"
        />
        <el-select
          v-model="usageQuery.status"
          class="w-32!"
          @change="onUsageFilterChange"
        >
          <el-option
            v-for="item in usageStatusOptions"
            :key="item.value"
            :label="item.label"
            :value="item.value"
          />
        </el-select>
        <el-button
          type="primary"
          :icon="useRenderIcon('ri/search-line')"
          :loading="usageLoading"
          @click="onUsageFilterChange"
        >
          查询
        </el-button>
        <el-button
          :icon="useRenderIcon('ep/refresh')"
          :loading="usageLoading"
          @click="loadUsages"
        >
          刷新
        </el-button>
      </div>

      <el-table
        v-loading="usageLoading"
        :data="usages"
        border
        class="usage-table"
        empty-text="暂无使用记录"
      >
        <el-table-column prop="id" label="ID" width="86" align="center" />
        <el-table-column label="用户 ID" width="110" align="center">
          <template #default="{ row }">{{ usageUserId(row) }}</template>
        </el-table-column>
        <el-table-column label="订单 ID" width="110" align="center">
          <template #default="{ row }">{{ usageOrderId(row) }}</template>
        </el-table-column>
        <el-table-column label="优惠码" min-width="150" show-overflow-tooltip>
          <template #default="{ row }">{{ row.code || "-" }}</template>
        </el-table-column>
        <el-table-column label="优惠" width="100" align="center">
          <template #default="{ row }">
            <span class="credit-price">-{{ usageDiscount(row) }}</span>
          </template>
        </el-table-column>
        <el-table-column label="状态" width="100" align="center">
          <template #default="{ row }">
            <el-tag :type="usageStatusMeta(row.status).type" effect="plain">
              {{ usageStatusMeta(row.status).label }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column label="创建时间" width="165">
          <template #default="{ row }">
            {{ formatTime(usageCreatedAt(row)) }}
          </template>
        </el-table-column>
        <el-table-column label="使用时间" width="165">
          <template #default="{ row }">
            {{ formatTime(usageUsedAt(row)) }}
          </template>
        </el-table-column>
        <el-table-column label="释放时间" width="165">
          <template #default="{ row }">
            {{ formatTime(usageReleasedAt(row)) }}
          </template>
        </el-table-column>
        <el-table-column label="更新时间" width="165">
          <template #default="{ row }">
            {{ formatTime(usageUpdatedAt(row)) }}
          </template>
        </el-table-column>
      </el-table>

      <div class="usage-pagination">
        <el-pagination
          v-model:current-page="usageQuery.currentPage"
          v-model:page-size="usageQuery.pageSize"
          background
          layout="total, sizes, prev, pager, next"
          :page-sizes="[10, 20, 50, 100]"
          :total="usageQuery.total"
          @size-change="onUsagePageSizeChange"
          @current-change="onUsageCurrentPageChange"
        />
      </div>
    </el-drawer>
  </div>
</template>

<style scoped>
.mall-coupons-page {
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

.coupon-cell {
  display: flex;
  flex-direction: column;
  gap: 5px;
  align-items: flex-start;
  text-align: left;
}

.coupon-cell strong {
  font-weight: 600;
  color: var(--el-text-color-primary);
}

.coupon-cell small,
.unit-text,
.window-cell {
  font-size: 12px;
  color: var(--el-text-color-secondary);
}

.credit-price {
  font-weight: 700;
  color: var(--el-color-warning);
}

.usage-cell {
  display: flex;
  flex-direction: column;
  gap: 6px;
  min-width: 120px;
  font-size: 12px;
  color: var(--el-text-color-regular);
}

.window-cell {
  display: flex;
  flex-direction: column;
  gap: 3px;
}

.coupon-form {
  padding-right: 10px;
}

.operation-cell {
  display: inline-flex;
  gap: 4px;
  align-items: center;
  white-space: nowrap;
}

.drawer-title {
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.drawer-title h3 {
  margin: 0;
  font-size: 16px;
  font-weight: 600;
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

.usage-toolbar {
  display: flex;
  flex-wrap: wrap;
  gap: 10px;
  align-items: center;
  margin-bottom: 12px;
}

.usage-table {
  width: 100%;
}

.usage-pagination {
  display: flex;
  justify-content: flex-end;
  margin-top: 14px;
}

@media (width <= 768px) {
  .panel-header {
    flex-direction: column;
  }

  .operation-cell {
    flex-direction: column;
    align-items: flex-start;
  }
}
</style>
