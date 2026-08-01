import { describe, expect, it } from "vitest";

import { formatDate, formatOptionalDate } from "./dates";

describe("date presentation", () => {
  it("formats valid dates in pt-BR local time", () => {
    expect(formatDate("2026-07-29T12:00:00Z")).toMatch(/\d{2}\/\d{2}\/2026/);
  });

  it("handles invalid and absent dates explicitly", () => {
    expect(formatDate("invalid")).toBe("Data inválida");
    expect(formatOptionalDate(null)).toBe("Ainda não disponível");
  });
});
