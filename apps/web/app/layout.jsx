import "./globals.css";

export const metadata = {
  title: {
    default: "LinkPay — Your business in one link",
    template: "%s | LinkPay",
  },
  description:
    "Show what you offer, turn enquiries into invoices, and keep payments moving with LinkPay.",
};

export default function RootLayout({ children }) {
  return (
    <html lang="en">
      <body>{children}</body>
    </html>
  );
}
