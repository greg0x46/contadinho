import dayjs from "dayjs";
import { describe, expect, it } from "vitest";

import { dailyAllowance, daysRemainingInMonth } from "./dailyAllowance";
import { divideBRLFloor } from "./money";

describe("daysRemainingInMonth", () => {
  it("counts today", () => {
    expect(daysRemainingInMonth(dayjs("2026-10-01T09:00:00"))).toBe(31);
    expect(daysRemainingInMonth(dayjs("2026-11-01T23:59:00"))).toBe(30);
    expect(daysRemainingInMonth(dayjs("2026-10-20T15:30:00"))).toBe(12);
  });

  it("is 1 on the last day of the month", () => {
    expect(daysRemainingInMonth(dayjs("2026-01-31T00:00:00"))).toBe(1);
    expect(daysRemainingInMonth(dayjs("2026-02-28T23:59:00"))).toBe(1);
    expect(daysRemainingInMonth(dayjs("2028-02-29T12:00:00"))).toBe(1);
    expect(daysRemainingInMonth(dayjs("2028-02-28T12:00:00"))).toBe(2);
  });
});

describe("dailyAllowance", () => {
  it("splits a positive low point over the days left, flooring to the cent", () => {
    const today = dayjs("2026-10-20T10:00:00");
    expect(dailyAllowance("1000.00", today)).toBe("83.33");
    expect(dailyAllowance("10.00", dayjs("2026-10-29T10:00:00"))).toBe("3.33");
    expect(dailyAllowance("855.00", dayjs("2026-10-22T10:00:00"))).toBe("85.50");
  });

  it("is zero when the low point is zero or negative", () => {
    const today = dayjs("2026-10-20T10:00:00");
    expect(dailyAllowance("0.00", today)).toBe("0.00");
    expect(dailyAllowance("-200.00", today)).toBe("0.00");
  });

  it("is the whole low point on the last day of the month", () => {
    expect(dailyAllowance("123.45", dayjs("2026-01-31T08:00:00"))).toBe("123.45");
    expect(dailyAllowance("123.45", dayjs("2026-02-28T08:00:00"))).toBe("123.45");
    expect(dailyAllowance("123.45", dayjs("2028-02-29T08:00:00"))).toBe("123.45");
  });

  it("includes today on the first day of the month", () => {
    expect(dailyAllowance("3100.00", dayjs("2026-10-01T08:00:00"))).toBe("100.00");
    expect(dailyAllowance("3000.00", dayjs("2026-11-01T08:00:00"))).toBe("100.00");
  });
});

describe("divideBRLFloor", () => {
  it("drops the leftover fraction of a cent", () => {
    expect(divideBRLFloor("10.00", 3)).toBe("3.33");
    expect(divideBRLFloor("0.05", 2)).toBe("0.02");
    expect(divideBRLFloor("1234567.89", 1)).toBe("1234567.89");
  });

  it("rejects a non-positive or fractional divisor", () => {
    expect(() => divideBRLFloor("10.00", 0)).toThrow(RangeError);
    expect(() => divideBRLFloor("10.00", 1.5)).toThrow(RangeError);
  });
});
