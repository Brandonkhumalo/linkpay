"use client";

import Link from "next/link";
import { usePathname, useRouter } from "next/navigation";
import { createContext, useContext, useEffect, useState } from "react";
import { api, errorMessage } from "../../lib/api";
import { Brand, Icon, Loading } from "../../components/ui";

const WorkspaceContext = createContext(null);
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
  useEffect(() => {
    let active = true;
    Promise.all([api("/me"), api("/businesses/current")])
      .then(([me, business]) => {
        if (active)
          setWorkspace({
            me,
            business,
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
            {links.map(([href, label, icon]) => (
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
              </Link>
            ))}
            <Link href="/shops" onClick={() => setMenu(false)}>
              <Icon name="shop" size={19} />
              Explore shops
            </Link>
          </nav>
          <div className="sidebar-bottom">
            <div className="sidebar-tip">
              <span className="tip-graphic">✦</span>
              <strong>Your link is your storefront</strong>
              <p>Share it wherever your customers are.</p>
              {workspace.business.published ? (
                <Link
                  href={`/store/${workspace.business.slug}`}
                  target="_blank"
                >
                  View storefront <Icon name="external" size={15} />
                </Link>
              ) : (
                <Link href="/app/storefront">Set up your shop →</Link>
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
              {links.find(([href]) => href === pathname)?.[1] || "Details"}
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
