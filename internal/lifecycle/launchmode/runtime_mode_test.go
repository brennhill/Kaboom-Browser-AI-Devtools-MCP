// Purpose: Tests for runtime mode detection and switching.
// Docs: docs/features/feature/mcp-persistent-server/index.md

package launchmode

import "testing"

func TestSelectRuntimeMode(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		bridge bool
		daemon bool
		want   RuntimeMode
	}{
		{
			name:   "bridge flag wins",
			bridge: true,
			daemon: true,
			want:   RuntimeBridge,
		},
		{
			name:   "daemon flag wins",
			daemon: true,
			want:   RuntimeDaemon,
		},
		{
			name: "defaults to bridge mode",
			want: RuntimeBridge,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := SelectRuntimeMode(tt.bridge, tt.daemon)
			if got != tt.want {
				t.Fatalf("SelectRuntimeMode() = %q, want %q", got, tt.want)
			}
		})
	}
}
