import { Customers } from "../../../components/list-pages";
import { pageNumber } from "../../../lib/api";
export const metadata = { title: "Customers" };
export default async function Page({ searchParams }) {
  const { page } = await searchParams;
  return <Customers page={pageNumber(page)} />;
}
