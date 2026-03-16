package broker

import (
	"fmt"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/fsnotify/fsnotify"
	"go.uber.org/zap"
)

const defaultDebounceDuration = 500 * time.Millisecond

// liveState bundles all configuration-derived state that can be hot-reloaded.
// It is immutable after construction; a new instance is built on each reload.
type liveState struct {
	config        *Config
	configManager *ConfigManager
	idpVerifiers  []IdpAndJwtVerifier
	auditSubject  string
}

// ConfigWatcher watches configuration files for changes and atomically
// swaps the live state used by auth request handlers.
type ConfigWatcher struct {
	configFiles []string // original file patterns (may contain globs)
	state       atomic.Pointer[liveState]
	ctx         *Context
	debounce    time.Duration
	reloadMu    sync.Mutex // serializes reload operations
	stopCh      chan struct{}
	watcher     *fsnotify.Watcher
}

// NewConfigWatcher creates a ConfigWatcher with the given initial state.
func NewConfigWatcher(ctx *Context, configFiles []string, initial *liveState) *ConfigWatcher {
	cw := &ConfigWatcher{
		configFiles: configFiles,
		ctx:         ctx,
		debounce:    defaultDebounceDuration,
		stopCh:      make(chan struct{}),
	}
	cw.state.Store(initial)
	return cw
}

// State returns the current live state. This is the hot-path read used
// by every auth request and is lock-free.
func (cw *ConfigWatcher) State() *liveState {
	return cw.state.Load()
}

// Start begins watching configuration file directories for changes.
// It resolves glob patterns to concrete paths and watches their parent
// directories (to handle Kubernetes ConfigMap symlink rotations).
func (cw *ConfigWatcher) Start() error {
	paths, err := resolveGlobPaths(cw.configFiles)
	if err != nil {
		return fmt.Errorf("failed to resolve config file paths: %w", err)
	}

	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return fmt.Errorf("failed to create file watcher: %w", err)
	}
	cw.watcher = watcher

	// Watch parent directories (not files) to handle symlink rotations
	dirs := uniqueDirs(paths)
	for _, dir := range dirs {
		if err := watcher.Add(dir); err != nil {
			watcher.Close()
			return fmt.Errorf("failed to watch directory %s: %w", dir, err)
		}
		zap.L().Debug("watching directory for config changes", zap.String("dir", dir))
	}

	// Build a set of absolute config file paths for filtering events
	configPathSet := make(map[string]struct{}, len(paths))
	for _, p := range paths {
		abs, err := filepath.Abs(p)
		if err != nil {
			abs = p
		}
		configPathSet[abs] = struct{}{}
	}

	go cw.watchLoop(configPathSet)

	zap.L().Info("config file watcher started", zap.Int("files", len(paths)), zap.Int("directories", len(dirs)))
	return nil
}

// Stop signals the watcher goroutine to exit and closes the fsnotify watcher.
func (cw *ConfigWatcher) Stop() {
	close(cw.stopCh)
	if cw.watcher != nil {
		cw.watcher.Close()
	}
}

func (cw *ConfigWatcher) watchLoop(configPaths map[string]struct{}) {
	var debounceTimer *time.Timer
	var debounceCh <-chan time.Time

	for {
		select {
		case <-cw.stopCh:
			if debounceTimer != nil {
				debounceTimer.Stop()
			}
			return

		case event, ok := <-cw.watcher.Events:
			if !ok {
				return
			}

			if !isRelevantEvent(event, configPaths) {
				continue
			}

			zap.L().Debug("config file change detected",
				zap.String("file", event.Name),
				zap.String("op", event.Op.String()))

			// Reset debounce timer
			if debounceTimer != nil {
				debounceTimer.Stop()
			}
			debounceTimer = time.NewTimer(cw.debounce)
			debounceCh = debounceTimer.C

		case err, ok := <-cw.watcher.Errors:
			if !ok {
				return
			}
			zap.L().Error("file watcher error", zap.Error(err))

		case <-debounceCh:
			debounceCh = nil
			cw.doReload()
		}
	}
}

func (cw *ConfigWatcher) doReload() {
	cw.reloadMu.Lock()
	defer cw.reloadMu.Unlock()

	start := time.Now()
	if err := cw.reload(); err != nil {
		zap.L().Error("config reload failed, keeping previous configuration", zap.Error(err))
	} else {
		zap.L().Info("config reloaded successfully", zap.Duration("duration", time.Since(start)))
	}
}

func (cw *ConfigWatcher) reload() error {
	// Build new ConfigManager from the same file patterns
	newCM, err := NewConfigManager(cw.configFiles)
	if err != nil {
		return fmt.Errorf("failed to create config manager: %w", err)
	}

	// Validate the new config renders correctly
	newConfig, err := newCM.GetConfig(make(map[string]interface{}))
	if err != nil {
		return fmt.Errorf("failed to validate new config: %w", err)
	}

	// Check if NATS identity fields changed (requires restart)
	current := cw.state.Load()
	if current.config.Service.CredsFile != newConfig.Service.CredsFile {
		zap.L().Warn("service.creds_file changed; NATS connection identity requires restart to take effect",
			zap.String("old", current.config.Service.CredsFile),
			zap.String("new", newConfig.Service.CredsFile))
	}

	// Recreate IDP verifiers with the new config
	newVerifiers, err := NewIdpVerifiers(cw.ctx, newConfig)
	if err != nil {
		return fmt.Errorf("failed to create IDP verifiers: %w", err)
	}

	auditSubject := newConfig.Service.Name + ".evt.audit.account.%s.user.%s.created"

	newState := &liveState{
		config:        newConfig,
		configManager: newCM,
		idpVerifiers:  newVerifiers,
		auditSubject:  auditSubject,
	}

	cw.state.Store(newState)

	zap.L().Info("configuration swapped",
		zap.Int("idp_count", len(newConfig.Idp)),
		zap.Int("role_binding_count", len(newConfig.Rbac.RoleBinding)),
		zap.Int("role_count", len(newConfig.Rbac.Roles)))

	return nil
}

// resolveGlobPaths expands glob patterns to concrete file paths.
func resolveGlobPaths(patterns []string) ([]string, error) {
	var paths []string
	for _, pattern := range patterns {
		if strings.ContainsAny(pattern, "*?[]") {
			matches, err := filepath.Glob(pattern)
			if err != nil {
				return nil, fmt.Errorf("invalid glob pattern %q: %w", pattern, err)
			}
			if len(matches) == 0 {
				return nil, fmt.Errorf("glob pattern %q did not match any files", pattern)
			}
			paths = append(paths, matches...)
		} else {
			paths = append(paths, pattern)
		}
	}
	return paths, nil
}

// uniqueDirs returns deduplicated parent directories of the given paths.
func uniqueDirs(paths []string) []string {
	seen := make(map[string]struct{})
	var dirs []string
	for _, p := range paths {
		dir := filepath.Dir(p)
		abs, err := filepath.Abs(dir)
		if err != nil {
			abs = dir
		}
		if _, ok := seen[abs]; !ok {
			seen[abs] = struct{}{}
			dirs = append(dirs, abs)
		}
	}
	return dirs
}

// isRelevantEvent checks if an fsnotify event corresponds to a watched config file
// and is a write or create operation.
func isRelevantEvent(event fsnotify.Event, configPaths map[string]struct{}) bool {
	if event.Op&(fsnotify.Write|fsnotify.Create) == 0 {
		return false
	}
	abs, err := filepath.Abs(event.Name)
	if err != nil {
		abs = event.Name
	}
	_, relevant := configPaths[abs]
	return relevant
}
