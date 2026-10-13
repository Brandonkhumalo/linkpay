import Link from "next/link";
import Image from "next/image";
import { Brand, Icon } from "../../../components/ui";
import PublicEnquiry from "../../../components/public-enquiry";
import { money } from "../../../lib/api";
import { publicApi } from "../../../lib/server-api";

export async function generateMetadata({ params }) {
  const { slug } = await params;
  const { business } = await publicApi(
    `/public/stores/${encodeURIComponent(slug)}`,
  );
  return {
    title: business.name,
    description: business.description || `Explore ${business.name} on LinkPay.`,
  };
}
export default async function StorePage({ params }) {
  const { slug } = await params;
  const { business, products } = await publicApi(
    `/public/stores/${encodeURIComponent(slug)}`,
  );
  const socials = [
    ["Instagram", business.instagram_url],
    ["Facebook", business.facebook_url],
    ["TikTok", business.tiktok_url],
    ["Website", business.website_url],
  ].filter(([, url]) => url);
  return (
    <>
      <header className="public-header">
        <Brand />
        <Link href="/shops">Explore shops ↗</Link>
      </header>
      <main className="public-store">
        <section
          className="store-banner"
          style={{ backgroundColor: `${business.theme_color}22` }}
        >
          <span className="eyebrow">WELCOME TO OUR STOREFRONT</span>
          <h1>{business.name}</h1>
          <p>
            {business.description || "Explore what we offer and get in touch."}
          </p>
          <div className="store-links">
            {business.whatsapp && (
              <a
                className="button button-dark small"
                href={`https://wa.me/${business.whatsapp.replace(/\D/g, "")}`}
                target="_blank"
                rel="noopener noreferrer"
              >
                <Icon name="chat" size={16} /> Chat on WhatsApp
              </a>
            )}
            {socials.map(([label, url]) => (
              <a
                key={label}
                href={url}
                target="_blank"
                rel="noopener noreferrer"
              >
                {label} ↗
              </a>
            ))}
          </div>
        </section>
        <div className="store-list-title">
          <div>
            <span className="eyebrow">EXPLORE</span>
            <h2>What we offer</h2>
          </div>
          <span className="helper">
            {products.length} listing{products.length === 1 ? "" : "s"}
          </span>
        </div>
        {products.length ? (
          <div className="store-grid">
            {products.map((p) => (
              <article className="store-item" key={p.id}>
                {p.has_image ? (
                  <Image
                    unoptimized
                    width={800}
                    height={475}
                    className="store-item-image"
                    src={`/api/v1/public/products/${p.id}/image`}
                    alt={p.name}
                  />
                ) : (
                  <div className="store-item-placeholder">✦</div>
                )}
                <div className="store-item-body">
                  <span className="eyebrow">{p.kind}</span>
                  <h3>{p.name}</h3>
                  <p>{p.description}</p>
                  <div className="store-item-price">
                    {money(p.price_minor, business.currency)}
                  </div>
                  <div className="store-item-actions">
                    <PublicEnquiry slug={slug} product={p} />
                    {business.whatsapp && (
                      <a
                        className="button button-light small"
                        href={`https://wa.me/${business.whatsapp.replace(/\D/g, "")}?text=${encodeURIComponent(`Hello ${business.name}, I'm interested in ${p.name}.`)}`}
                        target="_blank"
                        rel="noopener noreferrer"
                      >
                        WhatsApp ↗
                      </a>
                    )}
                  </div>
                </div>
              </article>
            ))}
          </div>
        ) : (
          <div className="empty">
            <h3>Coming soon</h3>
            <p>New listings will appear here shortly.</p>
          </div>
        )}
      </main>
      <footer className="site-footer">
        <Brand />
        <span>{business.name} on LinkPay</span>
      </footer>
    </>
  );
}
