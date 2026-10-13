import AuthForm from "../../components/auth-form";
export const metadata = { title: "Create an account" };
export default async function Page({ searchParams }) {
  const { trial } = await searchParams;
  return (
    <AuthForm
      mode="register"
      initialTrialPlan={trial === "business" ? "business" : "starter"}
    />
  );
}
