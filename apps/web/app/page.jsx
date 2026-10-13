import Link from "next/link";
import { Brand, Icon } from "../components/ui";

export default function Home() {
  return (
    <>
      <header className="site-nav">
        <Brand />
        <nav>
          <Link href="/shops">Explore shops & creators</Link>
          <a href="#features">Why LinkPay</a>
          <a href="#how-it-works">How it works</a>
          <a href="#pricing">Pricing</a>
        </nav>
        <div className="site-nav-actions">
          <Link href="/login">Log in</Link>
          <Link className="button button-dark" href="/register">
            Get started <Icon name="arrow" size={17} />
          </Link>
        </div>
      </header>
      <main>
        <section className="hero">
          <div className="hero-copy">
            <div className="pill">
              <span className="pill-dot" /> Made for modern businesses in
              Zimbabwe
            </div>
            <h1>
              One link.
              <br />
              <em>Every opportunity.</em>
            </h1>
            <p>
              Showcase products, services, or creator work. Turn enquiries and
              brand briefs into invoices, and stay on top of every payment.
            </p>
            <div className="hero-cta">
              <Link className="button button-primary" href="/register">
                Start your free storefront <Icon name="arrow" size={18} />
              </Link>
              <a className="text-link" href="#how-it-works">
                See how it works <span>↘</span>
              </a>
            </div>
            <div className="hero-proof">
              <div className="avatar-stack">
                <span>TM</span>
                <span>AK</span>
                <span>SN</span>
              </div>
              <span>Built for the way you do business</span>
            </div>
          </div>
          <div className="hero-visual" aria-label="Example LinkPay dashboard">
            <div className="float-card float-paid">
              <span className="float-icon">✓</span>
              <div>
                <strong>Payment received</strong>
                <small>Invoice #000024 · USD 120.00</small>
              </div>
            </div>
            <div className="demo-window">
              <div className="demo-bar">
                <span className="demo-logo">↗</span>
                <strong>
                  Good morning, Tendai <span>✳</span>
                </strong>
                <small>Wednesday, 12 March</small>
              </div>
              <div className="demo-content">
                <div className="demo-label">TOTAL REVENUE</div>
                <div className="demo-amount">
                  $12,840<span>.00</span>
                </div>
                <div className="demo-trend">
                  ↗ 18.4% <span>vs. last month</span>
                </div>
                <div className="demo-chart">
                  <span style={{ height: "40%" }} />
                  <span style={{ height: "54%" }} />
                  <span style={{ height: "44%" }} />
                  <span style={{ height: "65%" }} />
                  <span style={{ height: "58%" }} />
                  <span style={{ height: "81%" }} />
                  <span style={{ height: "72%" }} />
                  <span style={{ height: "93%" }} />
                  <span style={{ height: "77%" }} />
                  <span style={{ height: "100%" }} />
                  <span style={{ height: "85%" }} />
                  <span style={{ height: "96%" }} />
                </div>
                <div className="demo-summary">
                  <div>
                    <small>Outstanding</small>
                    <strong>$2,480.00</strong>
                  </div>
                  <div>
                    <small>Paid invoices</small>
                    <strong>32</strong>
                  </div>
                </div>
              </div>
            </div>
            <div className="float-card float-enquiry">
              <span className="float-enquiry-icon">✦</span>
              <div>
                <strong>New enquiry</strong>
                <small>Your storefront is working for you</small>
              </div>
            </div>
          </div>
        </section>
        <section className="logo-strip">
          <span>FROM FIRST HELLO TO FINAL PAYMENT</span>
          <div>
            <span>Discover</span>
            <b>→</b>
            <span>Connect</span>
            <b>→</b>
            <span>Invoice</span>
            <b>→</b>
            <span>Get paid</span>
          </div>
        </section>
        <section className="marketing-section features-section" id="features">
          <div className="section-intro">
            <span className="eyebrow">A better way to do business</span>
            <h2>
              Everything flows
              <br />
              <em>from one link.</em>
            </h2>
            <p>
              Spend less time juggling conversations and spreadsheets. Give
              every customer a smoother experience from discovery to payment.
            </p>
          </div>
          <div className="feature-cards">
            <article className="feature-card feature-green">
              <div className="feature-number">01 / SHOWCASE</div>
              <div className="feature-illustration feature-shop">
                <span>yourbrand.linkpay</span>
                <div>
                  <span>✦</span>
                  <span>◉</span>
                  <span>◈</span>
                </div>
              </div>
              <h3>A storefront that feels like you</h3>
              <p>
                Give your products and services a beautiful home you can share
                anywhere.
              </p>
            </article>
            <article className="feature-card feature-cream">
              <div className="feature-number">02 / CONNECT</div>
              <div className="feature-illustration feature-chat">
                <div>Hi! I&apos;m interested in your service 👋</div>
                <div>Let&apos;s make it happen ↗</div>
              </div>
              <h3>Turn interest into action</h3>
              <p>
                Collect enquiries in one place and keep customer details close
                at hand.
              </p>
            </article>
            <article className="feature-card feature-lilac">
              <div className="feature-number">03 / GET PAID</div>
              <div className="feature-illustration feature-invoice">
                <span>INVOICE #000024</span>
                <strong>USD 120.00</strong>
                <small>● Payment recorded</small>
              </div>
              <h3>Invoices made simple</h3>
              <p>
                Create, send, and track invoices, then follow up when a balance
                is due.
              </p>
            </article>
          </div>
        </section>
        <section className="how-section" id="how-it-works">
          <div>
            <span className="eyebrow">Simple by design</span>
            <h2>
              Go from idea to
              <br />
              <em>in business.</em>
            </h2>
            <p>
              Set up your business presence and start sharing it in minutes.
            </p>
            <Link className="button button-primary" href="/register">
              Create your account <Icon name="arrow" size={18} />
            </Link>
          </div>
          <ol>
            <li>
              <span>01</span>
              <div>
                <h3>Create your storefront</h3>
                <p>Add your business story, listings, and contact links.</p>
              </div>
            </li>
            <li>
              <span>02</span>
              <div>
                <h3>Share your link</h3>
                <p>Let customers browse and send enquiries from anywhere.</p>
              </div>
            </li>
            <li>
              <span>03</span>
              <div>
                <h3>Send an invoice</h3>
                <p>Turn a conversation into a clear, shareable invoice.</p>
              </div>
            </li>
            <li>
              <span>04</span>
              <div>
                <h3>Keep moving</h3>
                <p>Record payments and stay in control of what is due.</p>
              </div>
            </li>
          </ol>
        </section>
        <section className="marketing-section pricing-section" id="pricing">
          <div className="section-intro centered">
            <span className="eyebrow">Straightforward pricing</span>
            <h2>
              Start small. <em>Grow freely.</em>
            </h2>
            <p>
              Start on Free, or try Starter or Business for exactly 168 hours.
              Continue on Free afterward unless you choose a paid plan.
            </p>
          </div>
          <div className="pricing-cards">
            <article>
              <span className="eyebrow">FREE</span>
              <h3>For getting started</h3>
              <div className="plan-price">
                $0 <small>/ month</small>
              </div>
              <p>For the first steps of your business journey.</p>
              <ul>
                <li>5 products or services</li>
                <li>30 invoices per month</li>
                <li>Your own shareable storefront</li>
                <li>25 MB image storage</li>
              </ul>
              <Link className="button button-outline" href="/register">
                Create free account <Icon name="arrow" size={16} />
              </Link>
            </article>
            <article className="featured-plan">
              <span className="plan-popular">MOST POPULAR</span>
              <span className="eyebrow">STARTER</span>
              <h3>For finding your rhythm</h3>
              <div className="plan-price">
                $3 <small>/ month</small>
              </div>
              <p>More space and follow-up tools as you grow.</p>
              <ul>
                <li>30 products or services</li>
                <li>60 invoices per month</li>
                <li>100 reminder emails per month</li>
                <li>150 MB image storage</li>
              </ul>
              <Link
                className="button button-primary"
                href="/register?trial=starter"
              >
                Try Starter for 7 days <Icon name="arrow" size={16} />
              </Link>
            </article>
            <article>
              <span className="eyebrow">BUSINESS</span>
              <h3>For your next chapter</h3>
              <div className="plan-price">
                $5 <small>/ month</small>
              </div>
              <p>The capacity for a busier operation.</p>
              <ul>
                <li>500 products or services</li>
                <li>500 invoices per month</li>
                <li>500 reminder emails per month</li>
                <li>500 MB image storage</li>
              </ul>
              <Link
                className="button button-outline"
                href="/register?trial=business"
              >
                Try Business for 7 days <Icon name="arrow" size={16} />
              </Link>
            </article>
          </div>
          <p className="pricing-note">
            Trials start at signup, last exactly 168 hours, and require no
            payment details. They end on the Free plan unless you buy a plan.
            Paid plans use Paynow Zimbabwe checkout and renew manually. Prices
            shown in USD; annual prices are $36 Starter and $60 Business. Extra
            invoices cost $1 for 30 or $2 for 100. They carry over until used,
            after each month&apos;s plan invoices.
          </p>
        </section>
        <section className="final-cta">
          <div>
            <span className="eyebrow">READY WHEN YOU ARE</span>
            <h2>
              Make room for
              <br />
              <em>what&apos;s next.</em>
            </h2>
            <p>Your next customer could be one link away.</p>
            <Link className="button button-primary" href="/register">
              Start free today <Icon name="arrow" size={18} />
            </Link>
          </div>
          <div className="cta-orbit">
            <span>↗</span>
          </div>
        </section>
      </main>
      <footer className="site-footer">
        <Brand />
        <span>Made for the businesses that keep moving.</span>
        <div>
          <Link href="/shops">Explore shops</Link>
          <Link href="/login">Log in</Link>
          <Link href="/register">Get started</Link>
        </div>
        <small>© {new Date().getFullYear()} LinkPay</small>
      </footer>
    </>
  );
}
