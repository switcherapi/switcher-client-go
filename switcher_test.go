package client

import "testing"

func TestSwitcherValidateRequiresRemoteFields(t *testing.T) {
	BuildContext(Context{
		Domain: "My Domain",
	})

	err := GetSwitcher("").Validate()
	if err == nil {
		t.Fatalf("expected validation error")
	}

	expected := "something went wrong: missing or empty required fields (url, component, api_key)"
	if err.Error() != expected {
		t.Fatalf("expected %q, got %q", expected, err.Error())
	}
}

func TestSwitcherValidateWithoutConfiguredClient(t *testing.T) {
	var switcher *Switcher

	err := switcher.Validate()
	if err == nil {
		t.Fatalf("expected validation error")
	}

	expected := "something went wrong: client is not configured"
	if err.Error() != expected {
		t.Fatalf("expected %q, got %q", expected, err.Error())
	}
}

func TestSwitcherValidateRequiresKeyWhenRemoteContextIsValid(t *testing.T) {
	client := NewClient(Context{
		Domain:    "My Domain",
		URL:       "https://api.switcherapi.com",
		APIKey:    "[YOUR_API_KEY]",
		Component: "MyApp",
	})

	err := client.GetSwitcher("").Validate()
	if err == nil {
		t.Fatalf("expected validation error")
	}

	expected := "something went wrong: missing key field"
	if err.Error() != expected {
		t.Fatalf("expected %q, got %q", expected, err.Error())
	}
}

func TestSwitcherValidateSucceedsWhenRemoteContextAndKeyAreValid(t *testing.T) {
	client := NewClient(Context{
		Domain:    "My Domain",
		URL:       "https://api.switcherapi.com",
		APIKey:    "[YOUR_API_KEY]",
		Component: "MyApp",
	})

	err := client.GetSwitcher("FEATURE_LOGIN_V2").Validate()
	if err != nil {
		t.Fatalf("expected validation to succeed, got %v", err)
	}
}
