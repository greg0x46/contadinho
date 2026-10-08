import { Button } from "antd";
import { useNavigate } from "react-router-dom";

import { EmptyState, Page } from "../components/layout";

/**
 * A wrong address: the shell's own page header and one quiet empty state,
 * instead of antd's full-size illustration — nothing here needs a picture.
 * The way out is a button that navigates, not a link wrapping a button
 * (nested interactive controls).
 */
export function NotFoundPage() {
  const navigate = useNavigate();
  return (
    <Page title="Página não encontrada" width="narrow">
      <EmptyState
        title="O endereço acessado não corresponde a uma página do Julius."
        action={<Button onClick={() => navigate("/")}>Voltar para o Início</Button>}
      />
    </Page>
  );
}
