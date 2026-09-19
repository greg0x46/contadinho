import { AuthenticationSettings } from "../components/AuthenticationSettings";
import { SettingsPageContainer } from "../components/SettingsPageContainer";

export function SecuritySettingsPage() {
  return <SettingsPageContainer title="Segurança" subTitle="Gerencie sua senha e o acesso à sua conta">
    <AuthenticationSettings />
  </SettingsPageContainer>;
}
