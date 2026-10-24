package defaults

import "testing"

func TestPublishedImage(t *testing.T) {
	cases := map[string]string{
		ImageLocal:                  ImageBaseRef,
		"quarshell-sandbox:base":    ImageBaseRef,
		ImageGUI:                    ImageGUIRef,
		ImageGPU:                    ImageGPURef,
		ImageGUIGPU:                 ImageGUIGPURef,
		ImageCursor:                 ImageCursorRef,
		" quarshell-Sandbox:Claude": ImageClaudeRef,
		ImageCodex:                  ImageCodexRef,
	}
	for in, want := range cases {
		got, ok := PublishedImage(in)
		if !ok || got != want {
			t.Fatalf("PublishedImage(%q) = %q, %v; want %q", in, got, ok, want)
		}
	}
	for _, in := range []string{"", "debian:bookworm", "quarshell-sandbox:mine", "ghcr.io/klorq/quarshell/sandboxes/base:latest"} {
		if got, ok := PublishedImage(in); ok {
			t.Fatalf("PublishedImage(%q) = %q, want no mapping", in, got)
		}
	}
}
