package webcheck

import "testing"

func TestExtractSocialsFindsCompanyLinks(t *testing.T) {
	html := `<html><body>
		<a href="https://www.linkedin.com/shareArticle?mini=true">share</a>
		<a href="https://linkedin.com/company/acme-widgets">LinkedIn</a>
		<a href="https://twitter.com/intent/tweet?text=hi">tweet</a>
		<a href="https://x.com/acmewidgets">X</a>
		<a href="https://www.facebook.com/acmewidgets">Facebook</a>
	</body></html>`
	s := ExtractSocials(html)
	if s.LinkedIn != "https://linkedin.com/company/acme" && s.LinkedIn != "https://linkedin.com/company/acme-widgets" {
		t.Fatalf("linkedin: %+v", s)
	}
	if s.Twitter != "https://x.com/acmewidgets" {
		t.Fatalf("twitter: %+v", s)
	}
	if s.Facebook != "https://www.facebook.com/acmewidgets" {
		t.Fatalf("facebook: %+v", s)
	}
}

func TestExtractSocialsEmpty(t *testing.T) {
	if s := ExtractSocials(`<p>no links</p>`); s != (SocialLinks{}) {
		t.Fatalf("expected empty, got %+v", s)
	}
}
