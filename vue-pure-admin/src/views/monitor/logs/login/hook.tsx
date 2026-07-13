import dayjs from "dayjs";
import { getLoginLogsList } from "@/api/system";
import { downloadCsv, type CsvColumn } from "@/utils/csvExport";
import { message } from "@/utils/message";
import { usePublicHooks } from "@/views/system/hooks";
import type { PaginationProps } from "@pureadmin/table";
import { reactive, ref, onMounted, toRaw } from "vue";

type LoginLogRow = Record<string, any>;

const loginLogCsvColumns: CsvColumn<LoginLogRow>[] = [
  { header: "日志ID", value: row => row.id ?? "" },
  { header: "用户名", value: row => row.username ?? "" },
  { header: "登录状态", value: row => loginStatusLabel(row.status) },
  { header: "登录IP", value: row => row.ip ?? "" },
  { header: "登录地点", value: row => row.address ?? "" },
  { header: "浏览器", value: row => row.browser ?? "" },
  { header: "操作系统", value: row => row.system ?? "" },
  { header: "平台", value: row => row.platform ?? "" },
  { header: "登录行为", value: row => row.behavior ?? "" },
  { header: "备注", value: row => row.remark ?? "" },
  { header: "登录时间", value: row => formatLogTime(row.loginTime) }
];

export function useRole() {
  const form = reactive({
    username: "",
    status: "",
    loginTime: ""
  });
  const dataList = ref<LoginLogRow[]>([]);
  const loading = ref(true);
  const detailVisible = ref(false);
  const detailLog = ref<LoginLogRow | null>(null);
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
      label: "用户名",
      prop: "username",
      minWidth: 100
    },
    {
      label: "登录 IP",
      prop: "ip",
      minWidth: 140
    },
    {
      label: "登录地点",
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
      label: "登录状态",
      prop: "status",
      minWidth: 100,
      cellRenderer: ({ row, props }) => (
        <el-tag size={props.size} style={tagStyle.value(row.status)}>
          {loginStatusLabel(row.status)}
        </el-tag>
      )
    },
    {
      label: "登录行为",
      prop: "behavior",
      minWidth: 100
    },
    {
      label: "登录时间",
      prop: "loginTime",
      minWidth: 180,
      formatter: ({ loginTime }) => formatLogTime(loginTime)
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
      const { code, data } = await getLoginLogsList({
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

  function openDetail(row: LoginLogRow) {
    detailLog.value = row;
    detailVisible.value = true;
  }

  function exportCurrentPage() {
    if (dataList.value.length === 0) {
      message("当前筛选条件下没有可导出的登录日志", { type: "warning" });
      return;
    }
    downloadCsv(
      `admin-login-logs-${dayjs().format("YYYYMMDDHHmmss")}.csv`,
      loginLogCsvColumns,
      dataList.value
    );
    message(`已导出当前页 ${dataList.value.length} 条登录日志`, {
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
    loginStatusLabel
  };
}

function loginStatusLabel(status: unknown) {
  return Number(status) === 1 ? "成功" : "失败";
}

function formatLogTime(value?: unknown) {
  const timestamp = Number(value ?? 0);
  return timestamp > 0 ? dayjs(timestamp).format("YYYY-MM-DD HH:mm:ss") : "-";
}
