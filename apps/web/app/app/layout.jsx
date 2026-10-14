"use client";

import Link from "next/link";
import { usePathname, useRouter } from "next/navigation";
import { createContext, useContext, useEffect, useState } from "react";
import { api, errorMessage } from "../../lib/api";
import { Brand, Icon, Loading } from "../../components/ui";

const WorkspaceContext = createContext(null);
const countLabels = {
  "/app/activity": "unread updates",
  "/app/enquiries": "new enquiries",
  "/app/quotes": "accepted quotes awaiting an invoice",
};
export function useWorkspace() {
  return useContext(WorkspaceContext);
}

const links = [
  ["/app", "Overview", "grid"],
  ["/app/activity", "Activity", "bell"],
  ["/app/storefront", "Storefront", "shop"],
  ["/app/products", "Products & services", "box"],
  ["/app/enquiries", "Enquiries", "chat"],
  ["/app/quotes", "Quotes", "file"],
  ["/app/customers", "Customers", "users"],
  ["/app/followups", "Follow-ups", "bell"],
  ["/app/invoices", "Invoices", "file"],
  ["/app/reminders", "Reminders", "bell"],
  ["/app/billing", "Billing", "card"],
  ["/app/settings", "Settings", "gear"],
];

export default function WorkspaceLayout({ children }) {
  const router = useRouter();
  const pathname = usePathname();
  const [workspace, setWorkspace] = useState(null);
  const [error, setError] = useState("");
  const [menu, setMenu] = useState(false);
  const [counts, setCounts] = useState(null);
  useEffect(() => {
    let active = true;
    Promise.all([api("/me"), api("/businesses/current")])
      .then(([me, business]) => {
        if (active)
          setWorkspace({
            me,
            business,
            siteOrigin: window.location.origin,
            setBusiness: (business) =>
              setWorkspace((current) => ({ ...current, business })),
          });
      })
      .catch((e) => {
        if (!active) return;
        if (e.status === 401)
          router.replace(`/login?next=${encodeURIComponent(pathname)}`);
        else setError(errorMessage(e));
      });
    return () => {
      active = false;
    };
  }, [router, pathname]);
  useEffect(() => {
    if (!workspace?.business?.id) return;
    let active = true;
    async function loadCounts() {
      try {
        const nextCounts = await api("/notifications/counts", {
          cache: "no-store",
        });
        if (active) setCounts(nextCounts);
      } catch {
        // Keep the last known counts until the next poll.
      }
    }
    loadCounts();
    const timer = window.setInterval(loadCounts, 60_000);
    window.addEventListener("focus", loadCounts);
    window.addEventListener("linkpay:notifications-changed", loadCounts);
    return () => {
      active = false;
      window.clearInterval(timer);
      window.removeEventListener("focus", loadCounts);
      window.removeEventListener("linkpay:notifications-changed", loadCounts);
    };
  }, [workspace?.business?.id, pathname]);
  async function logout() {
    try {
      await api("/auth/logout", { method: "POST" });
    } finally {
      router.replace("/");
      router.refresh();
    }
  }
  if (error)
    return (
      <main className="full-error">
        <h1>Workspace unavailable</h1>
        <p>{error}</p>
        <button className="button" onClick={() => window.location.reload()}>
          Try again
        </button>
      </main>
    );
  if (!workspace)
    return (
      <div className="center-loading">
        <Brand />
        <Loading />
      </div>
    );
  const storefrontPath = `/store/${workspace.business.slug}`;
  return (
    <WorkspaceContext.Provider value={workspace}>
      <div className="app-shell">
        <aside className={`sidebar ${menu ? "sidebar-open" : ""}`}>
          <div className="sidebar-top">
            <Brand />
            <button
              className="mobile-close"
              aria-label="Close navigation"
              onClick={() => setMenu(false)}
            >
              ×
            </button>
          </div>
          <div className="workspace-switch">
            <span className="workspace-avatar">
              {workspace.business.name.slice(0, 1).toUpperCase()}
            </span>
            <span>
              <strong>{workspace.business.name}</strong>
              <small>{workspace.business.plan} workspace</small>
            </span>
            <span className="switch-chevron">⌄</span>
          </div>
          <span className="nav-caption">WORKSPACE</span>
          <nav className="side-nav" aria-label="Workspace">
            {links.map(([href, label, icon]) => {
              const count = counts?.[href.slice(5)];
              return (
                <Link
                  key={href}
                  href={href}
                  onClick={() => setMenu(false)}
                  className={
                    pathname === href ||
                    (href !== "/app" && pathname.startsWith(`${href}/`))
                      ? "active"
                      : ""
                  }
                >
                  <Icon name={icon} size={19} />
                  {label}
                  {count > 0 && (
                    <span
                      className="side-nav-count"
                      aria-label={`${count} ${countLabels[href]}`}
                    >
                      {count}
                    </span>
                  )}
                </Link>
              );
            })}
            <Link
              href="/app/shops"
              onClick={() => setMenu(false)}
              className={
                pathname.startsWith("/app/shops") ||
                pathname.startsWith("/app/store/")
                  ? "active"
                  : ""
              }
            >
              <Icon name="shop" size={19} />
              Explore shops
            </Link>
          </nav>
          <div className="sidebar-bottom">
            <div className="sidebar-tip">
              <span className="tip-graphic">✦</span>
              <strong>Your link is your storefront</strong>
              <p>
                {workspace.business.published
                  ? "Share it wherever your customers are."
                  : "Publish your storefront to make this link available."}
              </p>
              {workspace.business.published ? (
                <>
                  <Link
                    className="sidebar-storefront-url"
                    href={storefrontPath}
                    target="_blank"
                  >
                    {workspace.siteOrigin}
                    {storefrontPath}
                  </Link>
                  <Link href={storefrontPath} target="_blank">
                    View storefront <Icon name="external" size={15} />
                  </Link>
                </>
              ) : (
                <>
                  <span className="sidebar-storefront-url">
                    Reserved: {storefrontPath}
                  </span>
                  <Link href="/app/storefront" onClick={() => setMenu(false)}>
                    Set up your shop →
                  </Link>
                </>
              )}
            </div>
            <button className="logout-button" onClick={logout}>
              Log out <span>↗</span>
            </button>
          </div>
        </aside>
        {menu && (
          <button
            className="nav-scrim"
            aria-label="Close navigation"
            onClick={() => setMenu(false)}
          />
        )}
        <div className="app-main">
          <header className="app-topbar">
            <button
              className="mobile-menu"
              aria-label="Open navigation"
              onClick={() => setMenu(true)}
            >
              ☰
            </button>
            <span className="topbar-crumb">
              Workspace <span>/</span>{" "}
              {links.find(([href]) => href === pathname)?.[1] ||
                (pathname.startsWith("/app/store/")
                  ? "Explore shops"
                  : "Details")}
            </span>
            <div className="topbar-right">
              <span className="status-dot" /> Storefront{" "}
              {workspace.business.published ? "live" : "draft"}
              <span className="topbar-avatar">
                {workspace.me.email.slice(0, 1).toUpperCase()}
              </span>
            </div>
          </header>
          <main className="app-content">{children}</main>
        </div>
      </div>
    </WorkspaceContext.Provider>
  );
}
