import AuthForm from "../../components/auth-form";
export const metadata = { title: "Reset password" };
export default function Page() {
  return <AuthForm mode="forgot" />;
}
