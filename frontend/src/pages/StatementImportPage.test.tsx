import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { MemoryRouter } from "react-router-dom";
import { afterEach, expect, it, vi } from "vitest";
import * as statementApi from "../api/statementImports";
import { StatementImportPage } from "./StatementImportPage";

vi.mock("../api/statementImports");
afterEach(() => vi.clearAllMocks());

it("shows invalid rows and requires explicit partial import approval", async () => {
  const user = userEvent.setup();
  vi.mocked(statementApi.listImportAccounts).mockResolvedValue([]);
  vi.mocked(statementApi.listStatementImports).mockResolvedValue([]);
  vi.mocked(statementApi.previewStatement).mockResolvedValue({
    format: "flash_csv", format_version: "1", sha256: "abc", preview_fingerprint: "fingerprint", filename: "sample.csv",
    period_start: "2026-09-08", period_end: "2026-09-25", currency: "BRL",
    counts: { total: 2, new: 1, duplicate: 0, invalid: 1 }, warnings: [],
    rows: [
      { line_number: 2, occurred_at: "2026-09-08T06:17:00.000000000Z", description: "Depósito", amount: "50.00", currency: "BRL", payment_method: "Depósito", balance: "50.00", status: "new", errors: [], warnings: [] },
      { line_number: 3, occurred_at: null, description: "", amount: null, currency: null, payment_method: null, balance: null, status: "invalid", errors: ["data inválida"], warnings: [] },
    ],
  });
  vi.mocked(statementApi.confirmStatement).mockResolvedValue({ run_id: "run", account_id: "acc", counts: { total: 2, new: 1, duplicate: 0, invalid: 1 } });
  render(<MemoryRouter><StatementImportPage /></MemoryRouter>);
  await user.upload(screen.getByLabelText("Arquivo CSV"), new File(["synthetic"], "sample.csv", { type: "text/csv" }));
  expect(await screen.findByText("data inválida")).toBeVisible();
  expect(screen.getByText("08/09/2026 a 25/09/2026")).toBeVisible();
  await user.type(screen.getByLabelText("Nome da nova conta"), "Flash");
  const confirm = screen.getByRole("button", { name: "Importar 1 lançamento(s)" });
  expect(confirm).toBeDisabled();
  await user.click(screen.getByRole("checkbox", { name: /Importar somente as linhas válidas/ }));
  await user.click(confirm);
  await waitFor(() => expect(statementApi.confirmStatement).toHaveBeenCalledWith(expect.any(File), expect.any(Object), { newAccountName: "Flash" }, true));
  expect(await screen.findByText("Extrato importado")).toBeVisible();
});
