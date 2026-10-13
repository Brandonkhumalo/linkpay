import { Enquiries } from "../../../components/list-pages";
import { pageNumber } from "../../../lib/api";
export const metadata = { title: "Enquiries" };
export default async function Page({ searchParams }) {
  const { page } = await searchParams;
  return <Enquiries page={pageNumber(page)} />;
}
