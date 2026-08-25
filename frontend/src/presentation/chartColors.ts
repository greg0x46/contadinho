// Fixed categorical trio shared by every chart that plots money in/out/net —
// the timeline's accumulated result, the category evolution and the net worth
// history. Validated with the dataviz skill's validator (adjacent-pair
// CVD/normal-vision/contrast all PASS in both light and dark): slot 1 (blue)
// for income, slot 8 (red) for expense, slot 7 (violet) for the net result —
// chosen over an arbitrary hue cycle because these three roles repeat across
// every chart in the app and must always mean the same thing.
export const monthlyEvolutionColor = {
  income: "#2a78d6",
  expense: "#e34948",
  result: "#4a3aa7",
} as const;
