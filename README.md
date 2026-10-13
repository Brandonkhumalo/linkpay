# LinkPay

LinkPay is a multi-tenant storefront and invoicing application built with a Go API, Next.js App Router frontend, and PostgreSQL. A separate Go worker schedules reminders and reconciles old billing attempts.

## Included in this build

- Registration, password login and reset by email, server-side sessions, business profiles, and optional storefront publishing. Registration reserves a storefront slug; invoice-only accounts can leave it unpublished.
- Product and service listings with plan limits; WhatsApp links to the shop's international-format number, structured customer enquiries, and a public directory of published shops.
- Customer directory, draft invoices, numbered and branded PDF invoices, private issue links, and revocation when voiding an unpaid invoice.
- Atomic manual payment recording with idempotency keys, partial balances, derived statuses, and audit records.
- Email reminder scheduling for paid-plan tenants with a PostgreSQL job table, retry limits, and delivery history.
- Overview totals, monthly usage, and aggregate admin monitoring protected by an additional TOTP code.
- Routed, responsive Next.js landing, authentication, public storefront and invoice pages, and a dedicated workspace with separate pages for storefront, products, enquiries, customers, invoices, reminders, billing, and settings.
- Server-backed pagination with Previous and Next on public shops and private products, customers, enquiries, invoices, reminder deliveries, and billing payments. Each list returns `{items, page, has_next, has_prev}` with 20 records per page.
- Paynow Zimbabwe hosted checkout for prepaid Starter and Business plans (monthly or annual) and non-expiring invoice top-ups, signed callbacks, server-side status verification, payment history, and expiring plan entitlements.
- HTTPS social links on public storefronts.
- Optional creator profiles with niche, location, social and portfolio links, discoverable in the public directory. Creators can publish with a contact email or WhatsApp number.
- Service packages can use a fixed price or "Request a quote". Brand partners can send structured briefs with deliverables, optional budget, target date, and usage rights; creators review them in Enquiries and can start an invoice after agreeing terms.
- JPEG/PNG product images, re-encoded to remove metadata, with tenant-scoped upload and viewing rules. The Compose deployment persists images in `linkpay_media`; storage allowances are 25 MB Free, 150 MB Starter, and 500 MB Business.

Public storefront and invoice pages are server-rendered by Next.js. The browser uses same-origin API requests for authenticated workspace actions. Private list responses are paginated by the API, so older records remain reachable.

## Local startup

1. Copy `.env.example` to `.env`.
2. Set a strong `POSTGRES_PASSWORD` and a random `PUBLIC_TOKEN_SECRET` of at least 32 characters. Set `RAILWAY_DB_URL` to a PostgreSQL connection URL reachable from the app containers. Docker Compose maps it to the `DATABASE_URL` used by the Go services. For the bundled local PostgreSQL service, use a URL-encoded value such as `postgres://linkpay:<encoded-password>@db:5432/linkpay?sslmode=disable`. One way to generate secrets locally is `openssl rand -hex 32`.
3. Run `docker compose up --build -d`. Open `http://localhost:8080`. This starts PostgreSQL, applies every unapplied migration, then starts the API, worker, and Next.js web service. Run `docker compose ps` and `docker compose logs -f web api worker` to inspect health.

The migration service runs before the API and worker. It records applied versions in `schema_migrations`. Never point local Compose at a production database. For local Go development, use Go 1.25 or newer, run `go run ./cmd/migrate` and `go run ./cmd/server` from `apps/api` with the required environment variables. In another terminal, run `npm ci && npm run dev` from `apps/web`; Next.js proxies `/api/v1/*` to `http://localhost:8080` by default. The Docker build and CI use Go 1.27.2 and Node 22.

## Paynow billing

New accounts can start directly on Free without a trial, or choose a Starter or Business trial at signup. A paid plan trial starts at the registration transaction's PostgreSQL timestamp and ends exactly **168 hours** later. No payment method is required. If no paid purchase is verified, trial access automatically falls back to Free; existing accounts are not granted a trial retroactively. Paid plans renew manually and are never charged automatically.

| Package | Price | Listings | Invoices per month | Reminder emails per month | Image storage |
| --- | --- | ---: | ---: | ---: | ---: |
| Free | USD 0 | 5 | 30 | None | 25 MB |
| Starter | USD 3/month or USD 36/year | 30 | 60 | 100 | 150 MB |
| Business | USD 5/month or USD 60/year | 500 | 500 | 500 | 500 MB |

All packages include the storefront, customer and enquiry records, invoices, and manual payment recording. Automated reminder delivery requires configured SMTP. A verified paid plan purchase during a trial changes the active package immediately and extends the paid expiry from the later of payment verification or the current package expiry. If the paid plan purchase is refunded while the original trial is still within its 168-hour window, the remaining trial is restored.

Owners can also buy **30 extra invoices for USD 1** or **100 for USD 2** through Paynow. Verified top-ups remain on the business account until used and do not expire or reset monthly. Invoices use the current month's package allowance first, then purchased invoices. Refunded or disputed top-ups are removed from available credit; if their invoices were already used, future top-ups cover that deficit first. The billing page shows monthly package usage and the separate purchased balance.

Create an **Advanced Integration** in Paynow, then set `PAYNOW_INTEGRATION_ID` and `PAYNOW_INTEGRATION_KEY` in `.env`. Set `PUBLIC_URL` to the public HTTPS origin before testing checkout; Paynow must reach `/api/v1/billing/paynow/result`. Only business owners can start checkout. LinkPay validates Paynow's SHA-512 response hash, polls Paynow on important updates, verifies the reference and amount, and grants time-limited access in a database transaction. A browser return does not grant access. Lost callbacks can be recovered from the payment history or by the worker's delayed reconciliation.

Paid plans are **prepaid and manually renewed**. The integration does not tokenize payment instruments or attempt automatic recurring charges. Prices are USD 3/5 monthly and USD 36/60 annually; confirm the merchant integration's supported settlement currency and fees with Paynow before launch. Customer invoice payments remain merchant-managed and are recorded manually, as specified for the initial customer journey.

Creator collaboration is a direct enquiry and invoicing workflow. LinkPay does not verify audience metrics, broker agreements, hold funds in escrow, or automatically collect brand payments. Portfolio links are supplied by creators; brands should confirm deliverables and rights before paying.

Paynow starts new integrations in test mode. Complete Paynow's fake success and cancellation flows and request live activation using your merchant account. Test with a publicly reachable HTTPS domain or tunnel; checkout is deliberately unavailable when `PUBLIC_URL` is HTTP. See [Paynow test mode](https://developers.paynow.co.zw/docs/paynow/test_mode/) and [integration setup](https://developers.paynow.co.zw/docs/paynow/integration_generation/).

## Production Docker Compose

Set `DOMAIN` to the hostname with DNS pointing at the server and `PUBLIC_URL=https://<domain>`. Open ports 80 and 443, then run `docker compose --profile production up --build -d`. Caddy obtains and renews HTTPS certificates, proxies `/api/*` to the API and other requests to Next.js, and persists certificate data in Docker volumes. The Next.js direct port is bound to localhost; the API is internal only. Keep `.env` private, and use distinct strong secrets for PostgreSQL, public links, admin TOTP, and Paynow.

PostgreSQL and media use named volumes. Establish **off-host backups and tested restores of both volumes** before storing customer data. Docker Compose uses separate containers for PostgreSQL, the migration, API, worker, web, and Caddy services.

## Railway deployment

The repository-root `Dockerfile` builds one image containing Next.js, the Go API, and the worker. The container runs migrations before starting all three processes. Next.js serves the public `PORT` (default `8080`) and forwards `/api/v1/*` to the API on `127.0.0.1:8081`. `/healthz` checks both the web app and the API/database.

1. Keep one Railway application service connected to this repository's `main` branch with root directory `/` and the root `Dockerfile`. Clear any custom Dockerfile path, Start Command, or Pre-deploy Command left from an earlier setup. Attach the public domain to this service and set its healthcheck path to `/healthz`.
2. Add Railway PostgreSQL to the same project/environment. On the application service, set `DATABASE_URL` to a reference to its URL (for example, `${{Postgres.DATABASE_URL}}`, replacing `Postgres` with the actual database service name). Set `PUBLIC_TOKEN_SECRET` to a stable random value of at least 32 characters, `PUBLIC_URL` to the application's public HTTPS origin, and `PORT=8080` unless Railway assigns a different public port. The application will not start if the migration or database connection fails.
3. Attach a persistent volume at `/app/media` before allowing product image uploads. Keep **one application replica**: media files are local to the volume, and each replica would run the worker. Back up both PostgreSQL and uploaded media off-host.

The `.env` file is local and ignored by Git; set production variables in Railway. `RAILWAY_DB_URL` is a local Docker Compose convenience; the deployed Go binaries read `DATABASE_URL`. `DOMAIN` is only used by Caddy in the Compose deployment. The worker runs billing reconciliation even without SMTP; email reminders run only when `SMTP_HOST`, `SMTP_USER`, `SMTP_PASSWORD`, and `SMTP_FROM` are all configured. Set Paynow and admin variables as described below.

Run `deployments/backup.sh /path/to/backup-staging` to create a PostgreSQL custom-format dump and a media archive with owner-only permissions. Copy both files to encrypted off-host storage and rehearse restoration on a separate stack. The script checks both archives before naming them as completed backups; it does not schedule or transfer them.

The API and worker need `SMTP_HOST`, `SMTP_PORT`, `SMTP_USER`, `SMTP_PASSWORD`, and `SMTP_FROM` for password recovery and reminder delivery. They require TLS with verified certificates. Starter allows up to 100 reminder emails per month and Business up to 500. When SMTP is absent, automated reminders are disabled while billing reconciliation continues; password recovery will report temporary unavailability.

## Operations and security

- Set `PUBLIC_URL` to the actual HTTPS origin in deployment. Run behind a trusted HTTPS proxy; `Secure` cookies depend on this setting.
- Keep `PUBLIC_TOKEN_SECRET` stable. Changing it invalidates existing invoice links. Treat invoice links as bearer credentials and do not put sensitive payment credentials in invoice notes.
- Set `ADMIN_EMAIL` to an existing user and `ADMIN_TOTP_SECRET` to a base32 encoded 20-byte or longer secret to enable `/admin`. The admin page shows aggregate counts only and records successful views. Keep TOTP enrolment and recovery outside the application until a fuller admin identity flow exists.
- Back up PostgreSQL and rehearse a restore before accepting customer data. Maintain TLS, verified SMTP sending domains, and error monitoring for deployment.
- The API scopes all private records to the authenticated business. Registration creates one business per account in this build. Payment and invoice mutations use transactions and audit records.

## Scope still needed for a public launch

Paynow checkout is implemented, but this workspace has no Paynow merchant credentials or reachable callback URL, so no live or test-mode transaction has been verified here. The current admin view is read-only and does not change plans.

S3-compatible object storage, quotations, customer data export/deletion, email bounce processing, support controls, and detailed traffic analytics remain unimplemented. Product images live in one Docker volume, so multiple API replicas require shared storage or an object-store migration. Reminder delivery uses SMTP and a durable job table; SMTP acknowledgement cannot provide exactly-once delivery after a process crash, so an external provider with message idempotency is advisable before high-volume use. The in-memory IP limiter is suitable for one API instance and must move to a shared service or gateway when horizontally scaled.

No production deployment, live email delivery, or Paynow merchant flow has been validated here.

## Verification

From `apps/api`, run `go test ./...` and `go vet ./...`. From `apps/web`, run `npm ci`, `npm run lint`, and `npm run build`. To run the tenant-isolation and payment integration test, set `LINKPAY_TEST_DATABASE_URL` to a PostgreSQL database where the test user can create and drop schemas, then run `go test ./internal/platform -run TestTenantIsolationAndPaymentBoundary -v`. The test creates and drops its own randomly named schema. CI runs this test against PostgreSQL 16 and runs the official Go vulnerability checker.
