import ShopsPage from "../../shops/page";

export const metadata = { title: "Explore shops | LinkPay" };

export default function WorkspaceShopsPage({ searchParams }) {
  return <ShopsPage searchParams={searchParams} workspace />;
}
