// playbooks_adapter.go — Bridges the playbooks sub-package into the main package namespace.
// Why: Allows callers in the main package to use playbook functions without qualifying every call.

package main

import "github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/playbooks"

// Re-export package-level vars for main-package callers.
var (
	capabilityIndex   = playbooks.CapabilityIndex
	playbookMap       = playbooks.Playbooks
	guideContent      = playbooks.GuideContent
	quickstartContent = playbooks.QuickstartContent
	demoScripts       = playbooks.DemoScripts
)

// resolveResourceContent delegates to the playbooks sub-package.
func resolveResourceContent(uri string) (string, string, bool) {
	return playbooks.ResolveResourceContent(uri)
}

// tutorialFailureRecoveryPlaybooks delegates to the playbooks sub-package.
func tutorialFailureRecoveryPlaybooks() map[string]any {
	return playbooks.TutorialFailureRecoveryPlaybooks()
}
