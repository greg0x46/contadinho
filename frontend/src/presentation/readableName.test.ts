import { describe, expect, it } from "vitest";

import { readableName } from "./readableName";

describe("readableName", () => {
  it("drops legal-form boilerplate and title-cases a shouting institution", () => {
    expect(
      readableName(
        "CDB - NU FINANCEIRA S.A. - SOCIEDADE DE CREDITO, FINANCIAMENTO E INVESTIMENTO",
      ),
    ).toBe("CDB - Nu Financeira");
    expect(
      readableName(
        "Resgate CDB - NU FINANCEIRA S.A. - SOCIEDADE DE CREDITO, FINANCIAMENTO E INVESTIMENTO",
      ),
    ).toBe("Resgate CDB - Nu Financeira");
    expect(readableName("BANCO DO BRASIL S/A")).toBe("Banco do Brasil");
    expect(readableName("XP INVESTIMENTOS CCTVM S.A.")).toBe(
      "XP Investimentos Cctvm",
    );
  });

  it("leaves tickers, acronyms and already readable text alone", () => {
    expect(readableName("BBAS3")).toBe("BBAS3");
    expect(readableName("Resgate BBAS3")).toBe("Resgate BBAS3");
    expect(readableName("Resgate CDB - Nu Financeira")).toBe(
      "Resgate CDB - Nu Financeira",
    );
    expect(readableName("Tesouro IPCA+ 2029")).toBe("Tesouro IPCA+ 2029");
    expect(readableName("PIX ENVIADO")).toBe("PIX Enviado");
  });

  it("never returns an empty string", () => {
    expect(readableName("SOCIEDADE ANONIMA")).toBe("SOCIEDADE ANONIMA");
    expect(readableName("  Sem   descrição ")).toBe("Sem descrição");
  });
});
