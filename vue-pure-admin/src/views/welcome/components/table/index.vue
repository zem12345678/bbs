<script setup lang="ts">
import { computed, reactive, watch } from "vue";
import type { PaginationProps } from "@pureadmin/table";
import type { AdminOverviewDaily } from "@/api/admin";
import Empty from "./empty.svg?component";
import { columns } from "./columns";

const props = withDefaults(
  defineProps<{
    data?: AdminOverviewDaily[];
    loading?: boolean;
  }>(),
  {
    data: () => [],
    loading: false
  }
);

const pagination = reactive<PaginationProps>({
  pageSize: 7,
  currentPage: 1,
  layout: "prev, pager, next",
  total: 0,
  align: "center"
});

const pageRows = computed(() => {
  const start = (pagination.currentPage - 1) * pagination.pageSize;
  return props.data.slice(start, start + pagination.pageSize);
});

watch(
  () => props.data.length,
  total => {
    pagination.total = total;
    const maxPage = Math.max(1, Math.ceil(total / pagination.pageSize));
    if (pagination.currentPage > maxPage) {
      pagination.currentPage = maxPage;
    }
  },
  { immediate: true }
);

function onCurrentChange(page: number) {
  pagination.currentPage = page;
}
</script>

<template>
  <pure-table
    row-key="id"
    alignWhole="center"
    showOverflowTooltip
    :loading="loading"
    :loading-config="{ background: 'transparent' }"
    :data="pageRows"
    :columns="columns"
    :pagination="pagination"
    @page-current-change="onCurrentChange"
  >
    <template #empty>
      <el-empty description="暂无统计数据" :image-size="60">
        <template #image>
          <Empty />
        </template>
      </el-empty>
    </template>
  </pure-table>
</template>

<style lang="scss">
.pure-table-filter {
  .el-table-filter__list {
    min-width: 80px;
    padding: 0;

    li {
      line-height: 28px;
    }
  }
}
</style>

<style lang="scss" scoped>
:deep(.el-table) {
  --el-table-border: none;
  --el-table-border-color: transparent;

  .el-empty__description {
    margin: 0;
  }

  .el-scrollbar__bar {
    display: none;
  }
}
</style>
