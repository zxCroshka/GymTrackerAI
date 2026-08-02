import { ApiError, parseApiError } from "@/lib/api/errors";
import type { ApiEnvelope, ApiResponse } from "@/lib/api/types";

export type FetchImplementation = (input: RequestInfo | URL, init?: RequestInit) => Promise<Response>;

interface AuthBridge {
  getAccessToken(): string | null;
  refreshAccessToken(): Promise<boolean>;
}

export interface ApiRequestOptions extends Omit<RequestInit, "body"> {
  authenticated?: boolean;
  retryAuthentication?: boolean;
  json?: unknown;
  body?: BodyInit | null;
}

export class ApiClient {
  constructor(
    private readonly fetchImplementation: FetchImplementation,
    private readonly auth: AuthBridge,
  ) {}

  async request<T>(path: string, options: ApiRequestOptions = {}): Promise<ApiResponse<T>> {
    const response = await this.raw(path, options);

    if (response.status === 204) {
      return { data: undefined as T, meta: { request_id: response.headers.get("X-Request-ID") ?? "" }, etag: null, response };
    }

    let envelope: ApiEnvelope<T>;
    try {
      envelope = (await response.json()) as ApiEnvelope<T>;
    } catch (cause) {
      throw new ApiError({ message: "Invalid API response", status: response.status, code: "invalid_response", cause });
    }
    if (!envelope || !("data" in envelope) || !envelope.meta || typeof envelope.meta.request_id !== "string") {
      throw new ApiError({ message: "Invalid API envelope", status: response.status, code: "invalid_response" });
    }
    return { data: envelope.data, meta: envelope.meta, etag: response.headers.get("ETag"), response };
  }

  async raw(path: string, options: ApiRequestOptions = {}): Promise<Response> {
    const authenticated = options.authenticated ?? true;
    let response = await this.perform(path, options, authenticated);

    if (response.status === 401 && authenticated && (options.retryAuthentication ?? true)) {
      const refreshed = await this.auth.refreshAccessToken();
      if (refreshed) response = await this.perform(path, options, true);
    }

    if (!response.ok) throw await parseApiError(response);
    return response;
  }

  private async perform(path: string, options: ApiRequestOptions, authenticated: boolean): Promise<Response> {
    const headers = new Headers(options.headers);
    headers.set("Accept", "application/json");
    if (options.json !== undefined) headers.set("Content-Type", "application/json");
    if (authenticated) {
      const token = this.auth.getAccessToken();
      if (token) headers.set("Authorization", `Bearer ${token}`);
    }

    const { authenticated: _authenticated, retryAuthentication: _retry, json, ...requestOptions } = options;
    void _authenticated;
    void _retry;
    try {
      return await this.fetchImplementation(path, {
        ...requestOptions,
        headers,
        credentials: "include",
        body: json === undefined ? requestOptions.body : JSON.stringify(json),
      });
    } catch (cause) {
      throw new ApiError({ message: "Network request failed", status: 0, code: "network_error", cause });
    }
  }
}
