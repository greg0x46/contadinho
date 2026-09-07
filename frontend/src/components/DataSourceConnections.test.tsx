import { fireEvent, render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { MemoryRouter } from "react-router-dom";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { ApiError } from "../api/problems";
import { dataSource } from "../test/fixtures";
import { QueryTestProvider } from "../test/QueryTestProvider";
import { DataSourceConnections } from "./DataSourceConnections";
import * as dataSourcesApi from "../api/dataSources";

vi.mock("../api/dataSources");

const idleSync = { state: { kind: "idle" } as const, submit: vi.fn(), reset: vi.fn() };

const renderConnections = () =>
  render(
    <MemoryRouter>
      <QueryTestProvider>
        <DataSourceConnections sync={idleSync} />
      </QueryTestProvider>
    </MemoryRouter>,
  );

beforeEach(() => {
  vi.mocked(dataSourcesApi.listDataSources).mockResolvedValue([dataSource]);
  vi.mocked(dataSourcesApi.createDataSource).mockReset();
  vi.mocked(dataSourcesApi.updateDataSource).mockReset();
});

describe("connection registry", () => {
  it("tells the user what to do when nothing is registered yet", async () => {
    vi.mocked(dataSourcesApi.listDataSources).mockResolvedValue([]);
    renderConnections();

    expect(await screen.findByText("Nenhuma conexão cadastrada.")).toBeVisible();
    expect(screen.getByLabelText("Item ID (Pluggy)")).toBeVisible();
  });

  it("adds a connection with an optional nickname", async () => {
    vi.mocked(dataSourcesApi.createDataSource).mockResolvedValue(dataSource);
    renderConnections();

    await userEvent.type(await screen.findByLabelText("Item ID (Pluggy)"), "item-2");
    await userEvent.type(screen.getByLabelText("Apelido (opcional)"), "Empresa");
    await userEvent.click(screen.getByRole("button", { name: "Adicionar conexão" }));

    await waitFor(() =>
      expect(dataSourcesApi.createDataSource).toHaveBeenCalledWith({
        external_item_id: "item-2",
        label: "Empresa",
      }),
    );
  });

  it("reports why a duplicate item was refused", async () => {
    vi.mocked(dataSourcesApi.createDataSource).mockRejectedValue(
      new ApiError("conflict", "Conflict", {
        type: "/problems/data-source-exists",
        title: "Conexão já cadastrada",
        status: 409,
        detail: "Esse Item ID já está sincronizando neste Contadinho.",
      }),
    );
    renderConnections();

    await userEvent.type(await screen.findByLabelText("Item ID (Pluggy)"), "item-1");
    await userEvent.click(screen.getByRole("button", { name: "Adicionar conexão" }));

    expect(
      await screen.findByText("Esse Item ID já está sincronizando neste Contadinho."),
    ).toBeVisible();
  });

  it("retires a connection without deleting its history", async () => {
    const retired = { ...dataSource, is_active: false };
    vi.mocked(dataSourcesApi.updateDataSource).mockResolvedValue(retired);
    // The list is refetched after the change, so the second read is what the
    // user ends up looking at.
    vi.mocked(dataSourcesApi.listDataSources)
      .mockResolvedValueOnce([dataSource])
      .mockResolvedValue([retired]);
    renderConnections();

    await userEvent.click(await screen.findByRole("button", { name: "Desativar" }));

    await waitFor(() =>
      expect(dataSourcesApi.updateDataSource).toHaveBeenCalledWith(dataSource.id, {
        is_active: false,
      }),
    );
    expect(await screen.findByRole("button", { name: "Reativar" })).toBeVisible();
  });

  it("asks to sync only the chosen connection", async () => {
    renderConnections();
    await userEvent.click(await screen.findByRole("button", { name: /Sincronizar$/ }));
    expect(idleSync.submit).toHaveBeenCalledWith(dataSource.id);
  });

  // Confirming the inline editor always fires onChange, even with no edit —
  // opening it and blurring must not pin the displayed name as a permanent
  // label.
  it("does not update the label when the inline editor is confirmed unchanged", async () => {
    renderConnections();
    const list = await screen.findByLabelText("Conexões");
    await userEvent.click(within(list).getByRole("button", { name: "Renomear conexão" }));
    fireEvent.blur(within(list).getByRole("textbox"));

    await waitFor(() => expect(within(list).queryByRole("textbox")).not.toBeInTheDocument());
    expect(dataSourcesApi.updateDataSource).not.toHaveBeenCalled();
  });

  it("updates the label when the inline editor is confirmed with a real change", async () => {
    vi.mocked(dataSourcesApi.updateDataSource).mockResolvedValue({
      ...dataSource,
      label: "Nova conta",
      name: "Nova conta",
    });
    renderConnections();
    const list = await screen.findByLabelText("Conexões");
    await userEvent.click(within(list).getByRole("button", { name: "Renomear conexão" }));
    const textbox = within(list).getByRole("textbox");
    await userEvent.clear(textbox);
    await userEvent.type(textbox, "Nova conta");
    fireEvent.blur(textbox);

    await waitFor(() =>
      expect(dataSourcesApi.updateDataSource).toHaveBeenCalledWith(dataSource.id, {
        label: "Nova conta",
      }),
    );
  });
});
