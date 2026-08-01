import { Alert, Button, Card, Flex, Input, Typography } from "antd";
import { useState } from "react";

import { setup } from "../api/setup";
import type { SetupStatus } from "../api/contracts";

function errorMessage(error: unknown): string {
  return error instanceof Error ? error.message : "Não foi possível concluir a configuração.";
}

export function SetupPage({ onDone }: { onDone: (status: SetupStatus) => void }) {
  const [password, setPassword] = useState("");
  const [confirmPassword, setConfirmPassword] = useState("");
  const [clientId, setClientId] = useState("");
  const [clientSecret, setClientSecret] = useState("");
  const [itemId, setItemId] = useState("");
  const [submitting, setSubmitting] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const submit = async () => {
    if (password.length < 8) {
      setError("A senha deve ter ao menos 8 caracteres.");
      return;
    }
    if (password !== confirmPassword) {
      setError("As senhas não coincidem.");
      return;
    }
    if (clientId.trim() === "" || clientSecret.trim() === "" || itemId.trim() === "") {
      setError("Preencha as credenciais do Pluggy.");
      return;
    }
    setError(null);
    setSubmitting(true);
    try {
      const status = await setup({
        password,
        pluggy_client_id: clientId.trim(),
        pluggy_client_secret: clientSecret.trim(),
        pluggy_item_id: itemId.trim(),
      });
      onDone(status);
    } catch (err) {
      setError(errorMessage(err));
    } finally {
      setSubmitting(false);
    }
  };

  return (
    <Flex justify="center" align="center" style={{ minHeight: "100vh", padding: 24 }}>
      <Card title="Configurar o Contadinho" style={{ maxWidth: 440, width: "100%" }}>
        <Flex vertical gap="middle">
          <Typography.Paragraph type="secondary" style={{ marginBottom: 0 }}>
            Defina uma senha para proteger as credenciais do Pluggy em repouso e informe a conexão
            que será sincronizada. Essa senha será pedida novamente sempre que o servidor reiniciar.
          </Typography.Paragraph>
          {error && <Alert type="error" showIcon message={error} />}

          <div className="filter-field">
            <label htmlFor="setup-password">Senha</label>
            <Input.Password
              id="setup-password"
              value={password}
              onChange={(event) => setPassword(event.target.value)}
              autoComplete="new-password"
            />
          </div>
          <div className="filter-field">
            <label htmlFor="setup-password-confirm">Confirmar senha</label>
            <Input.Password
              id="setup-password-confirm"
              value={confirmPassword}
              onChange={(event) => setConfirmPassword(event.target.value)}
              autoComplete="new-password"
              onPressEnter={submit}
            />
          </div>
          <div className="filter-field">
            <label htmlFor="setup-client-id">Client ID (Pluggy)</label>
            <Input
              id="setup-client-id"
              value={clientId}
              onChange={(event) => setClientId(event.target.value)}
            />
          </div>
          <div className="filter-field">
            <label htmlFor="setup-client-secret">Client Secret (Pluggy)</label>
            <Input.Password
              id="setup-client-secret"
              value={clientSecret}
              onChange={(event) => setClientSecret(event.target.value)}
            />
          </div>
          <div className="filter-field">
            <label htmlFor="setup-item-id">Item ID (Pluggy)</label>
            <Input
              id="setup-item-id"
              value={itemId}
              onChange={(event) => setItemId(event.target.value)}
              onPressEnter={submit}
            />
          </div>

          <Button type="primary" block loading={submitting} onClick={submit}>
            Concluir configuração
          </Button>
        </Flex>
      </Card>
    </Flex>
  );
}
