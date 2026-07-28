// Purpose: Resolves first-party context and groups third-party origins for auditing.
// Why: Separates origin-scoping/filtering from entry-level risk classification logic.
// Docs: docs/features/feature/enterprise-audit/index.md

package thirdparty

import (
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/types"
	"sort"
	"strings"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/util"
)

// resolveCustomLists loads custom lists from file if needed.
func resolveCustomLists(params ThirdPartyParams) *CustomLists {
	if params.CustomLists != nil {
		return params.CustomLists
	}
	if params.CustomListsFile != "" {
		return loadCustomListsFile(params.CustomListsFile)
	}
	return nil
}

// buildFirstPartySet determines first-party origins from params and page URLs.
func buildFirstPartySet(params ThirdPartyParams, pageURLs []string, customLists *CustomLists) map[string]bool {
	firstParty := make(map[string]bool)
	if len(params.FirstPartyOrigins) > 0 {
		for _, origin := range params.FirstPartyOrigins {
			firstParty[origin] = true
		}
	} else {
		for _, pageURL := range pageURLs {
			if origin := util.ExtractOrigin(pageURL); origin != "" {
				firstParty[origin] = true
			}
		}
	}
	addInternalDomains(firstParty, customLists)
	return firstParty
}

// addInternalDomains treats custom internal domains as first-party.
func addInternalDomains(firstParty map[string]bool, customLists *CustomLists) {
	if customLists == nil {
		return
	}
	for _, internal := range customLists.Internal {
		origin := internal
		if !strings.Contains(origin, "://") {
			origin = "https://" + origin
		}
		if parsed := util.ExtractOrigin(origin); parsed != "" {
			firstParty[parsed] = true
		}
	}
}

// groupByThirdPartyOrigin groups bodies by third-party origin (excluding first-party).
func groupByThirdPartyOrigin(bodies []types.NetworkBody, firstParty map[string]bool) map[string]*originData {
	originMap := make(map[string]*originData)
	for _, body := range bodies {
		origin := util.ExtractOrigin(body.URL)
		if origin == "" || firstParty[origin] {
			continue
		}
		if _, ok := originMap[origin]; !ok {
			originMap[origin] = &originData{}
		}
		originMap[origin].bodies = append(originMap[origin].bodies, body)
		originMap[origin].urls = append(originMap[origin].urls, body.URL)
	}
	return originMap
}

// buildAllEntries creates ThirdPartyEntry structs for each origin.
func buildAllEntries(originMap map[string]*originData, customLists *CustomLists) []ThirdPartyEntry {
	entries := make([]ThirdPartyEntry, 0, len(originMap))
	for origin, data := range originMap {
		entries = append(entries, buildThirdPartyEntry(origin, data.bodies, data.urls, customLists))
	}
	return entries
}

// filterAndSort optionally removes static-only entries and sorts by risk level.
func filterAndSort(entries []ThirdPartyEntry, includeStatic *bool) []ThirdPartyEntry {
	if includeStatic != nil && !*includeStatic {
		var filtered []ThirdPartyEntry
		for _, entry := range entries {
			if entry.RiskLevel != "low" {
				filtered = append(filtered, entry)
			}
		}
		entries = filtered
	}
	sort.Slice(entries, func(i, j int) bool {
		return riskOrder(entries[i].RiskLevel) < riskOrder(entries[j].RiskLevel)
	})
	return entries
}
