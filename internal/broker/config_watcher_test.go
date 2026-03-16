package broker

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const testConfigTemplate = `
nats:
  url: "nats://localhost:4222"
service:
  name: "%s"
  description: "Test Service"
  version: "1.0.0"
  creds_file: "/path/to/creds"
  account:
    name: "test"
    signing_nkey: "SUAGJBPRRXFQL2DXLG4CXW5D6XTLJ4DDMMKHNCIAPNK2Y4IZFHTJM6HN"
    encryption:
      enabled: true
idp:
  - description: "Test IDP"
    issuer_url: "https://test.idp"
    client_id: "test-client"
    ignore_setup_error: true
rbac:
  user_accounts:
    - name: "test-account"
      public_key: "test-key"
      signing_nkey: "SUAGJBPRRXFQL2DXLG4CXW5D6XTLJ4DDMMKHNCIAPNK2Y4IZFHTJM6HN"
  role_binding:
    - user_account: "test-account"
      roles: ["test-role"]
  roles:
    - name: "test-role"
      permissions:
        pub:
          allow: ["test.>"]
`

func writeTestConfig(t *testing.T, path, serviceName string) {
	t.Helper()
	content := []byte(fmt.Sprintf(testConfigTemplate, serviceName))
	err := os.WriteFile(path, content, 0644)
	require.NoError(t, err)
}

func newTestLiveState(t *testing.T, configFile string) *LiveState {
	t.Helper()
	cm, err := NewConfigManager([]string{configFile})
	require.NoError(t, err)
	cfg, err := cm.GetConfig(make(map[string]interface{}))
	require.NoError(t, err)
	return &LiveState{
		config:        cfg,
		configManager: cm,
		idpVerifiers:  nil, // IDP verifiers require real OIDC endpoints
		auditSubject:  cfg.Service.Name + ".evt.audit.account.%s.user.%s.created",
	}
}

func fsnotifyEvent(name string, op fsnotify.Op) fsnotify.Event {
	return fsnotify.Event{Name: name, Op: op}
}

func TestConfigWatcher_State(t *testing.T) {
	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "config.yaml")
	writeTestConfig(t, configFile, "initial-service")

	initial := newTestLiveState(t, configFile)
	ctx := NewServerContext(nil)
	watcher := NewConfigWatcher(ctx, []string{configFile}, initial)

	state := watcher.State()
	require.NotNil(t, state)
	assert.Equal(t, "initial-service", state.config.Service.Name)
}

func TestConfigWatcher_Reload(t *testing.T) {
	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "config.yaml")
	writeTestConfig(t, configFile, "original-service")

	initial := newTestLiveState(t, configFile)
	ctx := NewServerContext(nil)
	watcher := NewConfigWatcher(ctx, []string{configFile}, initial)

	// Verify initial state
	assert.Equal(t, "original-service", watcher.State().config.Service.Name)

	// Update config file
	writeTestConfig(t, configFile, "updated-service")

	// Trigger reload directly
	err := watcher.reload()
	require.NoError(t, err)

	// Verify state was swapped
	assert.Equal(t, "updated-service", watcher.State().config.Service.Name)
}

func TestConfigWatcher_ReloadInvalidConfig(t *testing.T) {
	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "config.yaml")
	writeTestConfig(t, configFile, "good-service")

	initial := newTestLiveState(t, configFile)
	ctx := NewServerContext(nil)
	watcher := NewConfigWatcher(ctx, []string{configFile}, initial)

	// Write invalid YAML
	err := os.WriteFile(configFile, []byte("invalid: yaml: [broken"), 0644)
	require.NoError(t, err)

	// Reload should fail
	err = watcher.reload()
	assert.Error(t, err)

	// State should remain unchanged
	assert.Equal(t, "good-service", watcher.State().config.Service.Name)
}

func TestConfigWatcher_ReloadMissingRequiredField(t *testing.T) {
	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "config.yaml")
	writeTestConfig(t, configFile, "good-service")

	initial := newTestLiveState(t, configFile)
	ctx := NewServerContext(nil)
	watcher := NewConfigWatcher(ctx, []string{configFile}, initial)

	// Write config missing required fields
	err := os.WriteFile(configFile, []byte("nats:\n  url: \"nats://localhost:4222\"\n"), 0644)
	require.NoError(t, err)

	// Reload should fail validation
	err = watcher.reload()
	assert.Error(t, err)

	// State should remain unchanged
	assert.Equal(t, "good-service", watcher.State().config.Service.Name)
}

func TestConfigWatcher_FileWatchIntegration(t *testing.T) {
	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "config.yaml")
	writeTestConfig(t, configFile, "watch-test")

	initial := newTestLiveState(t, configFile)
	ctx := NewServerContext(nil)
	watcher := NewConfigWatcher(ctx, []string{configFile}, initial)
	// Use a short debounce for testing
	watcher.debounce = 100 * time.Millisecond

	err := watcher.Start()
	require.NoError(t, err)
	defer watcher.Stop()

	// Modify the config file
	writeTestConfig(t, configFile, "watch-updated")

	// Wait for debounce + reload to complete
	assert.Eventually(t, func() bool {
		return watcher.State().config.Service.Name == "watch-updated"
	}, 3*time.Second, 50*time.Millisecond, "state should be updated after file change")
}

func TestConfigWatcher_Debounce(t *testing.T) {
	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "config.yaml")
	writeTestConfig(t, configFile, "debounce-test")

	initial := newTestLiveState(t, configFile)
	ctx := NewServerContext(nil)
	watcher := NewConfigWatcher(ctx, []string{configFile}, initial)
	watcher.debounce = 200 * time.Millisecond

	err := watcher.Start()
	require.NoError(t, err)
	defer watcher.Stop()

	// Rapid-fire changes — only the last one should take effect
	for i := 0; i < 5; i++ {
		writeTestConfig(t, configFile, fmt.Sprintf("debounce-%d", i))
		time.Sleep(20 * time.Millisecond)
	}

	// Wait for debounce to settle and reload
	assert.Eventually(t, func() bool {
		name := watcher.State().config.Service.Name
		return name == "debounce-4"
	}, 3*time.Second, 50*time.Millisecond, "final state should reflect last write")
}

func TestConfigWatcher_ConcurrentStateAccess(t *testing.T) {
	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "config.yaml")
	writeTestConfig(t, configFile, "concurrent-test")

	initial := newTestLiveState(t, configFile)
	ctx := NewServerContext(nil)
	watcher := NewConfigWatcher(ctx, []string{configFile}, initial)

	var wg sync.WaitGroup
	const goroutines = 50

	// Concurrent reads while a reload happens
	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			state := watcher.State()
			assert.NotNil(t, state)
			assert.NotEmpty(t, state.config.Service.Name)
		}()
	}

	// Trigger a reload concurrently
	writeTestConfig(t, configFile, "concurrent-updated")
	wg.Add(1)
	go func() {
		defer wg.Done()
		_ = watcher.reload()
	}()

	wg.Wait()

	// After everything settles, state should be one of the two valid states
	name := watcher.State().config.Service.Name
	assert.Contains(t, []string{"concurrent-test", "concurrent-updated"}, name)
}

func TestResolveGlobPaths(t *testing.T) {
	tmpDir := t.TempDir()

	// Create test files
	for _, name := range []string{"a.yaml", "b.yaml", "c.txt"} {
		err := os.WriteFile(filepath.Join(tmpDir, name), []byte("test: true"), 0644)
		require.NoError(t, err)
	}

	t.Run("concrete path", func(t *testing.T) {
		paths, err := resolveGlobPaths([]string{filepath.Join(tmpDir, "a.yaml")})
		require.NoError(t, err)
		assert.Len(t, paths, 1)
	})

	t.Run("glob pattern", func(t *testing.T) {
		paths, err := resolveGlobPaths([]string{filepath.Join(tmpDir, "*.yaml")})
		require.NoError(t, err)
		assert.Len(t, paths, 2)
	})

	t.Run("no matches", func(t *testing.T) {
		_, err := resolveGlobPaths([]string{filepath.Join(tmpDir, "*.json")})
		assert.Error(t, err)
	})
}

func TestUniqueDirs(t *testing.T) {
	dirs := uniqueDirs([]string{"/a/b/file1.yaml", "/a/b/file2.yaml", "/c/d/file3.yaml"})
	assert.Len(t, dirs, 2)
}

func TestIsRelevantEvent(t *testing.T) {
	configPaths := map[string]struct{}{
		"/tmp/config.yaml": {},
	}

	t.Run("write to config file", func(t *testing.T) {
		e := fsnotifyEvent("/tmp/config.yaml", fsnotify.Write)
		assert.True(t, isRelevantEvent(e, configPaths))
	})

	t.Run("create config file", func(t *testing.T) {
		e := fsnotifyEvent("/tmp/config.yaml", fsnotify.Create)
		assert.True(t, isRelevantEvent(e, configPaths))
	})

	t.Run("remove config file", func(t *testing.T) {
		e := fsnotifyEvent("/tmp/config.yaml", fsnotify.Remove)
		assert.False(t, isRelevantEvent(e, configPaths))
	})

	t.Run("write to unrelated file", func(t *testing.T) {
		e := fsnotifyEvent("/tmp/other.yaml", fsnotify.Write)
		assert.False(t, isRelevantEvent(e, configPaths))
	})
}
