import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { MemoryRouter, Route, Routes } from "react-router-dom";
import { describe, expect, it } from "vitest";
import { SettingsPage } from "./SettingsPage";
import { SecuritySettingsPage } from "./SecuritySettingsPage";

describe("SettingsPage", () => {
  it("links to all seven sections", () => {
    render(<MemoryRouter><SettingsPage /></MemoryRouter>);
    const sections = { Geral: "geral", Segurança: "seguranca", Cenários: "cenarios", Automações: "automacoes", Categorias: "categorias", "Open Banking": "open-banking", "Ativos de investimento": "ativos-de-investimento" };
    for (const [name, path] of Object.entries(sections)) {
      expect(screen.getByRole("link", { name })).toHaveAttribute("href", `/configuracoes/${path}`);
    }
    expect(screen.queryByLabelText("Senha atual")).not.toBeInTheDocument();
  });

  it("opens a section using the keyboard and returns to the central", async () => {
    const user = userEvent.setup();
    render(<MemoryRouter initialEntries={["/configuracoes"]}><Routes>
      <Route path="/configuracoes" element={<SettingsPage />} />
      <Route path="/configuracoes/seguranca" element={<SecuritySettingsPage />} />
    </Routes></MemoryRouter>);
    screen.getByRole("link", { name: "Segurança" }).focus();
    await user.keyboard("{Enter}");
    expect(screen.getByLabelText("Senha atual")).toBeVisible();
    expect(screen.getByRole("link", { name: "Configurações" })).toHaveAttribute("href", "/configuracoes");
    await user.click(screen.getByRole("link", { name: "Voltar para configurações" }));
    expect(screen.getByRole("link", { name: "Geral" })).toBeVisible();
  });
});
