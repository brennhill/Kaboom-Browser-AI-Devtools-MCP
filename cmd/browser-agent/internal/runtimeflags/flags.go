// flags.go — Parses the daemon's canonical process-level CLI options.
// Docs: docs/features/feature/mcp-persistent-server/index.md

package runtimeflags

import (
	"flag"
	"io"
	"strings"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/serverdefaults"
)

type multiFlag []string

func (values *multiFlag) String() string { return strings.Join(*values, ", ") }
func (values *multiFlag) Set(value string) error {
	*values = append(*values, value)
	return nil
}

// Values is the fully parsed CLI surface. Validation and side effects remain in
// process composition because they require filesystem and lifecycle owners.
type Values struct {
	Port                    int
	MaxEntries              int
	FastPathMinSamples      int
	LogFile                 string
	APIKey                  string
	ClientID                string
	StateDir                string
	UploadDir               string
	FastPathMaxFailureRatio float64
	ShowVersion             bool
	ShowHelp                bool
	DoctorMode              bool
	StopMode                bool
	ConnectMode             bool
	BridgeMode              bool
	DaemonMode              bool
	EnableOSUpload          bool
	ParallelMode            bool
	ForceCleanup            bool
	InstallMode             bool
	UploadDenyPatterns      []string
	SSRFAllowedHosts        []string
	Arguments               []string
}

// Parse deterministically parses args without mutating flag.CommandLine.
func Parse(args []string, apiKeyDefault string) (Values, error) {
	set := flag.NewFlagSet("kaboom", flag.ContinueOnError)
	set.SetOutput(io.Discard)
	values := Values{}
	var uploadDenyPatterns multiFlag
	var ssrfAllowedHosts multiFlag
	set.IntVar(&values.Port, "port", serverdefaults.Port, "Port to listen on")
	set.StringVar(&values.LogFile, "log-file", "", "Path to log file (default: in runtime state dir)")
	set.IntVar(&values.MaxEntries, "max-entries", serverdefaults.MaxLogEntries, "Max log entries before rotation")
	set.IntVar(&values.FastPathMinSamples, "fastpath-min-samples", 50, "Minimum fast-path telemetry samples required when threshold check is enabled")
	set.Float64Var(&values.FastPathMaxFailureRatio, "fastpath-max-failure-ratio", -1, "Maximum allowed fast-path failure ratio in --doctor (set >=0 to enforce)")
	set.BoolVar(&values.ShowVersion, "version", false, "Show version")
	set.BoolVar(&values.ShowHelp, "help", false, "Show help")
	set.StringVar(&values.APIKey, "api-key", apiKeyDefault, "API key for HTTP authentication (optional, or KABOOM_API_KEY env)")
	set.BoolVar(&values.DoctorMode, "doctor", false, "Run setup diagnostics")
	set.BoolVar(&values.StopMode, "stop", false, "Stop the running server on the specified port")
	set.BoolVar(&values.ConnectMode, "connect", false, "Connect to existing server (multi-client mode)")
	set.StringVar(&values.ClientID, "client-id", "", "Override client ID (default: derived from CWD)")
	set.BoolVar(&values.BridgeMode, "bridge", false, "Run as stdio-to-HTTP bridge (spawns daemon if needed)")
	set.BoolVar(&values.DaemonMode, "daemon", false, "Run as background server daemon (internal use)")
	set.BoolVar(&values.ParallelMode, "parallel", false, "Enable isolated parallel daemon mode (skip takeover; requires unique port/state-dir)")
	set.StringVar(&values.StateDir, "state-dir", "", "Directory for runtime state (default: OS app state directory)")
	set.BoolVar(&values.EnableOSUpload, "enable-os-upload-automation", false, "Enable OS-level file upload automation (Stage 4: AppleScript/xdotool)")
	set.StringVar(&values.UploadDir, "upload-dir", "", "Directory from which file uploads are allowed (required for Stages 2-4)")
	set.BoolVar(&values.ForceCleanup, "force", false, "Force kill all running kaboom daemons")
	set.BoolVar(&values.InstallMode, "install", false, "Auto-install Kaboom to all detected MCP clients")
	set.Var(&uploadDenyPatterns, "upload-deny-pattern", "Additional sensitive path patterns to block (repeatable)")
	set.Var(&ssrfAllowedHosts, "ssrf-allow-host", "Host:port to allow for form submit SSRF (repeatable, test use)")
	if err := set.Parse(args); err != nil {
		return Values{}, err
	}
	values.UploadDenyPatterns = append([]string(nil), uploadDenyPatterns...)
	values.SSRFAllowedHosts = append([]string(nil), ssrfAllowedHosts...)
	values.Arguments = append([]string(nil), set.Args()...)
	return values, nil
}
