import { render, screen, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

const mocks = vi.hoisted(() => ({ replace: vi.fn(), pathname: "/profile", auth: { status: "unauthenticated", retryRestore: vi.fn() } }));

vi.mock("next/navigation", () => ({
  usePathname: () => mocks.pathname,
  useRouter: () => ({ replace: mocks.replace }),
}));
vi.mock("@/components/auth/auth-provider", () => ({ useAuth: () => mocks.auth }));

import { ProtectedRoute } from "@/components/auth/protected-route";

describe("ProtectedRoute", () => {
  beforeEach(() => mocks.replace.mockClear());

  it("redirects an unauthenticated user and preserves a safe next path", async () => {
    render(<ProtectedRoute><p>Закрытые данные</p></ProtectedRoute>);
    await waitFor(() => expect(mocks.replace).toHaveBeenCalledWith("/login?next=%2Fprofile"));
    expect(screen.queryByText("Закрытые данные")).not.toBeInTheDocument();
  });

  it("renders private content only for an authenticated session", () => {
    mocks.auth.status = "authenticated";
    render(<ProtectedRoute><p>Закрытые данные</p></ProtectedRoute>);
    expect(screen.getByText("Закрытые данные")).toBeInTheDocument();
    mocks.auth.status = "unauthenticated";
  });
});
