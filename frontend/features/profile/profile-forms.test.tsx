import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";

import { ImportProfileForm } from "@/features/profile/import-form";
import { ProfileForm } from "@/features/profile/profile-form";
import type { Profile } from "@/features/profile/types";

const profile: Profile = {
  user_id: "018f4ea8-89af-47d7-b7e3-43d94f83292e", name: "Руслан", sex: "male", birth_date: null, height_cm: 170,
  goal: "recomposition", experience_level: "intermediate", training_frequency: 4, timezone: "Europe/Moscow", unit_system: "metric",
  sleep_hours_average: 8, notes: [], version: 1, created_at: "2026-08-02T10:00:00Z", updated_at: "2026-08-02T10:00:00Z",
};

describe("profile forms", () => {
  it("converts profile numeric inputs to the backend JSON shape", async () => {
    const user = userEvent.setup();
    const submit = vi.fn().mockResolvedValue(undefined);
    render(<ProfileForm profile={profile} onSubmit={submit} />);
    const height = screen.getByLabelText("Рост, см");
    await user.clear(height);
    await user.type(height, "171.5");
    await user.click(screen.getByRole("button", { name: "Сохранить профиль" }));
    await waitFor(() => expect(submit).toHaveBeenCalledWith(expect.objectContaining({ height_cm: 171.5, training_frequency: 4, timezone: "Europe/Moscow" })));
  });

  it("rejects unknown import fields and submits a strict valid document", async () => {
    const user = userEvent.setup();
    const submit = vi.fn().mockResolvedValue(undefined);
    render(<ImportProfileForm onSubmit={submit} />);
    const input = screen.getByLabelText("JSON-профиль");
    await user.click(input);
    await user.paste(JSON.stringify({ name: "Руслан", unknown: true }));
    await user.click(screen.getByRole("button", { name: "Импортировать JSON" }));
    expect(await screen.findByText("JSON содержит неизвестные или некорректные поля")).toBeInTheDocument();
    await user.clear(input);
    await user.paste(JSON.stringify({ name: "Руслан", weight_kg: 66.7, notes: ["Цель — прогресс"] }));
    await user.click(screen.getByRole("button", { name: "Импортировать JSON" }));
    await waitFor(() => expect(submit).toHaveBeenCalledWith({ name: "Руслан", weight_kg: 66.7, notes: ["Цель — прогресс"] }));
  });
});
