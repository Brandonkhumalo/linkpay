import BillingClient from "../../../components/billing";
import { pageNumber } from "../../../lib/api";
export const metadata = { title: "Billing" };
export default async function Page({ searchParams }) {
  const { page } = await searchParams;
  return <BillingClient page={pageNumber(page)} />;
}
