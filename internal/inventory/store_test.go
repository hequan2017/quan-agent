package inventory

import "testing"

func TestLooksLikePrivateKey(t *testing.T) {
	valid := "-----BEGIN OPENSSH PRIVATE KEY-----\nabc\n-----END OPENSSH PRIVATE KEY-----"
	if !looksLikePrivateKey(valid) {
		t.Fatal("valid OpenSSH key was rejected")
	}
	if looksLikePrivateKey("ssh-ed25519 AAAA public-key") {
		t.Fatal("public key was accepted as private key")
	}
}

func TestValidHost(t *testing.T) {
	for _, host := range []string{"10.0.0.1", "server.example.com", "localhost", "2001:db8::1"} {
		if !validHost(host) {
			t.Errorf("validHost(%q) = false", host)
		}
	}
	for _, host := range []string{"", "bad host", "-server.example.com", "server_.example.com"} {
		if validHost(host) {
			t.Errorf("validHost(%q) = true", host)
		}
	}
}

func TestNormalizeTags(t *testing.T) {
	tags := normalizeTags([]string{"prod", " prod ", "web", ""})
	if len(tags) != 2 || tags[0] != "prod" || tags[1] != "web" {
		t.Fatalf("normalizeTags() = %#v", tags)
	}
}
