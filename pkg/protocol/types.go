// Package protocol implements the versioned, payment-free Science Ladder contract.
package protocol

import "time"

const APIVersion = "science-ladder/v1"
const PayloadType = "application/vnd.science-ladder.v1+json"
const ScoutVersion = "1.0.0"

type Source struct {
	URL        string `json:"url"`
	Title      string `json:"title"`
	Evidence   string `json:"evidence"`
	Location   string `json:"location"`
	AccessedAt string `json:"accessedAt,omitempty"`
}

type Candidate struct {
	APIVersion           string    `json:"apiVersion"`
	Kind                 string    `json:"kind"`
	ID                   string    `json:"id"`
	CreatedAt            time.Time `json:"createdAt"`
	Producer             string    `json:"producer"`
	PromptVersion        string    `json:"promptVersion"`
	Model                string    `json:"model,omitempty"`
	Disposition          string    `json:"disposition"` // viable, needs_work, rejected
	Sources              []Source  `json:"sources"`
	Uncertainties        []string  `json:"uncertainties"`
	RejectedAlternatives []string  `json:"rejectedAlternatives"`
	RepositoryPlan       []string  `json:"repositoryPlan"`
	Manifest             *Manifest `json:"manifest,omitempty"`
}

type Metric struct {
	Name              string `json:"name"`
	Direction         string `json:"direction"`
	Unit              string `json:"unit"`
	Quantum           string `json:"quantum"`
	BaselineTicks     string `json:"baselineTicks"`
	MinimumDeltaTicks string `json:"minimumDeltaTicks"`
	ToleranceTicks    string `json:"toleranceTicks"`
	DomainMinTicks    string `json:"domainMinTicks,omitempty"`
	DomainMaxTicks    string `json:"domainMaxTicks,omitempty"`
}

type Milestone struct {
	ID             string `json:"id"`
	Title          string `json:"title"`
	ThresholdTicks string `json:"thresholdTicks"`
	Rationale      string `json:"rationale"`
}

type SubmissionContract struct {
	AllowedPaths      []string `json:"allowedPaths"`
	AllowedExtensions []string `json:"allowedExtensions"`
	MaxBytes          int64    `json:"maxBytes"`
	MaxFiles          int      `json:"maxFiles"`
	License           string   `json:"license"`
}

type Validator struct {
	Profile            string   `json:"profile"`
	Entrypoint         []string `json:"entrypoint"`
	DependencyLock     string   `json:"dependencyLock"`
	RuntimeImageDigest string   `json:"runtimeImageDigest"`
}

type Suite struct {
	Visibility string     `json:"visibility"`
	Path       string     `json:"path"`
	Commitment string     `json:"commitment,omitempty"`
	RevealAt   *time.Time `json:"revealAt,omitempty"`
}

type Resources struct {
	Class          string `json:"class"`
	VCPU           int    `json:"vCpu"`
	MemoryMB       int    `json:"memoryMb"`
	TimeoutSeconds int    `json:"timeoutSeconds"`
	MaxOutputBytes int64  `json:"maxOutputBytes"`
}

type Fixture struct {
	Name            string `json:"name"`
	Path            string `json:"path"`
	ExpectedOutcome string `json:"expectedOutcome"`
	ExpectedTicks   string `json:"expectedTicks,omitempty"`
}

type Manifest struct {
	APIVersion           string             `json:"apiVersion"`
	Kind                 string             `json:"kind"`
	ID                   string             `json:"id"`
	CreatedAt            time.Time          `json:"createdAt"`
	Producer             string             `json:"producer"`
	Slug                 string             `json:"slug"`
	Title                string             `json:"title"`
	Summary              string             `json:"summary"`
	ScientificQuestion   string             `json:"scientificQuestion"`
	Evidence             []Source           `json:"evidence"`
	Impact               string             `json:"impact"`
	Limitations          []string           `json:"limitations"`
	SafetyClassification string             `json:"safetyClassification"`
	EconomicMode         string             `json:"economicMode"`
	Metric               Metric             `json:"metric"`
	HardGates            []string           `json:"hardGates"`
	Milestones           []Milestone        `json:"milestones"`
	Deadline             time.Time          `json:"deadline"`
	Submission           SubmissionContract `json:"submission"`
	Validator            Validator          `json:"validator"`
	Suite                Suite              `json:"suite"`
	Resources            Resources          `json:"resources"`
	Fixtures             []Fixture          `json:"fixtures"`
}

type Lock struct {
	SuiteDiskDigest string `json:"suiteDiskDigest"`
	DeploymentMode         string    `json:"deploymentMode"`
	OfficialAcceptance     bool      `json:"officialAcceptance"`
	APIVersion             string    `json:"apiVersion"`
	Kind                   string    `json:"kind"`
	ID                     string    `json:"id"`
	CreatedAt              time.Time `json:"createdAt"`
	Producer               string    `json:"producer"`
	ManifestDigest         string    `json:"manifestDigest"`
	SourceSnapshotDigest   string    `json:"sourceSnapshotDigest"`
	ValidatorImageDigest   string    `json:"validatorImageDigest"`
	ValidatorDiskDigest    string    `json:"validatorDiskDigest"`
	SuiteDigest            string    `json:"suiteDigest"`
	ExecutionProfileDigest string    `json:"executionProfileDigest"`
	ReviewDigests          []string  `json:"reviewDigests"`
	EconomicMode           string    `json:"economicMode"`
	Manifest               Manifest  `json:"manifest"`
}

type ValidatorResult struct {
	APIVersion string          `json:"apiVersion"`
	Kind       string          `json:"kind"`
	Score      string          `json:"score"`
	Gates      map[string]bool `json:"gates"`
}

type ObjectRef struct {
	Digest string `json:"digest"`
	Size   int64  `json:"size"`
	URL    string `json:"url"`
}

// RunnerJob is immutable and signed by the control plane. URLs are one-job reads.
type RunnerJob struct {
	HiddenSuite *HiddenSuiteGrant `json:"hiddenSuite,omitempty"`
	AcceptanceReceiptDigest string     `json:"acceptanceReceiptDigest,omitempty"`
	DeploymentMode          string     `json:"deploymentMode"`
	OfficialAcceptance      bool       `json:"officialAcceptance"`
	APIVersion              string     `json:"apiVersion"`
	Kind                    string     `json:"kind"`
	ID                      string     `json:"id"`
	CreatedAt               time.Time  `json:"createdAt"`
	Producer                string     `json:"producer"`
	ExpiresAt               time.Time  `json:"expiresAt"`
	Purpose                 string     `json:"purpose"` // preflight, submission, confirmation
	SubmissionID            string     `json:"submissionId,omitempty"`
	ChallengeLockDigest     string     `json:"challengeLockDigest"`
	ArtifactDigest          string     `json:"artifactDigest"`
	SuiteDigest             string     `json:"suiteDigest"`
	ExecutionProfileDigest  string     `json:"executionProfileDigest"`
	RunnerEpoch             string     `json:"runnerEpoch"`
	FencingToken            int64      `json:"fencingToken"`
	RequiredHostGroup       string     `json:"requiredHostGroup"`
	ExcludedHostIDs         []string   `json:"excludedHostIds"`
	ValidatorDisk           ObjectRef  `json:"validatorDisk"`
	SubmissionDisk          ObjectRef  `json:"submissionDisk"`
	SuiteDisk               ObjectRef  `json:"suiteDisk"`
	ChallengeDisk           ObjectRef  `json:"challengeDisk"`
	Manifest                Manifest   `json:"manifest"`
	SourceSnapshot          *ObjectRef `json:"sourceSnapshot,omitempty"`
}

// SuiteKeyMaterial is private. It must never appear in public receipts or exports.
type SuiteKeyMaterial struct {
	Key []byte `json:"key"`
	Salt []byte `json:"salt"`
	PlaintextDigest string `json:"plaintextDigest"`
	Commitment string `json:"commitment"`
}

type KeyCapsule struct {
	Algorithm string `json:"algorithm"`
	EphemeralPublicKey []byte `json:"ephemeralPublicKey"`
	Nonce []byte `json:"nonce"`
	Ciphertext []byte `json:"ciphertext"`
	ContextDigest string `json:"contextDigest"`
}

type HiddenSuiteGrant struct {
	Source ObjectRef `json:"source"`
	Commitment string `json:"commitment"`
	KeyCapsule KeyCapsule `json:"keyCapsule"`
}

type FixtureReport struct {
	Name            string `json:"name"`
	ExpectedOutcome string `json:"expectedOutcome"`
	Outcome         string `json:"outcome"`
	ScoreTicks      string `json:"scoreTicks,omitempty"`
	Passed          bool   `json:"passed"`
}

type BuildReport struct {
	ManifestDigest             string          `json:"manifestDigest"`
	SourceSnapshotDigest       string          `json:"sourceSnapshotDigest"`
	ValidatorDiskDigest        string          `json:"validatorDiskDigest"`
	RebuiltValidatorDiskDigest string          `json:"rebuiltValidatorDiskDigest"`
	ValidatorImageDigest       string          `json:"validatorImageDigest"`
	SuiteDigest                string          `json:"suiteDigest"`
	ExecutionProfileDigest     string          `json:"executionProfileDigest"`
	OfflineRebuild             bool            `json:"offlineRebuild"`
	Fixtures                   []FixtureReport `json:"fixtures"`
	HostileCorpusPassed        bool            `json:"hostileCorpusPassed"`
	ScansPassed                bool            `json:"scansPassed"`
	Passed                     bool            `json:"passed"`
	Findings                   []string        `json:"findings"`
	ValidatorDisk              *ObjectRef      `json:"validatorDisk,omitempty"`
	ChallengeDisk              *ObjectRef      `json:"challengeDisk,omitempty"`
	SuiteDisk                  *ObjectRef      `json:"suiteDisk,omitempty"`
	SubmissionDisk             *ObjectRef      `json:"submissionDisk,omitempty"`
}

type RunReceipt struct {
	AcceptanceReceiptDigest string          `json:"acceptanceReceiptDigest,omitempty"`
	DeploymentMode          string          `json:"deploymentMode"`
	OfficialAcceptance      bool            `json:"officialAcceptance"`
	APIVersion              string          `json:"apiVersion"`
	Kind                    string          `json:"kind"`
	ID                      string          `json:"id"`
	CreatedAt               time.Time       `json:"createdAt"`
	Producer                string          `json:"producer"`
	JobID                   string          `json:"jobId"`
	JobDigest               string          `json:"jobDigest"`
	ChallengeLockDigest     string          `json:"challengeLockDigest"`
	ArtifactDigest          string          `json:"artifactDigest"`
	SuiteDigest             string          `json:"suiteDigest"`
	ExecutionProfileDigest  string          `json:"executionProfileDigest"`
	RunnerEpoch             string          `json:"runnerEpoch"`
	FencingToken            int64           `json:"fencingToken"`
	HostID                  string          `json:"hostId"`
	HostGroup               string          `json:"hostGroup"`
	Official                bool            `json:"official"`
	Outcome                 string          `json:"outcome"`
	ScoreTicks              string          `json:"scoreTicks,omitempty"`
	Gates                   map[string]bool `json:"gates,omitempty"`
	DurationMillis          int64           `json:"durationMillis"`
	CleanupAttested         bool            `json:"cleanupAttested"`
	BuildReport             *BuildReport    `json:"buildReport,omitempty"`
}

type Signature struct {
	KeyID string `json:"keyid"`
	Sig   string `json:"sig"`
}

type Envelope struct {
	PayloadType string      `json:"payloadType"`
	Payload     string      `json:"payload"`
	Signatures  []Signature `json:"signatures"`
}

type Receipt struct {
	DeploymentMode     string         `json:"deploymentMode"`
	OfficialAcceptance bool           `json:"officialAcceptance"`
	APIVersion         string         `json:"apiVersion"`
	Kind               string         `json:"kind"`
	ID                 string         `json:"id"`
	CreatedAt          time.Time      `json:"createdAt"`
	Producer           string         `json:"producer"`
	SubjectDigest      string         `json:"subjectDigest"`
	EconomicMode       string         `json:"economicMode"`
	Data               map[string]any `json:"data"`
}
