import { Invoices } from "../../../components/list-pages";
import { pageNumber } from "../../../lib/api";
export const metadata = { title: "Invoices" };
export default async function Page({ searchParams }) {
  const { page } = await searchParams;
  return <Invoices page={pageNumber(page)} />;
}
