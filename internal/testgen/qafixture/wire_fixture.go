// wire_fixture.go — Defines and validates the versioned browser QA fixture wire contract.

package qafixture

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/url"
	"regexp"
)

const (
	CurrentVersion        = 1
	DefaultSetupTimeoutMs = 10_000
	MaxSetupTimeoutMs     = 30_000
	MaxStateBytes         = 64 * 1024
	maxStateEntries       = 100
)

var localePattern = regexp.MustCompile(`^[A-Za-z]{2,3}(?:-[A-Za-z0-9]{2,8})*$`)
var cookieNamePattern = regexp.MustCompile("^[!#$%&'*+\\-.^_`|~0-9A-Za-z]+$")

type WireQAFixture struct {
	Version        int                        `json:"version"`
	Target         WireQATarget               `json:"target,omitempty"`
	Viewport       WireQAViewport             `json:"viewport,omitempty"`
	Locale         string                     `json:"locale,omitempty"`
	Permissions    []string                   `json:"permissions,omitempty"`
	Network        WireQANetwork              `json:"network,omitempty"`
	Cookies        []WireQACookie             `json:"cookies,omitempty"`
	LocalStorage   map[string]string          `json:"local_storage,omitempty"`
	SessionStorage map[string]string          `json:"session_storage,omitempty"`
	FeatureFlags   map[string]bool            `json:"feature_flags,omitempty"`
	SeedData       map[string]json.RawMessage `json:"seed_data,omitempty"`
	UserState      string                     `json:"user_state,omitempty"`
	AuthRole       string                     `json:"auth_role,omitempty"`
	SetupTimeoutMs int                        `json:"setup_timeout_ms,omitempty"`
}

type WireQATarget struct {
	URL string `json:"url,omitempty"`
}

type WireQAViewport struct {
	Width  int `json:"width,omitempty"`
	Height int `json:"height,omitempty"`
}

type WireQANetwork struct {
	Profile string `json:"profile,omitempty"`
}

type WireQACookie struct {
	Name     string `json:"name"`
	Value    string `json:"value"`
	Domain   string `json:"domain,omitempty"`
	Path     string `json:"path,omitempty"`
	Secure   bool   `json:"secure,omitempty"`
	HTTPOnly bool   `json:"http_only,omitempty"`
	SameSite string `json:"same_site,omitempty"`
}

var supportedPermissions = map[string]struct{}{
	"camera": {}, "clipboard_read": {}, "clipboard_write": {},
	"geolocation": {}, "microphone": {}, "notifications": {},
}

var supportedNetworkProfiles = map[string]struct{}{
	"online": {}, "offline": {}, "slow_3g": {}, "fast_3g": {},
}

// Parse decodes one strict fixture document and validates every bound before
// browser state can be read or mutated.
func Parse(raw json.RawMessage) (WireQAFixture, error) {
	var fixture WireQAFixture
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&fixture); err != nil {
		return WireQAFixture{}, fmt.Errorf("invalid fixture JSON: %w", err)
	}
	if err := requireJSONEnd(decoder); err != nil {
		return WireQAFixture{}, err
	}
	if fixture.SetupTimeoutMs == 0 {
		fixture.SetupTimeoutMs = DefaultSetupTimeoutMs
	}
	if err := fixture.validate(); err != nil {
		return WireQAFixture{}, err
	}
	return fixture, nil
}

func requireJSONEnd(decoder *json.Decoder) error {
	var extra json.RawMessage
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		return errors.New("invalid fixture JSON: multiple values are not allowed")
	}
	return nil
}

func (fixture WireQAFixture) validate() error {
	if fixture.Version != CurrentVersion {
		return fmt.Errorf("unsupported fixture version %d; use version %d", fixture.Version, CurrentVersion)
	}
	if err := validateTarget(fixture.Target); err != nil {
		return err
	}
	if err := validateViewport(fixture.Viewport); err != nil {
		return err
	}
	if fixture.Locale != "" && !localePattern.MatchString(fixture.Locale) {
		return errors.New("locale must be a valid language tag")
	}
	if err := validatePermissions(fixture.Permissions); err != nil {
		return err
	}
	if err := validateFixtureFields(fixture); err != nil {
		return err
	}
	if err := validateCookies(fixture.Cookies); err != nil {
		return err
	}
	if err := validateStateCardinality(fixture); err != nil {
		return err
	}
	if stateSize(fixture) > MaxStateBytes {
		return fmt.Errorf("state payload exceeds %d bytes", MaxStateBytes)
	}
	return nil
}

func validatePermissions(permissions []string) error {
	seenPermissions := make(map[string]struct{}, len(permissions))
	for _, permission := range permissions {
		if _, ok := supportedPermissions[permission]; !ok {
			return errors.New("permissions contains an unsupported capability")
		}
		if _, duplicate := seenPermissions[permission]; duplicate {
			return errors.New("permissions contains a duplicate capability")
		}
		seenPermissions[permission] = struct{}{}
	}
	return nil
}

func validateFixtureFields(fixture WireQAFixture) error {
	if fixture.Network.Profile != "" {
		if _, ok := supportedNetworkProfiles[fixture.Network.Profile]; !ok {
			return errors.New("network.profile is unsupported")
		}
	}
	if fixture.UserState != "" && fixture.UserState != "fresh" && fixture.UserState != "returning" {
		return errors.New("user_state must be fresh or returning")
	}
	if len(fixture.AuthRole) > 64 {
		return errors.New("auth_role exceeds 64 characters")
	}
	if fixture.SetupTimeoutMs < 100 || fixture.SetupTimeoutMs > MaxSetupTimeoutMs {
		return fmt.Errorf("setup_timeout_ms must be between 100 and %d", MaxSetupTimeoutMs)
	}
	return nil
}

func validateCookies(cookies []WireQACookie) error {
	if len(cookies) > maxStateEntries {
		return errors.New("cookies exceeds 100 entries")
	}
	for _, cookie := range cookies {
		if !cookieNamePattern.MatchString(cookie.Name) {
			return errors.New("cookies contains an invalid cookie name")
		}
		if !validSameSite(cookie.SameSite) {
			return errors.New("cookies contains an unsupported same_site value")
		}
	}
	return nil
}

func validSameSite(value string) bool {
	return value == "" || value == "strict" || value == "lax" || value == "none"
}

func validateStateCardinality(fixture WireQAFixture) error {
	counts := []struct {
		name  string
		count int
	}{
		{"local_storage", len(fixture.LocalStorage)},
		{"session_storage", len(fixture.SessionStorage)},
		{"feature_flags", len(fixture.FeatureFlags)},
		{"seed_data", len(fixture.SeedData)},
	}
	for _, state := range counts {
		if state.count > maxStateEntries {
			return fmt.Errorf("%s exceeds %d entries", state.name, maxStateEntries)
		}
	}
	return nil
}

func validateTarget(target WireQATarget) error {
	if target.URL == "" {
		return nil
	}
	parsed, err := url.Parse(target.URL)
	if err != nil || parsed.Host == "" || (parsed.Scheme != "http" && parsed.Scheme != "https") {
		return errors.New("target.url must be an absolute HTTP or HTTPS URL")
	}
	return nil
}

func validateViewport(viewport WireQAViewport) error {
	if viewport.Width == 0 && viewport.Height == 0 {
		return nil
	}
	if viewport.Width < 320 || viewport.Width > 7680 {
		return errors.New("viewport.width must be between 320 and 7680")
	}
	if viewport.Height < 240 || viewport.Height > 4320 {
		return errors.New("viewport.height must be between 240 and 4320")
	}
	return nil
}

func stateSize(fixture WireQAFixture) int {
	data, err := json.Marshal(struct {
		Cookies        []WireQACookie             `json:"cookies"`
		LocalStorage   map[string]string          `json:"local_storage"`
		SessionStorage map[string]string          `json:"session_storage"`
		FeatureFlags   map[string]bool            `json:"feature_flags"`
		SeedData       map[string]json.RawMessage `json:"seed_data"`
	}{fixture.Cookies, fixture.LocalStorage, fixture.SessionStorage, fixture.FeatureFlags, fixture.SeedData})
	if err != nil {
		return MaxStateBytes + 1
	}
	return len(data)
}
