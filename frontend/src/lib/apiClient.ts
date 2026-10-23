/**
 * Typed HTTP client for the TigerSwap API.
 *
 * Responsibilities:
 *  - resolve the API base URL from configuration, with a same-origin default
 *    so the browser never talks to an arbitrary host;
 *  - attach the session bearer token;
 *  - enforce a request timeout and a maximum response size;
 *  - surface failures as a typed error so callers can react to status codes;
 *  - never log or forward secrets.
 *
 * Responses are returned as `unknown`; callers must validate them with the
 * helpers in `validation.ts` before use.
 */

/** Error raised for any non-success API response. */
export class ApiError extends Error {
  readonly status: number;
  readonly code: string;

  constructor(message: string, status: number, code = "api_error") {
    super(message);
    this.name = "ApiError";
    this.status = status;
    this.code = code;
  }

  /** True when the session is missing or no longer valid. */
  get isUnauthorized(): boolean {
    return this.status === 401;
  }

  /** True when the caller lacks permission for the resource. */
  get isForbidden(): boolean {
    return this.status === 403;
  }

  /** True when the resource does not exist. */
  get isNotFound(): boolean {
    return this.status === 404;
  }
}

/** Raised when the network itself failed or the request timed out. */
export class NetworkError extends Error {
  constructor(message: string) {
    super(message);
    this.name = "NetworkError";
  }
}

export interface RequestOptions {
  method?: "GET" | "POST" | "PUT" | "PATCH" | "DELETE";
  body?: unknown;
  /** Extra headers, e.g. an idempotency key. */
  headers?: Record<string, string>;
  /** Overrides the default timeout. */
  timeoutMs?: number;
  /** Set to false to omit the bearer token (e.g. for login). */
  authenticated?: boolean;
  /** Allows the caller to cancel an in-flight request. */
  signal?: AbortSignal;
  /** Query parameters, encoded safely. */
  query?: Record<string, string | number | boolean | undefined>;
}

const DEFAULT_TIMEOUT_MS = 20_000;

/** 8 MiB: large enough for a page of market data, small enough to bound memory. */
const MAX_RESPONSE_BYTES = 8 * 1024 * 1024;

/**
 * Resolves the API base URL.
 *
 * An explicit `NEXT_PUBLIC_API_URL` wins. Otherwise the client uses a
 * relative base, which keeps requests same-origin and lets the Next.js rewrite
 * in `next.config.js` proxy them to the backend.
 */
export function resolveApiBase(): string {
  const configured = process.env.NEXT_PUBLIC_API_URL?.trim();
  if (configured) {
    return configured.replace(/\/+$/, "");
  }
  return "/api/v1";
}

/** In-memory session token. Never written to persistent storage. */
let sessionToken: string | null = null;

export function setSessionToken(token: string | null): void {
  sessionToken = token && token.length > 0 ? token : null;
}

export function getSessionToken(): string | null {
  return sessionToken;
}

export function clearSessionToken(): void {
  sessionToken = null;
}

function buildUrl(base: string, path: string, query?: RequestOptions["query"]): string {
  const normalised = path.startsWith("/") ? path : `/${path}`;
  let url = `${base}${normalised}`;

  if (query) {
    const params = new URLSearchParams();
    for (const [key, value] of Object.entries(query)) {
      if (value === undefined) {
        continue;
      }
      params.append(key, String(value));
    }
    const serialised = params.toString();
    if (serialised) {
      url += `?${serialised}`;
    }
  }

  return url;
}

/**
 * Performs a request and returns the decoded JSON body as `unknown`.
 *
 * The body is size-capped and parsed defensively: a malformed or oversized
 * response becomes an `ApiError` rather than reaching the caller.
 */
export async function apiRequest(path: string, options: RequestOptions = {}): Promise<unknown> {
  const {
    method = "GET",
    body,
    headers = {},
    timeoutMs = DEFAULT_TIMEOUT_MS,
    authenticated = true,
    signal,
    query,
  } = options;

  const url = buildUrl(resolveApiBase(), path, query);

  const controller = new AbortController();
  const timeout = setTimeout(() => controller.abort(), timeoutMs);

  // Honour an external abort signal in addition to the timeout.
  if (signal) {
    if (signal.aborted) {
      clearTimeout(timeout);
      throw new NetworkError("request aborted");
    }
    signal.addEventListener("abort", () => controller.abort(), { once: true });
  }

  const requestHeaders: Record<string, string> = {
    Accept: "application/json",
    ...headers,
  };

  if (body !== undefined) {
    requestHeaders["Content-Type"] = "application/json";
  }

  if (authenticated && sessionToken) {
    requestHeaders.Authorization = `Bearer ${sessionToken}`;
  }

  let response: Response;
  try {
    response = await fetch(url, {
      method,
      headers: requestHeaders,
      body: body === undefined ? undefined : JSON.stringify(body),
      signal: controller.signal,
      credentials: "same-origin",
      cache: "no-store",
    });
  } catch (error) {
    if (error instanceof DOMException && error.name === "AbortError") {
      throw new NetworkError("request timed out");
    }
    throw new NetworkError(error instanceof Error ? error.message : "network request failed");
  } finally {
    clearTimeout(timeout);
  }

  const declaredLength = Number(response.headers.get("content-length") ?? "0");
  if (Number.isFinite(declaredLength) && declaredLength > MAX_RESPONSE_BYTES) {
    throw new ApiError(
      "response exceeds the maximum allowed size",
      response.status,
      "response_too_large",
    );
  }

  const text = await response.text();

  if (text.length > MAX_RESPONSE_BYTES) {
    throw new ApiError(
      "response exceeds the maximum allowed size",
      response.status,
      "response_too_large",
    );
  }

  if (!response.ok) {
    throw new ApiError(extractErrorMessage(text, response.status), response.status);
  }

  if (text.length === 0) {
    return null;
  }

  try {
    return JSON.parse(text) as unknown;
  } catch {
    throw new ApiError("response was not valid JSON", response.status, "invalid_json");
  }
}

/**
 * Pulls a human-readable message out of an error body without trusting its
 * shape. Falls back to a generic message for the status code.
 */
function extractErrorMessage(text: string, status: number): string {
  if (text.length > 0 && text.length < 4096) {
    try {
      const parsed = JSON.parse(text) as unknown;
      if (parsed && typeof parsed === "object") {
        const record = parsed as Record<string, unknown>;
        for (const key of ["error", "message", "detail"]) {
          const value = record[key];
          if (typeof value === "string" && value.length > 0) {
            // Strip control characters so the message cannot break a log line.
            return value.replace(/[\u0000-\u001f\u007f]/g, "");
          }
        }
      }
    } catch {
      // Fall through to the generic message.
    }
  }

  if (status === 401) return "authentication required";
  if (status === 403) return "permission denied";
  if (status === 404) return "resource not found";
  if (status === 429) return "too many requests";
  if (status >= 500) return "the service is temporarily unavailable";
  return "request failed";
}

export const api = {
  get: (path: string, options?: Omit<RequestOptions, "method" | "body">) =>
    apiRequest(path, { ...options, method: "GET" }),

  post: (path: string, body?: unknown, options?: Omit<RequestOptions, "method" | "body">) =>
    apiRequest(path, { ...options, method: "POST", body }),

  put: (path: string, body?: unknown, options?: Omit<RequestOptions, "method" | "body">) =>
    apiRequest(path, { ...options, method: "PUT", body }),

  delete: (path: string, body?: unknown, options?: Omit<RequestOptions, "method" | "body">) =>
    apiRequest(path, { ...options, method: "DELETE", body }),
};
