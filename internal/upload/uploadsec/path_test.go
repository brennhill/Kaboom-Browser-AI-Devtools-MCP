// Purpose: Tests for upload security constraints and file validation.
// Docs: docs/features/feature/file-upload/index.md

// security_test.go — Tests for folder-scoped permissions and sensitive path denylist.
package uploadsec

import (
	"net"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// testSecurity returns a permissive Security config for tests.
func testSecurity(t *testing.T) *Security {
	t.Helper()
	return &Security{uploadDir: "/"}
}

// testSecurityWithDir returns a Security scoped to a specific directory.
func testSecurityWithDir(t *testing.T, dir string) *Security {
	t.Helper()
	resolved, err := filepath.EvalSymlinks(dir)
	if err != nil {
		t.Fatalf("testSecurityWithDir: EvalSymlinks(%s) failed: %v", dir, err)
	}
	return &Security{uploadDir: resolved}
}

// testSecurityNoDir returns a Security with no upload-dir (Stage 1 only).
func testSecurityNoDir(t *testing.T) *Security {
	t.Helper()
	return &Security{}
}

// ============================================
// 1. Denylist Matching
// ============================================

func TestSecurity_Denylist_SSHKeys(t *testing.T) {
	home, _ := os.UserHomeDir()
	paths := []string{
		filepath.Join(home, ".ssh", "id_rsa"),
		filepath.Join(home, ".ssh", "id_ed25519"),
		filepath.Join(home, ".ssh", "config"),
	}
	for _, p := range paths {
		t.Run(filepath.Base(p), func(t *testing.T) {
			pattern, matched := MatchesDenylist(p)
			if !matched {
				t.Errorf("MatchesDenylist(%q) should match SSH directory", p)
			}
			if !strings.Contains(pattern, ".ssh") {
				t.Errorf("pattern should mention .ssh, got %q", pattern)
			}
		})
	}
}

func TestSecurity_Denylist_AWSCredentials(t *testing.T) {
	home, _ := os.UserHomeDir()
	p := filepath.Join(home, ".aws", "credentials")
	pattern, matched := MatchesDenylist(p)
	if !matched {
		t.Errorf("MatchesDenylist(%q) should match AWS credentials", p)
	}
	if !strings.Contains(pattern, ".aws") {
		t.Errorf("pattern should mention .aws, got %q", pattern)
	}
}

func TestSecurity_Denylist_EnvFiles(t *testing.T) {
	tests := []struct {
		name string
		path string
	}{
		{"dotenv", "/app/project/.env"},
		{"dotenv_local", "/app/project/.env.local"},
		{"dotenv_production", "/app/project/.env.production"},
		{"nested", "/deep/nested/dir/.env"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, matched := MatchesDenylist(tc.path)
			if !matched {
				t.Errorf("MatchesDenylist(%q) should match .env pattern", tc.path)
			}
		})
	}
}

func TestSecurity_Denylist_KeyFiles(t *testing.T) {
	exts := []string{".pem", ".key", ".p12", ".pfx", ".keystore"}
	for _, ext := range exts {
		t.Run(ext, func(t *testing.T) {
			p := "/some/path/server" + ext
			_, matched := MatchesDenylist(p)
			if !matched {
				t.Errorf("MatchesDenylist(%q) should match key extension", p)
			}
		})
	}
}

func TestSecurity_Denylist_GitConfig(t *testing.T) {
	p := "/app/repo/.git/config"
	_, matched := MatchesDenylist(p)
	if !matched {
		t.Errorf("MatchesDenylist(%q) should match .git/config", p)
	}
}

func TestSecurity_Denylist_SystemFiles(t *testing.T) {
	paths := []string{"/etc/shadow", "/etc/passwd", "/etc/sudoers"}
	for _, p := range paths {
		t.Run(filepath.Base(p), func(t *testing.T) {
			_, matched := MatchesDenylist(p)
			if !matched {
				t.Errorf("MatchesDenylist(%q) should match system file", p)
			}
		})
	}
}

func TestSecurity_Denylist_SafePaths(t *testing.T) {
	safe := []string{
		"/Users/brenn/Videos/video.mp4",
		"/tmp/upload/report.pdf",
		"/home/user/documents/photo.jpg",
		"/Users/brenn/Uploads/data.csv",
	}
	for _, p := range safe {
		t.Run(filepath.Base(p), func(t *testing.T) {
			_, matched := MatchesDenylist(p)
			if matched {
				t.Errorf("MatchesDenylist(%q) should NOT match (safe path)", p)
			}
		})
	}
}

// ============================================
// 2. User Deny Patterns
// ============================================

func TestSecurity_UserDenyPatterns(t *testing.T) {
	patterns := []string{"*.sqlite", "company-secrets/*"}

	_, matched := MatchesUserDenylist("/app/data/users.sqlite", patterns)
	if !matched {
		t.Error("should match *.sqlite pattern via basename")
	}

	_, matched = MatchesUserDenylist("/app/docs/readme.md", patterns)
	if matched {
		t.Error("should NOT match for a safe .md file")
	}

	_, matched = MatchesUserDenylist("/deep/nested/path/db.sqlite", patterns)
	if !matched {
		t.Error("should match *.sqlite pattern via basename on nested path")
	}

	_, matched = MatchesUserDenylist("/any/path", nil)
	if matched {
		t.Error("nil patterns should never match")
	}
}

func TestSecurity_PathsEqualFold(t *testing.T) {
	t.Parallel()

	if runtime.GOOS == "darwin" || runtime.GOOS == "windows" {
		if !PathsEqualFold("/Users/Test", "/users/test") {
			t.Error("should be case-insensitive on darwin/windows")
		}
	} else {
		if PathsEqualFold("/Users/Test", "/users/test") {
			t.Error("should be case-sensitive on linux")
		}
	}

	if !PathsEqualFold("/same/path", "/same/path") {
		t.Error("identical paths should always match")
	}
}

func TestSecurity_PathHasPrefixFold(t *testing.T) {
	t.Parallel()

	if !PathHasPrefixFold("/uploads/sub/file", "/uploads") {
		t.Error("should match prefix")
	}
	if PathHasPrefixFold("/other/path", "/uploads") {
		t.Error("should not match different prefix")
	}
	if PathHasPrefixFold("/up", "/uploads") {
		t.Error("shorter string should not match longer prefix")
	}

	if runtime.GOOS == "darwin" || runtime.GOOS == "windows" {
		if !PathHasPrefixFold("/Uploads/Sub", "/uploads") {
			t.Error("should be case-insensitive on darwin/windows")
		}
	}
}

// ============================================
// 3. ValidateUploadDir (startup validation)
// ============================================

func TestSecurity_ValidateUploadDir_Valid(t *testing.T) {
	dir := t.TempDir()
	dir, _ = filepath.EvalSymlinks(dir)
	sec, err := ValidateUploadDir(dir, nil)
	if err != nil {
		t.Fatalf("ValidateUploadDir(%q) should succeed, got: %v", dir, err)
	}
	if sec.uploadDir == "" {
		t.Error("uploadDir should be set")
	}
}

func TestSecurity_ValidateUploadDir_Empty(t *testing.T) {
	sec, err := ValidateUploadDir("", nil)
	if err != nil {
		t.Fatalf("empty dir should succeed: %v", err)
	}
	if sec.uploadDir != "" {
		t.Error("uploadDir should be empty for no flag")
	}
}

func TestSecurity_ValidateUploadDir_Relative(t *testing.T) {
	_, err := ValidateUploadDir("relative/path", nil)
	if err == nil {
		t.Fatal("relative path should fail")
	}
	if !strings.Contains(err.Error(), "absolute path") {
		t.Errorf("error should mention absolute path, got: %v", err)
	}
}

func TestSecurity_ValidateUploadDir_NotExists(t *testing.T) {
	_, err := ValidateUploadDir("/nonexistent/dir/12345", nil)
	if err == nil {
		t.Error("nonexistent dir should fail")
	}
}

func TestSecurity_ValidateUploadDir_IsFile(t *testing.T) {
	f := filepath.Join(t.TempDir(), "file.txt")
	os.WriteFile(f, []byte("x"), 0o644)

	_, err := ValidateUploadDir(f, nil)
	if err == nil {
		t.Fatal("file (not directory) should fail")
	}
	if !strings.Contains(err.Error(), "not a directory") {
		t.Errorf("error should mention not a directory, got: %v", err)
	}
}

func TestSecurity_ValidateUploadDir_Symlink(t *testing.T) {
	real := t.TempDir()
	link := filepath.Join(t.TempDir(), "link")
	os.Symlink(real, link)

	_, err := ValidateUploadDir(link, nil)
	if err == nil {
		t.Fatal("symlink dir should fail")
	}
	if !strings.Contains(err.Error(), "symlink") {
		t.Errorf("error should mention symlink, got: %v", err)
	}
}

// Regression: a real directory reached through a SYMLINKED ANCESTOR (e.g. macOS
// /var -> /private/var, Windows 8.3 short paths under %TEMP%) must be accepted —
// only a directly symlinked upload dir is rejected (see _Symlink above). The old
// validator compared EvalSymlinks(dir) against the raw path and wrongly rejected
// these, which broke daemon startup (and the upgrade-e2e gate) on macOS/Windows.
func TestSecurity_ValidateUploadDir_SymlinkedAncestor(t *testing.T) {
	base := t.TempDir()
	realParent := filepath.Join(base, "real")
	uploadDir := filepath.Join(realParent, "uploads")
	if err := os.MkdirAll(uploadDir, 0o750); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	linkParent := filepath.Join(base, "link")
	if err := os.Symlink(realParent, linkParent); err != nil {
		t.Skipf("cannot create symlink on this platform: %v", err)
	}
	viaSymlinkedAncestor := filepath.Join(linkParent, "uploads")
	if _, err := ValidateUploadDir(viaSymlinkedAncestor, nil); err != nil {
		t.Fatalf("upload dir reached via a symlinked ancestor should be accepted, got: %v", err)
	}
}

func TestSecurity_ValidateUploadDir_HomeDir(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Skip("cannot determine home dir")
	}
	_, err = ValidateUploadDir(home, nil)
	if err == nil {
		t.Fatal("home directory should be rejected")
	}
	if !strings.Contains(err.Error(), "subdirectory") {
		t.Errorf("error should mention subdirectory, got: %v", err)
	}
}

func TestSecurity_ValidateUploadDir_SensitiveDir(t *testing.T) {
	home, _ := os.UserHomeDir()
	sshDir := filepath.Join(home, ".ssh")
	if _, err := os.Stat(sshDir); err != nil {
		t.Skip(".ssh directory does not exist")
	}

	_, err := ValidateUploadDir(sshDir, nil)
	if err == nil {
		t.Error("~/.ssh should be rejected as upload dir")
	}
}

// ============================================
// 4. ValidateFilePath (per-request validation)
// ============================================

func TestSecurity_ValidateFilePath_ValidFile(t *testing.T) {
	dir := t.TempDir()
	f := filepath.Join(dir, "test.mp4")
	os.WriteFile(f, []byte("video data"), 0o644)

	sec := testSecurityWithDir(t, dir)
	result, err := sec.ValidateFilePath(f, true)
	if err != nil {
		t.Fatalf("valid file should pass: %v", err)
	}
	if result.ResolvedPath == "" {
		t.Error("resolved path should be set")
	}
}

func TestSecurity_ValidateFilePath_RelativePath(t *testing.T) {
	sec := testSecurity(t)
	_, err := sec.ValidateFilePath("../etc/passwd", false)
	if err == nil {
		t.Fatal("relative path should fail")
	}
	if !strings.Contains(err.Error(), "absolute path") {
		t.Errorf("error should mention absolute path, got: %v", err)
	}
}

func TestSecurity_ValidateFilePath_DotDotTraversal(t *testing.T) {
	sec := testSecurity(t)
	_, err := sec.ValidateFilePath("/tmp/safe/../../../etc/shadow", false)
	if err != nil {
		return
	}
	t.Error("path traversal via .. should be caught")
}

func TestSecurity_ValidateFilePath_SymlinkToSensitive(t *testing.T) {
	home, _ := os.UserHomeDir()
	sshDir := filepath.Join(home, ".ssh")
	if _, err := os.Stat(sshDir); err != nil {
		t.Skip("~/.ssh does not exist")
	}

	dir := t.TempDir()
	link := filepath.Join(dir, "innocent.txt")
	target := filepath.Join(sshDir, "known_hosts")
	if _, err := os.Stat(target); err != nil {
		t.Skip("~/.ssh/known_hosts does not exist")
	}
	os.Symlink(target, link)

	sec := testSecurityWithDir(t, dir)
	_, err := sec.ValidateFilePath(link, true)
	if err == nil {
		t.Error("symlink to ~/.ssh should be denied")
	}
}

func TestSecurity_ValidateFilePath_DenylistBlocks(t *testing.T) {
	dir := t.TempDir()
	envFile := filepath.Join(dir, ".env")
	os.WriteFile(envFile, []byte("SECRET=abc"), 0o644)

	sec := testSecurityWithDir(t, dir)
	_, err := sec.ValidateFilePath(envFile, true)
	if err == nil {
		t.Error(".env file should be denied by denylist")
	}
	if _, ok := err.(*PathDeniedError); !ok {
		t.Errorf("error should be PathDeniedError, got %T: %v", err, err)
	}
}

func TestSecurity_ValidateFilePath_KeyFileBlocked(t *testing.T) {
	dir := t.TempDir()
	pemFile := filepath.Join(dir, "server.pem")
	os.WriteFile(pemFile, []byte("-----BEGIN CERTIFICATE-----"), 0o644)

	sec := testSecurityWithDir(t, dir)
	_, err := sec.ValidateFilePath(pemFile, true)
	if err == nil {
		t.Error(".pem file should be denied")
	}
}

// ============================================
// 5. Upload-Dir Enforcement (Stage 1 vs 2-4)
// ============================================

func TestSecurity_Stage1_WorksWithoutUploadDir(t *testing.T) {
	dir := t.TempDir()
	f := filepath.Join(dir, "video.mp4")
	os.WriteFile(f, []byte("data"), 0o644)

	sec := testSecurityNoDir(t)
	result, err := sec.ValidateFilePath(f, false)
	if err != nil {
		t.Fatalf("Stage 1 without upload-dir should work: %v", err)
	}
	if result.ResolvedPath == "" {
		t.Error("resolved path should be set")
	}
}

func TestSecurity_Stage2_RequiresUploadDir(t *testing.T) {
	dir := t.TempDir()
	f := filepath.Join(dir, "video.mp4")
	os.WriteFile(f, []byte("data"), 0o644)

	sec := testSecurityNoDir(t)
	_, err := sec.ValidateFilePath(f, true)
	if err == nil {
		t.Error("Stage 2+ without upload-dir should fail")
	}
	if _, ok := err.(*UploadDirRequiredError); !ok {
		t.Errorf("should be UploadDirRequiredError, got %T: %v", err, err)
	}
}

func TestSecurity_OutsideUploadDir_Blocked(t *testing.T) {
	uploadDir := t.TempDir()
	otherDir := t.TempDir()
	f := filepath.Join(otherDir, "video.mp4")
	os.WriteFile(f, []byte("data"), 0o644)

	sec := testSecurityWithDir(t, uploadDir)
	_, err := sec.ValidateFilePath(f, true)
	if err == nil {
		t.Error("file outside upload-dir should be blocked")
	}
}

func TestSecurity_InsideUploadDir_SubdirAllowed(t *testing.T) {
	dir := t.TempDir()
	sub := filepath.Join(dir, "sub", "deep")
	os.MkdirAll(sub, 0o755)
	f := filepath.Join(sub, "video.mp4")
	os.WriteFile(f, []byte("data"), 0o644)

	sec := testSecurityWithDir(t, dir)
	result, err := sec.ValidateFilePath(f, true)
	if err != nil {
		t.Fatalf("file in subdirectory of upload-dir should be allowed: %v", err)
	}
	if result.ResolvedPath == "" {
		t.Error("resolved path should be set")
	}
}

// ============================================
// 6. Error Message Quality
// ============================================

func TestSecurity_PathDeniedError_Format(t *testing.T) {
	err := &PathDeniedError{
		Path:      "/home/user/.ssh/id_rsa",
		Pattern:   "~/.ssh",
		UploadDir: "/home/user/Uploads",
	}
	msg := err.Error()
	if !strings.Contains(msg, ".ssh/id_rsa") {
		t.Errorf("error should contain the path, got: %s", msg)
	}
	if !strings.Contains(msg, "~/.ssh") {
		t.Errorf("error should contain the pattern, got: %s", msg)
	}
	if !strings.Contains(msg, "/home/user/Uploads") {
		t.Errorf("error should contain the upload dir, got: %s", msg)
	}
}

func TestSecurity_UploadDirRequiredError_Format(t *testing.T) {
	err := &UploadDirRequiredError{}
	msg := err.Error()
	if !strings.Contains(msg, "--upload-dir") {
		t.Errorf("error should mention --upload-dir, got: %s", msg)
	}
}

// ============================================
// 7. IsWithinDir edge cases
// ============================================

func TestSecurity_IsWithinDir(t *testing.T) {
	tests := []struct {
		name     string
		file     string
		dir      string
		expected bool
	}{
		{"direct child", "/uploads/file.mp4", "/uploads", true},
		{"nested", "/uploads/sub/file.mp4", "/uploads", true},
		{"same dir", "/uploads", "/uploads", true},
		{"outside", "/other/file.mp4", "/uploads", false},
		{"prefix attack", "/uploads-evil/file.mp4", "/uploads", false},
		{"parent", "/", "/uploads", false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := IsWithinDir(tc.file, tc.dir)
			if got != tc.expected {
				t.Errorf("IsWithinDir(%q, %q) = %v, want %v", tc.file, tc.dir, got, tc.expected)
			}
		})
	}
}

// ============================================
// 9. SSRF: unspecified addresses blocked
// ============================================

func TestSecurity_IsPrivateIP_Unspecified(t *testing.T) {
	unspecified := []string{"0.0.0.0", "0.0.0.1", "::"}
	for _, ipStr := range unspecified {
		t.Run(ipStr, func(t *testing.T) {
			ip := net.ParseIP(ipStr)
			if ip == nil {
				t.Fatalf("failed to parse %q", ipStr)
			}
			if !IsPrivateIP(ip) {
				t.Errorf("IsPrivateIP(%s) should return true for unspecified address", ipStr)
			}
		})
	}
}

// ============================================
// 10. Hardlink detection
// ============================================

func TestSecurity_Hardlink_Detected(t *testing.T) {
	dir := t.TempDir()
	original := filepath.Join(dir, "original.txt")
	os.WriteFile(original, []byte("secret"), 0o644)

	hardlink := filepath.Join(dir, "hardlink.txt")
	if err := os.Link(original, hardlink); err != nil {
		t.Skip("cannot create hardlinks on this filesystem")
	}

	info, err := os.Stat(hardlink)
	if err != nil {
		t.Fatal(err)
	}

	if err := CheckHardlink(info); err == nil {
		t.Error("CheckHardlink should detect file with nlink > 1")
	}
}

func TestSecurity_NoHardlink_Allowed(t *testing.T) {
	dir := t.TempDir()
	f := filepath.Join(dir, "single.txt")
	os.WriteFile(f, []byte("normal"), 0o644)

	info, err := os.Stat(f)
	if err != nil {
		t.Fatal(err)
	}

	if err := CheckHardlink(info); err != nil {
		t.Errorf("CheckHardlink should allow nlink=1 file: %v", err)
	}
}

// ============================================
// 12. Case-insensitive denylist on macOS
// ============================================

func TestSecurity_Denylist_CaseInsensitive(t *testing.T) {
	home, _ := os.UserHomeDir()
	if home == "" {
		t.Skip("HOME not set")
	}

	upperSSH := filepath.Join(home, ".SSH", "id_rsa")
	_, matched := MatchesDenylist(upperSSH)

	if runtime.GOOS == "darwin" || runtime.GOOS == "windows" {
		if !matched {
			t.Errorf("MatchesDenylist(%q) should match on case-insensitive OS", upperSSH)
		}
	}
}

// DNS fail-closed now lives in ssrf_resolver_test.go, where the resolver is
// injected. The version that stood here queried the machine's real resolver, so a
// network that answers NXDOMAIN with an address turned it red — and because it
// called t.Error and then dereferenced the nil error, it panicked and took every
// other test in this package down with it.

// ============================================
// 14. Root credentials in absolute denylist
// ============================================

func TestSecurity_Denylist_RootSSH(t *testing.T) {
	_, matched := MatchesDenylist("/root/.ssh/id_rsa")
	if !matched {
		t.Error("MatchesDenylist should block /root/.ssh/id_rsa (absolute path, no HOME dependency)")
	}
}

func TestSecurity_Denylist_RootAWS(t *testing.T) {
	_, matched := MatchesDenylist("/root/.aws/credentials")
	if !matched {
		t.Error("MatchesDenylist should block /root/.aws/credentials")
	}
}
