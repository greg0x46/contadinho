import {
  ApiOutlined,
  BankOutlined,
  BarChartOutlined,
  BulbOutlined,
  FundOutlined,
  HomeOutlined,
  RedoOutlined,
  RiseOutlined,
  SettingOutlined,
  TagsOutlined,
  ThunderboltOutlined,
  TransactionOutlined,
  WalletOutlined,
} from "@ant-design/icons";
import ProLayout from "@ant-design/pro-layout";
import { Link, Outlet, useLocation } from "react-router-dom";

import { SkipLink } from "../components/SkipLink";

export function App() {
  const location = useLocation();

  return (
    <>
      <SkipLink />
      <ProLayout
        className="app-layout"
        title="Contadinho"
        logo={false}
        location={location}
        route={{
          routes: [
            { path: "/", name: "Home", icon: <HomeOutlined /> },
            { path: "/relatorio-financeiro", name: "Relatório financeiro", icon: <BarChartOutlined /> },
            { path: "/patrimonio-liquido", name: "Patrimônio líquido", icon: <FundOutlined /> },
            { path: "/open-banking", name: "Open Banking", icon: <ApiOutlined /> },
            { path: "/contas-e-cartoes", name: "Contas e cartões", icon: <BankOutlined /> },
            { path: "/transacoes", name: "Transações", icon: <TransactionOutlined /> },
            { path: "/automacoes", name: "Automações", icon: <ThunderboltOutlined /> },
            { path: "/pendencias", name: "Pendências", icon: <WalletOutlined /> },
            { path: "/recorrencias", name: "Recorrências", icon: <RedoOutlined /> },
            { path: "/cenarios", name: "Cenários", icon: <BulbOutlined /> },
            { path: "/investimentos", name: "Investimentos", icon: <RiseOutlined /> },
            { path: "/categorias", name: "Categorias", icon: <TagsOutlined /> },
            { path: "/configuracoes", name: "Configurações", icon: <SettingOutlined /> },
          ],
        }}
        menuItemRender={(item, defaultDom) =>
          item.path === undefined ? defaultDom : <Link to={item.path}>{defaultDom}</Link>
        }
        layout="mix"
        splitMenus={false}
        fixedHeader
        fixSiderbar
      >
        <div id="conteudo-principal" tabIndex={-1}>
          <Outlet />
        </div>
      </ProLayout>
    </>
  );
}
