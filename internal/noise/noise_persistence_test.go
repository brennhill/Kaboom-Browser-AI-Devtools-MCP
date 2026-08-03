// Purpose: Tests for noise rule persistence across sessions.
// Docs: docs/features/feature/noise-filtering/index.md

package noise

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/persistence"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/statediag"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/statefault"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/types"
)

func newNoiseTestSessionStore(t *testing.T) *persistence.SessionStore {
	t.Helper()
	store, err := persistence.NewSessionStoreWithInterval(t.TempDir(), time.Hour, nil)
	if err != nil {
		t.Fatalf("NewSessionStoreWithInterval() error = %v", err)
	}
	t.Cleanup(func() { store.Shutdown() })
	return store
}

type noiseMemoryStore map[string][]byte

func (store noiseMemoryStore) Save(namespace, key string, data []byte) error {
	store[namespace+"/"+key] = append([]byte(nil), data...)
	return nil
}
func (store noiseMemoryStore) Load(namespace, key string) ([]byte, error) {
	return append([]byte(nil), store[namespace+"/"+key]...), nil
}
func (store noiseMemoryStore) List(string) ([]string, error) { return []string{"rules"}, nil }
func (store noiseMemoryStore) Delete(namespace, key string) error {
	delete(store, namespace+"/"+key)
	return nil
}

func TestNoisePersistenceUsesCanonicalFaultFallbacks(t *testing.T) {
	const private = "private-noise-rule"
	valid, err := json.Marshal(PersistedNoiseData{Version: 1, NextUserID: 1})
	if err != nil {
		t.Fatal(err)
	}
	for _, kind := range statefault.Kinds() {
		t.Run(string(kind), func(t *testing.T) {
			collector := statediag.NewCollector()
			base := noiseMemoryStore{"noise/rules": valid}
			store := statefault.NewStore(base, statefault.New(kind, private))
			config := NewNoiseConfigWithStore(store, collector)

			if kind == statefault.Write || kind == statefault.Sync || kind == statefault.Rename ||
				kind == statefault.DirectorySync || kind == statefault.Quota {
				if err := config.AddRules([]NoiseRule{{Category: "console", Classification: "repetitive", MatchSpec: NoiseMatchSpec{MessageRegex: "fault"}}}); err != nil {
					t.Fatal(err)
				}
			}
			diagnostics := collector.Snapshot()
			if kind == statefault.Restart {
				if len(diagnostics) != 0 {
					t.Fatalf("restart diagnostics = %#v, want clean durable reload", diagnostics)
				}
				return
			}
			if len(diagnostics) != 1 || diagnostics[0].Name != "noise_rule_state" || diagnostics[0].Fix == "" {
				t.Fatalf("%s diagnostics = %#v, want actionable noise-rule incident", kind, diagnostics)
			}
			if strings.Contains(diagnostics[0].Detail, private) {
				t.Fatal("Doctor diagnostic leaked private noise state")
			}
		})
	}
}

func TestNoiseConfigWithStorePersistsAndReloadsUserRules(t *testing.T) {
	t.Parallel()

	store := newNoiseTestSessionStore(t)
	nc := NewNoiseConfigWithStore(store, nil)

	if err := nc.AddRules([]NoiseRule{
		{
			Category:       "console",
			Classification: "repetitive",
			MatchSpec: NoiseMatchSpec{
				MessageRegex: "persist-me",
			},
		},
	}); err != nil {
		t.Fatalf("AddRules() error = %v", err)
	}
	nc.DismissNoise(`/health`, "network", "noisy health checks")

	if !nc.IsConsoleNoise(types.LogEntry{
		"level":   "info",
		"message": "persist-me message",
		"source":  "http://localhost:3000/app.js",
	}) {
		t.Fatal("expected rule to match before reload")
	}

	reloaded := NewNoiseConfigWithStore(store, nil)
	rules := reloaded.ListRules()

	foundPersistedRule := false
	foundDismissRule := false
	for _, r := range rules {
		if r.MatchSpec.MessageRegex == "persist-me" {
			foundPersistedRule = true
		}
		if r.Classification == "dismissed" && r.MatchSpec.URLRegex == `/health` {
			foundDismissRule = true
		}
	}
	if !foundPersistedRule {
		t.Fatal("expected persisted user rule to be loaded")
	}
	if !foundDismissRule {
		t.Fatal("expected persisted dismiss rule to be loaded")
	}

	if err := reloaded.AddRules([]NoiseRule{
		{
			Category:       "console",
			Classification: "repetitive",
			MatchSpec: NoiseMatchSpec{
				MessageRegex: "next-id",
			},
		},
	}); err != nil {
		t.Fatalf("AddRules(reloaded) error = %v", err)
	}

	var nextID string
	for _, r := range reloaded.ListRules() {
		if r.MatchSpec.MessageRegex == "next-id" {
			nextID = r.ID
			break
		}
	}
	if nextID != "user_3" {
		t.Fatalf("reloaded AddRules() id = %q, want user_3", nextID)
	}

	data, err := store.Load("noise", "rules")
	if err != nil {
		t.Fatalf("store.Load(noise/rules) error = %v", err)
	}

	var persisted PersistedNoiseData
	if err := json.Unmarshal(data, &persisted); err != nil {
		t.Fatalf("unmarshal persisted rules error = %v", err)
	}
	for _, r := range persisted.Rules {
		if strings.HasPrefix(r.ID, "builtin_") {
			t.Fatalf("persisted rules should not include built-ins, found %q", r.ID)
		}
	}
}

func TestNoiseConfigWithStoreLoadsValidRulesOnly(t *testing.T) {
	t.Parallel()

	store := newNoiseTestSessionStore(t)
	persisted := PersistedNoiseData{
		Version:    1,
		NextUserID: 2,
		Statistics: NoiseStatistics{
			TotalFiltered: 12,
			PerRule: map[string]int{
				"user_5": 12,
			},
			LastSignalAt: time.Now(),
			LastNoiseAt:  time.Now(),
		},
		Rules: []NoiseRule{
			{
				ID:             "builtin_bad",
				Category:       "console",
				Classification: "repetitive",
				MatchSpec: NoiseMatchSpec{
					MessageRegex: "ignore-me",
				},
				CreatedAt: time.Now(),
			},
			{
				ID:             "user_9",
				Category:       "console",
				Classification: "repetitive",
				MatchSpec: NoiseMatchSpec{
					MessageRegex: "[",
				},
				CreatedAt: time.Now(),
			},
			{
				ID:             "user_5",
				Category:       "console",
				Classification: "repetitive",
				MatchSpec: NoiseMatchSpec{
					MessageRegex: "keep-me",
				},
				CreatedAt: time.Now(),
			},
		},
	}

	data, err := json.Marshal(persisted)
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}
	if err := store.Save("noise", "rules", data); err != nil {
		t.Fatalf("store.Save(noise/rules) error = %v", err)
	}

	nc := NewNoiseConfigWithStore(store, nil)
	rules := nc.ListRules()

	hasKeep := false
	hasInvalid := false
	hasBuiltin := false
	for _, r := range rules {
		switch r.ID {
		case "user_5":
			hasKeep = true
		case "user_9":
			hasInvalid = true
		case "builtin_bad":
			hasBuiltin = true
		}
	}
	if !hasKeep {
		t.Fatal("expected valid persisted user rule to be loaded")
	}
	if hasInvalid {
		t.Fatal("invalid-regex persisted rule should be skipped")
	}
	if hasBuiltin {
		t.Fatal("built-in rules in persisted file should be skipped")
	}
	stats := nc.GetStatistics()
	if stats.TotalFiltered != 12 || stats.PerRule["user_5"] != 12 {
		t.Fatalf("reloaded stats = %+v, want TotalFiltered=12 and user_5=12", stats)
	}

	if err := nc.AddRules([]NoiseRule{
		{
			Category:       "console",
			Classification: "repetitive",
			MatchSpec: NoiseMatchSpec{
				MessageRegex: "after-desync",
			},
		},
	}); err != nil {
		t.Fatalf("AddRules() error = %v", err)
	}

	var id string
	for _, r := range nc.ListRules() {
		if r.MatchSpec.MessageRegex == "after-desync" {
			id = r.ID
			break
		}
	}
	if id != "user_6" {
		t.Fatalf("AddRules() id after desync recovery = %q, want user_6", id)
	}
}

func TestNoiseConfigWithStoreIgnoresCorruptOrUnsupportedData(t *testing.T) {
	t.Parallel()

	t.Run("corrupt_json", func(t *testing.T) {
		store := newNoiseTestSessionStore(t)
		if err := store.Save("noise", "rules", []byte("{")); err != nil {
			t.Fatalf("store.Save() error = %v", err)
		}

		diagnostics := statediag.NewCollector()
		nc := NewNoiseConfigWithStore(store, diagnostics)
		got := diagnostics.Snapshot()
		if len(got) != 1 {
			t.Fatal("corrupt persisted data should retain a Doctor diagnostic")
		}
		if got[0].Name != "noise_rule_state" {
			t.Fatalf("diagnostic name = %q, want noise_rule_state", got[0].Name)
		}
		if got[0].Fix == "" {
			t.Fatal("corrupt persisted data should provide remediation")
		}
		for _, r := range nc.ListRules() {
			if strings.HasPrefix(r.ID, "user_") || strings.HasPrefix(r.ID, "dismiss_") {
				t.Fatalf("corrupt persisted data should not load user rules, got %q", r.ID)
			}
		}
	})

	t.Run("unsupported_version", func(t *testing.T) {
		store := newNoiseTestSessionStore(t)
		data, err := json.Marshal(PersistedNoiseData{
			Version:    99,
			NextUserID: 2,
			Rules: []NoiseRule{
				{
					ID:             "user_1",
					Category:       "console",
					Classification: "repetitive",
					MatchSpec: NoiseMatchSpec{
						MessageRegex: "should-not-load",
					},
				},
			},
		})
		if err != nil {
			t.Fatalf("json.Marshal() error = %v", err)
		}
		if err := store.Save("noise", "rules", data); err != nil {
			t.Fatalf("store.Save() error = %v", err)
		}

		diagnostics := statediag.NewCollector()
		nc := NewNoiseConfigWithStore(store, diagnostics)
		got := diagnostics.Snapshot()
		if len(got) != 1 || got[0].Name != "noise_rule_state" {
			t.Fatalf("unsupported version diagnostic = %#v", got)
		}
		for _, r := range nc.ListRules() {
			if r.ID == "user_1" {
				t.Fatal("unsupported persistence version should be ignored")
			}
		}
	})
}

func TestAddRulesRejectsUnsafeRegexPatterns(t *testing.T) {
	t.Parallel()

	nc := NewNoiseConfig()
	err := nc.AddRules([]NoiseRule{
		{
			Category:       "console",
			Classification: "repetitive",
			MatchSpec: NoiseMatchSpec{
				MessageRegex: strings.Repeat("a", 513),
			},
		},
	})
	if err == nil || !strings.Contains(err.Error(), "maximum length") {
		t.Fatalf("expected max-length regex validation error, got %v", err)
	}

	err = nc.AddRules([]NoiseRule{
		{
			Category:       "console",
			Classification: "repetitive",
			MatchSpec: NoiseMatchSpec{
				MessageRegex: "(a+)+",
			},
		},
	})
	if err == nil || !strings.Contains(err.Error(), "nested quantifiers") {
		t.Fatalf("expected nested-quantifier regex validation error, got %v", err)
	}
}
