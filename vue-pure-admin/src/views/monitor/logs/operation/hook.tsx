import dayjs from "dayjs";
import { getOperationLogsList } from "@/api/system";
import { downloadCsv, type CsvColumn } from "@/utils/csvExport";
import { message } from "@/utils/message";
import { usePublicHooks } from "@/views/system/hooks";
import type { PaginationProps } from "@pureadmin/table";
import { reactive, ref, onMounted, toRaw } from "vue";

type OperationLogRow = Record<string, any>;

const operationLogCsvColumns: CsvColumn<OperationLogRow>[] = [
  { header: "日志ID", value: row => row.id ?? "" },
  { header: "操作人员", value: row => row.username ?? "" },
  { header: "所属模块", value: row => row.module ?? "" },
  { header: "业务类型", value: row => row.summary ?? "" },
  { header: "请求方法", value: row => row.method ?? "" },
  { header: "处理器", value: row => row.handler ?? "" },
  { header: "请求地址", value: row => row.url ?? "" },
  { header: "操作IP", value: row => row.ip ?? "" },
  { header: "操作地点", value: row => row.address ?? "" },
  { header: "操作状态", value: row => operationStatusLabel(row.status) },
  { header: "响应结果", value: row => row.result ?? "" },
  { header: "耗时", value: row => operationLatency(row) },
  { header: "请求参数", value: row => row.params ?? "" },
  { header: "User-Agent", value: row => row.userAgent ?? "" },
  { header: "操作时间", value: row => formatLogTime(row.operatingTime) }
];

export function useRole() {
  const form = reactive({
    module: "",
    status: "",
    operatingTime: ""
  });
  const dataList = ref<OperationLogRow[]>([]);
  const loading = ref(true);
  const detailVisible = ref(false);
  const detailLog = ref<OperationLogRow | null>(null);
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
      label: "操作人员",
      prop: "username",
      minWidth: 100
    },
    {
      label: "所属模块",
      prop: "module",
      minWidth: 140
    },
    {
      label: "操作概要",
      prop: "summary",
      minWidth: 140
    },
    {
      label: "请求方法",
      prop: "method",
      minWidth: 100
    },
    {
      label: "操作 IP",
      prop: "ip",
      minWidth: 100
    },
    {
      label: "操作地点",
      prop: "address",
      minWidth: 140
    },
    {
      label: "操作系统",
      prop: "system",
      minWidth: 100
    },
    {
      label: "浏览器类型",
      prop: "browser",
      minWidth: 100
    },
    {
      label: "操作状态",
      prop: "status",
      minWidth: 100,
      cellRenderer: ({ row, props }) => (
        <el-tag size={props.size} style={tagStyle.value(row.status)}>
          {operationStatusLabel(row.status)}
        </el-tag>
      )
    },
    {
      label: "操作时间",
      prop: "operatingTime",
      minWidth: 180,
      formatter: ({ operatingTime }) => formatLogTime(operatingTime)
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
      const { code, data } = await getOperationLogsList({
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

  function openDetail(row: OperationLogRow) {
    detailLog.value = row;
    detailVisible.value = true;
  }

  function exportCurrentPage() {
    if (dataList.value.length === 0) {
      message("当前筛选条件下没有可导出的操作日志", { type: "warning" });
      return;
    }
    downloadCsv(
      `admin-operation-logs-${dayjs().format("YYYYMMDDHHmmss")}.csv`,
      operationLogCsvColumns,
      dataList.value
    );
    message(`已导出当前页 ${dataList.value.length} 条操作日志`, {
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
    operationLatency,
    operationStatusLabel
  };
}

function operationStatusLabel(status: unknown) {
  return Number(status) === 1 ? "成功" : "失败";
}

function operationLatency(row: OperationLogRow | null) {
  if (!row) return "-";
  if (row.latencyTime) return row.latencyTime;
  return row.takesTime ? `${row.takesTime}ms` : "-";
}

function formatLogTime(value?: unknown) {
  const timestamp = Number(value ?? 0);
  return timestamp > 0 ? dayjs(timestamp).format("YYYY-MM-DD HH:mm:ss") : "-";
}
