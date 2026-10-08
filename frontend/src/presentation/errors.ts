/**
 * The text to show for a failed request: the API client already turns
 * problem+json responses into an `Error` with a readable message, so use it;
 * anything else (a thrown string, a network failure with no message) falls
 * back to the caller's own copy.
 *
 * One shared helper instead of a copy per page; screens still being migrated
 * keep their local `errorMessage` until they are touched.
 */
export function errorMessage(error: unknown, fallback: string): string {
  return error instanceof Error && error.message !== "" ? error.message : fallback;
}
