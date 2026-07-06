import dayjs from "dayjs";
import editForm from "../form.vue";
import { message } from "@/utils/message";
import { hasPerms } from "@/utils/auth";
import {
  createDepartment,
  deleteDepartment,
  getDeptList,
  updateDepartment
} from "@/api/system";
import { usePublicHooks } from "../../hooks";
import { addDialog } from "@/components/ReDialog";
import { reactive, ref, onMounted, h } from "vue";
import type { FormItemProps } from "../utils/types";
import { cloneDeep, isAllEmpty, deviceDetection } from "@pureadmin/utils";

export function useDept() {
  const form = reactive({
    name: "",
    status: null
  });

  const formRef = ref();
  const dataList = ref([]);
  const loading = ref(true);
  const { tagStyle } = usePublicHooks();

  function errorMessage(error: unknown) {
    const response = (error as any)?.response?.data;
    return (
      response?.message ?? response?.reason ?? (error as Error)?.message ?? ""
    );
  }

  const columns: TableColumnList = [
    {
      label: "部门名称",
      prop: "name",
      width: 180,
      align: "left"
    },
    {
      label: "排序",
      prop: "sort",
      minWidth: 70
    },
    {
      label: "状态",
      prop: "status",
      minWidth: 100,
      cellRenderer: ({ row, props }) => (
        <el-tag size={props.size} style={tagStyle.value(row.status)}>
          {row.status === 1 ? "启用" : "停用"}
        </el-tag>
      )
    },
    {
      label: "创建时间",
      minWidth: 200,
      prop: "createTime",
      formatter: ({ createTime }) =>
        dayjs(createTime).format("YYYY-MM-DD HH:mm:ss")
    },
    {
      label: "备注",
      prop: "remark",
      minWidth: 320
    },
    {
      label: "操作",
      fixed: "right",
      width: 210,
      slot: "operation"
    }
  ];

  function handleSelectionChange() {}

  function resetForm(formEl) {
    if (!formEl) return;
    formEl.resetFields();
    onSearch();
  }

  async function onSearch() {
    loading.value = true;
    const { code, data } = await getDeptList();
    if (code === 0) {
      let newData = data;
      if (!isAllEmpty(form.name)) {
        newData = filterDeptTree(newData, item => item.name.includes(form.name));
      }
      if (!isAllEmpty(form.status)) {
        newData = filterDeptTree(newData, item => item.status === form.status);
      }
      dataList.value = newData;
    }

    setTimeout(() => {
      loading.value = false;
    }, 500);
  }

  function collectTreeIds(row, ids = new Set<number>()) {
    const id = Number(row?.id ?? 0);
    if (id > 0) {
      ids.add(id);
    }
    for (const child of row?.children ?? []) {
      collectTreeIds(child, ids);
    }
    return ids;
  }

  function formatHigherDeptOptions(treeList, excludedIds = new Set<number>()) {
    if (!treeList || !treeList.length) return [];
    return treeList
      .filter(item => !excludedIds.has(Number(item.id)))
      .map(item => ({
        ...item,
        disabled: item.status === 0,
        children: formatHigherDeptOptions(item.children ?? [], excludedIds)
      }));
  }

  function openDialog(title = "新增", row?: FormItemProps) {
    if (title === "新增" && !hasPerms("system:create_system_dept")) {
      message("没有新增部门权限", { type: "warning" });
      return;
    }
    if (title !== "新增" && !hasPerms("system:update_system_dept")) {
      message("没有修改部门权限", { type: "warning" });
      return;
    }
    addDialog({
      title: `${title}部门`,
      props: {
        formInline: {
          higherDeptOptions: formatHigherDeptOptions(
            cloneDeep(dataList.value),
            title === "新增" ? new Set<number>() : collectTreeIds(row)
          ),
          parentId: row?.parentId ?? 0,
          name: row?.name ?? "",
          principal: row?.principal ?? "",
          phone: row?.phone ?? "",
          email: row?.email ?? "",
          sort: row?.sort ?? 0,
          status: row?.status ?? 1,
          remark: row?.remark ?? ""
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
          try {
            if (title === "新增") {
              await createDepartment(curData as any);
            } else if ((row as any)?.id) {
              await updateDepartment((row as any).id, curData as any);
            }
            message(`已${title === "新增" ? "新增" : "更新"}部门 ${curData.name}`, {
              type: "success"
            });
            done(); // 关闭弹框
            onSearch(); // 刷新表格数据
          } catch (error) {
            message(errorMessage(error) || `${title}部门失败`, {
              type: "error"
            });
          }
        }
        FormRef.validate(valid => {
          if (valid) {
            chores();
          }
        });
      }
    });
  }

  async function handleDelete(row) {
    if (!hasPerms("system:delete_system_dept")) {
      message("没有删除部门权限", { type: "warning" });
      return;
    }
    if (row?.children?.length > 0) {
      message("该部门存在子部门，请先删除子部门", { type: "warning" });
      return;
    }
    try {
      await deleteDepartment(row.id);
      message(`已删除部门 ${row.name}`, { type: "success" });
      onSearch();
    } catch (error) {
      message(errorMessage(error) || "删除部门失败", { type: "error" });
    }
  }

  function filterDeptTree(treeList, predicate: (item: any) => boolean) {
    return treeList
      .map(item => {
        const children = filterDeptTree(item.children ?? [], predicate);
        if (predicate(item) || children.length) {
          return { ...item, children };
        }
        return null;
      })
      .filter(Boolean);
  }

  onMounted(() => {
    onSearch();
  });

  return {
    form,
    loading,
    columns,
    dataList,
    /** 搜索 */
    onSearch,
    /** 重置 */
    resetForm,
    /** 新增、修改部门 */
    openDialog,
    /** 删除部门 */
    handleDelete,
    handleSelectionChange
  };
}
