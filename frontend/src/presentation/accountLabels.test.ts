import { describe, expect, it } from "vitest";

import { bankAccount, creditAccount } from "../test/accountFixtures";
import {
  accountDisplayName,
  accountHeaderTitle,
  accountIdentityLine,
  accountMetaLine,
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
  // The record has one name: the list row and the heading of its detail page.
  it("is the very name the list row shows, whatever the institution", () => {
    const account = { ...bankAccount, institution_name: "Nu Pagamentos S.A. - Instituição de Pagamento" };
    expect(accountHeaderTitle(account)).toBe("Conta Corrente");
    expect(accountHeaderTitle(account)).toBe(accountDisplayName(account));
  });

  // institution_name is already the display-safe field (see contracts.ts):
  // when it is null — a non-real institution like Pluggy's proxy connector,
  // filtered out server-side — this must never fall back to the raw
  // `institution` field.
  it("never falls back to the raw institution field", () => {
    const account = { ...bankAccount, name: null, institution: "MeuPluggy", institution_name: null };
    expect(accountHeaderTitle(account)).toBe("Conta sem nome");
  });

  it("falls back to the short institution name for an account with no name of its own", () => {
    const account = {
      ...bankAccount,
      name: null,
      institution_name: "Nu Pagamentos S.A. - Instituição de Pagamento",
    };
    expect(accountHeaderTitle(account)).toBe("Nu Pagamentos");
  });
});

describe("accountMetaLine", () => {
  it("adds the short institution to the identity line, since the name does not carry it", () => {
    const account = { ...bankAccount, institution_name: "Nu Pagamentos S.A. - Instituição de Pagamento" };
    expect(accountMetaLine(account)).toBe("Conta corrente · •••• 3456 · Nu Pagamentos");
  });

  it("does not repeat the institution when it is the name", () => {
    const account = { ...bankAccount, name: null, institution_name: "Nu Pagamentos S.A." };
    expect(accountMetaLine(account)).toBe("Conta corrente · •••• 3456");
  });

  it("is empty for an account with nothing to say", () => {
    const account = { ...bankAccount, number: null, account_subtype: null, institution_name: null };
    expect(accountMetaLine(account)).toBe("");
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
