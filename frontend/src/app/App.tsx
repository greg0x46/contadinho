import {
  ApiOutlined,
  CreditCardOutlined,
  DollarOutlined,
  HomeOutlined,
  RiseOutlined,
  SettingOutlined,
  TagsOutlined,
  ThunderboltOutlined,
  TransactionOutlined,
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
            { path: "/open-banking", name: "Open Banking", icon: <ApiOutlined /> },
            { path: "/transacoes", name: "Transações", icon: <TransactionOutlined /> },
            { path: "/automacoes", name: "Automações", icon: <ThunderboltOutlined /> },
            { path: "/dividas", name: "Dívidas", icon: <CreditCardOutlined /> },
            { path: "/contas-a-receber", name: "Contas a receber", icon: <DollarOutlined /> },
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
