import type { APIErrorResponse } from "./types";

/** Thrown for non-2xx API responses with structured fields when available. */
export class ApiRequestError extends Error {
  readonly status: number;
  readonly code?: string;
  readonly details?: Record<string, unknown>;

  constructor(message: string, status: number, code?: string, details?: Record<string, unknown>) {
    super(message);
    this.name = "ApiRequestError";
    this.status = status;
    this.code = code;
    this.details = details;
  }
}

export function parseAPIErrorBody(raw: unknown): APIErrorResponse | null {
  if (!raw || typeof raw !== "object") {
    return null;
  }
  const o = raw as Record<string, unknown>;
  if (typeof o.error !== "string") {
    return null;
  }
  return {
    error: o.error,
    code: typeof o.code === "string" ? o.code : undefined,
    details:
      o.details !== undefined && o.details !== null && typeof o.details === "object"
        ? (o.details as Record<string, unknown>)
        : undefined
  };
}

/** Human-readable message for dashboards and debugging (multi-line when details exist). */
export function formatQueryError(error: unknown): string {
  if (error instanceof ApiRequestError) {
    const lines: string[] = [];
    if (error.code) {
      lines.push(`Code: ${error.code}`);
    }
    lines.push(error.message || "Unknown error");
    if (error.details && Object.keys(error.details).length > 0) {
      lines.push("", "Details:", JSON.stringify(error.details, null, 2));
    }
    return lines.join("\n");
  }
  if (error instanceof Error) {
    return error.message;
  }
  return typeof error === "string" ? error : "An unexpected error occurred.";
}
