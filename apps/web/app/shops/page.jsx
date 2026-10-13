import Link from "next/link";
import { notFound } from "next/navigation";
import { Brand, Icon } from "../../components/ui";
import { publicApi } from "../../lib/server-api";

export const metadata = {
  title: "Explore shops and creators | LinkPay",
  description: "Browse businesses and creators on LinkPay.",
};

export default async function ShopsPage({ searchParams }) {
  const { page: rawPage, type: rawType } = await searchParams;
  const type = rawType || "";
  const page = rawPage === undefined ? 1 : Number(rawPage);
  if (
    (rawPage !== undefined && typeof rawPage !== "string") ||
    !Number.isInteger(page) ||
    page < 1 ||
    page > 10000 || !["", "business", "creator"].includes(type)
  ) {
    notFound();
  }
  const directory = await publicApi(`/public/stores?page=${page}&type=${type}`);
  const pageHref = (nextPage) => `/shops?page=${nextPage}${type ? `&type=${type}` : ""}`;
  return (
    <>
      <header className="public-header">
        <Brand />
        <Link href="/register">Create your profile ↗</Link>
      </header>
      <main className="public-store shop-directory">
        <section className="store-banner directory-banner">
          <span className="eyebrow">THE LINKPAY DIRECTORY</span>
          <h1>Explore shops and creators.</h1>
          <p>
            Find businesses and creators, see what they offer, and get in touch directly.
          </p>
          <Link className="text-link" href="/">
            About LinkPay ↗
          </Link>
        </section>
        <nav className="store-links" aria-label="Profile types" style={{ marginTop: 24 }}>
          <Link href="/shops">All profiles</Link>
          <Link href="/shops?type=business">Shops</Link>
          <Link href="/shops?type=creator">Creators</Link>
        </nav>
        <div className="store-list-title">
          <div>
            <span className="eyebrow">DISCOVER</span>
            <h2>{type === "creator" ? "Available creators" : type === "business" ? "Available shops" : "All profiles"}</h2>
          </div>
          <span className="helper">Page {directory.page}</span>
        </div>
        {directory.items.length ? (
          <div className="store-grid">
            {directory.items.map((shop) => (
              <article className="store-item directory-card" key={shop.slug}>
                <div
                  className="directory-card-art"
                  style={{ backgroundColor: `${shop.theme_color}22` }}
                >
                  <span>{shop.name.slice(0, 1).toUpperCase()}</span>
                </div>
                <div className="store-item-body">
                  <span className="eyebrow">{shop.profile_type === "creator" ? "LINKPAY CREATOR" : "LINKPAY SHOP"}</span>
                  <h3>{shop.name}</h3>
                  {shop.profile_type === "creator" && (shop.creator_niche || shop.creator_location) && (
                    <p className="helper">{[shop.creator_niche, shop.creator_location].filter(Boolean).join(" · ")}</p>
                  )}
                  <p>
                    {shop.description || "Explore this business on LinkPay."}
                  </p>
                  <div className="store-item-actions">
                    <Link
                      className="button button-dark small"
                      href={`/store/${shop.slug}`}
                    >
                      {shop.profile_type === "creator" ? "View creator" : "Visit shop"} <Icon name="arrow" size={14} />
                    </Link>
                    {shop.whatsapp && (
                      <a
                        className="button button-light small"
                        href={`https://wa.me/${shop.whatsapp.replace(/\D/g, "")}`}
                        target="_blank"
                        rel="noopener noreferrer"
                        aria-label={`Chat with ${shop.name} on WhatsApp`}
                      >
                        <Icon name="chat" size={16} /> WhatsApp
                      </a>
                    )}
                  </div>
                </div>
              </article>
            ))}
          </div>
        ) : (
          <div className="empty">
            <h3>{page === 1 ? "No profiles yet" : "No profiles on this page"}</h3>
            <p>Check back soon or create your own storefront.</p>
            <Link className="button button-dark" href="/register">
              Get started
            </Link>
          </div>
        )}
        <nav className="pagination" aria-label="Shop pages">
          {directory.has_prev ? (
            <Link
              className="button button-light"
              href={pageHref(page - 1)}
            >
              ← Previous
            </Link>
          ) : (
            <span />
          )}
          <span>Page {directory.page}</span>
          {directory.has_next ? (
            <Link
              className="button button-light"
              href={pageHref(page + 1)}
            >
              Next →
            </Link>
          ) : (
            <span />
          )}
        </nav>
      </main>
      <footer className="site-footer">
        <Brand />
        <span>Discover businesses and creators on LinkPay.</span>
        <Link href="/">Home</Link>
      </footer>
    </>
  );
}
