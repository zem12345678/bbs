const ALL_OVERVIEW_REQUESTS_FAILED = "个人工作台加载失败，请稍后重试。";

export function dashboardOverviewLoadState(results = []) {
  const failedCount = results.filter((result) => result?.error).length;

  if (results.length > 0 && failedCount === results.length) {
    return { error: ALL_OVERVIEW_REQUESTS_FAILED, notice: "" };
  }

  return {
    error: "",
    notice: failedCount > 0 ? `部分数据暂不可用（${failedCount}/${results.length}）` : ""
  };
}

export function dashboardOverviewMetric(result, value, label) {
  return { value: result?.error ? "—" : value, label };
}
