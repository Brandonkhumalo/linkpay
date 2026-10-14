import Link from "next/link";
import { Brand } from "../../../components/ui";

export default function StoreNotFound() {
  return (
    <div className="auth-plain">
      <header className="public-header">
        <Brand />
      </header>
      <section className="auth-card">
        <span className="eyebrow">STOREFRONT UNAVAILABLE</span>
        <h1>This storefront isn&apos;t available yet.</h1>
        <p>
          The link may be incorrect, or the storefront may not be published yet.
          If this is your storefront, sign in to finish setup and publish it.
        </p>
        <Link className="button button-dark" href="/app/storefront">
          Set up your storefront →
        </Link>
        <Link className="button button-light" href="/shops">
          Explore shops →
        </Link>
      </section>
    </div>
  );
}
