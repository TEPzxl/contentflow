import { Workbench } from "@/features/workbench/workbench";

type HomePageProps = {
  searchParams?: Promise<Record<string, string | string[] | undefined>>;
};

export default async function HomePage({ searchParams }: HomePageProps) {
  const params = (await searchParams) ?? {};
  const auth = Array.isArray(params.auth) ? params.auth[0] : params.auth;
  return <Workbench initialAuthMode={auth === "register" ? "register" : "login"} />;
}
