// Purpose: Defines SARIF 2.1.0 specification types and constants for accessibility report output.
// Why: Centralizes SARIF type definitions so conversion and file modules share a single schema.
package sarif

// SARIF 2.1.0 specification constants
const (
	defaultToolVersion = "dev"
	sarifSpecVersion   = "2.1.0"
	sarifSchemaURL     = "https://raw.githubusercontent.com/oasis-tcs/sarif-spec/main/sarif-2.1/schema/sarif-schema-2.1.0.json"
)

// SARIFLog is the top-level SARIF 2.1.0 object.
type SARIFLog struct {
	Schema  string     `json:"$schema"`
	Version string     `json:"version"`
	Runs    []sarifRun `json:"runs"`
}

// sarifRun represents a single analysis run.
type sarifRun struct {
	Tool    sarifTool     `json:"tool"`
	Results []sarifResult `json:"results"`
}

// sarifTool describes the analysis tool.
type sarifTool struct {
	Driver sarifDriver `json:"driver"`
}

// sarifDriver describes the tool driver (primary component).
type sarifDriver struct {
	Name           string      `json:"name"`
	Version        string      `json:"version"`
	InformationURI string      `json:"informationUri"` // SPEC:SARIF
	Rules          []sarifRule `json:"rules"`
}

// sarifRule describes a single analysis rule.
type sarifRule struct {
	ID               string               `json:"id"`
	ShortDescription sarifMessage         `json:"shortDescription"` // SPEC:SARIF
	FullDescription  sarifMessage         `json:"fullDescription"`  // SPEC:SARIF
	HelpURI          string               `json:"helpUri"`          // SPEC:SARIF
	Properties       *sarifRuleProperties `json:"properties,omitempty"`
}

// sarifRuleProperties holds additional rule metadata.
type sarifRuleProperties struct {
	Tags []string `json:"tags,omitempty"`
}

// sarifResult represents a single analysis finding.
type sarifResult struct {
	RuleID    string          `json:"ruleId"`    // SPEC:SARIF
	RuleIndex int             `json:"ruleIndex"` // SPEC:SARIF
	Level     string          `json:"level"`
	Message   sarifMessage    `json:"message"`
	Locations []sarifLocation `json:"locations"`
}

// sarifMessage is a simple text message.
type sarifMessage struct {
	Text string `json:"text"`
}

// sarifLocation represents a finding location.
type sarifLocation struct {
	PhysicalLocation sarifPhysicalLocation `json:"physicalLocation"` // SPEC:SARIF
}

// sarifPhysicalLocation describes the physical location of a finding.
type sarifPhysicalLocation struct {
	ArtifactLocation sarifArtifactLocation `json:"artifactLocation"` // SPEC:SARIF
	Region           sarifRegion           `json:"region"`
}

// sarifArtifactLocation identifies the artifact (file, DOM element, etc.).
type sarifArtifactLocation struct {
	URI       string `json:"uri"`
	URIBaseID string `json:"uriBaseId,omitempty"` // SPEC:SARIF
}

// sarifRegion describes a region within an artifact.
type sarifRegion struct {
	Snippet sarifSnippet `json:"snippet"`
}

// sarifSnippet contains a text snippet of the region.
type sarifSnippet struct {
	Text string `json:"text"`
}

// SARIFExportOptions controls the export behavior.
type SARIFExportOptions struct {
	Scope         string `json:"scope"`
	IncludePasses bool   `json:"include_passes"`
	SaveTo        string `json:"save_to"`
	Version       string `json:"version,omitempty"`
}

type axeResult struct {
	Violations   []axeViolation `json:"violations"`
	Passes       []axeViolation `json:"passes"`
	Incomplete   []axeViolation `json:"incomplete"`
	Inapplicable []axeViolation `json:"inapplicable"`
}

type axeViolation struct {
	ID          string    `json:"id"`
	Impact      string    `json:"impact"`
	Description string    `json:"description"`
	Help        string    `json:"help"`
	HelpURL     string    `json:"helpUrl"` // SPEC:axe-core
	Tags        []string  `json:"tags"`
	Nodes       []axeNode `json:"nodes"`
}

type axeNode struct {
	HTML   string   `json:"html"`
	Target []string `json:"target"`
	Impact string   `json:"impact"`
}
