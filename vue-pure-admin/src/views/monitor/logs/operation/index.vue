<script setup lang="ts">
import { ref } from "vue";
import { useRole } from "./hook";
import { getPickerShortcuts } from "../../utils";
import { PureTableBar } from "@/components/RePureTableBar";
import { useRenderIcon } from "@/components/ReIcon/src/hooks";

import Refresh from "~icons/ep/refresh";

defineOptions({
  name: "OperationLog"
});

const formRef = ref();
const tableRef = ref();

const {
  form,
  loading,
  columns,
  dataList,
  pagination,
  detailVisible,
  detailLog,
  onSearch,
  resetForm,
  handleSizeChange,
  handleCurrentChange,
  exportCurrentPage,
  formatLogTime,
  operationLatency,
  operationStatusLabel
} = useRole();
</script>

<template>
  <div class="main">
    <el-form
      ref="formRef"
      :inline="true"
      :model="form"
      class="search-form bg-bg_color w-full pl-8 pt-3 overflow-auto"
    >
      <el-form-item label="所属模块" prop="module">
        <el-input
          v-model="form.module"
          placeholder="请输入所属模块"
          clearable
          class="w-42.5!"
        />
      </el-form-item>
      <el-form-item label="操作状态" prop="status">
        <el-select
          v-model="form.status"
          placeholder="请选择"
          clearable
          class="w-37.5!"
        >
          <el-option label="成功" value="1" />
          <el-option label="失败" value="0" />
        </el-select>
      </el-form-item>
      <el-form-item label="操作时间" prop="operatingTime">
        <el-date-picker
          v-model="form.operatingTime"
          :shortcuts="getPickerShortcuts()"
          type="datetimerange"
          range-separator="至"
          start-placeholder="开始日期时间"
          end-placeholder="结束日期时间"
        />
      </el-form-item>
      <el-form-item>
        <el-button
          type="primary"
          :icon="useRenderIcon('ri:search-line')"
          :loading="loading"
          @click="onSearch"
        >
          搜索
        </el-button>
        <el-button :icon="useRenderIcon(Refresh)" @click="resetForm(formRef)">
          重置
        </el-button>
        <el-button
          :icon="useRenderIcon('ri:file-download-line')"
          @click="exportCurrentPage"
        >
          导出当前页
        </el-button>
      </el-form-item>
    </el-form>

    <PureTableBar title="操作日志" :columns="columns" @refresh="onSearch">
      <template v-slot="{ size, dynamicColumns }">
        <pure-table
          ref="tableRef"
          row-key="id"
          align-whole="center"
          table-layout="auto"
          :loading="loading"
          :size="size"
          adaptive
          :adaptiveConfig="{ offsetBottom: 108 }"
          :data="dataList"
          :columns="dynamicColumns"
          :pagination="{ ...pagination, size }"
          :header-cell-style="{
            background: 'var(--el-fill-color-light)',
            color: 'var(--el-text-color-primary)'
          }"
          @page-size-change="handleSizeChange"
          @page-current-change="handleCurrentChange"
        />
      </template>
    </PureTableBar>

    <el-drawer v-model="detailVisible" title="操作日志详情" size="620px">
      <el-descriptions v-if="detailLog" :column="1" border>
        <el-descriptions-item label="日志 ID">
          {{ detailLog.id || "-" }}
        </el-descriptions-item>
        <el-descriptions-item label="操作人员">
          {{ detailLog.username || detailLog.operatorName || "-" }}
        </el-descriptions-item>
        <el-descriptions-item label="所属模块">
          {{ detailLog.module || detailLog.title || "-" }}
        </el-descriptions-item>
        <el-descriptions-item label="业务类型">
          {{ detailLog.summary || detailLog.businessType || "-" }}
        </el-descriptions-item>
        <el-descriptions-item label="请求方法">
          {{ detailLog.method || "-" }}
        </el-descriptions-item>
        <el-descriptions-item label="处理器">
          {{ detailLog.handler || "-" }}
        </el-descriptions-item>
        <el-descriptions-item label="请求地址">
          {{ detailLog.url || "-" }}
        </el-descriptions-item>
        <el-descriptions-item label="操作状态">
          <el-tag :type="Number(detailLog.status) === 1 ? 'success' : 'danger'">
            {{ operationStatusLabel(detailLog.status) }}
          </el-tag>
        </el-descriptions-item>
        <el-descriptions-item label="响应结果">
          {{ detailLog.result || "-" }}
        </el-descriptions-item>
        <el-descriptions-item label="耗时">
          {{ operationLatency(detailLog) }}
        </el-descriptions-item>
        <el-descriptions-item label="操作 IP">
          {{ detailLog.ip || "-" }}
        </el-descriptions-item>
        <el-descriptions-item label="操作地点">
          {{ detailLog.address || detailLog.location || "-" }}
        </el-descriptions-item>
        <el-descriptions-item label="浏览器 / 系统">
          {{ detailLog.browser || "-" }} / {{ detailLog.system || "-" }}
        </el-descriptions-item>
        <el-descriptions-item label="操作时间">
          {{ formatLogTime(detailLog.operatingTime) }}
        </el-descriptions-item>
        <el-descriptions-item label="请求参数">
          <pre class="log-detail-code">{{ detailLog.params || "-" }}</pre>
        </el-descriptions-item>
        <el-descriptions-item label="User-Agent">
          <pre class="log-detail-code">{{ detailLog.userAgent || "-" }}</pre>
        </el-descriptions-item>
      </el-descriptions>
      <el-empty v-else description="暂无日志详情" />
    </el-drawer>
  </div>
</template>

<style lang="scss" scoped>
:deep(.el-dropdown-menu__item i) {
  margin: 0;
}

.main-content {
  margin: 24px 24px 0 !important;
}

.search-form {
  :deep(.el-form-item) {
    margin-bottom: 12px;
  }
}

.log-detail-code {
  max-height: 220px;
  padding: 10px;
  margin: 0;
  overflow: auto;
  white-space: pre-wrap;
  word-break: break-all;
  background: var(--el-fill-color-light);
  border-radius: 6px;
}
</style>
