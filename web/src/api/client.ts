export class ApiError extends Error {
  constructor(
    readonly status: number,
    readonly code: string,
    message: string,
    readonly fields: Record<string, string> = {},
  ) {
    super(message);
  }
}

async function request<T>(method: string, path: string, body?: unknown): Promise<T> {
  const form = body instanceof FormData; // the browser sets the multipart Content-Type (with its boundary)
  const res = await fetch(`/api${path}`, {
    method,
    credentials: "include",
    headers: body === undefined || form ? undefined : { "Content-Type": "application/json" },
    body: body === undefined ? undefined : form ? body : JSON.stringify(body),
  });
  if (res.status === 204) return undefined as T;
  if (!res.ok) throw await errorOf(res);
  return (await res.json().catch(() => null)) as T;
}

/** The ApiError for a refused response, read from its `{error: {code, message, fields}}` body. */
export async function errorOf(res: Response): Promise<ApiError> {
  const e = (await res.json().catch(() => null))?.error;
  return new ApiError(res.status, e?.code ?? "error", e?.message ?? "Algo deu errado. Tente de novo.", e?.fields);
}

export const api = {
  get: <T>(path: string) => request<T>("GET", path),
  post: <T>(path: string, body?: unknown) => request<T>("POST", path, body ?? {}),
  upload: <T>(path: string, form: FormData) => request<T>("POST", path, form),
  put: <T>(path: string, body: unknown) => request<T>("PUT", path, body),
  patch: <T>(path: string, body: unknown) => request<T>("PATCH", path, body),
  del: (path: string) => request<void>("DELETE", path),
};

export const isUnauthenticated = (e: unknown) => e instanceof ApiError && e.status === 401;

/** Text for a form's general error area: the message, or else the field errors the form has no place for (`shown` lists the ones it has), so none is lost. */
export function generalError(e: ApiError, shown: string[]): string {
  const keys = Object.keys(e.fields);
  if (keys.length === 0) return e.message;
  const cap = (s: string) => s[0].toUpperCase() + s.slice(1);
  return keys.filter((k) => !shown.includes(k)).map((k) => `${cap(e.fields[k])}.`).join(" ");
}
