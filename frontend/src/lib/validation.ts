/**
 * Runtime validation for data crossing a trust boundary.
 *
 * Everything the API returns is `unknown` until it has been checked here.
 * These helpers are deliberately small and explicit: each one either returns a
 * value of the expected type or throws, so a malformed response can never be
 * rendered as if it were valid data.
 */

export class ValidationError extends Error {
  readonly path: string;

  constructor(message: string, path = "") {
    super(path ? `${path}: ${message}` : message);
    this.name = "ValidationError";
    this.path = path;
  }
}

export type Validator<T> = (value: unknown, path?: string) => T;

function fail(message: string, path: string): never {
  throw new ValidationError(message, path);
}

/** Validates a string. */
export const string = (value: unknown, path = "value"): string => {
  if (typeof value !== "string") {
    fail(`expected a string, received ${describe(value)}`, path);
  }
  return value;
};

/** Validates a finite number. Rejects NaN and Infinity. */
export const number = (value: unknown, path = "value"): number => {
  const parsed = typeof value === "string" && value.trim() !== "" ? Number(value) : value;
  if (typeof parsed !== "number" || !Number.isFinite(parsed)) {
    fail(`expected a finite number, received ${describe(value)}`, path);
  }
  return parsed;
};

/** Validates an integer. */
export const integer = (value: unknown, path = "value"): number => {
  const parsed = number(value, path);
  if (!Number.isInteger(parsed)) {
    fail(`expected an integer, received ${parsed}`, path);
  }
  return parsed;
};

/** Validates a non-negative number. */
export const nonNegative = (value: unknown, path = "value"): number => {
  const parsed = number(value, path);
  if (parsed < 0) {
    fail(`expected a non-negative number, received ${parsed}`, path);
  }
  return parsed;
};

/** Validates a boolean. */
export const boolean = (value: unknown, path = "value"): boolean => {
  if (typeof value !== "boolean") {
    fail(`expected a boolean, received ${describe(value)}`, path);
  }
  return value;
};

/** Validates a value against a fixed set of string literals. */
export function oneOf<T extends string>(allowed: readonly T[]): Validator<T> {
  return (value: unknown, path = "value"): T => {
    const parsed = string(value, path);
    if (!(allowed as readonly string[]).includes(parsed)) {
      fail(`expected one of ${allowed.join(", ")}, received ${parsed}`, path);
    }
    return parsed as T;
  };
}

/** Validates an array whose elements all satisfy `element`. */
export function array<T>(element: Validator<T>): Validator<T[]> {
  return (value: unknown, path = "value"): T[] => {
    if (!Array.isArray(value)) {
      fail(`expected an array, received ${describe(value)}`, path);
    }
    return value.map((item, index) => element(item, `${path}[${index}]`));
  };
}

/**
 * Validates an object against a field spec.
 *
 * Fields listed in `optional` may be absent or undefined; every other listed
 * field is required.
 */
export function object<T>(spec: {
  [K in keyof T]: Validator<T[K]>;
}, options: { optional?: readonly (keyof T)[]; path?: string } = {}): Validator<T> {
  const optional = new Set<keyof T>(options.optional ?? []);
  const basePath = options.path ?? "value";

  return (value: unknown, path = basePath): T => {
    if (value === null || typeof value !== "object" || Array.isArray(value)) {
      fail(`expected an object, received ${describe(value)}`, path);
    }

    const record = value as Record<string, unknown>;
    const result = {} as T;

    for (const key of Object.keys(spec) as (keyof T)[]) {
      const fieldPath = `${path}.${String(key)}`;
      if (record[key as string] === undefined) {
        if (optional.has(key)) {
          continue;
        }
        // A field wrapped in `withDefault` supplies its own value when the key
        // is absent. Without this check the required-field guard would fire
        // first and the default could never apply, so every such field would
        // behave as if it were required.
        if (!hasDefault(spec[key])) {
          fail("required field is missing", fieldPath);
        }
      }
      result[key] = spec[key](record[key as string], fieldPath);
    }

    return result;
  };
}

/** Validates a value that may be null. */
export function nullable<T>(inner: Validator<T>): Validator<T | null> {
  return (value: unknown, path = "value"): T | null => {
    if (value === null || value === undefined) {
      return null;
    }
    return inner(value, path);
  };
}

/** Validates a hex address (0x followed by 40 hex characters). */
export const address = (value: unknown, path = "value"): string => {
  const parsed = string(value, path);
  if (!/^0x[0-9a-fA-F]{40}$/.test(parsed)) {
    fail("expected a 20-byte hex address", path);
  }
  return parsed;
};

/** Validates a hex string of any length prefixed with 0x. */
export const hex = (value: unknown, path = "value"): string => {
  const parsed = string(value, path);
  if (!/^0x[0-9a-fA-F]*$/.test(parsed)) {
    fail("expected a 0x-prefixed hex string", path);
  }
  return parsed;
};

/**
 * Validates a token symbol.
 *
 * Restricted to an allow-list of characters so a symbol can never carry
 * markup into the DOM.
 */
export const symbol = (value: unknown, path = "value"): string => {
  const parsed = string(value, path);
  if (parsed.length === 0 || parsed.length > 16) {
    fail("symbol must be between 1 and 16 characters", path);
  }
  if (!/^[A-Za-z0-9._-]+$/.test(parsed)) {
    fail("symbol contains disallowed characters", path);
  }
  return parsed;
};

/**
 * Validates a market symbol such as `BTC/RUSD`.
 *
 * Derivatives listings, perpetual markets and margin pairs all use the
 * `BASE/QUOTE` form, which the single-token `symbol` validator rejects. Both
 * halves are drawn from the same allow-list as `symbol`, so a market symbol can
 * no more carry markup into the DOM than a token symbol can.
 */
export const marketSymbol = (value: unknown, path = "value"): string => {
  const parsed = string(value, path);
  if (parsed.length === 0 || parsed.length > 41) {
    fail("market symbol must be between 1 and 41 characters", path);
  }
  if (!/^[A-Za-z0-9._-]{1,20}(\/[A-Za-z0-9._-]{1,20})?$/.test(parsed)) {
    fail("market symbol must be a symbol or a BASE/QUOTE pair", path);
  }
  return parsed;
};

/**
 * Validates free text that will be displayed.
 *
 * Length-capped and stripped of control characters. The value is returned as
 * data, never as markup: React escapes it on render.
 */
export const displayText = (value: unknown, path = "value", maxLength = 256): string => {
  const parsed = string(value, path);
  if (parsed.length > maxLength) {
    fail(`text exceeds ${maxLength} characters`, path);
  }
  return parsed.replace(/[\u0000-\u0008\u000b\u000c\u000e-\u001f\u007f]/g, "");
};

/** Validates a value that must be `true`. */
export const literalTrue = (value: unknown, path = "value"): true => {
  if (value !== true) {
    fail("expected true", path);
  }
  return true;
};

/** Validates a value that must be `false`. */
export const literalFalse = (value: unknown, path = "value"): false => {
  if (value !== false) {
    fail("expected false", path);
  }
  return false;
};

/** Validates an ISO-8601 timestamp string. */
export const timestamp = (value: unknown, path = "value"): string => {
  const parsed = string(value, path);
  const milliseconds = Date.parse(parsed);
  if (Number.isNaN(milliseconds)) {
    fail("expected an ISO-8601 timestamp", path);
  }
  return parsed;
};

/**
 * Marker set by `withDefault` so the object validator can tell a
 * default-supplying field from a required one.
 */
const SUPPLIES_DEFAULT = Symbol("tigerswap.suppliesDefault");

/** True when a validator supplies its own value for a missing key. */
export function hasDefault(validator: unknown): boolean {
  return (
    typeof validator === "function" &&
    (validator as unknown as Record<symbol, unknown>)[SUPPLIES_DEFAULT] === true
  );
}

/** Wraps a validator so a missing value becomes a default. */
export function withDefault<T>(inner: Validator<T>, fallback: T): Validator<T> {
  const wrapped: Validator<T> = (value: unknown, path = "value"): T => {
    if (value === undefined || value === null) {
      return fallback;
    }
    return inner(value, path);
  };
  Object.defineProperty(wrapped, SUPPLIES_DEFAULT, { value: true });
  return wrapped;
}

/**
 * Runs a validator and returns a result object instead of throwing.
 *
 * Useful where a single bad field should degrade one row rather than the whole
 * page.
 */
export function tryValidate<T>(
  validator: Validator<T>,
  value: unknown,
  path = "value",
): { ok: true; value: T } | { ok: false; error: ValidationError } {
  try {
    return { ok: true, value: validator(value, path) };
  } catch (error) {
    if (error instanceof ValidationError) {
      return { ok: false, error };
    }
    return {
      ok: false,
      error: new ValidationError(error instanceof Error ? error.message : "validation failed", path),
    };
  }
}

/** Validates each element of an array, dropping the ones that fail. */
export function arrayLenient<T>(element: Validator<T>): Validator<T[]> {
  return (value: unknown, path = "value"): T[] => {
    if (!Array.isArray(value)) {
      fail(`expected an array, received ${describe(value)}`, path);
    }
    const out: T[] = [];
    value.forEach((item, index) => {
      const result = tryValidate(element, item, `${path}[${index}]`);
      if (result.ok) {
        out.push(result.value);
      }
    });
    return out;
  };
}

/**
 * Parsers for values typed into admin forms.
 *
 * Form inputs arrive as strings, so these convert and range-check them before
 * anything is sent to the server. They throw `ValidationError` on bad input,
 * which the calling component shows to the operator.
 */

/** Parses a decimal number from a form field. */
export function parseDecimal(raw: string, path = "value"): string {
  const trimmed = raw.trim();
  if (!/^-?\d+(\.\d+)?$/.test(trimmed)) {
    fail("expected a decimal number", path);
  }
  return trimmed;
}

/** Parses a base-10 integer from a form field, optionally range-checked. */
export function parseInteger(raw: string, path = "value", bounds: { min?: number; max?: number } = {}): number {
  const trimmed = raw.trim();
  if (!/^\d+$/.test(trimmed)) {
    fail("expected a whole number", path);
  }
  const parsed = Number(trimmed);
  if (!Number.isSafeInteger(parsed)) {
    fail("number is out of range", path);
  }
  if (bounds.min !== undefined && parsed < bounds.min) {
    fail(`must be at least ${bounds.min}`, path);
  }
  if (bounds.max !== undefined && parsed > bounds.max) {
    fail(`must be at most ${bounds.max}`, path);
  }
  return parsed;
}

/** Parses a ratio in [0, 1] from a form field, as used by fee and LTV inputs. */
export function parseRatio(raw: string, path = "value"): number {
  const trimmed = raw.trim();
  if (!/^\d+(\.\d+)?$/.test(trimmed)) {
    fail("expected a ratio between 0 and 1", path);
  }
  const parsed = Number(trimmed);
  if (!Number.isFinite(parsed) || parsed < 0 || parsed > 1) {
    fail("ratio must be between 0 and 1", path);
  }
  return parsed;
}

/** Parses a BASE/QUOTE market symbol from a form field. */
export function parseSymbol(raw: string, path = "value"): string {
  const trimmed = raw.trim().toUpperCase();
  if (!/^[A-Z0-9._-]{1,20}\/[A-Z0-9._-]{1,20}$/.test(trimmed)) {
    fail("symbol must have the form BASE/QUOTE", path);
  }
  return trimmed;
}

function describe(value: unknown): string {
  if (value === null) return "null";
  if (Array.isArray(value)) return "an array";
  return typeof value;
}
