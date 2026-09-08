package main

import (
	"context"
	"flag"
	"os"
	"testing"

	"github.com/matryer/is"
)

func withCleanFlags(t *testing.T, args []string) {
	t.Helper()

	oldArgs := os.Args
	oldCommandLine := flag.CommandLine
	t.Cleanup(func() {
		os.Args = oldArgs
		flag.CommandLine = oldCommandLine
	})

	flag.CommandLine = flag.NewFlagSet(args[0], flag.ContinueOnError)
	os.Args = args
}

// HARM-004: locks all current defaults. Removing a field requires
// proving it has no effect.
func TestDefaultFlags(t *testing.T) {
	is := is.New(t)

	flags := defaultFlags()

	expected := map[flagType]string{
		listenAddress:       "0.0.0.0",
		servicePort:         "8080",
		controlPort:         "8000",
		enableTracing:       "true",
		policiesFile:        "/opt/diwise/config/authz.rego",
		authzAccessObject:   "false",
		configurationFile:   "/opt/diwise/config/config.yaml",
		devicesFile:         "/opt/diwise/config/devices.csv",
		dbHost:              "",
		dbUser:              "",
		dbPassword:          "",
		dbPort:              "5432",
		dbName:              "diwise",
		dbSSLMode:           "disable",
		seedExistingDevices: "true",
		allowedSeedTenants:  "default",
		devmode:             "false",
	}

	is.Equal(len(flags), len(expected))
	for key, want := range expected {
		is.Equal(flags[key], want)
	}
}

// HARM-004: locks env override precedence over defaults.
func TestEnvOverrides(t *testing.T) {
	is := is.New(t)
	withCleanFlags(t, []string{"iot-device-mgmt"})

	t.Setenv("LISTEN_ADDRESS", "127.0.0.1")
	t.Setenv("SERVICE_PORT", "9090")
	t.Setenv("CONTROL_PORT", "9001")
	t.Setenv("ENABLE_TRACING", "false")
	t.Setenv("AUTHZ_ACCESS_OBJECT_ENABLED", "true")
	t.Setenv("ALLOWED_SEED_TENANTS", "a,b")
	t.Setenv("SEED_EXISTING_DEVICES", "false")
	t.Setenv("POSTGRES_HOST", "db")

	_, flags := parseExternalConfig(context.Background(), defaultFlags())

	is.Equal(flags[listenAddress], "127.0.0.1")
	is.Equal(flags[servicePort], "9090")
	is.Equal(flags[controlPort], "9001")
	is.Equal(flags[enableTracing], "false")
	is.Equal(flags[authzAccessObject], "true")
	is.Equal(flags[allowedSeedTenants], "a,b")
	is.Equal(flags[seedExistingDevices], "false")
	is.Equal(flags[dbHost], "db")
}

// HARM-004: locks CLI-over-env precedence. Note: devmode is parsed and
// stored but currently unused after parsing; this test locks the parse
// behavior so a future removal is a deliberate decision.
func TestCLIOverridesEnv(t *testing.T) {
	is := is.New(t)
	withCleanFlags(t, []string{"iot-device-mgmt", "-policies=/tmp/p.rego", "-devmode=true", "-authz-access-object=true"})

	t.Setenv("POLICIES_FILE", "/tmp/env.rego")

	_, flags := parseExternalConfig(context.Background(), defaultFlags())

	is.Equal(flags[policiesFile], "/tmp/p.rego")
	is.Equal(flags[devmode], "true")
	is.Equal(flags[authzAccessObject], "true")
}

// REV-015: the two bool toggles intentionally use different
// interpretations. Tests target the production seams so a changed
// interpretation breaks them.
func TestBoolToggleInterpretations(t *testing.T) {
	for _, tc := range []struct {
		name    string
		value   string
		tracing bool
		seed    bool
	}{
		{"exact true", "true", true, true},
		{"uppercase TRUE", "TRUE", false, true},
		{"numeric 1", "1", false, true},
		{"false", "false", false, false},
		{"numeric 0", "0", false, false},
		{"empty", "", false, false},
		{"invalid", "bogus", false, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			is := is.New(t)

			flags := defaultFlags()
			flags[enableTracing] = tc.value
			flags[seedExistingDevices] = tc.value

			is.Equal(tracingEnabled(flags), tc.tracing)
			is.Equal(seedExistingDevicesEnabled(flags), tc.seed)
		})
	}
}
