import AuthForm from "../../components/auth-form";
export const metadata = { title: "Create an account" };
export default function Page() {
  return <AuthForm mode="register" />;
}
