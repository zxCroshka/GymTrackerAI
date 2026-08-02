import type { ProblemDetails } from "@/lib/api/types";

export class ApiError extends Error {
  readonly status: number;
  readonly code: string;
  readonly requestId: string | null;
  readonly retryAfterSeconds: number | null;

  constructor(options: {
    message: string;
    status: number;
    code: string;
    requestId?: string | null;
    retryAfterSeconds?: number | null;
    cause?: unknown;
  }) {
    super(options.message, { cause: options.cause });
    this.name = "ApiError";
    this.status = options.status;
    this.code = options.code;
    this.requestId = options.requestId ?? null;
    this.retryAfterSeconds = options.retryAfterSeconds ?? null;
  }
}

function retryAfter(response: Response): number | null {
  const raw = response.headers.get("Retry-After");
  if (!raw) return null;
  const seconds = Number.parseInt(raw, 10);
  return Number.isFinite(seconds) && seconds >= 0 ? seconds : null;
}

export async function parseApiError(response: Response): Promise<ApiError> {
  let problem: ProblemDetails = {};
  try {
    const raw = await response.text();
    if (raw) problem = JSON.parse(raw) as ProblemDetails;
  } catch {
    problem = {};
  }

  return new ApiError({
    status: response.status,
    code: problem.code ?? `http_${response.status}`,
    message: problem.detail ?? problem.title ?? "API request failed",
    requestId: problem.request_id ?? response.headers.get("X-Request-ID"),
    retryAfterSeconds: retryAfter(response),
  });
}

const messages: Record<string, string> = {
  network_error: "Не удалось связаться с сервером. Проверьте подключение и повторите попытку.",
  invalid_response: "Сервер вернул неожиданный ответ. Повторите попытку.",
  invalid_credentials: "Неверный email или пароль.",
  email_already_exists: "Аккаунт с таким email уже существует.",
  validation_failed: "Проверьте заполненные поля.",
  precondition_failed: "Данные изменились. Обновите страницу и повторите попытку.",
  precondition_required: "Не удалось определить текущую версию данных. Обновите страницу.",
  unauthorized: "Сессия истекла. Войдите снова.",
  invalid_refresh_token: "Сессия истекла. Войдите снова.",
  invalid_origin: "Запрос отклонён политикой безопасности.",
  rate_limited: "Слишком много запросов. Подождите и повторите попытку.",
  internal_error: "На сервере произошла ошибка. Повторите попытку позже.",
};

export function apiErrorMessage(error: unknown): string {
  if (error instanceof ApiError) {
    return messages[error.code] ?? (error.status >= 500 ? messages.internal_error : "Не удалось выполнить запрос.");
  }
  return "Произошла непредвиденная ошибка.";
}
