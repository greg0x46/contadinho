import {
  BankOutlined,
  FundOutlined,
  HomeOutlined,
  RedoOutlined,
  RiseOutlined,
  SettingOutlined,
  TransactionOutlined,
  WalletOutlined,
} from "@ant-design/icons";
import ProLayout from "@ant-design/pro-layout";
import { Link, Outlet, useLocation } from "react-router-dom";

import julius from "../assets/julius.png";
import { SkipLink } from "../components/SkipLink";
import { colors } from "../theme/tokens";

const headerBg = "#000000";
const subtitle = "De graça !?";

export function App() {
  const location = useLocation();

  return (
    <>
      <SkipLink />
      <ProLayout
        className="app-layout"
        title="Julius"
        logo={julius}
        token={{
          header: {
            heightLayoutHeader: 64,
            colorBgHeader: headerBg,
            colorBgScrollHeader: headerBg,
            colorHeaderTitle: colors.bgContainer,
          },
          sider: {
            // Defaults to a neutral gray/black highlight regardless of
            // colorPrimary, so the active nav item has to be told explicitly
            // to pick up the theme's accent color.
            colorTextMenuSelected: colors.primary,
            colorBgMenuItemSelected: `${colors.primary}1f`,
            // Defaults to fully transparent, and pro-layout paints this
            // straight onto the fixed sider's own DOM node via its
            // CSS-in-JS, which wins the cascade over the opaque background
            // global.css sets on .ant-pro-sider — scrolling page content
            // then shows through the "solid" sider. Same class of bug the
            // header comment below already works around; fix it the same
            // way, via the token instead of CSS.
            colorMenuBackground: colors.bgContainer,
          },
        }}
        location={location}
        menuProps={{ selectedKeys: location.pathname.startsWith("/configuracoes") ? ["/configuracoes"] : undefined }}
        route={{
          routes: [
            { path: "/", name: "Home", icon: <HomeOutlined /> },
            { path: "/transacoes", name: "Transações", icon: <TransactionOutlined /> },
            { path: "/patrimonio-liquido", name: "Patrimônio líquido", icon: <FundOutlined /> },
            { path: "/contas-e-cartoes", name: "Contas e cartões", icon: <BankOutlined /> },
            { path: "/pendencias", name: "Pendências", icon: <WalletOutlined /> },
            { path: "/recorrencias", name: "Recorrências", icon: <RedoOutlined /> },
            { path: "/investimentos", name: "Investimentos", icon: <RiseOutlined /> },
            { path: "/configuracoes", name: "Configurações", icon: <SettingOutlined /> },
          ],
        }}
        menuItemRender={(item, defaultDom) =>
          item.path === undefined ? defaultDom : <Link to={item.path}>{defaultDom}</Link>
        }
        // The desktop global header doesn't go through menuHeaderRender at
        // all — pro-layout renders it via this separate prop instead — so
        // the title+subtitle stack needs its own override here.
        headerTitleRender={(logoDom, titleDom) => (
          <a>
            {logoDom}
            <span className="app-brand-text">
              {titleDom}
              <span className="app-brand-subtitle">{subtitle}</span>
            </span>
          </a>
        )}
        menuHeaderRender={(logoDom, _title, siderProps) =>
          // Desktop keeps the brand only in the global header (mix layout's
          // default, rendered above via headerTitleRender). On mobile that
          // header collapses to just the menu button, so this renders the
          // logo + name both there and atop the nav drawer instead of
          // leaving the product with no identity.
          siderProps && !siderProps.isMobile ? (
            false
          ) : (
            <span className="app-mobile-brand">
              {logoDom}
              <span className="app-brand-text">
                <span className="app-brand-title">Julius</span>
                <span className="app-brand-subtitle">{subtitle}</span>
              </span>
            </span>
          )
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
