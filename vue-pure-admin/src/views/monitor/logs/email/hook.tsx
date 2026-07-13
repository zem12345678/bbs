import dayjs from "dayjs";
import { getEmailLogsList } from "@/api/system";
import { downloadCsv, type CsvColumn } from "@/utils/csvExport";
import { message } from "@/utils/message";
import { usePublicHooks } from "@/views/system/hooks";
import type { PaginationProps } from "@pureadmin/table";
import { reactive, ref, onMounted, toRaw } from "vue";

type EmailLogRow = Record<string, any>;

const emailLogCsvColumns: CsvColumn<EmailLogRow>[] = [
  { header: "日志ID", value: row => row.id ?? "" },
  { header: "收件人", value: row => row.to ?? "" },
  { header: "邮件主题", value: row => row.subject ?? "" },
  { header: "模板Key", value: row => row.templateKey ?? "" },
  { header: "服务商", value: row => row.provider ?? "" },
  { header: "发送状态", value: row => emailStatusLabel(row.status) },
  { header: "错误信息", value: row => row.error ?? "" },
  { header: "创建时间", value: row => formatLogTime(row.createdAt) },
  { header: "更新时间", value: row => formatLogTime(row.updatedAt) }
];

export function useEmailLogs() {
  const form = reactive({
    keyword: "",
    status: ""
  });
  const dataList = ref<EmailLogRow[]>([]);
  const loading = ref(true);
  const detailVisible = ref(false);
  const detailLog = ref<EmailLogRow | null>(null);
  const { tagStyle } = usePublicHooks();

  const pagination = reactive<PaginationProps>({
    total: 0,
    pageSize: 10,
    currentPage: 1,
    background: true
  });

  const columns: TableColumnList = [
    {
      label: "序号",
      prop: "id",
      minWidth: 90
    },
    {
      label: "收件人",
      prop: "to",
      minWidth: 180,
      showOverflowTooltip: true
    },
    {
      label: "邮件主题",
      prop: "subject",
      minWidth: 220,
      showOverflowTooltip: true
    },
    {
      label: "模板 Key",
      prop: "templateKey",
      minWidth: 150,
      showOverflowTooltip: true
    },
    {
      label: "服务商",
      prop: "provider",
      minWidth: 120,
      showOverflowTooltip: true
    },
    {
      label: "发送状态",
      prop: "status",
      minWidth: 100,
      cellRenderer: ({ row, props }) => (
        <el-tag size={props.size} style={tagStyle.value(row.status)}>
          {emailStatusLabel(row.status)}
        </el-tag>
      )
    },
    {
      label: "错误信息",
      prop: "error",
      minWidth: 220,
      showOverflowTooltip: true,
      formatter: ({ error }) => error || "-"
    },
    {
      label: "创建时间",
      prop: "createdAt",
      minWidth: 180,
      formatter: ({ createdAt }) => formatLogTime(createdAt)
    },
    {
      label: "操作",
      fixed: "right",
      width: 90,
      cellRenderer: ({ row }) => (
        <el-button link type="primary" onClick={() => openDetail(row)}>
          详情
        </el-button>
      )
    }
  ];

  function handleSizeChange(val: number) {
    pagination.pageSize = val;
    pagination.currentPage = 1;
    onSearch();
  }

  function handleCurrentChange(val: number) {
    pagination.currentPage = val;
    onSearch();
  }

  async function onSearch() {
    loading.value = true;
    try {
      const { code, data } = await getEmailLogsList({
        ...toRaw(form),
        currentPage: pagination.currentPage,
        pageSize: pagination.pageSize
      });
      if (code === 0) {
        dataList.value = data.list;
        pagination.total = data.total;
        pagination.pageSize = data.pageSize;
        pagination.currentPage = data.currentPage;
      }
    } finally {
      loading.value = false;
    }
  }

  function openDetail(row: EmailLogRow) {
    detailLog.value = row;
    detailVisible.value = true;
  }

  function exportCurrentPage() {
    if (dataList.value.length === 0) {
      message("当前筛选条件下没有可导出的邮件日志", { type: "warning" });
      return;
    }
    downloadCsv(
      `admin-email-logs-${dayjs().format("YYYYMMDDHHmmss")}.csv`,
      emailLogCsvColumns,
      dataList.value
    );
    message(`已导出当前页 ${dataList.value.length} 条邮件日志`, {
      type: "success"
    });
  }

  const resetForm = formEl => {
    if (!formEl) return;
    formEl.resetFields();
    onSearch();
  };

  onMounted(() => {
    onSearch();
  });

  return {
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
  };
}

function emailStatusLabel(status: unknown) {
  return Number(status) === 1 ? "成功" : "失败";
}

function formatLogTime(value?: unknown) {
  const timestamp = Number(value ?? 0);
  return timestamp > 0 ? dayjs(timestamp).format("YYYY-MM-DD HH:mm:ss") : "-";
}
