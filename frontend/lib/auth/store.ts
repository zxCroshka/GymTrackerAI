import { ApiClient, type FetchImplementation } from "@/lib/api/client";
import { ApiError } from "@/lib/api/errors";

export interface AuthData {
  user_id: string;
  email: string;
  access_token: string;
  access_expires_at: string;
}

export interface Credentials {
  email: string;
  password: string;
}

export interface Session {
  userId: string;
  email: string;
  accessToken: string;
  accessExpiresAt: string;
}

export type AuthStatus = "bootstrapping" | "authenticated" | "unauthenticated" | "error";
export type AuthReason = "initial" | "login" | "register" | "refresh" | "expired" | "logout" | "unavailable";

export interface AuthSnapshot {
  status: AuthStatus;
  session: Session | null;
  reason: AuthReason;
}

type Listener = () => void;

export const serverAuthSnapshot: AuthSnapshot = { status: "bootstrapping", session: null, reason: "initial" };

export class AuthStore {
  readonly api: ApiClient;
  private snapshot: AuthSnapshot = serverAuthSnapshot;
  private readonly listeners = new Set<Listener>();
  private refreshPromise: Promise<boolean> | null = null;

  constructor(fetchImplementation: FetchImplementation = (input, init) => fetch(input, init)) {
    this.api = new ApiClient(fetchImplementation, {
      getAccessToken: () => this.snapshot.session?.accessToken ?? null,
      refreshAccessToken: () => this.refresh(),
    });
  }

  getSnapshot = (): AuthSnapshot => this.snapshot;

  subscribe = (listener: Listener): (() => void) => {
    this.listeners.add(listener);
    return () => this.listeners.delete(listener);
  };

  async restore(): Promise<void> {
    if (this.snapshot.status !== "bootstrapping" && this.snapshot.status !== "error") return;
    try {
      const restored = await this.refresh("initial");
      if (!restored) this.setSnapshot({ status: "unauthenticated", session: null, reason: "initial" });
    } catch {
      this.setSnapshot({ status: "error", session: null, reason: "unavailable" });
    }
  }

  async login(credentials: Credentials): Promise<void> {
    const result = await this.api.request<AuthData>("/api/v1/auth/login", {
      method: "POST",
      json: credentials,
      authenticated: false,
      retryAuthentication: false,
    });
    this.accept(result.data, "login");
  }

  async register(credentials: Credentials): Promise<void> {
    const result = await this.api.request<AuthData>("/api/v1/auth/register", {
      method: "POST",
      json: credentials,
      authenticated: false,
      retryAuthentication: false,
    });
    this.accept(result.data, "register");
  }

  async refresh(reason: AuthReason = "refresh"): Promise<boolean> {
    if (this.refreshPromise) return this.refreshPromise;
    this.refreshPromise = this.refreshInternal(reason).finally(() => {
      this.refreshPromise = null;
    });
    return this.refreshPromise;
  }

  async logout(): Promise<void> {
    try {
      if (this.snapshot.session) {
        await this.api.request<void>("/api/v1/auth/logout", { method: "POST" });
      }
    } finally {
      this.setSnapshot({ status: "unauthenticated", session: null, reason: "logout" });
    }
  }

  private async refreshInternal(reason: AuthReason): Promise<boolean> {
    try {
      const result = await this.api.request<AuthData>("/api/v1/auth/refresh", {
        method: "POST",
        authenticated: false,
        retryAuthentication: false,
      });
      this.accept(result.data, reason);
      return true;
    } catch (error) {
      if (error instanceof ApiError && (error.status === 401 || error.status === 403)) {
        const expirationReason: AuthReason = this.snapshot.status === "authenticated" ? "expired" : "initial";
        this.setSnapshot({ status: "unauthenticated", session: null, reason: expirationReason });
        return false;
      }
      throw error;
    }
  }

  private accept(data: AuthData, reason: AuthReason): void {
    this.setSnapshot({
      status: "authenticated",
      reason,
      session: {
        userId: data.user_id,
        email: data.email,
        accessToken: data.access_token,
        accessExpiresAt: data.access_expires_at,
      },
    });
  }

  private setSnapshot(snapshot: AuthSnapshot): void {
    this.snapshot = snapshot;
    for (const listener of this.listeners) listener();
  }
}

export const authStore = new AuthStore();
