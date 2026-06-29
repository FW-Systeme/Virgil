package process

import (
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidate_ValidNode(t *testing.T) {
	p := Process{Name: "my-app", Type: TypeNode, Port: 3000, Entry: "./app.js"}
	assert.NoError(t, p.Validate())
}

func TestValidate_ValidStatic(t *testing.T) {
	p := Process{Name: "my-site", Type: TypeStatic, Port: 8080, BuildDir: "./dist"}
	assert.NoError(t, p.Validate())
}

func TestValidate_MissingName(t *testing.T) {
	p := Process{Name: "", Type: TypeNode, Port: 3000}
	assert.ErrorContains(t, p.Validate(), "name is required")
}

func TestValidate_InvalidType(t *testing.T) {
	p := Process{Name: "app", Type: "invalid", Port: 3000}
	assert.ErrorContains(t, p.Validate(), "type must be")
}

func TestValidate_ZeroPort(t *testing.T) {
	p := Process{Name: "app", Type: TypeNode, Port: 0}
	assert.ErrorContains(t, p.Validate(), "port must be a positive integer")
}

func TestValidate_NegativePort(t *testing.T) {
	p := Process{Name: "app", Type: TypeNode, Port: -1}
	assert.ErrorContains(t, p.Validate(), "port must be a positive integer")
}

func TestValidate_NodeMissingEntry(t *testing.T) {
	p := Process{Name: "app", Type: TypeNode, Port: 3000}
	assert.ErrorContains(t, p.Validate(), "entry is required for node apps")
}

func TestValidate_StaticMissingBuildDir(t *testing.T) {
	p := Process{Name: "app", Type: TypeStatic, Port: 8080}
	assert.ErrorContains(t, p.Validate(), "build_dir is required for static apps")
}

func TestParseEcosystemFile_SingleApp(t *testing.T) {
	input := `{"name":"my-app","type":"node","entry":"./app.js","port":3000}`
	apps, err := ParseEcosystemFile(strings.NewReader(input))
	require.NoError(t, err)
	require.Len(t, apps, 1)
	assert.Equal(t, "my-app", apps[0].Name)
}

func TestParseEcosystemFile_AppsArray(t *testing.T) {
	input := `{"apps":[{"name":"a1","type":"node","entry":"e1","port":3000},{"name":"a2","type":"static","build_dir":"bd","port":8080}]}`
	apps, err := ParseEcosystemFile(strings.NewReader(input))
	require.NoError(t, err)
	require.Len(t, apps, 2)
	assert.Equal(t, "a1", apps[0].Name)
	assert.Equal(t, "a2", apps[1].Name)
}

func TestParseEcosystemFile_EmptyAppsArray(t *testing.T) {
	input := `{"apps":[]}`
	_, err := ParseEcosystemFile(strings.NewReader(input))
	assert.ErrorContains(t, err, "apps array is empty")
}

func TestParseEcosystemFile_InvalidJSON(t *testing.T) {
	input := `{broken`
	_, err := ParseEcosystemFile(strings.NewReader(input))
	assert.ErrorContains(t, err, "invalid JSON")
}

func TestParseEcosystemFile_ExtraFields(t *testing.T) {
	input := `{"name":"app","type":"node","entry":"e","port":3000,"extra":"field"}`
	apps, err := ParseEcosystemFile(strings.NewReader(input))
	require.NoError(t, err)
	require.Len(t, apps, 1)
	assert.Equal(t, "app", apps[0].Name)
}

func TestParseEcosystemFile_ObjectWithoutNameOrApps(t *testing.T) {
	input := `{"foo":"bar"}`
	_, err := ParseEcosystemFile(strings.NewReader(input))
	assert.ErrorContains(t, err, "must contain either an 'apps' array or a valid process object")
}

func fixedTime() time.Time {
	return time.Date(2025, 6, 15, 10, 30, 0, 0, time.UTC)
}
