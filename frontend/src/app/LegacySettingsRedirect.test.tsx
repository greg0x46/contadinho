import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { MemoryRouter, Route, Routes, useLocation, useNavigate } from "react-router-dom";
import { describe, expect, it } from "vitest";
import { LegacySettingsRedirect } from "./LegacySettingsRedirect";

function Destination() {
  const location = useLocation();
  const navigate = useNavigate();
  return <><p>{location.pathname}{location.search}{location.hash}</p><button onClick={() => navigate(-1)}>Voltar</button></>;
}

describe("LegacySettingsRedirect", () => {
  it.each(["cenarios", "automacoes", "categorias", "open-banking", "open-banking/sync-runs/123"])("preserves %s and replaces the history entry", async (path) => {
    const user = userEvent.setup();
    render(<MemoryRouter initialEntries={["/origem", `/${path}?page=2#detalhes`]} initialIndex={1}><Routes>
      <Route path="/origem" element={<p>Origem</p>} />
      <Route path={`/${path}`} element={<LegacySettingsRedirect />} />
      <Route path="/configuracoes/*" element={<Destination />} />
    </Routes></MemoryRouter>);
    expect(await screen.findByText(`/configuracoes/${path}?page=2#detalhes`)).toBeVisible();
    await user.click(screen.getByRole("button", { name: "Voltar" }));
    expect(screen.getByText("Origem")).toBeVisible();
  });
});
