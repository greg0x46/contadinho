import { Alert, Button, Card, Flex, Input } from "antd";
import { useState } from "react";
import { login } from "../api/auth";

export function LoginPage({ onDone }: { onDone: () => void }) {
  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const submit = async () => {
    setBusy(true); setError(null);
    try { await login(email, password); setPassword(""); onDone(); }
    catch (err) { setError(err instanceof Error ? err.message : "Não foi possível entrar."); }
    finally { setBusy(false); }
  };
  return <Flex justify="center" align="center" style={{ minHeight: "100vh", padding: 24 }}>
    <Card title="Entrar no Contadinho" style={{ maxWidth: 380, width: "100%" }}>
      <form onSubmit={(event) => { event.preventDefault(); void submit(); }}>
        <Flex vertical gap="middle">
          {error && <Alert type="error" showIcon message={error} />}
          <label htmlFor="login-email">E-mail</label>
          <Input id="login-email" type="email" autoComplete="username" required value={email} onChange={(e) => setEmail(e.target.value)} />
          <label htmlFor="login-password">Senha</label>
          <Input.Password id="login-password" autoComplete="current-password" required value={password} onChange={(e) => setPassword(e.target.value)} />
          <Button type="primary" htmlType="submit" loading={busy}>Entrar</Button>
        </Flex>
      </form>
    </Card>
  </Flex>;
}
