package report

import (
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"html"
	"os"
	"regexp"
	"strings"
	"testing"

	"github.com/fikrigatrh/codebase-flow-mapper/web"
)

// Every inline script must be allowed by exactly its CSP hash, or the browser silently blocks it.
func TestCSPHashesMatchScripts(t *testing.T) {
	var b bytes.Buffer
	if err := Write(&b, []byte(`{"module":"m"}`), "m", "vscode"); err != nil {
		t.Fatal(err)
	}
	page := b.String()
	csp := regexp.MustCompile(`script-src ([^"]*)"`).FindStringSubmatch(page)
	if csp == nil {
		t.Fatal("no script-src in CSP")
	}
	allowed := html.UnescapeString(csp[1]) // html/template entity-escapes ' and + in attributes
	for _, m := range regexp.MustCompile(`(?s)<script>(.*?)</script>`).FindAllStringSubmatch(page, -1) {
		sum := sha256.Sum256([]byte(m[1]))
		if h := "'sha256-" + base64.StdEncoding.EncodeToString(sum[:]) + "'"; !strings.Contains(allowed, h) {
			t.Errorf("inline script (%d bytes) not allowed by CSP", len(m[1]))
		}
	}
	if !strings.Contains(page, `data-editor="vscode"`) || strings.Contains(page, "<script src") {
		t.Error("editor attribute missing or an external script is referenced")
	}
}

func TestRejectsBreakoutSequences(t *testing.T) {
	for _, bad := range []string{`{"x":"</script>"}`, `{"x":"</SCRIPT"}`, `{"x":"<!--"}`} {
		if err := Write(&bytes.Buffer{}, []byte(bad), "m", ""); err == nil {
			t.Errorf("accepted %q", bad)
		}
	}
}

// The vendored files must stay byte-identical to the pins in web/vendor/VERSIONS.md.
func TestVendorPins(t *testing.T) {
	versions, err := os.ReadFile("../../web/vendor/VERSIONS.md")
	if err != nil {
		t.Fatal(err)
	}
	for name, body := range map[string]string{"cytoscape.min.js": web.Cytoscape, "cytoscape-dagre.min.js": web.CytoscapeDagre} {
		sum := sha256.Sum256([]byte(body))
		if !bytes.Contains(versions, []byte(hex.EncodeToString(sum[:]))) {
			t.Errorf("%s does not match its sha256 in VERSIONS.md", name)
		}
	}
}
