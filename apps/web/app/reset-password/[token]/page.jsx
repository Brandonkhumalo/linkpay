import AuthForm from "../../../components/auth-form";
export const metadata = { title: "Choose a new password" };
export default async function Page({ params }) {
  const { token } = await params;
  return <AuthForm mode="reset" token={token} />;
}
