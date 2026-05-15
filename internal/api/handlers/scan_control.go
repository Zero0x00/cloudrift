package handlers

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/sts"
	"github.com/neo4j/neo4j-go-driver/v5/neo4j"

	"github.com/Zero0x00/cloudrift/internal/alerting"
	cloudriftaws "github.com/Zero0x00/cloudrift/internal/aws"
	"github.com/Zero0x00/cloudrift/internal/api/schema"
	"github.com/Zero0x00/cloudrift/internal/config"
	"github.com/Zero0x00/cloudrift/internal/graph"
	"github.com/Zero0x00/cloudrift/internal/models"
	"github.com/Zero0x00/cloudrift/internal/scanrun"
	"github.com/Zero0x00/cloudrift/internal/scans"
)

const scanStartVersion = "0.1.0"
const runHistoryLimit = 10

type scanControlCenter struct {
	outputDir  string
	configPath string
	alertSvc   *alerting.Service

	mu      sync.RWMutex
	current schema.ScanRunStatusResponse
	history []schema.ScanRunHistoryItem
}

func NewScanControlCenter(outputDir, configPath string) *scanControlCenter {
	return &scanControlCenter{
		outputDir:  outputDir,
		configPath: configPath,
		current: schema.ScanRunStatusResponse{
			Status:  "idle",
			Stage:   "idle",
			Message: "no scan is running",
		},
		history: make([]schema.ScanRunHistoryItem, 0, runHistoryLimit),
	}
}

func (s *scanControlCenter) SetAlertService(svc *alerting.Service) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.alertSvc = svc
}

func (s *scanControlCenter) RuntimeStatus() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		cfg, err := config.Load(s.configPath)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "config_load_error", "failed to load runtime config", nil)
			return
		}
		profiles := discoverAWSProfiles()
		resp := schema.RuntimeStatusResponse{
			AWSProfiles:      profiles,
			DefaultProfile:   defaultProfile(cfg),
			OpenAIConfigured: strings.TrimSpace(os.Getenv(strings.TrimSpace(cfg.Embeddings.OpenaiAPIKeyEnv))) != "",
			Neo4jConfigured:  neo4jConfigured(cfg),
			SlackConfigured:  strings.TrimSpace(os.Getenv("CLOUDRIFT_SLACK_WEBHOOK_URL")) != "",
			EmailConfigured:  strings.TrimSpace(os.Getenv("CLOUDRIFT_ALERT_EMAIL_TO")) != "",
		}
		writeJSON(w, http.StatusOK, resp)
	}
}

func (s *scanControlCenter) ValidateProfile() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		cfg, err := config.Load(s.configPath)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "config_load_error", "failed to load runtime config", nil)
			return
		}
		var req schema.ValidateProfileRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid_json", "invalid request JSON", nil)
			return
		}
		profile := strings.TrimSpace(req.Profile)
		if profile == "" {
			profile = defaultProfile(cfg)
		}
		resp, status := validateCredentialSource(r.Context(), profile)
		writeJSON(w, status, resp)
	}
}

func (s *scanControlCenter) StartScan() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		cfg, err := config.Load(s.configPath)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "config_load_error", "failed to load runtime config", nil)
			return
		}
		var req schema.ScanStartRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid_json", "invalid request JSON", nil)
			return
		}
		req.Profile = strings.TrimSpace(req.Profile)
		req.Module = strings.TrimSpace(req.Module)
		req.Provider = strings.TrimSpace(req.Provider)
		if req.Profile == "" {
			req.Profile = defaultProfile(cfg)
		}
		if req.Provider != "" && req.Provider != "openai" && req.Provider != "local" {
			writeError(w, http.StatusBadRequest, "invalid_provider", "provider must be openai|local when set", nil)
			return
		}
		if req.Module == "" {
			req.Module = "all"
		}
		if req.Module != "all" && req.Module != "orphaned_edge" && req.Module != "external_access" {
			writeError(w, http.StatusBadRequest, "invalid_module", "module must be all|orphaned_edge|external_access", nil)
			return
		}

		s.mu.Lock()
		if s.current.Status == "running" {
			s.mu.Unlock()
			writeError(w, http.StatusConflict, "scan_running", "a scan is already running", nil)
			return
		}
		runID := fmt.Sprintf("run-%d", time.Now().UTC().UnixNano())
		now := time.Now().UTC()
		s.current = schema.ScanRunStatusResponse{
			RunID:         runID,
			Status:        "running",
			Stage:         "starting",
			Message:       "validating profile and preparing scan",
			Profile:       req.Profile,
			Module:        req.Module,
			NoHTTP:        req.NoHTTP,
			Neo4j:         req.Neo4j,
			Provider:      req.Provider,
			StartedAt:     now,
			LastUpdatedAt: now,
		}
		s.mu.Unlock()

		go s.runScanAsync(req, cfg, runID)

		writeJSON(w, http.StatusAccepted, schema.ScanStartResponse{
			RunID:   runID,
			Status:  "running",
			Message: "scan started",
		})
	}
}

func (s *scanControlCenter) CurrentRunStatus() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		s.mu.RLock()
		current := s.current
		s.mu.RUnlock()
		writeJSON(w, http.StatusOK, current)
	}
}

func (s *scanControlCenter) RunHistory() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		s.mu.RLock()
		// Use a non-nil empty slice so JSON is [] not null (matches API shape-stability docs).
		items := append([]schema.ScanRunHistoryItem{}, s.history...)
		s.mu.RUnlock()
		writeJSON(w, http.StatusOK, schema.ScanRunHistoryResponse{Items: items})
	}
}

func (s *scanControlCenter) CurrentProgressEvent() schema.ScanProgressEvent {
	s.mu.RLock()
	current := s.current
	s.mu.RUnlock()
	return schema.ScanProgressEvent{
		EventType:         "progress",
		ScanID:            current.ScanID,
		Stage:             current.Stage,
		Message:           current.Message,
		CompletedAccounts: 0,
		TotalAccounts:     0,
		Timestamp:         time.Now().UTC(),
	}
}

func (s *scanControlCenter) runScanAsync(req schema.ScanStartRequest, cfg *config.Config, runID string) {
	s.updateRun(runID, "running", "validating_profile", "validating AWS profile")
	if resp, _ := validateCredentialSource(context.Background(), req.Profile); !resp.OK {
		s.failRun(runID, fmt.Sprintf("profile validation failed: %s", resp.Message))
		return
	}
	s.updateRun(runID, "running", "scanning", "running scan")

	scanID, err := scanrun.Run(context.Background(), s.outputDir, scanStartVersion)
	if err != nil {
		s.failRun(runID, "scan failed")
		return
	}
	s.updateRunWithScanID(runID, "running", "scan_complete", "scan artifacts created", scanID)

	if req.Neo4j {
		s.updateRun(runID, "running", "neo4j_export", "exporting scan to Neo4j")
		if err := exportScanToNeo4j(context.Background(), cfg, filepath.Join(s.outputDir, scanID)); err != nil {
			s.failRun(runID, "scan completed, but Neo4j export failed")
			return
		}
	}

	s.mu.RLock()
	alertSvc := s.alertSvc
	s.mu.RUnlock()
	if alertSvc != nil {
		// Alerting failure should not fail scan completion path.
		_, _ = alertSvc.EvaluateEnabledRulesForScan(scanID)
	}

	s.finishRun(runID, scanID, "scan completed")
}

func (s *scanControlCenter) updateRun(runID, status, stage, message string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.current.RunID != runID {
		return
	}
	s.current.Status = status
	s.current.Stage = stage
	s.current.Message = message
	s.current.LastUpdatedAt = time.Now().UTC()
}

func (s *scanControlCenter) updateRunWithScanID(runID, status, stage, message, scanID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.current.RunID != runID {
		return
	}
	s.current.Status = status
	s.current.Stage = stage
	s.current.Message = message
	s.current.ScanID = scanID
	s.current.LastUpdatedAt = time.Now().UTC()
}

func (s *scanControlCenter) failRun(runID, message string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.current.RunID != runID {
		return
	}
	now := time.Now().UTC()
	s.current.Status = "failed"
	s.current.Stage = "failed"
	s.current.Message = message
	s.current.FinishedAt = now
	s.current.LastUpdatedAt = now
	s.appendHistoryLocked(schema.ScanRunHistoryItem{
		RunID:      s.current.RunID,
		StartedAt:  s.current.StartedAt,
		FinishedAt: s.current.FinishedAt,
		Status:     s.current.Status,
		Profile:    s.current.Profile,
		Module:     s.current.Module,
		NoHTTP:     s.current.NoHTTP,
		Neo4j:      s.current.Neo4j,
		Message:    s.current.Message,
	})
}

func (s *scanControlCenter) finishRun(runID, scanID, message string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.current.RunID != runID {
		return
	}
	now := time.Now().UTC()
	s.current.Status = "completed"
	s.current.Stage = "completed"
	s.current.Message = message
	s.current.ScanID = scanID
	s.current.FinishedAt = now
	s.current.LastUpdatedAt = now
	s.appendHistoryLocked(schema.ScanRunHistoryItem{
		RunID:      s.current.RunID,
		StartedAt:  s.current.StartedAt,
		FinishedAt: s.current.FinishedAt,
		Status:     s.current.Status,
		Profile:    s.current.Profile,
		Module:     s.current.Module,
		NoHTTP:     s.current.NoHTTP,
		Neo4j:      s.current.Neo4j,
		Message:    s.current.Message,
	})
}

func (s *scanControlCenter) appendHistoryLocked(item schema.ScanRunHistoryItem) {
	// Newest first for operator UX; bounded ring behavior via truncation.
	s.history = append([]schema.ScanRunHistoryItem{item}, s.history...)
	if len(s.history) > runHistoryLimit {
		s.history = s.history[:runHistoryLimit]
	}
}

func validateCredentialSource(ctx context.Context, profile string) (schema.ValidateProfileResponse, int) {
	profile = strings.TrimSpace(profile)
	loadOpts := []func(*awsconfig.LoadOptions) error{}
	if profile != "" {
		loadOpts = append(loadOpts, awsconfig.WithSharedConfigProfile(profile))
	}
	cfg, err := awsconfig.LoadDefaultConfig(ctx, loadOpts...)
	if err != nil {
		resp := schema.ValidateProfileResponse{OK: false, Profile: profile, Message: "could not resolve credentials/config from selected source"}
		if cloudriftaws.IsSSOExpiredError(err) {
			resp.Message = ssoExpiredMessage(profile)
			resp.SSOLoginRequired = true
			resp.SSOCommand = cloudriftaws.SSOLoginCommand(profile)
			return resp, http.StatusOK
		}
		return resp, http.StatusBadRequest
	}
	cctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	_, err = sts.NewFromConfig(cfg).GetCallerIdentity(cctx, &sts.GetCallerIdentityInput{})
	if err != nil {
		resp := schema.ValidateProfileResponse{OK: false, Profile: profile, Message: "credential source resolved, but caller identity check failed"}
		if cloudriftaws.IsSSOExpiredError(err) {
			resp.Message = ssoExpiredMessage(profile)
			resp.SSOLoginRequired = true
			resp.SSOCommand = cloudriftaws.SSOLoginCommand(profile)
			return resp, http.StatusOK
		}
		return resp, http.StatusBadRequest
	}
	msg := "profile is valid and credentials are usable"
	if profile == "" {
		msg = "ambient credential source is valid (env/role/shared config)"
	}
	return schema.ValidateProfileResponse{OK: true, Profile: profile, Message: msg}, http.StatusOK
}

func ssoExpiredMessage(profile string) string {
	if profile == "" {
		return "AWS SSO session expired"
	}
	return fmt.Sprintf("AWS SSO session expired for profile %q", profile)
}

func (s *scanControlCenter) SSOLogin() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req schema.SSOLoginRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid_json", "invalid request JSON", nil)
			return
		}
		profile := strings.TrimSpace(req.Profile)
		ssoCmd := cloudriftaws.SSOLoginCommand(profile)
		started, err := cloudriftaws.TriggerSSOLogin(profile)
		if !started {
			writeJSON(w, http.StatusOK, schema.SSOLoginResponse{
				Started: false,
				Message: "aws CLI not found; run manually: " + ssoCmd,
				Command: ssoCmd,
			})
			return
		}
		if err != nil {
			writeJSON(w, http.StatusOK, schema.SSOLoginResponse{
				Started: false,
				Message: "failed to start aws sso login; run manually: " + ssoCmd,
				Command: ssoCmd,
			})
			return
		}
		writeJSON(w, http.StatusOK, schema.SSOLoginResponse{
			Started: true,
			Message: "browser opened for SSO authentication; validate the profile once login completes",
			Command: ssoCmd,
		})
	}
}

func defaultProfile(cfg *config.Config) string {
	if fromEnv := strings.TrimSpace(os.Getenv("AWS_PROFILE")); fromEnv != "" {
		return fromEnv
	}
	return strings.TrimSpace(cfg.AWS.ManagementProfile)
}

func neo4jConfigured(cfg *config.Config) bool {
	envName := strings.TrimSpace(cfg.Neo4j.PasswordEnv)
	if strings.TrimSpace(cfg.Neo4j.URI) == "" || strings.TrimSpace(cfg.Neo4j.Username) == "" || envName == "" {
		return false
	}
	return strings.TrimSpace(os.Getenv(envName)) != ""
}

func discoverAWSProfiles() []string {
	paths := awsProfileFilePaths()

	seen := map[string]struct{}{}
	for _, p := range paths {
		f, err := os.Open(p)
		if err != nil {
			continue
		}
		scanner := bufio.NewScanner(f)
		for scanner.Scan() {
			ln := scanner.Text()
			ln = strings.TrimSpace(ln)
			if !strings.HasPrefix(ln, "[") || !strings.HasSuffix(ln, "]") {
				continue
			}
			section := strings.TrimSuffix(strings.TrimPrefix(ln, "["), "]")
			section = strings.TrimSpace(section)
			section = strings.TrimPrefix(section, "profile ")
			section = strings.TrimSpace(section)
			if section == "" {
				continue
			}
			seen[section] = struct{}{}
		}
		_ = f.Close()
	}
	out := make([]string, 0, len(seen))
	for k := range seen {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

func awsProfileFilePaths() []string {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil
	}
	base := filepath.Join(home, ".aws")
	paths := []string{
		filepath.Join(base, "config"),
		filepath.Join(base, "credentials"),
	}
	if cfg, ok := safeAWSConfigPathFromEnv("AWS_CONFIG_FILE", base); ok {
		paths[0] = cfg
	}
	if creds, ok := safeAWSConfigPathFromEnv("AWS_SHARED_CREDENTIALS_FILE", base); ok {
		paths[1] = creds
	}
	return paths
}

func safeAWSConfigPathFromEnv(envName, allowedDir string) (string, bool) {
	raw := strings.TrimSpace(os.Getenv(envName))
	if raw == "" {
		return "", false
	}
	clean := filepath.Clean(raw)
	if clean == "" {
		return "", false
	}
	if !filepath.IsAbs(clean) {
		return "", false
	}
	rel, err := filepath.Rel(allowedDir, clean)
	if err != nil || rel == "." || strings.HasPrefix(rel, "..") {
		return "", false
	}
	return clean, true
}

func exportScanToNeo4j(ctx context.Context, cfg *config.Config, scanPath string) error {
	uri := strings.TrimSpace(cfg.Neo4j.URI)
	user := strings.TrimSpace(cfg.Neo4j.Username)
	passEnv := strings.TrimSpace(cfg.Neo4j.PasswordEnv)
	if uri == "" || user == "" || passEnv == "" {
		return errors.New("neo4j configuration is incomplete")
	}
	pass := strings.TrimSpace(os.Getenv(passEnv))
	if pass == "" {
		return fmt.Errorf("neo4j password env %s is empty", passEnv)
	}
	driver, err := neo4j.NewDriverWithContext(uri, neo4j.BasicAuth(user, pass, ""))
	if err != nil {
		return err
	}
	defer driver.Close(ctx)
	ex := graph.NewDriverExecer(driver, "")
	meta, findings, err := scans.LoadScanArtifacts(filepath.Dir(scanPath), filepath.Base(scanPath))
	if err != nil {
		return err
	}
	if meta == nil {
		return errors.New("scan metadata missing")
	}
	for _, ddl := range graph.SchemaStatements() {
		if err := ex.Run(ctx, ddl, nil); err != nil {
			return err
		}
	}
	return graph.WriteScan(ctx, ex, *meta, []models.AssetNode{}, []models.Relationship{}, findings)
}
