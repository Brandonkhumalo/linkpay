import Link from "next/link";
import { Brand } from "../components/ui";
export default function NotFound() {
  return (
    <div className="auth-plain">
      <header className="public-header">
        <Brand />
      </header>
      <section className="auth-card">
        <span className="eyebrow">PAGE NOT FOUND</span>
        <h1>We couldn&apos;t find that page.</h1>
        <p>The link may have changed or is no longer available.</p>
        <Link className="button button-dark" href="/">
          Go home →
        </Link>
      </section>
    </div>
  );
}
