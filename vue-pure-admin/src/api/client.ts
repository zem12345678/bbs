export type GatewayEnvelope<T> = {
  service?: string;
  trace_id?: string;
  request_id?: string;
  http_code?: number;
  code: number;
  reason?: string;
  message?: string;
  meta?: Record<string, unknown>;
  data: T;
};

export type PureResult<T> = {
  code: number;
  message: string;
  data: T;
};

export type PureTableResult<T> = PureResult<{
  list: T[];
  total: number;
  pageSize: number;
  currentPage: number;
}>;

export function unwrapGatewayData<T>(result: GatewayEnvelope<T> | T): T {
  if (isGatewayEnvelope<T>(result)) {
    return result.data;
  }
  return result;
}

export function toPureResult<T>(
  data: T,
  message = "操作成功"
): PureResult<T> {
  return {
    code: 0,
    message,
    data
  };
}

export function toPureTableResult<T>(
  result: {
    items?: T[];
    list?: T[];
    total?: number;
    page?: number;
    size?: number;
    currentPage?: number;
    pageSize?: number;
  },
  message = "查询成功"
): PureTableResult<T> {
  return {
    code: 0,
    message,
    data: {
      list: result.items ?? result.list ?? [],
      total: Number(result.total ?? 0),
      pageSize: Number(result.size ?? result.pageSize ?? 20),
      currentPage: Number(result.page ?? result.currentPage ?? 1)
    }
  };
}

function isGatewayEnvelope<T>(value: unknown): value is GatewayEnvelope<T> {
  return (
    !!value &&
    typeof value === "object" &&
    "code" in value &&
    "data" in value
  );
}
