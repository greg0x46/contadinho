import { describe, expect, it } from "vitest";
import { parseInvestmentAsset, parseInvestmentAssetClassification } from "./contracts";

const cryptoETF = {
  id: "bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbb2", name: "Hashdex", ticker: "HASH11",
  asset_type: "ETF de criptoativos", asset_class: "crypto", currency_code: "BRL",
  quote_source: "b3", quote_symbol: "HASH11",
  created_at: "2026-09-01T12:00:00Z", updated_at: "2026-09-01T12:00:00Z",
};

describe("asset financial classification contracts", () => {
  it("accepts financial exposure independently from the price market", () => {
    expect(parseInvestmentAsset(cryptoETF)).toEqual(cryptoETF);
    const classes = [{ asset_class: "crypto", label: "Criptoativos", types: [
      { name: "Criptomoeda", quote_market: "crypto" }, { name: "ETF de criptoativos", quote_market: "b3" },
    ] }];
    expect(parseInvestmentAssetClassification({ items: classes })).toEqual(classes);
  });

  it("rejects providers used as financial classes or price markets", () => {
    expect(() => parseInvestmentAsset({ ...cryptoETF, asset_class: "yahoo" })).toThrow();
    expect(() => parseInvestmentAssetClassification({ items: [{ asset_class: "crypto", label: "Criptoativos",
      types: [{ name: "Criptomoeda", quote_market: "coingecko" }] }] })).toThrow();
  });
});
