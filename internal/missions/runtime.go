package missions

import (
	"time"

	"github.com/litescript/ls-horizons/internal/dsn"
)

// BuildSpotlightState computes the current spotlight state for a spacecraft.
// Returns nil if the spacecraft has no matching mission profile.
func BuildSpotlightState(now time.Time, sc *dsn.Spacecraft) *SpotlightState {
	if sc == nil {
		return nil
	}

	profile := ResolveProfile(sc.Name)
	if profile == nil {
		return nil
	}

	st := &SpotlightState{
		Profile:     profile,
		IsPreLaunch: now.Before(profile.StartTime),
		IsComplete:  now.After(profile.EndTime),
	}

	// Compute current phase
	for _, phase := range profile.Phases {
		if !now.Before(phase.Start) && now.Before(phase.End) {
			st.CurrentPhase = phase.Name
			break
		}
	}
	if st.CurrentPhase == "" {
		if st.IsPreLaunch {
			st.CurrentPhase = "Pre-Launch"
		} else {
			st.CurrentPhase = "Mission Complete"
		}
	}

	// MET (relative to first event / launch)
	if len(profile.Events) > 0 && !st.IsPreLaunch {
		st.MET = now.Sub(profile.Events[0].Time)
	}

	// Find next event and countdown
	for i := range profile.Events {
		if now.Before(profile.Events[i].Time) {
			st.NextEvent = &profile.Events[i]
			st.Countdown = profile.Events[i].Time.Sub(now)
			break
		}
	}

	// Build timeline items
	st.Timeline = buildTimeline(now, profile)

	return st
}

func buildTimeline(now time.Time, profile *MissionProfile) []TimelineItem {
	items := make([]TimelineItem, len(profile.Events))
	for i, evt := range profile.Events {
		status := TimelineFuture
		if !now.Before(evt.Time) {
			status = TimelinePast
		}
		items[i] = TimelineItem{
			Key:    evt.Key,
			Label:  evt.Name,
			Time:   evt.Time,
			Status: status,
		}
	}

	// Mark the most recent past event as "current" (the active milestone)
	for i := len(items) - 1; i >= 0; i-- {
		if items[i].Status == TimelinePast {
			// Only mark current if the mission isn't fully complete
			if !now.After(profile.EndTime) {
				items[i].Status = TimelineCurrent
			}
			break
		}
	}

	return items
}
