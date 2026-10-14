import StorePage, { generateMetadata } from "../../../store/[slug]/page";

export { generateMetadata };

export default function WorkspaceStorePage({ params }) {
  return <StorePage params={params} workspace />;
}
