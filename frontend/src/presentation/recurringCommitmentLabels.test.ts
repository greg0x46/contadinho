import { describe, expect, it } from "vitest";

import {
  formatNextDay,
  nextOccurrenceDate,
  recurrenceScheduleSentence,
  recurrenceStanding,
} from "./recurringCommitmentLabels";

const monthly = { cadence: "monthly" as const, month_of_year: null, start_date: "2025-01-01", end_date: null };

describe("nextOccurrenceDate", () => {
  it("is this month when the day has not passed, next month when it has", () => {
    expect(nextOccurrenceDate({ ...monthly, day_of_month: 10 }, "2026-10-02")).toBe("2026-10-10");
    expect(nextOccurrenceDate({ ...monthly, day_of_month: 10 }, "2026-10-10")).toBe("2026-10-10");
    expect(nextOccurrenceDate({ ...monthly, day_of_month: 1 }, "2026-10-02")).toBe("2026-11-01");
  });

  it("lands day 31 on the last day of a shorter month, like the backend", () => {
    expect(nextOccurrenceDate({ ...monthly, day_of_month: 31 }, "2026-02-02")).toBe("2026-02-28");
  });

  it("waits for the start date and stops at the end date", () => {
    expect(nextOccurrenceDate({ ...monthly, day_of_month: 5, start_date: "2027-03-01" }, "2026-10-02")).toBe(
      "2027-03-05",
    );
    expect(nextOccurrenceDate({ ...monthly, day_of_month: 5, end_date: "2026-09-30" }, "2026-10-02")).toBeNull();
  });

  it("only counts the month of an annual schedule", () => {
    const annual = { ...monthly, cadence: "annual" as const, month_of_year: 3, day_of_month: 18 };
    expect(nextOccurrenceDate(annual, "2026-10-02")).toBe("2027-03-18");
    expect(nextOccurrenceDate(annual, "2026-03-10")).toBe("2026-03-18");
  });
});

describe("recurrence copy", () => {
  it("formats the next day with the year only when it is not the current one", () => {
    expect(formatNextDay("2026-10-10", "2026-10-02")).toBe("10/10");
    expect(formatNextDay("2027-03-18", "2026-10-02")).toBe("18/03/2027");
  });

  it("reads the schedule as a sentence", () => {
    expect(recurrenceScheduleSentence("monthly", 10, null)).toBe("todo dia 10");
    expect(recurrenceScheduleSentence("annual", 18, 3)).toBe("dia 18 de março");
  });

  it("reports a standing only when paused or ended", () => {
    expect(recurrenceStanding({ is_active: true, end_date: null }, "2026-10-02")).toBeNull();
    expect(recurrenceStanding({ is_active: false, end_date: null }, "2026-10-02")).toBe("paused");
    expect(recurrenceStanding({ is_active: true, end_date: "2026-09-30" }, "2026-10-02")).toBe("ended");
  });
});
