import { describe, expect, it } from "vitest";

import { resolveTheme } from "./theme";

describe("resolveTheme", () => {
  it("uses a stored supported theme", () => {
    expect(resolveTheme("light", true)).toBe("light");
    expect(resolveTheme("dark", false)).toBe("dark");
  });

  it("falls back to the operating-system preference", () => {
    expect(resolveTheme(null, true)).toBe("dark");
    expect(resolveTheme("unsupported", false)).toBe("light");
  });
});
