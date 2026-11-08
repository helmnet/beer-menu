import AdminPage from "./pages/AdminPage";
import DisplayPage from "./pages/DisplayPage";

/**
 * App picks the page from the current URL: `/display` and anything below `/display/`
 * render DisplayPage, every other path renders AdminPage.
 */
export default function App() {
  const path = window.location.pathname;
  const isDisplay = path === "/display" || path.startsWith("/display/");
  return isDisplay ? <DisplayPage /> : <AdminPage />;
}
