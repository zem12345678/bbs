const columns: TableColumnList = [
  {
    sortable: true,
    label: "统计日期",
    prop: "date",
    minWidth: 120
  },
  {
    sortable: true,
    label: "新用户",
    prop: "newUsers"
  },
  {
    sortable: true,
    label: "文章",
    prop: "newArticles"
  },
  {
    sortable: true,
    label: "话题",
    prop: "newTopics"
  },
  {
    sortable: true,
    label: "评论",
    prop: "newComments"
  },
  {
    sortable: true,
    label: "举报",
    prop: "reports"
  },
  {
    sortable: true,
    label: "待处理",
    prop: "pendingReports",
    cellRenderer: ({ row }) => {
      const value = Number(row.pendingReports ?? 0);
      return (
        <el-tag type={value > 0 ? "warning" : "success"} effect="plain">
          {value}
        </el-tag>
      );
    }
  }
];

export { columns };
