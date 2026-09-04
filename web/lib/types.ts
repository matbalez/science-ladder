export type Json =
  null | boolean | number | string | Json[] | { [key: string]: Json };
export interface Finding {
  code?: string;
  message?: string;
  severity?: string;
  path?: string;
  detail?: string;
  [key: string]: unknown;
}
export interface User {
  id: string;
  githubId: string;
  login: string;
  avatarUrl?: string;
  role: string;
  invited: boolean;
}
export interface Session {
  user: User | null;
  quotas: { remaining: number; activeLimit: number };
  capabilities: { creation: boolean; submission: boolean; review: boolean };
  configuration: {
    githubAuth: boolean;
    scientificReview: boolean;
    officialRunner: boolean;
  };
}
export interface Milestone {
  id: string;
  label: string;
  thresholdTicks: string;
  claimedBy?: string;
  claimedAt?: string;
}
export interface Submission {
  id: string;
  versionId: string;
  sequence: number;
  status: string;
  outcome: string;
  verificationPolicy?: "platform" | "independent";
  verificationStatus?: "" | "platform_verified" | "independently_replicated";
  independentReplication?: boolean;
  scoreTicks?: string;
  artifactDigest?: string;
  repository?: string;
  sourceCommit?: string;
  public: boolean;
  attribution: {
    model?: string;
    harness?: string;
    disclosure?: string;
    platformSeeded?: boolean;
  };
  createdAt: string;
  receiptDigest?: string;
  adjudicationDigest?: string;
  claims: unknown[];
  runs: Record<string, unknown>[];
}
export interface Challenge {
  id: string;
  slug: string;
  title: string;
  summary: string;
  domain: string;
  status: string;
  reviewStatus: string;
  intakeStatus: string;
  economicMode: "none";
  verificationPolicy?: "platform" | "independent";
  versionId: string;
  repository: string;
  sourceCommit: string;
  createdAt: string;
  deadline: string;
  metric: {
    name: string;
    direction: "maximize" | "minimize";
    units: string;
    quantum: string;
    baselineTicks: string;
  };
  milestones: Milestone[];
  verifiedBest?: { submissionId: string; scoreTicks: string };
  publicFrontier?: { submissionId: string; scoreTicks: string };
  badges: string[];
  manifest?: Record<string, unknown>;
  candidate?: Record<string, unknown>;
  reviews?: Record<string, unknown>[];
  submissions?: Submission[];
}
export interface Candidate {
  id: string;
  status: string;
  candidate: Record<string, unknown>;
  findings: Finding[];
  createdAt?: string;
}
export interface Intent {
  id: string;
  versionId: string;
  status: string;
  repository: string;
  sourceCommit?: string;
  artifactDigest?: string;
  findings: Finding[];
  submissionId?: string;
  createdAt: string;
}
export interface Preflight {
  id: string;
  versionId: string;
  status: string;
  findings: Finding[];
  reports?: Record<string, unknown> | Record<string, unknown>[];
  createdAt: string;
}
