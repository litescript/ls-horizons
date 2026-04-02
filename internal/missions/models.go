package missions

import "time"

// MissionAccent defines the color theme for a mission spotlight.
type MissionAccent struct {
	Primary   string // Main accent color (lipgloss color string)
	Secondary string // Secondary accent
	Dim       string // Dimmed/muted variant
}

// MissionEvent represents a discrete event in a mission timeline.
type MissionEvent struct {
	Name string
	Time time.Time
	Key  string // Short key for timeline rail (e.g., "LAUNCH", "TLI")
}

// MissionPhase represents a named phase spanning a time range.
type MissionPhase struct {
	Name  string
	Start time.Time
	End   time.Time
}

// MissionProfile is the static definition of a curated mission.
type MissionProfile struct {
	ID          string
	DisplayName string
	Subtitle    string
	HeroText    string
	Aliases     []string // Name/code match keys (e.g., "EM2", "ORION")
	Crewed      bool
	PrimaryBody string // E.g., "Moon"
	Accent      MissionAccent
	StartTime   time.Time
	EndTime     time.Time
	Events      []MissionEvent
	Phases      []MissionPhase
}

// SpotlightState is the computed runtime state for rendering a mission spotlight.
type SpotlightState struct {
	Profile      *MissionProfile
	CurrentPhase string
	NextEvent    *MissionEvent
	Countdown    time.Duration // Time until next event (negative if past)
	MET          time.Duration // Mission elapsed time (from launch)
	Timeline     []TimelineItem
	IsPreLaunch  bool
	IsComplete   bool
}

// TimelineItem is a compact entry for the timeline rail.
type TimelineItem struct {
	Key    string
	Label  string
	Time   time.Time
	Status TimelineStatus
}

// TimelineStatus indicates where an event stands relative to now.
type TimelineStatus int

const (
	TimelinePast    TimelineStatus = iota
	TimelineCurrent
	TimelineFuture
)
