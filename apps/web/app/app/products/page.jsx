import { Products } from "../../../components/list-pages";
import { pageNumber } from "../../../lib/api";
export const metadata = { title: "Products & services" };
export default async function Page({ searchParams }) {
  const { page } = await searchParams;
  return <Products page={pageNumber(page)} />;
}
