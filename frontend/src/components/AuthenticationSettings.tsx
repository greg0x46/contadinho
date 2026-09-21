import { useQuery, useQueryClient } from "@tanstack/react-query";
import { Alert, Button, Card, Flex, Input, Skeleton, Typography } from "antd";
import { useState } from "react";
import { changePassword, getSession, logout, setAuthenticationEnabled } from "../api/auth";

export function AuthenticationSettings() {
  const client = useQueryClient();
  const { data: session, isLoading } = useQuery({
    queryKey: ["auth-session"],
    queryFn: ({ signal }) => getSession(signal),
  });
  const [current, setCurrent] = useState("");
  const [password, setPassword] = useState("");
  const [confirmation, setConfirmation] = useState("");
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const run = async (action: () => Promise<void>) => {
    setBusy(true); setError(null);
    try { await action(); } catch (err) { setError(err instanceof Error ? err.message : "Não foi possível concluir."); }
    finally { setBusy(false); }
  };
  const toggleAuthentication = async (enabled: boolean) => {
    setBusy(true); setError(null);
    try {
      const next = await setAuthenticationEnabled(enabled);
      client.setQueryData(["auth-session"], next);
    } catch (err) {
      setError(err instanceof Error ? err.message : "Não foi possível alterar a autenticação.");
    } finally { setBusy(false); }
  };
  if (isLoading || !session) return <Card title="Acesso" style={{ marginBottom: 24 }}><Skeleton active /></Card>;
  if (!session.authentication_enabled) return <Card title="Acesso" style={{ marginBottom: 24 }}>
    <Flex vertical gap="middle" style={{ maxWidth: 560 }}>
      {error && <Alert type="error" message={error} showIcon />}
      <Alert
        type="warning"
        showIcon
        message="Autenticação desativada"
        description="Qualquer pessoa com acesso a esta instância pode consultar e alterar os dados."
      />
      <Button
        type="primary"
        onClick={() => void toggleAuthentication(true)}
        loading={busy}
        style={{ alignSelf: "flex-start" }}
      >
        Ativar autenticação
      </Button>
    </Flex>
  </Card>;
  return <Card title="Acesso" style={{ marginBottom: 24 }}>
    <form onSubmit={(event) => {
      event.preventDefault();
      if (password !== confirmation) { setError("As senhas não coincidem."); return; }
      void run(() => changePassword(current, password));
    }}>
      <Flex vertical gap="middle" style={{ maxWidth: 440 }}>
        {error && <Alert type="error" message={error} showIcon />}
        <label htmlFor="current-password">Senha atual</label>
        <Input.Password id="current-password" autoComplete="current-password" required value={current} onChange={(e) => setCurrent(e.target.value)} />
        <Typography.Text type="secondary">A autenticação está ativada. Somente sessões válidas acessam os dados.</Typography.Text>
        <label htmlFor="new-password">Nova senha</label>
        <Input.Password id="new-password" autoComplete="new-password" required value={password} onChange={(e) => setPassword(e.target.value)} />
        <label htmlFor="confirm-password">Repita a nova senha</label>
        <Input.Password id="confirm-password" autoComplete="new-password" required value={confirmation} onChange={(e) => setConfirmation(e.target.value)} />
        <Button htmlType="submit" loading={busy}>Alterar senha e encerrar sessões</Button>
        <Button onClick={() => void run(logout)} loading={busy}>Sair</Button>
        <Button danger onClick={() => void toggleAuthentication(false)} loading={busy}>Desativar autenticação</Button>
      </Flex>
    </form>
  </Card>;
}
