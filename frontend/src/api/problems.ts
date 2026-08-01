import type { Problem } from "./contracts";

export type ApiErrorKind = "transport" | "response" | "not_found" | "conflict" | "unauthorized";

export class ApiError extends Error {
  constructor(
    public readonly kind: ApiErrorKind,
    message: string,
    public readonly problem?: Problem,
  ) {
    super(message);
    this.name = "ApiError";
  }
}

export const isAbortError = (error: unknown): boolean =>
  error instanceof DOMException && error.name === "AbortError";
