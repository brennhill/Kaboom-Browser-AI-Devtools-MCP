// Purpose: Provides non-stuttering type aliases for the security package.
// Why: Improves call-site readability while preserving backward compatibility for existing API names.

package security

type (
	DiffManager        = SecurityDiffManager
	Snapshot           = SecuritySnapshot
	Cookie             = SecurityCookie
	DiffResult         = SecurityDiffResult
	Change             = SecurityChange
	DiffSummary        = SecurityDiffSummary
	SnapshotListEntry  = SecuritySnapshotListEntry
	Finding            = SecurityFinding
	ScanInput          = SecurityScanInput
	ScanResult         = SecurityScanResult
	Scanner            = SecurityScanner
)
