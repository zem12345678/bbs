import dayjs from "dayjs";
import editForm from "../form.vue";
import { message } from "@/utils/message";
import { hasPerms } from "@/utils/auth";
import { ElMessageBox } from "element-plus";
import { usePublicHooks } from "../../hooks";
import { transformI18n } from "@/plugins/i18n";
import { addDialog } from "@/components/ReDialog";
import type { FormItemProps } from "../utils/types";
import type { PaginationProps } from "@pureadmin/table";
import { deviceDetection } from "@pureadmin/utils";
import {
  createRole,
  deleteRole,
  getRoleList,
  getRoleMenu,
  getRoleMenuIds,
  updateRole,
  updateRoleMenuIds
} from "@/api/system";
import { type Ref, reactive, ref, onMounted, h, toRaw, watch } from "vue";

const protectedRoleCodes = ["admin", "superadmin"];

export function isProtectedRole(row?: any) {
  const code = String(row?.code ?? row?.key ?? "").toLowerCase();
  return protectedRoleCodes.includes(code);
}

export function useRole(treeRef: Ref) {
  const form = reactive({
    name: "",
    code: "",
    status: ""
  });
  const curRow = ref();
  const formRef = ref();
  const dataList = ref([]);
  const treeIds = ref([]);
  const treeData = ref([]);
  const isShow = ref(false);
  const loading = ref(true);
  const isLinkage = ref(false);
  const treeSearchValue = ref();
  const switchLoadMap = ref({});
  const isExpandAll = ref(false);
  const isSelectAll = ref(false);
  const { switchStyle } = usePublicHooks();
  const treeProps = {
    value: "id",
    label: "title",
    children: "children"
  };
  const pagination = reactive<PaginationProps>({
    total: 0,
    pageSize: 10,
    currentPage: 1,
    background: true
  });
  const columns: TableColumnList = [
    {
      label: "角色编号",
      prop: "id"
    },
    {
      label: "角色名称",
      prop: "name"
    },
    {
      label: "角色标识",
      prop: "code"
    },
    {
      label: "状态",
      cellRenderer: scope => (
        <el-switch
          size={scope.props.size === "small" ? "small" : "default"}
          loading={switchLoadMap.value[scope.index]?.loading}
          v-model={scope.row.status}
          active-value={1}
          inactive-value={0}
          active-text="已启用"
          inactive-text="已停用"
          inline-prompt
          disabled={
            isProtectedRole(scope.row) || !hasPerms("system:update_system_role")
          }
          style={switchStyle.value}
          onChange={() => onChange(scope as any)}
        />
      ),
      minWidth: 90
    },
    {
      label: "备注",
      prop: "remark",
      minWidth: 160
    },
    {
      label: "创建时间",
      prop: "createTime",
      minWidth: 160,
      formatter: ({ createTime }) =>
        dayjs(createTime).format("YYYY-MM-DD HH:mm:ss")
    },
    {
      label: "操作",
      fixed: "right",
      width: 210,
      slot: "operation"
    }
  ];
  // const buttonClass = computed(() => {
  //   return [
  //     "h-5!",
  //     "reset-margin",
  //     "text-gray-500!",
  //     "dark:text-white!",
  //     "dark:hover:text-primary!"
  //   ];
  // });

  function onChange({ row, index }) {
    if (isProtectedRole(row)) {
      message("内置管理员角色不能修改", { type: "warning" });
      return;
    }
    if (!hasPerms("system:update_system_role")) {
      row.status = row.status === 0 ? 1 : 0;
      message("没有修改角色权限", { type: "warning" });
      return;
    }
    ElMessageBox.confirm(
      `确认要<strong>${
        row.status === 0 ? "停用" : "启用"
      }</strong><strong style='color:var(--el-color-primary)'>${
        row.name
      }</strong>吗?`,
      "系统提示",
      {
        confirmButtonText: "确定",
        cancelButtonText: "取消",
        type: "warning",
        dangerouslyUseHTMLString: true,
        draggable: true
      }
    )
      .then(async () => {
        switchLoadMap.value[index] = Object.assign(
          {},
          switchLoadMap.value[index],
          {
            loading: true
          }
        );
        await updateRole(row.id, row);
        setTimeout(() => {
          switchLoadMap.value[index] = Object.assign(
            {},
            switchLoadMap.value[index],
            {
              loading: false
            }
          );
          message(`已${row.status === 0 ? "停用" : "启用"}${row.name}`, {
            type: "success"
          });
        }, 300);
      })
      .catch(() => {
        row.status === 0 ? (row.status = 1) : (row.status = 0);
      });
  }

  async function handleDelete(row) {
    if (isProtectedRole(row)) {
      message("内置管理员角色不能修改", { type: "warning" });
      return;
    }
    if (!hasPerms("system:delete_system_role")) {
      message("没有删除角色权限", { type: "warning" });
      return;
    }
    await deleteRole(row.id);
    message(`已删除角色 ${row.name}`, { type: "success" });
    onSearch();
  }

  function handleSizeChange(val: number) {
    pagination.pageSize = val;
    pagination.currentPage = 1;
    onSearch();
  }

  function handleCurrentChange(val: number) {
    pagination.currentPage = val;
    onSearch();
  }

  function handleSelectionChange() {}

  async function onSearch() {
    loading.value = true;
    const { code, data } = await getRoleList({
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

    setTimeout(() => {
      loading.value = false;
    }, 500);
  }

  const resetForm = formEl => {
    if (!formEl) return;
    formEl.resetFields();
    onSearch();
  };

  function openDialog(title = "新增", row?: FormItemProps) {
    if (row && isProtectedRole(row)) {
      message("内置管理员角色不能修改", { type: "warning" });
      return;
    }
    if (title === "新增" && !hasPerms("system:create_system_role")) {
      message("没有新增角色权限", { type: "warning" });
      return;
    }
    if (title !== "新增" && !hasPerms("system:update_system_role")) {
      message("没有修改角色权限", { type: "warning" });
      return;
    }
    addDialog({
      title: `${title}角色`,
      props: {
        formInline: {
          name: row?.name ?? "",
          code: row?.code ?? "",
          remark: row?.remark ?? "",
          status: row?.status ?? 1,
          admin: row?.admin ?? false,
          sort: row?.sort ?? 0
        }
      },
      width: "40%",
      draggable: true,
      fullscreen: deviceDetection(),
      fullscreenIcon: true,
      closeOnClickModal: false,
      contentRenderer: () => h(editForm, { ref: formRef, formInline: null }),
      beforeSure: (done, { options }) => {
        const FormRef = formRef.value.getRef();
        const curData = options.props.formInline as FormItemProps;
        async function chores() {
          if (title === "新增") {
            await createRole(curData as any);
          } else if ((row as any)?.id) {
            await updateRole((row as any).id, curData as any);
          }
          message(`已${title === "新增" ? "新增" : "更新"}角色 ${curData.name}`, {
            type: "success"
          });
          done(); // 关闭弹框
          onSearch(); // 刷新表格数据
        }
        FormRef.validate(valid => {
          if (valid) {
            chores();
          }
        });
      }
    });
  }

  /** 菜单权限 */
  async function handleMenu(row?: any) {
    if (!row?.id) {
      curRow.value = null;
      isShow.value = false;
      return;
    }
    if (isProtectedRole(row)) {
      message("内置管理员角色权限不能修改", { type: "warning" });
      return;
    }
    if (!hasPerms("system:assign_system_role_menus")) {
      message("没有分配角色权限", { type: "warning" });
      return;
    }
    const { id } = row;
    curRow.value = row;
    isShow.value = true;
    const { code, data } = await getRoleMenuIds({ id });
    if (code === 0) {
      treeRef.value.setCheckedKeys(data);
    }
  }

  /** 高亮当前权限选中行 */
  function rowStyle({ row: { id } }) {
    return {
      cursor: "pointer",
      background: id === curRow.value?.id ? "var(--el-fill-color-light)" : ""
    };
  }

  /** 菜单权限-保存 */
  async function handleSave() {
    if (!curRow.value?.id) {
      return;
    }
    if (isProtectedRole(curRow.value)) {
      message("内置管理员角色权限不能修改", { type: "warning" });
      return;
    }
    if (!hasPerms("system:assign_system_role_menus")) {
      message("没有分配角色权限", { type: "warning" });
      return;
    }
    const { id, name } = curRow.value;
    await updateRoleMenuIds(id, treeRef.value.getCheckedKeys());
    message(`角色 ${name} 的菜单权限已保存`, {
      type: "success"
    });
  }

  const onQueryChanged = (query: string) => {
    treeRef.value!.filter(query);
  };

  const filterMethod = (query: string, node) => {
    return transformI18n(node.title)!.includes(query);
  };

  onMounted(async () => {
    onSearch();
    const { code, data } = await getRoleMenu();
    if (code === 0) {
      treeIds.value = collectTreeIds(data);
      treeData.value = data;
    }
  });

  function collectTreeIds(treeList) {
    return treeList.flatMap(item => [
      item.id,
      ...collectTreeIds(item.children ?? [])
    ]);
  }

  watch(isExpandAll, val => {
    val
      ? treeRef.value.setExpandedKeys(treeIds.value)
      : treeRef.value.setExpandedKeys([]);
  });

  watch(isSelectAll, val => {
    val
      ? treeRef.value.setCheckedKeys(treeIds.value)
      : treeRef.value.setCheckedKeys([]);
  });

  return {
    form,
    isShow,
    curRow,
    loading,
    columns,
    rowStyle,
    dataList,
    treeData,
    treeProps,
    isLinkage,
    pagination,
    isExpandAll,
    isSelectAll,
    treeSearchValue,
    // buttonClass,
    onSearch,
    resetForm,
    openDialog,
    handleMenu,
    handleSave,
    handleDelete,
    isProtectedRole,
    filterMethod,
    transformI18n,
    onQueryChanged,
    // handleDatabase,
    handleSizeChange,
    handleCurrentChange,
    handleSelectionChange
  };
}
