package shared

import "testing"

func TestValidateSharePath(t *testing.T) {
	tests := []struct {
		name          string
		path          string
		allowedShares []string
		wantErr       bool
	}{
		// --- Windows UNC / SMB paths ---
		{
			name:          "not a share path (local drive)",
			path:          `C:\foo\bar`,
			allowedShares: []string{`\\NAS01\media`},
			wantErr:       true,
		},
		{
			name:          "UNC valid prefix match",
			path:          `\\NAS01\media\video.mkv`,
			allowedShares: []string{`\\NAS01\media`},
			wantErr:       false,
		},
		{
			name:          "UNC path not in allowed list",
			path:          `\\NAS02\other\file.mkv`,
			allowedShares: []string{`\\NAS01\media`},
			wantErr:       true,
		},
		{
			name:          "UNC case insensitive prefix matching",
			path:          `\\NAS01\MEDIA\video.mkv`,
			allowedShares: []string{`\\NAS01\media`},
			wantErr:       false,
		},
		{
			name:          "UNC case insensitive prefix matching reversed",
			path:          `\\NAS01\media\video.mkv`,
			allowedShares: []string{`\\NAS01\MEDIA`},
			wantErr:       false,
		},
		{
			name:          "empty allowed shares list",
			path:          `\\NAS01\media\video.mkv`,
			allowedShares: []string{},
			wantErr:       true,
		},
		{
			name:          "nil allowed shares list",
			path:          `\\NAS01\media\video.mkv`,
			allowedShares: nil,
			wantErr:       true,
		},
		{
			name:          "UNC multiple allowed shares second one matches",
			path:          `\\NAS02\archive\old.mkv`,
			allowedShares: []string{`\\NAS01\media`, `\\NAS02\archive`},
			wantErr:       false,
		},
		{
			name:          "UNC multiple allowed shares first one matches",
			path:          `\\NAS01\media\new.mkv`,
			allowedShares: []string{`\\NAS01\media`, `\\NAS02\archive`},
			wantErr:       false,
		},
		// --- POSIX / NFS mount paths ---
		{
			name:          "NFS absolute path valid",
			path:          "/mnt/nas/media/video.mkv",
			allowedShares: []string{"/mnt/nas/media"},
			wantErr:       false,
		},
		{
			name:          "NFS absolute path not in allowed list",
			path:          "/mnt/nas/other/file.mkv",
			allowedShares: []string{"/mnt/nas/media"},
			wantErr:       true,
		},
		{
			name:          "NFS paths are case sensitive",
			path:          "/mnt/nas/MEDIA/video.mkv",
			allowedShares: []string{"/mnt/nas/media"},
			wantErr:       true,
		},
		{
			name:          "NFS path matches second allowed share",
			path:          "/mnt/encodes/output.mkv",
			allowedShares: []string{"/mnt/nas/media", "/mnt/encodes"},
			wantErr:       false,
		},
		{
			name:          "NFS root path rejected when no allowed shares",
			path:          "/mnt/nas/file.mkv",
			allowedShares: nil,
			wantErr:       true,
		},
		// --- Mixed allowed_shares (UNC + NFS on same agent) ---
		{
			name:          "UNC path matches in mixed allowed list",
			path:          `\\NAS01\media\video.mkv`,
			allowedShares: []string{`\\NAS01\media`, "/mnt/nas/media"},
			wantErr:       false,
		},
		{
			name:          "NFS path matches in mixed allowed list",
			path:          "/mnt/nas/media/video.mkv",
			allowedShares: []string{`\\NAS01\media`, "/mnt/nas/media"},
			wantErr:       false,
		},
		// --- Invalid formats ---
		{
			name:          "relative path rejected",
			path:          "foo/bar/file.mkv",
			allowedShares: []string{"/mnt/nas"},
			wantErr:       true,
		},
		{
			name:          "empty path rejected",
			path:          "",
			allowedShares: []string{"/mnt/nas"},
			wantErr:       true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateSharePath(tt.path, tt.allowedShares)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateSharePath(%q, %v) error = %v, wantErr %v",
					tt.path, tt.allowedShares, err, tt.wantErr)
			}
		})
	}
}

func TestIsSharePath(t *testing.T) {
	tests := []struct {
		name string
		path string
		want bool
	}{
		// UNC paths
		{
			name: "UNC path with server and share",
			path: `\\NAS01\media\file.mkv`,
			want: true,
		},
		{
			name: "UNC path bare backslashes",
			path: `\\`,
			want: true,
		},
		// POSIX / NFS paths
		{
			name: "NFS absolute path",
			path: "/mnt/nas/media/file.mkv",
			want: true,
		},
		{
			name: "POSIX root",
			path: "/",
			want: true,
		},
		// Invalid
		{
			name: "local drive path",
			path: `C:\foo\bar`,
			want: false,
		},
		{
			name: "relative path",
			path: `foo\bar`,
			want: false,
		},
		{
			name: "single backslash",
			path: `\foo`,
			want: false,
		},
		{
			name: "empty string",
			path: "",
			want: false,
		},
		{
			name: "forward slashes without leading slash",
			path: "server/share",
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsSharePath(tt.path)
			if got != tt.want {
				t.Errorf("IsSharePath(%q) = %v, want %v", tt.path, got, tt.want)
			}
		})
	}
}

func TestIsUNCPath_Backward(t *testing.T) {
	// IsUNCPath is deprecated but must continue to return true only for UNC paths.
	if !IsUNCPath(`\\NAS01\media\file.mkv`) {
		t.Error("IsUNCPath should still return true for UNC paths")
	}
	if IsUNCPath("/mnt/nas/file.mkv") {
		t.Error("IsUNCPath should return false for POSIX paths (use IsSharePath instead)")
	}
}
