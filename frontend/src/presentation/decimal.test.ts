import { describe, expect, it } from "vitest";
import { multiplyDecimals, subtractDecimals, sumDecimals } from "./decimal";
describe("investment decimal amounts", () => {
 it("funds fractional purchases exactly, including acquisition costs", () => {
  expect(sumDecimals([multiplyDecimals("3", "0.331"), "0.001", "0.002"])).toBe("0.996");
 });
 it("preserves amounts beyond Number's integer precision", () => {
  expect(subtractDecimals("9007199254740993.01", "9007199254740993.00")).toBe("0.01");
  expect(multiplyDecimals("9007199254740993", "2")).toBe("18014398509481986.00");
 });
});
