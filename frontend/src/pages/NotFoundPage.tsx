import { Result } from "antd";
import { Link } from "react-router-dom";

export function NotFoundPage() {
  return (
    <Result
      status="404"
      title={<h1>Página não encontrada</h1>}
      subTitle="O endereço acessado não corresponde a uma página do Contadinho."
      extra={<Link to="/">Voltar para Home</Link>}
    />
  );
}
