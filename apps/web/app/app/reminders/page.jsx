import RemindersClient from "../../../components/reminders";
import { pageNumber } from "../../../lib/api";
export const metadata = { title: "Reminders" };
export default async function Page({ searchParams }) {
  const { page } = await searchParams;
  return <RemindersClient page={pageNumber(page)} />;
}
