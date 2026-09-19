import { InvestmentAssetSettings } from "../components/InvestmentAssetSettings";
import { SettingsPageContainer } from "../components/SettingsPageContainer";

export function InvestmentAssetsSettingsPage() {
  return <SettingsPageContainer title="Ativos de investimento" subTitle="Gerencie o cadastro dos seus ativos">
    <InvestmentAssetSettings />
  </SettingsPageContainer>;
}
