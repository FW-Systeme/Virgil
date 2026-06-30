package update

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"github.com/chris576/vigil/internal/process"
)

type service struct {
	store   process.Store
	restart RestartFunc
}

func NewService(store process.Store, restart RestartFunc) Service {
	return &service{store: store, restart: restart}
}

func (s *service) Update(ctx context.Context, name string, version string) error {
	p, err := s.store.Load(name)
	if err != nil {
		return fmt.Errorf("loading process: %w", err)
	}

	if p.UpdateScript == "" {
		return ErrNotScript
	}

	workingDir := p.WorkingDir
	if workingDir == "" {
		return fmt.Errorf("working_dir not set")
	}

	releasesDir := filepath.Join(workingDir, "releases")
	incomingDir := p.IncomingDir
	if incomingDir == "" {
		incomingDir = filepath.Join(workingDir, "incoming")
	}
	sharedDir := filepath.Join(workingDir, "shared")
	currentSymlink := filepath.Join(workingDir, "current")

	unlock, err := lock(workingDir)
	if err != nil {
		return err
	}
	defer unlock()

	for _, d := range []string{releasesDir, sharedDir, incomingDir} {
		if err := os.MkdirAll(d, 0755); err != nil {
			return fmt.Errorf("creating dir %s: %w", d, err)
		}
	}

	if version == "" {
		version, err = findVersion(incomingDir)
		if err != nil {
			return err
		}
	}

	pkgPath := filepath.Join(incomingDir, version+".tar.gz")
	if err := verifyIntegrity(pkgPath); err != nil {
		return err
	}

	releaseDir := filepath.Join(releasesDir, version)
	if _, err := os.Stat(releaseDir); err == nil {
		os.RemoveAll(releaseDir)
	}
	if err := os.MkdirAll(releaseDir, 0755); err != nil {
		return fmt.Errorf("creating release dir: %w", err)
	}

	script := p.UpdateScript

	cleanup := func() { os.RemoveAll(releaseDir) }

	if err := runScript(script, "extract", pkgPath, releaseDir); err != nil {
		cleanup()
		return err
	}
	if err := runScript(script, "deps", releaseDir); err != nil {
		cleanup()
		return err
	}
	if err := runScript(script, "migrate", releaseDir, sharedDir); err != nil {
		cleanup()
		return err
	}
	if err := linkShared(sharedDir, releaseDir); err != nil {
		cleanup()
		return fmt.Errorf("linking shared: %w", err)
	}

	runScript(script, "health-check", releaseDir)

	oldTarget := ""
	if current, err := os.Readlink(currentSymlink); err == nil {
		oldTarget = current
	}

	if err := switchSymlink(currentSymlink, releaseDir); err != nil {
		cleanup()
		return fmt.Errorf("switching symlink: %w", err)
	}

	if err := s.restart(ctx, name); err != nil {
		rollbackSymlink(currentSymlink, oldTarget)
		s.restart(ctx, name)
		cleanup()
		return fmt.Errorf("restart after update: %w", err)
	}

	if err := runScript(script, "health-check", releaseDir); err != nil {
		rollbackSymlink(currentSymlink, oldTarget)
		s.restart(ctx, name)
		return ErrRolledBack
	}

	keep := p.KeepReleases
	if keep <= 0 {
		keep = 3
	}
	if err := cleanupReleases(releasesDir, version, keep); err != nil {
		return fmt.Errorf("cleaning up releases: %w", err)
	}

	return nil
}

func lock(workingDir string) (func(), error) {
	lockPath := filepath.Join(workingDir, ".vigil.lock")
	f, err := os.OpenFile(lockPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0644)
	if err != nil {
		if os.IsExist(err) {
			return nil, ErrLocked
		}
		return nil, fmt.Errorf("creating lock: %w", err)
	}
	fmt.Fprintf(f, "%d\n", os.Getpid())
	f.Close()
	return func() { os.Remove(lockPath) }, nil
}

func findVersion(incomingDir string) (string, error) {
	entries, err := os.ReadDir(incomingDir)
	if err != nil {
		if os.IsNotExist(err) {
			return "", ErrNoPackage
		}
		return "", fmt.Errorf("reading incoming dir: %w", err)
	}
	for _, e := range entries {
		name := e.Name()
		if strings.HasSuffix(name, ".tar.gz") && !strings.HasSuffix(name, ".sha256") {
			return strings.TrimSuffix(name, ".tar.gz"), nil
		}
	}
	return "", ErrNoPackage
}

func verifyIntegrity(pkgPath string) error {
	sumFile := pkgPath + ".sha256"
	expectedRaw, err := os.ReadFile(sumFile)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("reading checksum: %w", err)
	}
	expected := strings.TrimSpace(string(expectedRaw))

	data, err := os.ReadFile(pkgPath)
	if err != nil {
		return fmt.Errorf("reading package: %w", err)
	}
	hash := sha256.Sum256(data)
	got := hex.EncodeToString(hash[:])

	if !strings.EqualFold(expected, got) {
		return ErrIntegrity
	}
	return nil
}

func runScript(script string, subcommand string, args ...string) error {
	cmdArgs := append([]string{subcommand}, args...)
	cmd := exec.Command(script, cmdArgs...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("%w: %s %s: %v", ErrScriptFailed, script, strings.Join(cmdArgs, " "), err)
	}
	return nil
}

func linkShared(sharedDir, releaseDir string) error {
	entries, err := os.ReadDir(sharedDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	for _, e := range entries {
		src := filepath.Join(sharedDir, e.Name())
		dst := filepath.Join(releaseDir, e.Name())
		if _, err := os.Lstat(dst); err == nil {
			os.Remove(dst)
		}
		if err := os.Symlink(src, dst); err != nil {
			return fmt.Errorf("symlinking %s: %w", e.Name(), err)
		}
	}
	return nil
}

func switchSymlink(symlinkPath, target string) error {
	tmp := symlinkPath + ".tmp"
	if err := os.Symlink(target, tmp); err != nil {
		return err
	}
	return os.Rename(tmp, symlinkPath)
}

func rollbackSymlink(symlinkPath, oldTarget string) {
	if oldTarget == "" {
		return
	}
	tmp := symlinkPath + ".tmp"
	if err := os.Symlink(oldTarget, tmp); err != nil {
		return
	}
	os.Rename(tmp, symlinkPath)
}

func cleanupReleases(releasesDir, currentVersion string, keep int) error {
	entries, err := os.ReadDir(releasesDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	type release struct {
		name string
		info os.FileInfo
	}

	var releases []release
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		name := e.Name()
		if name == currentVersion {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		releases = append(releases, release{name: name, info: info})
	}

	sort.Slice(releases, func(i, j int) bool {
		return compareVersions(releases[i].name, releases[j].name) > 0
	})

	maxOld := keep - 1
	if maxOld < 0 {
		maxOld = 0
	}
	for i := maxOld; i < len(releases); i++ {
		os.RemoveAll(filepath.Join(releasesDir, releases[i].name))
	}
	return nil
}

func compareVersions(a, b string) int {
	a = strings.TrimLeft(a, "vV")
	b = strings.TrimLeft(b, "vV")

	partsA := strings.Split(a, ".")
	partsB := strings.Split(b, ".")

	maxLen := len(partsA)
	if len(partsB) > maxLen {
		maxLen = len(partsB)
	}

	for i := 0; i < maxLen; i++ {
		var numA, numB int
		if i < len(partsA) {
			fmt.Sscanf(partsA[i], "%d", &numA)
		}
		if i < len(partsB) {
			fmt.Sscanf(partsB[i], "%d", &numB)
		}
		if numA != numB {
			return numA - numB
		}
	}
	return 0
}
