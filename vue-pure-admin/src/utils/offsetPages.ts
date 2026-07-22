export const OFFSET_LIST_PAGE_SIZE = 100;
export const OFFSET_LIST_PAGE_CONCURRENCY = 4;

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

export type OffsetPageLoadOptions = {
  concurrency?: number;
};

export async function loadAllOffsetPages<T>(
  loadPage: (params: OffsetPageParams) => Promise<OffsetPageResponse<T>>,
  options: OffsetPageLoadOptions = {}
): Promise<OffsetPageResult<T>> {
  const firstResponse = await loadPage({
    limit: OFFSET_LIST_PAGE_SIZE,
    offset: 0
  });
  if (firstResponse.code !== 0) {
    return {
      code: firstResponse.code,
      message: firstResponse.message,
      items: [],
      total: 0
    };
  }

  const firstItems = firstResponse.data?.items ?? [];
  const reportedTotal = Number(firstResponse.data?.total);
  const total =
    Number.isFinite(reportedTotal) && reportedTotal >= firstItems.length
      ? reportedTotal
      : firstItems.length;
  if (firstItems.length === 0 || firstItems.length >= total) {
    return {
      code: 0,
      message: firstResponse.message,
      items: firstItems,
      total
    };
  }

  const offsets: number[] = [];
  for (
    let offset = firstItems.length;
    offset < total;
    offset += OFFSET_LIST_PAGE_SIZE
  ) {
    offsets.push(offset);
  }

  const pages = new Array<T[]>(offsets.length);
  const concurrency = normalizePageConcurrency(options.concurrency);
  let nextPageIndex = 0;
  let failure: { code: number; message?: string } | undefined;
  const workers = Array.from(
    { length: Math.min(concurrency, offsets.length) },
    async () => {
      while (!failure) {
        const pageIndex = nextPageIndex;
        nextPageIndex += 1;
        const offset = offsets[pageIndex];
        if (offset === undefined) return;

        const response = await loadPage({
          limit: OFFSET_LIST_PAGE_SIZE,
          offset
        });
        if (response.code !== 0) {
          failure = { code: response.code, message: response.message };
          return;
        }
        pages[pageIndex] = response.data?.items ?? [];
      }
    }
  );
  await Promise.all(workers);

  if (failure) {
    return {
      code: failure.code,
      message: failure.message,
      items: [],
      total: 0
    };
  }
  return {
    code: 0,
    message: firstResponse.message,
    items: firstItems.concat(...pages),
    total
  };
}

function normalizePageConcurrency(value: number | undefined) {
  const requested = Number(value);
  if (!Number.isFinite(requested)) return OFFSET_LIST_PAGE_CONCURRENCY;
  return Math.max(
    1,
    Math.min(OFFSET_LIST_PAGE_CONCURRENCY, Math.floor(requested))
  );
}
