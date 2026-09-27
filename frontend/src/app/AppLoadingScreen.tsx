import juliusLoading from "../assets/julius-loading.gif";

export function AppLoadingScreen() {
  return (
    <div className="app-loading-screen" role="status">
      <img src={juliusLoading} alt="" className="app-loading-screen-gif" />
      <p className="app-loading-screen-caption">Contando cada centavo..</p>
    </div>
  );
}
