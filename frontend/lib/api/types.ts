export interface ApiMeta {
  request_id: string;
  next_cursor?: string;
}

export interface ApiEnvelope<T> {
  data: T;
  meta: ApiMeta;
}

export interface ApiResponse<T> {
  data: T;
  meta: ApiMeta;
  etag: string | null;
  response: Response;
}

export interface ProblemDetails {
  type?: string;
  title?: string;
  status?: number;
  detail?: string;
  instance?: string;
  code?: string;
  request_id?: string;
}
