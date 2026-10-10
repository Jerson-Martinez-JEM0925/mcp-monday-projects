package monday

import (
	"strings"
	"testing"
)

func TestFormatUpdateBodyRendersMarkdownAsMondayHTML(t *testing.T) {
	got := FormatUpdateBody("## Resultado\n\n**OK** y [documentación](https://example.com/docs) con `gcloud get`.")
	want := "<h2>Resultado</h2>\n<p><strong>OK</strong> y <a href=\"https://example.com/docs\">documentación</a> con <code>gcloud get</code>.</p>"
	if got != want {
		t.Fatalf("formatted body = %q, want %q", got, want)
	}
}

func TestFormatUpdateBodyEscapesRawHTMLAndPreservesPlaceholders(t *testing.T) {
	got := FormatUpdateBody("<script>alert(\"x\")</script>\n\n<VM_NAME> [ZONE]")
	if strings.Contains(got, "<script>") || strings.Contains(got, "</script>") {
		t.Fatalf("raw HTML was not escaped: %q", got)
	}
	for _, want := range []string{"&lt;script&gt;", "[ZONE]", "&lt;VM_NAME&gt;"} {
		if !strings.Contains(got, want) {
			t.Fatalf("formatted body %q does not contain %q", got, want)
		}
	}
}

func TestFormatUpdateBodyRendersCodeListsAndBackticks(t *testing.T) {
	input := "- first\n- second\n\n```bash\ngcloud compute instances list --project $PROJECT\n kubectl get pods -A\n```"
	got := FormatUpdateBody(input)
	for _, want := range []string{"<ul><li>first</li><li>second</li></ul>", `<pre><code class="language-bash">`, "gcloud compute instances list --project $PROJECT", "kubectl get pods -A", "</code></pre>"} {
		if !strings.Contains(got, want) {
			t.Fatalf("formatted body %q does not contain %q", got, want)
		}
	}
}

func TestFormatUpdateBodyRendersTablesAndEscapesCells(t *testing.T) {
	input := "| Recurso | Estado |\n| --- | :---: |\n| GKE | <Ready> |"
	got := FormatUpdateBody(input)
	for _, want := range []string{"<table>", "<thead>", "<th>Recurso</th>", `<th align="center">Estado</th>`, "&lt;Ready&gt;", "</table>"} {
		if !strings.Contains(got, want) {
			t.Fatalf("formatted table %q does not contain %q", got, want)
		}
	}
}

func TestFormatUpdateBodyRejectsUnsafeLinksButKeepsText(t *testing.T) {
	got := FormatUpdateBody("[unsafe](javascript:alert(1)) [safe](https://example.com)")
	if strings.Contains(got, `href="javascript:`) {
		t.Fatalf("unsafe link became an href: %q", got)
	}
	if !strings.Contains(got, "unsafe") || !strings.Contains(got, `href="https://example.com"`) {
		t.Fatalf("link text/content was lost: %q", got)
	}
}
