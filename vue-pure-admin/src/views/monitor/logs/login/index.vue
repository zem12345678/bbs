<script setup lang="ts">
import { ref } from "vue";
import { useRole } from "./hook";
import { getPickerShortcuts } from "../../utils";
import { PureTableBar } from "@/components/RePureTableBar";
import { useRenderIcon } from "@/components/ReIcon/src/hooks";

import Refresh from "~icons/ep/refresh";

defineOptions({
  name: "LoginLog"
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
  loginStatusLabel
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
      <el-form-item label="用户名" prop="username">
        <el-input
          v-model="form.username"
          placeholder="请输入用户名"
          clearable
          class="w-37.5!"
        />
      </el-form-item>
      <el-form-item label="登录状态" prop="status">
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
      <el-form-item label="登录时间" prop="loginTime">
        <el-date-picker
          v-model="form.loginTime"
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

    <PureTableBar title="登录日志" :columns="columns" @refresh="onSearch">
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

    <el-drawer v-model="detailVisible" title="登录日志详情" size="520px">
      <el-descriptions v-if="detailLog" :column="1" border>
        <el-descriptions-item label="日志 ID">
          {{ detailLog.id || "-" }}
        </el-descriptions-item>
        <el-descriptions-item label="用户名">
          {{ detailLog.username || "-" }}
        </el-descriptions-item>
        <el-descriptions-item label="登录状态">
          <el-tag :type="Number(detailLog.status) === 1 ? 'success' : 'danger'">
            {{ loginStatusLabel(detailLog.status) }}
          </el-tag>
        </el-descriptions-item>
        <el-descriptions-item label="登录 IP">
          {{ detailLog.ip || "-" }}
        </el-descriptions-item>
        <el-descriptions-item label="登录地点">
          {{ detailLog.address || detailLog.location || "-" }}
        </el-descriptions-item>
        <el-descriptions-item label="浏览器">
          {{ detailLog.browser || "-" }}
        </el-descriptions-item>
        <el-descriptions-item label="操作系统">
          {{ detailLog.system || detailLog.os || "-" }}
        </el-descriptions-item>
        <el-descriptions-item label="平台">
          {{ detailLog.platform || "-" }}
        </el-descriptions-item>
        <el-descriptions-item label="登录行为">
          {{ detailLog.behavior || detailLog.message || "-" }}
        </el-descriptions-item>
        <el-descriptions-item label="备注">
          {{ detailLog.remark || "-" }}
        </el-descriptions-item>
        <el-descriptions-item label="登录时间">
          {{ formatLogTime(detailLog.loginTime) }}
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
</style>
