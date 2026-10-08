// The Y axis exists to give the curve a scale, not to be read to the cent —
// so its ticks are rounded to thousands and carry no "R$": the currency is
// named by the figures above the chart and by the tooltip, and dropping it
// keeps the axis narrow enough that a tick never wraps on a phone.

/** pt-BR number with at most one decimal: 1.5 -> "1,5", 2 -> "2". */
function oneDecimal(value: number): string {
  return value.toFixed(1).replace(/\.0$/, "").replace(".", ",");
}

export function formatAxisMoney(value: number): string {
  const abs = Math.abs(value);
  // 999_500 is the first value that rounds to "1000 mil": from there it is "mi".
  if (abs >= 999_500) return `${value < 0 ? "-" : ""}${oneDecimal(abs / 1_000_000)} mi`;
  if (abs >= 1_000) {
    // Below 10 mil one decimal when it is not a whole thousand, so 1_500 and
    // 2_000 are not both "2 mil"; from 10 mil up whole thousands are enough.
    const text = abs < 10_000 ? oneDecimal(Math.round(abs / 100) / 10) : String(Math.round(abs / 1_000));
    return `${value < 0 ? "-" : ""}${text} mil`;
  }
  const rounded = Math.round(abs);
  // A value that rounds to zero is "0", never "-0".
  return `${value < 0 && rounded !== 0 ? "-" : ""}${rounded}`;
}
