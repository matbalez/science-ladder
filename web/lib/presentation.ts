// Editorial display copy for the frozen launch challenge. Its archived proposal
// and scoring contract remain available unchanged in the public export.
export function displaySummary(challenge: {
  repository: string;
  sourceCommit: string;
  summary: string;
}): string {
  if (
    challenge.repository === "matbalez/science-ladder-quiet-echoes" &&
    challenge.sourceCommit === "f42f527e97563b1c068a1835732c6da44f21223f"
  ) {
    return "Find a 512-sign sequence with lower autocorrelation energy than the published reference.";
  }
  return challenge.summary;
}
