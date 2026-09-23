import { describe, expect, it } from "vitest";

import { bankAccount, creditAccount } from "../test/accountFixtures";
import {
  accountDisplayName,
  accountHeaderTitle,
  accountIdentityLine,
  shortInstitutionName,
} from "./accountLabels";

describe("shortInstitutionName", () => {
  it("drops the legal-entity suffix and trailing descriptor", () => {
    expect(shortInstitutionName("Nu Pagamentos S.A. - Instituição de Pagamento")).toBe("Nu Pagamentos");
  });

  it("leaves a name with no suffix or descriptor untouched", () => {
    expect(shortInstitutionName("Banco Exemplo")).toBe("Banco Exemplo");
  });

  it("strips S/A and Ltda spellings too", () => {
    expect(shortInstitutionName("Banco Exemplo S/A")).toBe("Banco Exemplo");
    expect(shortInstitutionName("Corretora Exemplo Ltda.")).toBe("Corretora Exemplo");
  });
});

describe("accountHeaderTitle", () => {
  it("prefers the short institution name over the account's own name", () => {
    const account = { ...bankAccount, institution_name: "Nu Pagamentos S.A. - Instituição de Pagamento" };
    expect(accountHeaderTitle(account)).toBe("Nu Pagamentos");
  });

  // institution_name is already the display-safe field (see contracts.ts):
  // when it is null — a non-real institution like Pluggy's proxy connector,
  // filtered out server-side — this must never fall back to the raw
  // `institution` field, only to the account's own name.
  it("falls back to the account name when institution_name is unavailable", () => {
    const account = { ...bankAccount, institution: "MeuPluggy", institution_name: null };
    expect(accountHeaderTitle(account)).toBe(bankAccount.name);
  });
});

describe("accountIdentityLine", () => {
  it("joins the subtype label and masked number", () => {
    expect(accountIdentityLine(bankAccount)).toBe("Conta corrente · •••• 3456");
  });

  it("renders the credit card's subtype with no number available", () => {
    expect(accountIdentityLine(creditAccount)).toBe("Cartão de crédito");
  });
});

describe("accountDisplayName", () => {
  it("never falls back to the raw institution field", () => {
    const account = { ...bankAccount, name: null, institution: "MeuPluggy", institution_name: null };
    expect(accountDisplayName(account)).toBe("Conta sem nome");
  });
});
