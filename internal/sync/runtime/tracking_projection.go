// tracking_projection.go — Single ordering rule for the daemon's tracked-tab projection.
// Purpose: Every observation of the tracked tab — a synchronous switch_tab result or a
// /sync heartbeat — enters the projection through observeTrackingLocked.
// Why: A heartbeat the extension snapshotted before a switch (tab A, perhaps carrying a
// late navigation URL) can arrive after the synchronous switch_tab result confirmed tab B.
// Applying it last-writer-wins would silently retarget commands back to A.
// Docs: docs/features/feature/tab-tracking-ux/index.md

package syncruntime

import "time"

// staleHeartbeatAllowance bounds how many heartbeats reporting a different tab are held
// after a command-confirmed switch. The extension persists the new tab before it returns
// the switch_tab result, so only heartbeats already snapshotted before that persist can
// disagree; past this bound the disagreement is a genuine later selection (user re-picked a
// tab, or B closed) and the extension — the owner of tab selection — wins.
const staleHeartbeatAllowance = 2

type trackingSource int

const (
	trackingSourceHeartbeat trackingSource = iota
	trackingSourceCommandConfirmed
)

// trackingObservation is one report of the tracked tab. Tab-scoped status fields are
// only carried by heartbeats; a command-confirmed observation leaves them untouched.
type trackingObservation struct {
	source        trackingSource
	enabled       bool
	tabID         int
	url           string
	title         string
	tabStatus     string
	active        *bool
	cspRestricted bool
	cspLevel      string
}

func heartbeatTrackingObservation(settings *SyncSettings) trackingObservation {
	return trackingObservation{
		source:        trackingSourceHeartbeat,
		enabled:       settings.TrackingEnabled,
		tabID:         settings.TrackedTabID,
		url:           settings.TrackedTabURL,
		title:         settings.TrackedTabTitle,
		tabStatus:     normalizedTabStatus(settings.TabStatus),
		active:        settings.TrackedTabActive,
		cspRestricted: settings.CspRestricted,
		cspLevel:      settings.CspLevel,
	}
}

// observeTrackingLocked is THE ordering rule (caller holds r.mu):
//
//  1. A command-confirmed observation (synchronous switch_tab result) applies at once and
//     opens a hold naming the confirmed tab.
//  2. While a hold is open, a heartbeat naming the held tab applies and closes the hold.
//  3. While a hold is open, a heartbeat naming another tab is stale: its tab-scoped fields
//     are dropped, up to staleHeartbeatAllowance times; after that it applies and closes
//     the hold (the extension owns selection).
//  4. With no hold open, heartbeats apply as reported (background convergence).
//
// Returns whether the observation was applied.
func (r *Runtime) observeTrackingLocked(obs trackingObservation, now time.Time) bool {
	s := &r.state
	if obs.source == trackingSourceCommandConfirmed {
		s.trackingHoldTabID = obs.tabID
		s.trackingHeldHeartbeats = 0
		r.applyTrackingIdentityLocked(obs, now)
		return true
	}
	if s.trackingHoldTabID != 0 && obs.tabID != s.trackingHoldTabID &&
		s.trackingHeldHeartbeats < staleHeartbeatAllowance {
		s.trackingHeldHeartbeats++
		return false
	}
	s.trackingHoldTabID = 0
	s.trackingHeldHeartbeats = 0
	changed := s.tabStatus != obs.tabStatus || !optionalBoolEqual(s.trackedTabActive, obs.active)
	s.tabStatus = obs.tabStatus
	s.trackedTabActive = obs.active
	s.cspRestricted = obs.cspRestricted
	s.cspLevel = obs.cspLevel
	if r.applyTrackingIdentityLocked(obs, now) || changed {
		r.signalTrackingChangeLocked()
	}
	return true
}

// applyTrackingIdentityLocked writes the identity fields and reports whether they changed.
// Command-confirmed changes signal waiters here; heartbeat callers signal once overall.
func (r *Runtime) applyTrackingIdentityLocked(obs trackingObservation, now time.Time) bool {
	s := &r.state
	changed := s.trackingEnabled != obs.enabled || s.trackedTabID != obs.tabID ||
		s.trackedTabURL != obs.url || s.trackedTabTitle != obs.title
	s.trackingEnabled = obs.enabled
	s.trackedTabID = obs.tabID
	s.trackedTabURL = obs.url
	s.trackedTabTitle = obs.title
	s.trackingUpdated = now
	if changed && obs.source == trackingSourceCommandConfirmed {
		r.signalTrackingChangeLocked()
	}
	return changed
}
