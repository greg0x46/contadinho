import { useState } from "react";

import { InvestmentAssetSettings } from "../components/InvestmentAssetSettings";
import { PageAction } from "../components/layout";
import { SettingsPageContainer } from "../components/SettingsPageContainer";

export function InvestmentAssetsSettingsPage() {
  const [creating, setCreating] = useState(false);
  return (
    <SettingsPageContainer
      title="Ativos de investimento"
      subTitle="Os instrumentos usados nas posições. Alterações valem para todas as contas que usam o ativo."
      compactMobileHeader
      extra={<PageAction label="Novo ativo" shortLabel="Novo" onClick={() => setCreating(true)} />}
    >
      <InvestmentAssetSettings creating={creating} onCreatingClose={() => setCreating(false)} />
    </SettingsPageContainer>
  );
}
