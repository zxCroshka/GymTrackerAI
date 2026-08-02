import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";

import { AuthForm } from "@/features/auth/auth-form";

describe("AuthForm", () => {
  it("validates login fields and submits normalized credentials", async () => {
    const user = userEvent.setup();
    const submit = vi.fn().mockResolvedValue(undefined);
    render(<AuthForm mode="login" onSubmit={submit} />);

    await user.click(screen.getByRole("button", { name: "Войти" }));
    expect(await screen.findByText("Введите email")).toBeInTheDocument();
    expect(screen.getByText("Минимум 12 символов")).toBeInTheDocument();

    await user.type(screen.getByLabelText("Email"), "Athlete@Example.com");
    await user.type(screen.getByLabelText("Пароль"), "very-long-password");
    await user.click(screen.getByRole("button", { name: "Войти" }));

    await waitFor(() => expect(submit).toHaveBeenCalledWith({ email: "athlete@example.com", password: "very-long-password" }));
  });

  it("rejects mismatching registration passwords", async () => {
    const user = userEvent.setup();
    const submit = vi.fn().mockResolvedValue(undefined);
    render(<AuthForm mode="register" onSubmit={submit} />);
    await user.type(screen.getByLabelText("Email"), "athlete@example.com");
    await user.type(screen.getByLabelText("Пароль"), "very-long-password");
    await user.type(screen.getByLabelText("Повторите пароль"), "different-password");
    await user.click(screen.getByRole("button", { name: "Создать аккаунт" }));
    expect(await screen.findByText("Пароли не совпадают")).toBeInTheDocument();
    expect(submit).not.toHaveBeenCalled();
  });
});
