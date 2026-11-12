package agent

import (
	"sync"
	"time"
)

// State is what an agent session is doing.
type State int

const (
	// StateIdle means a turn ended and the agent is ready.
	StateIdle State = iota
	// StateWorking means the agent is running a turn.
	StateWorking
	// StateWaiting means the agent needs the user.
	StateWaiting
	// StateEnded means the session is over.
	StateEnded
)

// Session is what is known about one terminal's agent.
type Session struct {
	Agent     string
	SessionID string
	State     State
	Detail    string
	// Attention is a message the user has not seen.
	Attention string
	// FinishedUnseen is set when a turn ended while the user was elsewhere.
	FinishedUnseen bool
	StartedAt      time.Time
	UpdatedAt      time.Time
	// IndicatorSince is when the session took the indicator it shows, so
	// alerts can be ordered. Zero while it shows none.
	IndicatorSince time.Time
}

// IsUnread reports a finished turn or a message the user has not seen.
func (s Session) IsUnread() bool { return s.FinishedUnseen || s.Attention != "" }

// Update is one applied event, with the terminal's session before and after.
type Update struct {
	Surface  string
	Kind     Kind
	Previous *Session
	Session  Session
}

// Alerts reports whether the event is one the user should be told about.
func (u Update) Alerts() bool {
	switch u.Kind {
	case KindNeedsInput, KindTurnEnded, KindAttention:
		return true
	default:
		return false
	}
}

// RepeatsWait reports a wait for the same reason the terminal was already
// waiting for, which the user has already been told about.
func (u Update) RepeatsWait() bool {
	return u.Kind == KindNeedsInput && u.Previous != nil &&
		u.Previous.State == StateWaiting && u.Previous.Detail == u.Session.Detail
}

// Indicator is the one mark shown for a group of terminals, ordered by how
// urgently they need the user.
type Indicator int

const (
	// IndicatorDone means a turn finished unseen.
	IndicatorDone Indicator = iota
	// IndicatorWorking means the agent is running.
	IndicatorWorking
	// IndicatorWaiting means the agent needs the user.
	IndicatorWaiting
)

// Status is an indicator and when it was taken.
type Status struct {
	Indicator Indicator
	Since     time.Time
}

// IndicatorFor returns the mark a session shows, or none.
func IndicatorFor(s Session) (Indicator, bool) {
	switch s.State {
	case StateWaiting:
		return IndicatorWaiting, true
	case StateWorking:
		return IndicatorWorking, true
	case StateIdle:
		if s.FinishedUnseen {
			return IndicatorDone, true
		}
	}
	return 0, false
}

// Store tracks each terminal's agent, built from the events cde sends.
// Indicator observers are notified only when the visible state changes. All
// state is guarded by mu; observers are called after it is released, since
// they may read the store again.
type Store struct {
	mu       sync.RWMutex
	sessions map[string]Session
	// IsLive tells whether a terminal with that id is open; events for any
	// other id are dropped.
	IsLive func(id string) bool
	// IsVisible tells whether the user is looking at a terminal, in which
	// case a finished turn is already seen.
	IsVisible func(id string) bool
	// OnChange runs with the terminal id when its visible indicator changes.
	OnChange func(id string)
	// OnUpdate runs for every applied event.
	OnUpdate func(u Update)
	// OnUnreadChange runs when a terminal gains or loses an unread alert.
	OnUnreadChange func()
}

// NewStore makes an empty store.
func NewStore() *Store {
	return &Store{
		sessions:  map[string]Session{},
		IsLive:    func(string) bool { return true },
		IsVisible: func(string) bool { return false },
	}
}

// hooks copies the observer functions, which are set once before events
// arrive, so they can be called after the state lock is released.
func (s *Store) hooks() (isLive, isVisible func(string) bool, onChange func(string), onUpdate func(Update), onUnread func()) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.IsLive, s.IsVisible, s.OnChange, s.OnUpdate, s.OnUnreadChange
}

// Sessions returns a copy of the stored sessions by terminal id.
func (s *Store) Sessions() map[string]Session {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make(map[string]Session, len(s.sessions))
	for id, session := range s.sessions {
		out[id] = session
	}
	return out
}

// Apply folds an event into the store.
func (s *Store) Apply(event Event) {
	if event.Kind == KindUnknown {
		return
	}
	isLive, isVisible, onChange, onUpdate, onUnread := s.hooks()
	if event.SurfaceID == "" || !isLive(event.SurfaceID) {
		return
	}
	surface := event.SurfaceID
	visible := isVisible(surface)

	s.mu.Lock()
	previous, had := s.sessions[surface]

	// Resuming or compacting announces the running session again, and
	// startup hooks can land after its turn has begun.
	restatesLive := event.Kind == KindSessionStarted && had &&
		previous.SessionID == event.SessionID && previous.State != StateEnded
	// Each hook reaches the socket in its own process, so a later hook can
	// overtake an earlier one.
	if had && event.Kind != KindSessionStarted && event.At.Before(previous.UpdatedAt) {
		s.mu.Unlock()
		return
	}

	var session Session
	if had && !((event.Kind == KindSessionStarted && !restatesLive) || previous.State == StateEnded) {
		session = previous
	} else {
		session = Session{Agent: event.Agent, SessionID: event.SessionID, StartedAt: event.At, UpdatedAt: event.At}
	}
	session.Agent = event.Agent
	session.SessionID = event.SessionID
	if event.At.After(session.UpdatedAt) {
		session.UpdatedAt = event.At
	}
	if event.Kind != KindAttention && !restatesLive {
		session.Detail = event.Detail
	}

	switch event.Kind {
	case KindSessionStarted, KindUnknown:
	case KindTurnStarted, KindWorking:
		session.State = StateWorking
		session.FinishedUnseen = false
	case KindNeedsInput:
		session.State = StateWaiting
	case KindTurnEnded:
		session.State = StateIdle
		session.FinishedUnseen = !visible
	case KindSessionEnded:
		session.State = StateEnded
	case KindAttention:
		session.Attention = event.Detail
	}

	indicatorChanged := !sameIndicator(had, previous, session)
	if indicatorChanged {
		if _, ok := IndicatorFor(session); ok {
			session.IndicatorSince = event.At
		} else {
			session.IndicatorSince = time.Time{}
		}
	}

	s.sessions[surface] = session
	unreadChanged := (had && previous.IsUnread()) != session.IsUnread()
	var update Update
	if onUpdate != nil {
		update = Update{Surface: surface, Kind: event.Kind, Session: session}
		if had {
			p := previous
			update.Previous = &p
		}
	}
	s.mu.Unlock()

	if onUpdate != nil {
		onUpdate(update)
	}
	if unreadChanged && onUnread != nil {
		onUnread()
	}
	if indicatorChanged && onChange != nil {
		onChange(surface)
	}
}

func sameIndicator(had bool, previous, current Session) bool {
	var pi Indicator
	pok := false
	if had {
		pi, pok = IndicatorFor(previous)
	}
	ci, cok := IndicatorFor(current)
	return pok == cok && pi == ci
}

// Route reads a desktop notification a terminal's agent asked for. It reports
// false when the agent does not turn it into an event, in which case the app
// posts it as the terminal's own.
func (s *Store) Route(surface, message string, at time.Time) (Event, bool) {
	s.mu.RLock()
	session, ok := s.sessions[surface]
	s.mu.RUnlock()
	if !ok || session.State == StateEnded {
		return Event{}, false
	}
	adapter, ok := AdapterNamed(session.Agent)
	if !ok || adapter.TranslateNotification == nil {
		return Event{}, false
	}
	kind, detail, ok := adapter.TranslateNotification(message)
	if !ok {
		return Event{}, false
	}
	return Event{
		Version:   1,
		Agent:     session.Agent,
		Kind:      kind,
		SessionID: session.SessionID,
		SurfaceID: surface,
		At:        at,
		Detail:    detail,
	}, true
}

// MarkSeen clears a terminal's unread alerts.
func (s *Store) MarkSeen(surface string) {
	s.mu.Lock()
	session, ok := s.sessions[surface]
	if !ok || !session.IsUnread() {
		s.mu.Unlock()
		return
	}
	previousIndicator, previousOK := IndicatorFor(session)
	session.FinishedUnseen = false
	session.Attention = ""
	if _, ok := IndicatorFor(session); !ok {
		session.IndicatorSince = time.Time{}
	}
	s.sessions[surface] = session
	currentIndicator, currentOK := IndicatorFor(session)
	onChange, onUnread := s.OnChange, s.OnUnreadChange
	s.mu.Unlock()

	if previousOK != currentOK || previousIndicator != currentIndicator {
		if onChange != nil {
			onChange(surface)
		}
	}
	if onUnread != nil {
		onUnread()
	}
}

// UnreadCount counts terminals that finished or asked for attention unseen.
func (s *Store) UnreadCount(surfaces []string) int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	count := 0
	for _, surface := range surfaces {
		if session, ok := s.sessions[surface]; ok && session.IsUnread() {
			count++
		}
	}
	return count
}

// PendingCount counts terminals whose agent is waiting or finished unseen.
func (s *Store) PendingCount(surfaces []string) int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	count := 0
	for _, surface := range surfaces {
		session, ok := s.sessions[surface]
		if !ok || session.State == StateEnded {
			continue
		}
		if session.IsUnread() || session.State == StateWaiting {
			count++
		}
	}
	return count
}

// IndicatorForSurfaces returns the most urgent mark among the terminals.
func (s *Store) IndicatorForSurfaces(surfaces []string) (Indicator, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var best Indicator
	found := false
	for _, surface := range surfaces {
		session, ok := s.sessions[surface]
		if !ok {
			continue
		}
		indicator, ok := IndicatorFor(session)
		if !ok {
			continue
		}
		if !found || indicator > best {
			best, found = indicator, true
		}
	}
	return best, found
}

// StatusForSurfaces returns the most urgent status among the terminals,
// dated from when it was taken.
func (s *Store) StatusForSurfaces(surfaces []string) (Status, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var best Status
	found := false
	for _, surface := range surfaces {
		session, ok := s.sessions[surface]
		if !ok {
			continue
		}
		indicator, ok := IndicatorFor(session)
		if !ok {
			continue
		}
		since := session.IndicatorSince
		if since.IsZero() {
			since = session.UpdatedAt
		}
		if !found || indicator > best.Indicator || (indicator == best.Indicator && since.After(best.Since)) {
			best, found = Status{Indicator: indicator, Since: since}, true
		}
	}
	return best, found
}

// RunningAgent returns the agent's name in a terminal, until its session ends.
func (s *Store) RunningAgent(surface string) string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	session, ok := s.sessions[surface]
	if !ok || session.State == StateEnded || session.Agent == "" {
		return ""
	}
	return session.Agent
}

// MostPending returns the surface among these that most needs the user: one
// waiting for input first, then the most recently finished or attention.
func (s *Store) MostPending(surfaces []string) (string, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	best, found := "", false
	var bestIndicator Indicator
	var bestSince time.Time
	for _, id := range surfaces {
		session, ok := s.sessions[id]
		if !ok || session.State == StateEnded {
			continue
		}
		if !session.IsUnread() && session.State != StateWaiting {
			continue
		}
		indicator, ok := IndicatorFor(session)
		if !ok {
			continue
		}
		since := session.IndicatorSince
		if since.IsZero() {
			since = session.UpdatedAt
		}
		if !found || indicator > bestIndicator || (indicator == bestIndicator && since.After(bestSince)) {
			best, found, bestIndicator, bestSince = id, true, indicator, since
		}
	}
	return best, found
}

// Remove drops a terminal's session.
func (s *Store) Remove(surface string) {
	s.mu.Lock()
	session, ok := s.sessions[surface]
	wasUnread := ok && session.IsUnread()
	delete(s.sessions, surface)
	onUnread := s.OnUnreadChange
	s.mu.Unlock()
	if wasUnread && onUnread != nil {
		onUnread()
	}
}
