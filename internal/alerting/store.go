package alerting

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"
)

const (
	alertingDirName  = "_alerting"
	rulesFileName    = "rules.json"
	eventsFileName   = "events.json"
	routingFileName  = "routing.json"
	maxEventHistory  = 500
	routingInitialJSON = `{
  "teams": [],
  "account_teams": []
}
`
)

type Store struct {
	outputDir string
	mu        sync.Mutex
}

func NewStore(outputDir string) *Store {
	return &Store{outputDir: outputDir}
}

func (s *Store) ListRules() ([]AlertRule, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	rules, err := s.loadRulesLocked()
	if err != nil {
		return nil, err
	}
	sort.Slice(rules, func(i, j int) bool {
		if rules[i].CreatedAt.Equal(rules[j].CreatedAt) {
			return rules[i].ID < rules[j].ID
		}
		return rules[i].CreatedAt.Before(rules[j].CreatedAt)
	})
	return rules, nil
}

func (s *Store) GetRule(id string) (*AlertRule, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	rules, err := s.loadRulesLocked()
	if err != nil {
		return nil, err
	}
	for _, rule := range rules {
		if rule.ID == id {
			cp := rule
			return &cp, nil
		}
	}
	return nil, os.ErrNotExist
}

func (s *Store) UpsertRule(rule AlertRule) (AlertRule, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now().UTC()
	rules, err := s.loadRulesLocked()
	if err != nil {
		return AlertRule{}, err
	}

	updated := false
	for i := range rules {
		if rules[i].ID != rule.ID {
			continue
		}
		rule.CreatedAt = rules[i].CreatedAt
		rule.UpdatedAt = now
		if rule.CreatedAt.IsZero() {
			rule.CreatedAt = now
		}
		rules[i] = rule
		updated = true
		break
	}
	if !updated {
		if rule.CreatedAt.IsZero() {
			rule.CreatedAt = now
		}
		rule.UpdatedAt = now
		rules = append(rules, rule)
	}
	if err := s.writeRulesLocked(rules); err != nil {
		return AlertRule{}, err
	}
	return rule, nil
}

func (s *Store) RecordEvent(event AlertEvent) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	events, err := s.loadEventsLocked()
	if err != nil {
		return err
	}
	events = append([]AlertEvent{event}, events...)
	if len(events) > maxEventHistory {
		events = events[:maxEventHistory]
	}
	return s.writeEventsLocked(events)
}

func (s *Store) LoadRoutingCatalog() (RoutingCatalog, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	path, err := s.ensureFileLocked(routingFileName, []byte(routingInitialJSON))
	if err != nil {
		return RoutingCatalog{}, err
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return RoutingCatalog{}, err
	}
	raw = bytes.TrimSpace(raw)
	var c RoutingCatalog
	if len(raw) == 0 {
		return RoutingCatalog{}, nil
	}
	if err := json.Unmarshal(raw, &c); err != nil {
		return RoutingCatalog{}, fmt.Errorf("routing catalog unmarshal: %w", err)
	}
	return c, nil
}

func (s *Store) SaveRoutingCatalog(c RoutingCatalog) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	raw, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	path, err := s.ensureFileLocked(routingFileName, []byte(routingInitialJSON))
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(raw, '\n'), 0o600)
}

func (s *Store) ListEvents(limit int) ([]AlertEvent, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	events, err := s.loadEventsLocked()
	if err != nil {
		return nil, err
	}
	if limit > 0 && len(events) > limit {
		events = events[:limit]
	}
	return events, nil
}

func (s *Store) loadRulesLocked() ([]AlertRule, error) {
	path, err := s.ensureFileLocked(rulesFileName, []byte("[]\n"))
	if err != nil {
		return nil, err
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var rules []AlertRule
	if len(raw) == 0 {
		return []AlertRule{}, nil
	}
	if err := json.Unmarshal(raw, &rules); err != nil {
		return nil, fmt.Errorf("alert rules unmarshal: %w", err)
	}
	if rules == nil {
		return []AlertRule{}, nil
	}
	return rules, nil
}

func (s *Store) writeRulesLocked(rules []AlertRule) error {
	if rules == nil {
		rules = []AlertRule{}
	}
	raw, err := json.MarshalIndent(rules, "", "  ")
	if err != nil {
		return err
	}
	path, err := s.ensureFileLocked(rulesFileName, []byte("[]\n"))
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(raw, '\n'), 0o600)
}

func (s *Store) loadEventsLocked() ([]AlertEvent, error) {
	path, err := s.ensureFileLocked(eventsFileName, []byte("[]\n"))
	if err != nil {
		return nil, err
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var events []AlertEvent
	if len(raw) == 0 {
		return []AlertEvent{}, nil
	}
	if err := json.Unmarshal(raw, &events); err != nil {
		return nil, fmt.Errorf("alert events unmarshal: %w", err)
	}
	if events == nil {
		return []AlertEvent{}, nil
	}
	return events, nil
}

func (s *Store) writeEventsLocked(events []AlertEvent) error {
	if events == nil {
		events = []AlertEvent{}
	}
	raw, err := json.MarshalIndent(events, "", "  ")
	if err != nil {
		return err
	}
	path, err := s.ensureFileLocked(eventsFileName, []byte("[]\n"))
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(raw, '\n'), 0o600)
}

func (s *Store) ensureFileLocked(name string, initial []byte) (string, error) {
	if s.outputDir == "" {
		return "", errors.New("alerting store output dir is empty")
	}
	dir := filepath.Join(s.outputDir, alertingDirName)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	path := filepath.Join(dir, name)
	if _, err := os.Stat(path); err == nil {
		return path, nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return "", err
	}
	if err := os.WriteFile(path, initial, 0o600); err != nil {
		return "", err
	}
	return path, nil
}
