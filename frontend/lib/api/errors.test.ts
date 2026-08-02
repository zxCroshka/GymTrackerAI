import { describe, expect, it } from "vitest";

import { ApiError, apiErrorMessage, parseApiError } from "@/lib/api/errors";

describe("API errors", () => {
  it("parses the backend problem envelope and retry metadata", async () => {
    const error = await parseApiError(new Response(JSON.stringify({ code: "rate_limited", detail: "Try later", request_id: "request-1" }), {
      status: 429,
      headers: { "Content-Type": "application/problem+json", "Retry-After": "17" },
    }));

    expect(error).toMatchObject({ status: 429, code: "rate_limited", requestId: "request-1", retryAfterSeconds: 17 });
    expect(apiErrorMessage(error)).toContain("Слишком много запросов");
  });

  it("does not expose an unknown backend detail to the interface", () => {
    const error = new ApiError({ status: 500, code: "database_internal", message: "sensitive detail" });
    expect(apiErrorMessage(error)).toBe("На сервере произошла ошибка. Повторите попытку позже.");
  });
});
