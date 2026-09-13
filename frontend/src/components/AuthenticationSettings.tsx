import { Alert, Button, Card, Flex, Input } from "antd";
import { useState } from "react";
import { changePassword, logout } from "../api/auth";

export function AuthenticationSettings() {
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
        <label htmlFor="new-password">Nova senha (15–128 caracteres)</label>
        <Input.Password id="new-password" autoComplete="new-password" required value={password} onChange={(e) => setPassword(e.target.value)} />
        <label htmlFor="confirm-password">Repita a nova senha</label>
        <Input.Password id="confirm-password" autoComplete="new-password" required value={confirmation} onChange={(e) => setConfirmation(e.target.value)} />
        <Button htmlType="submit" loading={busy}>Alterar senha e encerrar sessões</Button>
        <Button onClick={() => void run(logout)} loading={busy}>Sair</Button>
      </Flex>
    </form>
  </Card>;
}
