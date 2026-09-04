import { ChallengeDetail } from "@/components/challenge-detail";
export default async function Page({
  params,
}: {
  params: Promise<{ slug: string }>;
}) {
  const { slug } = await params;
  return <ChallengeDetail slug={slug} />;
}
