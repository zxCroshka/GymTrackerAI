import { describe, expect, it, vi } from "vitest";

import type { FetchImplementation } from "@/lib/api/client";
import { AuthStore, type AuthData } from "@/lib/auth/store";

const firstAuth: AuthData = {
  user_id: "018f4ea8-89af-47d7-b7e3-43d94f83292e",
  email: "athlete@example.com",
  access_token: "access-token-one",
  access_expires_at: "2026-08-02T12:15:00Z",
};

function dataResponse<T>(data: T, status = 200): Response {
  return new Response(JSON.stringify({ data, meta: { request_id: "request-1" } }), { status, headers: { "Content-Type": "application/json" } });
}

function unauthorized(): Response {
  return new Response(JSON.stringify({ code: "unauthorized", detail: "Authentication is required", request_id: "request-2" }), { status: 401, headers: { "Content-Type": "application/problem+json" } });
}

describe("AuthStore", () => {
  it("restores a session using the refresh cookie without browser token storage", async () => {
    const fetchMock = vi.fn<FetchImplementation>().mockResolvedValueOnce(dataResponse(firstAuth));
    const store = new AuthStore(fetchMock);
    localStorage.clear();

    await store.restore();

    expect(store.getSnapshot()).toMatchObject({ status: "authenticated", session: { accessToken: "access-token-one" } });
    expect(fetchMock).toHaveBeenCalledWith("/api/v1/auth/refresh", expect.objectContaining({ method: "POST", credentials: "include" }));
    expect(localStorage.length).toBe(0);
  });

  it("refreshes once after a private API 401 and retries with the new access token", async () => {
    const renewed = { ...firstAuth, access_token: "access-token-two", access_expires_at: "2026-08-02T12:30:00Z" };
    const fetchMock = vi.fn<FetchImplementation>()
      .mockResolvedValueOnce(dataResponse(firstAuth))
      .mockResolvedValueOnce(unauthorized())
      .mockResolvedValueOnce(dataResponse(renewed))
      .mockResolvedValueOnce(dataResponse({ user_id: firstAuth.user_id }));
    const store = new AuthStore(fetchMock);
    await store.login({ email: firstAuth.email, password: "very-long-password" });

    await store.api.request("/api/v1/profile");

    const oldHeaders = fetchMock.mock.calls[1][1]?.headers as Headers;
    const refreshHeaders = fetchMock.mock.calls[2][1]?.headers as Headers;
    const newHeaders = fetchMock.mock.calls[3][1]?.headers as Headers;
    expect(oldHeaders.get("Authorization")).toBe("Bearer access-token-one");
    expect(refreshHeaders.get("Authorization")).toBeNull();
    expect(newHeaders.get("Authorization")).toBe("Bearer access-token-two");
    expect(store.getSnapshot().session?.accessToken).toBe("access-token-two");
  });

  it("moves to unauthenticated when the refresh cookie is invalid", async () => {
    const fetchMock = vi.fn<FetchImplementation>().mockResolvedValueOnce(unauthorized());
    const store = new AuthStore(fetchMock);
    await store.restore();
    expect(store.getSnapshot()).toMatchObject({ status: "unauthenticated", session: null });
  });
});
