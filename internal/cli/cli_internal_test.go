package cli

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/chris576/vigil/internal/nginx"
	"github.com/chris576/vigil/internal/process"
	"github.com/chris576/vigil/internal/systemd"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockSystemd struct {
	systemd.Client
	startCalled bool
	stopCalled  bool
	err         error
}

func (m *mockSystemd) StartUnit(ctx context.Context, name string) error {
	m.startCalled = true
	return m.err
}
func (m *mockSystemd) StopUnit(ctx context.Context, name string) error {
	m.stopCalled = true
	return m.err
}
func (m *mockSystemd) RestartUnit(ctx context.Context, name string) error {
	return m.err
}
func (m *mockSystemd) EnableUnit(ctx context.Context, name string) error { return m.err }
func (m *mockSystemd) DisableUnit(ctx context.Context, name string) error { return m.err }
func (m *mockSystemd) UnitStatus(ctx context.Context, name string) (string, string, error) {
	return "active", "running", m.err
}
func (m *mockSystemd) CreateUnitFile(name string, content []byte) error { return m.err }
func (m *mockSystemd) RemoveUnitFile(name string) error                 { return m.err }
func (m *mockSystemd) Reload(ctx context.Context) error                 { return m.err }
func (m *mockSystemd) Close() error                                    { return nil }
func (m *mockSystemd) Logs(ctx context.Context, name string, lines int, follow bool) (io.ReadCloser, error) {
	if m.err != nil {
		return nil, m.err
	}
	return io.NopCloser(strings.NewReader("")), nil
}
func (m *mockSystemd) SetupLogging(ctx context.Context, name string, logPath string, maxSize string, rotate int) error {
	return m.err
}
func (m *mockSystemd) RemoveLogging(ctx context.Context, name string) error {
	return m.err
}

type mockNginx struct {
	nginx.Client
	err error
}

func (m *mockNginx) EnableSite(name string, port int, domain, root string) error { return m.err }
func (m *mockNginx) DisableSite(name string) error                               { return m.err }
func (m *mockNginx) RemoveSiteConfig(name string) error                          { return m.err }
func (m *mockNginx) SiteEnabled(name string) (bool, error)                       { return true, m.err }
func (m *mockNginx) Reload(ctx context.Context) error                            { return m.err }
func (m *mockNginx) Close() error                                                { return nil }
func (m *mockNginx) LogFile(name string) string { return "" }
func (m *mockNginx) Logs(ctx context.Context, name string, lines int, follow bool) (io.ReadCloser, error) {
	if m.err != nil {
		return nil, m.err
	}
	return io.NopCloser(strings.NewReader("")), nil
}
func (m *mockNginx) SetupLogging(name string, logPath string, maxSize string, rotate int) error {
	return m.err
}
func (m *mockNginx) RemoveLogging(name string) error {
	return m.err
}

type mockStore struct {
	process.Store
	processes map[string]process.Process
	err       error
}

func (m *mockStore) Load(name string) (process.Process, error) {
	p, ok := m.processes[name]
	if !ok {
		return process.Process{}, fmt.Errorf("not found")
	}
	return p, nil
}
func (m *mockStore) Save(p process.Process) error  { return m.err }
func (m *mockStore) Delete(name string) error       { return m.err }
func (m *mockStore) List() ([]process.Process, error) {
	var list []process.Process
	for _, p := range m.processes {
		list = append(list, p)
	}
	return list, m.err
}

func testPM() *process.Manager {
	store := &mockStore{processes: map[string]process.Process{}}
	sd := &mockSystemd{}
	ng := &mockNginx{}
	return process.New(store, sd, ng)
}

func testPMWithProcesses(procs map[string]process.Process) *process.Manager {
	store := &mockStore{processes: procs}
	return process.New(store, &mockSystemd{}, &mockNginx{})
}

func executeWithPM(t *testing.T, pm *process.Manager, args []string) (string, error) {
	t.Helper()
	cmd := NewRootCmd()
	cmd.PersistentPreRunE = func(cmd *cobra.Command, args []string) error {
		cmd.SetContext(pmCtx(cmd.Context(), pm))
		return nil
	}
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	if args != nil {
		cmd.SetArgs(args)
	}
	err := cmd.Execute()
	return buf.String(), err
}

func TestExecute(t *testing.T) {
	SetVersion("test")
	err := Execute()
	require.NoError(t, err)
}

func TestExecuteWithVersion(t *testing.T) {
	SetVersion("1.2.3")
	cmd := NewRootCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"version"})
	err := cmd.Execute()
	require.NoError(t, err)
	assert.Equal(t, "1.2.3\n", buf.String())
}

func TestHelp(t *testing.T) {
	out, err := executeWithPM(t, testPM(), []string{"--help"})
	require.NoError(t, err)
	assert.Contains(t, out, "Usage:")
	assert.Contains(t, out, "add")
	assert.Contains(t, out, "remove")
	assert.Contains(t, out, "list")
	assert.Contains(t, out, "start")
	assert.Contains(t, out, "stop")
	assert.Contains(t, out, "restart")
	assert.Contains(t, out, "version")
	assert.Contains(t, out, "logs")
	assert.Contains(t, out, "logsave")
}

func TestVersion(t *testing.T) {
	SetVersion("1.0.0")
	out, err := executeWithPM(t, testPM(), []string{"version"})
	require.NoError(t, err)
	assert.Equal(t, "1.0.0\n", out)
}

func TestNoArgsShowsHelp(t *testing.T) {
	out, err := executeWithPM(t, testPM(), nil)
	require.NoError(t, err)
	assert.Contains(t, out, "Usage:")
}

func TestUnknownCommand(t *testing.T) {
	_, err := executeWithPM(t, testPM(), []string{"unknown"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unknown command")
}

func TestAdd_NoArgs(t *testing.T) {
	_, err := executeWithPM(t, testPM(), []string{"add"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "accepts 1 arg")
}

func TestAdd_MissingFlags(t *testing.T) {
	_, err := executeWithPM(t, testPM(), []string{"add", "my-app"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "required flag")
}

func TestAdd_Success(t *testing.T) {
	out, err := executeWithPM(t, testPM(), []string{"add", "my-app", "--type", "node", "--entry", "./app.js", "--port", "3000"})
	require.NoError(t, err)
	assert.Contains(t, out, "Registered")
}

func TestRemove_NoArgs(t *testing.T) {
	_, err := executeWithPM(t, testPM(), []string{"remove"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "accepts 1 arg")
}

func TestRemove_Success(t *testing.T) {
	pm := testPMWithProcesses(map[string]process.Process{
		"my-app": {Name: "my-app", Type: process.TypeNode},
	})
	out, err := executeWithPM(t, pm, []string{"remove", "my-app"})
	require.NoError(t, err)
	assert.Contains(t, out, "Removed")
}

func TestRemove_NotFound(t *testing.T) {
	_, err := executeWithPM(t, testPM(), []string{"remove", "ghost"})
	require.Error(t, err)
}

func TestList_Empty(t *testing.T) {
	out, err := executeWithPM(t, testPM(), []string{"list"})
	require.NoError(t, err)
	assert.Contains(t, out, "No apps")
}

func TestList_WithApps(t *testing.T) {
	pm := testPMWithProcesses(map[string]process.Process{
		"alpha": {Name: "alpha", Type: process.TypeNode, Port: 3000, Enabled: true},
		"beta":  {Name: "beta", Type: process.TypeStatic, Port: 8080, Enabled: true},
	})
	out, err := executeWithPM(t, pm, []string{"list"})
	require.NoError(t, err)
	assert.Contains(t, out, "alpha")
	assert.Contains(t, out, "beta")
}

func TestStart_Success(t *testing.T) {
	pm := testPMWithProcesses(map[string]process.Process{
		"my-app": {Name: "my-app", Type: process.TypeNode},
	})
	out, err := executeWithPM(t, pm, []string{"start", "my-app"})
	require.NoError(t, err)
	assert.Contains(t, out, "Started")
}

func TestStart_NotFound(t *testing.T) {
	_, err := executeWithPM(t, testPM(), []string{"start", "ghost"})
	require.Error(t, err)
}

func TestStop_Success(t *testing.T) {
	pm := testPMWithProcesses(map[string]process.Process{
		"my-app": {Name: "my-app", Type: process.TypeNode},
	})
	out, err := executeWithPM(t, pm, []string{"stop", "my-app"})
	require.NoError(t, err)
	assert.Contains(t, out, "Stopped")
}

func TestStop_NotFound(t *testing.T) {
	_, err := executeWithPM(t, testPM(), []string{"stop", "ghost"})
	require.Error(t, err)
}

func TestRestart_Success(t *testing.T) {
	pm := testPMWithProcesses(map[string]process.Process{
		"my-app": {Name: "my-app", Type: process.TypeNode},
	})
	out, err := executeWithPM(t, pm, []string{"restart", "my-app"})
	require.NoError(t, err)
	assert.Contains(t, out, "Restarted")
}

func TestRestart_NotFound(t *testing.T) {
	_, err := executeWithPM(t, testPM(), []string{"restart", "ghost"})
	require.Error(t, err)
}

func TestStartCmd_NoPM(t *testing.T) {
	cmd := newStartCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"my-app"})
	err := cmd.Execute()
	require.Error(t, err)
}

func TestStopCmd_NoPM(t *testing.T) {
	cmd := newStopCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"my-app"})
	err := cmd.Execute()
	require.Error(t, err)
}

func TestRestartCmd_NoPM(t *testing.T) {
	cmd := newRestartCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"my-app"})
	err := cmd.Execute()
	require.Error(t, err)
}

func TestRemoveCmd_NoPM(t *testing.T) {
	cmd := newRemoveCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"my-app"})
	err := cmd.Execute()
	require.Error(t, err)
}

func TestListCmd_NoPM(t *testing.T) {
	cmd := newListCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	err := cmd.Execute()
	require.Error(t, err)
}

func TestInitCmd_CreatesFile(t *testing.T) {
	out, err := executeWithPM(t, testPM(), []string{"init", "--output", t.TempDir() + "/eco.json"})
	require.NoError(t, err)
	assert.Contains(t, out, "Wrote template")
}

func TestContextHelpers(t *testing.T) {
	pm := testPM()
	ctx := pmCtx(context.Background(), pm)
	got, ok := pmFromCtx(ctx)
	assert.True(t, ok)
	assert.Same(t, pm, got)

	_, ok = pmFromCtx(context.Background())
	assert.False(t, ok)
}

func TestAdd_WithConfigFile_Single(t *testing.T) {
	dir := t.TempDir()
	ecoFile := dir + "/eco.json"
	err := os.WriteFile(ecoFile, []byte(`{"name":"my-app","type":"node","entry":"./app.js","port":3000}`), 0600)
	require.NoError(t, err)
	out, err := executeWithPM(t, testPM(), []string{"add", "--config", ecoFile})
	require.NoError(t, err)
	assert.Contains(t, out, "1 app(s) registered")
}

func TestAdd_WithConfigFile_Array(t *testing.T) {
	dir := t.TempDir()
	ecoFile := dir + "/eco.json"
	err := os.WriteFile(ecoFile, []byte(`{"apps":[{"name":"a1","type":"node","entry":"e1","port":3000},{"name":"a2","type":"static","build_dir":"bd","port":8080}]}`), 0600)
	require.NoError(t, err)
	out, err := executeWithPM(t, testPM(), []string{"add", "--config", ecoFile})
	require.NoError(t, err)
	assert.Contains(t, out, "2 app(s) registered")
}

func TestAdd_WithConfigFile_NameFilter(t *testing.T) {
	dir := t.TempDir()
	ecoFile := dir + "/eco.json"
	err := os.WriteFile(ecoFile, []byte(`{"apps":[{"name":"app1","type":"node","entry":"e1","port":3000},{"name":"app2","type":"node","entry":"e2","port":3001}]}`), 0600)
	require.NoError(t, err)
	out, err := executeWithPM(t, testPM(), []string{"add", "app1", "--config", ecoFile})
	require.NoError(t, err)
	assert.Contains(t, out, "1 app(s) registered")
}

func TestAdd_WithConfigFile_NameFilterNotFound(t *testing.T) {
	dir := t.TempDir()
	ecoFile := dir + "/eco.json"
	err := os.WriteFile(ecoFile, []byte(`{"apps":[{"name":"app1","type":"node","entry":"e1","port":3000}]}`), 0600)
	require.NoError(t, err)
	_, err = executeWithPM(t, testPM(), []string{"add", "unknown", "--config", ecoFile})
	require.Error(t, err)
}

func TestAdd_WithConfigFile_InvalidJSON(t *testing.T) {
	dir := t.TempDir()
	ecoFile := dir + "/eco.json"
	err := os.WriteFile(ecoFile, []byte("{broken"), 0600)
	require.NoError(t, err)
	_, err = executeWithPM(t, testPM(), []string{"add", "--config", ecoFile})
	require.Error(t, err)
}

func TestAdd_Static(t *testing.T) {
	out, err := executeWithPM(t, testPM(), []string{"add", "my-site", "--type", "static", "--build-dir", "./dist", "--port", "8080"})
	require.NoError(t, err)
	assert.Contains(t, out, "Registered")
}

func TestAdd_WithConfigAndNameArg(t *testing.T) {
	dir := t.TempDir()
	ecoFile := dir + "/eco.json"
	err := os.WriteFile(ecoFile, []byte(`{"apps":[{"name":"app1","type":"node","entry":"e1","port":3000},{"name":"app2","type":"node","entry":"e2","port":3001}]}`), 0600)
	require.NoError(t, err)
	out, err := executeWithPM(t, testPM(), []string{"add", "app1", "--config", ecoFile})
	require.NoError(t, err)
	assert.Contains(t, out, "1 app(s) registered")
}

func TestStart_MissingName(t *testing.T) {
	_, err := executeWithPM(t, testPM(), []string{"start"})
	require.Error(t, err)
}

func TestStop_MissingName(t *testing.T) {
	_, err := executeWithPM(t, testPM(), []string{"stop"})
	require.Error(t, err)
}

func TestRestart_MissingName(t *testing.T) {
	_, err := executeWithPM(t, testPM(), []string{"restart"})
	require.Error(t, err)
}

func TestAdd_WithFlagsOnly(t *testing.T) {
	out, err := executeWithPM(t, testPM(), []string{"add", "my-api", "--type", "node", "--port", "3000", "--entry", "app.js"})
	require.NoError(t, err)
	assert.Contains(t, out, "Registered")
}

func TestAdd_StaticWithEntry(t *testing.T) {
	out, err := executeWithPM(t, testPM(), []string{"add", "my-site", "--type", "static", "--build-dir", "./dist", "--port", "8080", "--entry", "ignored.js"})
	require.NoError(t, err)
	assert.Contains(t, out, "Registered")
}

func TestAdd_WithInvalidType(t *testing.T) {
	_, err := executeWithPM(t, testPM(), []string{"add", "my-app", "--type", "invalid", "--port", "3000"})
	require.Error(t, err)
}

func TestAdd_WithMissingBuildDir(t *testing.T) {
	_, err := executeWithPM(t, testPM(), []string{"add", "my-site", "--type", "static", "--port", "8080"})
	require.Error(t, err)
}

func TestAdd_WithMissingEntry(t *testing.T) {
	_, err := executeWithPM(t, testPM(), []string{"add", "my-app", "--type", "node", "--port", "3000"})
	require.Error(t, err)
}

func TestAdd_ConfigAndNameArgTooMany(t *testing.T) {
	_, err := executeWithPM(t, testPM(), []string{"add", "a", "b", "--config", "eco.json"})
	require.Error(t, err)
}

func TestInitCmd_HelpShowsFlag(t *testing.T) {
	out, err := executeWithPM(t, testPM(), []string{"init", "--help"})
	require.NoError(t, err)
	assert.Contains(t, out, "--output")
}

func TestInitCmd_CustomOutput(t *testing.T) {
	dir := t.TempDir()
	out, err := executeWithPM(t, testPM(), []string{"init", "--output", dir + "/test-eco.json"})
	require.NoError(t, err)
	assert.Contains(t, out, "Wrote template")
	_, err = os.Stat(dir + "/test-eco.json")
	require.NoError(t, err)
}

func TestCommandFlags(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		wantOut string
	}{
		{"add help shows type flag", []string{"add", "--help"}, "--type"},
		{"add help shows port flag", []string{"add", "--help"}, "--port"},
		{"remove help shows usage", []string{"remove", "--help"}, "remove <name>"},
		{"list help shows usage", []string{"list", "--help"}, "list"},
		{"start help shows usage", []string{"start", "--help"}, "start"},
		{"stop help shows usage", []string{"stop", "--help"}, "stop"},
		{"restart help shows usage", []string{"restart", "--help"}, "restart"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			out, err := executeWithPM(t, testPM(), tt.args)
			require.NoError(t, err)
			assert.Contains(t, out, tt.wantOut)
		})
	}
}

func TestAdd_DuplicateWithoutForce(t *testing.T) {
	pm := testPMWithProcesses(map[string]process.Process{
		"my-app": {Name: "my-app", Type: process.TypeNode},
	})
	out, err := executeWithPM(t, pm, []string{"add", "my-app", "--type", "node", "--entry", "./app.js", "--port", "3000"})
	require.Error(t, err)
	assert.Contains(t, out, "already exists")
}

func TestAdd_DuplicateWithForce(t *testing.T) {
	pm := testPMWithProcesses(map[string]process.Process{
		"my-app": {Name: "my-app", Type: process.TypeNode},
	})
	out, err := executeWithPM(t, pm, []string{"add", "my-app", "--type", "static", "--build-dir", "./dist", "--port", "8080", "--force"})
	require.NoError(t, err)
	assert.Contains(t, out, "Registered")
}

func TestAdd_WithConfigFile_Duplicate(t *testing.T) {
	dir := t.TempDir()
	ecoFile := dir + "/eco.json"
	err := os.WriteFile(ecoFile, []byte(`{"name":"my-app","type":"node","entry":"./app.js","port":3000}`), 0600)
	require.NoError(t, err)

	pm := testPMWithProcesses(map[string]process.Process{
		"my-app": {Name: "my-app", Type: process.TypeNode},
	})
	out, err := executeWithPM(t, pm, []string{"add", "--config", ecoFile})
	require.Error(t, err)
	assert.Contains(t, out, "already exists")
}

func TestAdd_WithConfigFile_Force(t *testing.T) {
	dir := t.TempDir()
	ecoFile := dir + "/eco.json"
	err := os.WriteFile(ecoFile, []byte(`{"name":"my-app","type":"static","build_dir":"./dist","port":8080}`), 0600)
	require.NoError(t, err)

	pm := testPMWithProcesses(map[string]process.Process{
		"my-app": {Name: "my-app", Type: process.TypeNode},
	})
	out, err := executeWithPM(t, pm, []string{"add", "--config", ecoFile, "--force"})
	require.NoError(t, err)
	assert.Contains(t, out, "1 app(s) registered")
}

func TestList_WithDisabledApp(t *testing.T) {
	pm := testPMWithProcesses(map[string]process.Process{
		"alpha": {Name: "alpha", Type: process.TypeNode, Port: 3000, Enabled: false},
	})
	out, err := executeWithPM(t, pm, []string{"list"})
	require.NoError(t, err)
	assert.Contains(t, out, "disabled")
}

func TestLogs_Help(t *testing.T) {
	out, err := executeWithPM(t, testPM(), []string{"logs", "--help"})
	require.NoError(t, err)
	assert.Contains(t, out, "--lines")
	assert.Contains(t, out, "--follow")
	assert.Contains(t, out, "--output")
}

func TestLogs_MissingName(t *testing.T) {
	_, err := executeWithPM(t, testPM(), []string{"logs"})
	require.Error(t, err)
}

func TestLogs_Success(t *testing.T) {
	pm := testPMWithProcesses(map[string]process.Process{
		"my-app": {Name: "my-app", Type: process.TypeNode},
	})
	out, err := executeWithPM(t, pm, []string{"logs", "my-app"})
	require.NoError(t, err)
	// No error, output is empty since mock returns empty reader
	assert.Empty(t, out)
}

func TestLogSave_Help(t *testing.T) {
	out, err := executeWithPM(t, testPM(), []string{"logsave", "--help"})
	require.NoError(t, err)
	assert.Contains(t, out, "enable")
	assert.Contains(t, out, "disable")
	assert.Contains(t, out, "status")
}

func TestLogSave_Enable_Help(t *testing.T) {
	out, err := executeWithPM(t, testPM(), []string{"logsave", "enable", "--help"})
	require.NoError(t, err)
	assert.Contains(t, out, "--max-size")
	assert.Contains(t, out, "--output")
	assert.Contains(t, out, "--rotate")
}

func TestLogSave_Enable_MissingName(t *testing.T) {
	_, err := executeWithPM(t, testPM(), []string{"logsave", "enable"})
	require.Error(t, err)
}

func TestLogSave_Disable_MissingName(t *testing.T) {
	_, err := executeWithPM(t, testPM(), []string{"logsave", "disable"})
	require.Error(t, err)
}

func TestLogSave_Status_MissingName(t *testing.T) {
	_, err := executeWithPM(t, testPM(), []string{"logsave", "status"})
	require.Error(t, err)
}

func TestLogs_NoPM(t *testing.T) {
	cmd := newLogsCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"my-app"})
	err := cmd.Execute()
	require.Error(t, err)
}

func TestLogSaveEnable_NoPM(t *testing.T) {
	cmd := newLogSaveEnableCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"my-app"})
	err := cmd.Execute()
	require.Error(t, err)
}

func TestLogSaveDisable_NoPM(t *testing.T) {
	cmd := newLogSaveDisableCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"my-app"})
	err := cmd.Execute()
	require.Error(t, err)
}
