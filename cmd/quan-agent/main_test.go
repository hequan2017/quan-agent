package main

import "testing"

func TestValidateListenAddress(t *testing.T) {
	for _, address := range []string{"127.0.0.1:18090", "[::1]:18090", "localhost:18090"} {
		if err := validateListenAddress(address); err != nil {
			t.Errorf("validateListenAddress(%q) error = %v", address, err)
		}
	}
	for _, address := range []string{"0.0.0.0:18090", "192.168.1.2:18090", ":18090"} {
		if err := validateListenAddress(address); err == nil {
			t.Errorf("validateListenAddress(%q) should fail", address)
		}
	}
}
