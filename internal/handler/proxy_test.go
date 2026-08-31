package handler

import (
	"strings"
	"testing"
)

func TestArtifactTicketDoesNotExposeCredential(t *testing.T) {
	token, err := issueArtifactTicket("admin-secret", "user@example.com")
	if err != nil || strings.Contains(token, "admin-secret") {
		t.Fatalf("invalid ticket: %q %v", token, err)
	}
	credential, email, path, ok := artifactAuth(authPrefix + token + "/artifact")
	if !ok || credential != "admin-secret" || email != "user@example.com" || path != "/artifact" {
		t.Fatalf("ticket lookup failed: %q %q %q %v", credential, email, path, ok)
	}
	if cookies := mergeProxyCookies("claude2api_auth=admin-secret; browser=1", "sessionKey=2"); strings.Contains(cookies, "admin-secret") {
		t.Fatalf("auth cookie leaked upstream: %s", cookies)
	}
}
