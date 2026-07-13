<script setup lang="ts">
import { ref } from "vue";
import { useEmailLogs } from "./hook";
import { PureTableBar } from "@/components/RePureTableBar";
import { useRenderIcon } from "@/components/ReIcon/src/hooks";

import Refresh from "~icons/ep/refresh";

defineOptions({
  name: "EmailLog"
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
  emailStatusLabel
} = useEmailLogs();
</script>

<template>
  <div class="main">
    <el-form
      ref="formRef"
      :inline="true"
      :model="form"
      class="search-form bg-bg_color w-full pl-8 pt-3 overflow-auto"
    >
      <el-form-item label="关键词" prop="keyword">
        <el-input
          v-model="form.keyword"
          placeholder="收件人 / 主题 / 模板 Key"
          clearable
          class="w-56!"
          @keyup.enter="onSearch"
        />
      </el-form-item>
      <el-form-item label="发送状态" prop="status">
        <el-select
          v-model="form.status"
          placeholder="请选择"
          clearable
          class="w-37.5!"
        >
          <el-option label="成功" value="1" />
          <el-option label="失败" value="2" />
        </el-select>
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

    <PureTableBar title="邮件日志" :columns="columns" @refresh="onSearch">
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

    <el-drawer v-model="detailVisible" title="邮件日志详情" size="560px">
      <el-descriptions v-if="detailLog" :column="1" border>
        <el-descriptions-item label="日志 ID">
          {{ detailLog.id || "-" }}
        </el-descriptions-item>
        <el-descriptions-item label="收件人">
          {{ detailLog.to || detailLog.mail_to || "-" }}
        </el-descriptions-item>
        <el-descriptions-item label="邮件主题">
          {{ detailLog.subject || "-" }}
        </el-descriptions-item>
        <el-descriptions-item label="模板 Key">
          {{ detailLog.templateKey || detailLog.template_key || "-" }}
        </el-descriptions-item>
        <el-descriptions-item label="服务商">
          {{ detailLog.provider || "-" }}
        </el-descriptions-item>
        <el-descriptions-item label="发送状态">
          <el-tag :type="Number(detailLog.status) === 1 ? 'success' : 'danger'">
            {{ emailStatusLabel(detailLog.status) }}
          </el-tag>
        </el-descriptions-item>
        <el-descriptions-item label="错误信息">
          <pre class="log-detail-code">{{ detailLog.error || "-" }}</pre>
        </el-descriptions-item>
        <el-descriptions-item label="创建时间">
          {{ formatLogTime(detailLog.createdAt) }}
        </el-descriptions-item>
        <el-descriptions-item label="更新时间">
          {{ formatLogTime(detailLog.updatedAt) }}
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
