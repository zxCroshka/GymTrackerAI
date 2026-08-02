import { z } from "zod";

const nullableNumberInput = (minimum: number, maximum: number, message: string) =>
  z.string().trim().refine((value) => value === "" || (Number.isFinite(Number(value)) && Number(value) >= minimum && Number(value) <= maximum), message).transform((value) => value === "" ? null : Number(value));

export const profileFormSchema = z.object({
  name: z.string().trim().max(100, "Максимум 100 символов").transform((value) => value || null),
  sex: z.enum(["", "male", "female", "other", "prefer_not_to_say"]).transform((value) => value || null),
  birth_date: z.string().refine((value) => value === "" || /^\d{4}-\d{2}-\d{2}$/.test(value), "Введите дату в формате ГГГГ-ММ-ДД").transform((value) => value || null),
  height_cm: nullableNumberInput(50, 300, "Рост должен быть от 50 до 300 см"),
  goal: z.enum(["", "muscle_gain", "weight_loss", "recomposition", "strength", "maintenance"]).transform((value) => value || null),
  experience_level: z.enum(["", "beginner", "intermediate", "advanced"]).transform((value) => value || null),
  training_frequency: nullableNumberInput(1, 7, "Частота должна быть от 1 до 7").refine((value) => value === null || Number.isInteger(value), "Введите целое число"),
  timezone: z.string().trim().min(1, "Укажите часовой пояс").max(255, "Значение слишком длинное"),
  unit_system: z.enum(["metric", "imperial"]),
});

export type ProfileFormInput = z.input<typeof profileFormSchema>;
export type ProfilePatch = z.output<typeof profileFormSchema>;

const optionalBoundedNumber = (minimum: number, maximum: number) => z.number().finite().min(minimum).max(maximum).optional();
const measurementsSchema = z.object({
  chest_cm: optionalBoundedNumber(5, 400), waist_cm: optionalBoundedNumber(5, 400), hips_cm: optionalBoundedNumber(5, 400), neck_cm: optionalBoundedNumber(5, 200), biceps_cm: optionalBoundedNumber(5, 200),
}).strict().refine((value) => Object.keys(value).length > 0, "Добавьте хотя бы один замер");

export const profileImportSchema = z.object({
  name: z.string().trim().min(1).max(100).optional(),
  sex: z.enum(["male", "female", "other", "prefer_not_to_say"]).optional(),
  height_cm: optionalBoundedNumber(50, 300),
  weight_kg: optionalBoundedNumber(20, 700),
  goal: z.enum(["muscle_gain", "weight_loss", "recomposition", "strength", "maintenance"]).optional(),
  training_frequency: z.number().int().min(1).max(7).optional(),
  experience_level: z.enum(["beginner", "intermediate", "advanced"]).optional(),
  sleep_hours_average: optionalBoundedNumber(0, 24),
  measurements: measurementsSchema.optional(),
  notes: z.array(z.string().trim().min(1).max(1000)).max(20).optional(),
}).strict().refine((value) => Object.keys(value).length > 0, "JSON-профиль не может быть пустым");

export type ProfileImport = z.infer<typeof profileImportSchema>;

export const importFormSchema = z.object({ json: z.string().trim().min(1, "Вставьте JSON-профиль") }).transform((value, context) => {
  try {
    const parsed = profileImportSchema.safeParse(JSON.parse(value.json));
    if (!parsed.success) {
      context.addIssue({ code: z.ZodIssueCode.custom, path: ["json"], message: "JSON содержит неизвестные или некорректные поля" });
      return z.NEVER;
    }
    return { json: value.json, profile: parsed.data };
  } catch {
    context.addIssue({ code: z.ZodIssueCode.custom, path: ["json"], message: "Не удалось разобрать JSON" });
    return z.NEVER;
  }
});

export type ImportFormInput = z.input<typeof importFormSchema>;
export type ImportFormOutput = z.output<typeof importFormSchema>;
