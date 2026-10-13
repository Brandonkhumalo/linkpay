import Link from "next/link";
import { notFound } from "next/navigation";
import { Brand, Icon } from "../../components/ui";
import { publicApi } from "../../lib/server-api";

export const metadata = {
  title: "Explore shops | LinkPay",
  description: "Browse businesses and shops on LinkPay.",
};

export default async function ShopsPage({ searchParams }) {
  const { page: rawPage } = await searchParams;
  const page = rawPage === undefined ? 1 : Number(rawPage);
  if (
    (rawPage !== undefined && typeof rawPage !== "string") ||
    !Number.isInteger(page) ||
    page < 1 ||
    page > 10000
  ) {
    notFound();
  }
  const directory = await publicApi(`/public/stores?page=${page}`);
  return (
    <>
      <header className="public-header">
        <Brand />
        <Link href="/register">List your shop ↗</Link>
      </header>
      <main className="public-store shop-directory">
        <section className="store-banner directory-banner">
          <span className="eyebrow">THE LINKPAY DIRECTORY</span>
          <h1>Explore local shops.</h1>
          <p>
            Find businesses, see what they offer, and get in touch directly.
          </p>
          <Link className="text-link" href="/">
            About LinkPay ↗
          </Link>
        </section>
        <div className="store-list-title">
          <div>
            <span className="eyebrow">DISCOVER</span>
            <h2>Available shops</h2>
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
                  <span className="eyebrow">LINKPAY SHOP</span>
                  <h3>{shop.name}</h3>
                  <p>
                    {shop.description || "Explore this business on LinkPay."}
                  </p>
                  <div className="store-item-actions">
                    <Link
                      className="button button-dark small"
                      href={`/store/${shop.slug}`}
                    >
                      Visit shop <Icon name="arrow" size={14} />
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
            <h3>{page === 1 ? "No shops yet" : "No shops on this page"}</h3>
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
              href={`/shops?page=${page - 1}`}
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
              href={`/shops?page=${page + 1}`}
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
        <span>Discover businesses on LinkPay.</span>
        <Link href="/">Home</Link>
      </footer>
    </>
  );
}
