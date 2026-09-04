import { SubmissionDetail } from "@/components/submission";
export const metadata = { title: "Submission & receipts" };
export default async function Page({
  params,
}: {
  params: Promise<{ id: string }>;
}) {
  const { id } = await params;
  return <SubmissionDetail id={id} />;
}
