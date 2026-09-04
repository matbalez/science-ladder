import { ReviewConsole } from "@/components/review";
export const metadata = { title: "Editorial review" };
export default async function Page({
  searchParams,
}: {
  searchParams: Promise<{
    challenge?: string | string[];
    version?: string | string[];
  }>;
}) {
  const query = await searchParams;
  const challenge =
    typeof query.challenge === "string" ? query.challenge.slice(0, 200) : "";
  const version =
    typeof query.version === "string" ? query.version.slice(0, 100) : "";
  return (
    <ReviewConsole
      key={`${challenge}:${version}`}
      initialChallenge={challenge}
      initialVersion={version}
    />
  );
}
