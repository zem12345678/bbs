export const OFFSET_LIST_PAGE_SIZE = 100;

type OffsetPage<T> = {
  items?: T[];
  total?: number;
};

export type OffsetPageParams = {
  limit: number;
  offset: number;
};

export type OffsetPageResponse<T> = {
  code: number;
  message?: string;
  data?: OffsetPage<T>;
};

export type OffsetPageResult<T> = {
  code: number;
  message?: string;
  items: T[];
  total: number;
};

export async function loadAllOffsetPages<T>(
  loadPage: (params: OffsetPageParams) => Promise<OffsetPageResponse<T>>
): Promise<OffsetPageResult<T>> {
  const items: T[] = [];
  let offset = 0;
  let total = 0;
  while (true) {
    const response = await loadPage({
      limit: OFFSET_LIST_PAGE_SIZE,
      offset
    });
    if (response.code !== 0) {
      return {
        code: response.code,
        message: response.message,
        items: [],
        total: 0
      };
    }
    const pageItems = response.data?.items ?? [];
    items.push(...pageItems);
    const reportedTotal = Number(response.data?.total);
    total =
      Number.isFinite(reportedTotal) && reportedTotal >= items.length
        ? reportedTotal
        : items.length;
    if (pageItems.length === 0 || items.length >= total) {
      return { code: 0, message: response.message, items, total };
    }
    offset += pageItems.length;
  }
}
