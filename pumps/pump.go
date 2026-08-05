package pumps

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/TykTechnologies/storage/kv/resolver"
	"github.com/TykTechnologies/tyk-pump/analytics"
	"github.com/kelseyhightower/envconfig"
	"github.com/sirupsen/logrus"
)

const PUMPS_ENV_PREFIX = "TYK_PMP_PUMPS"
const PUMPS_ENV_META_PREFIX = "_META"

// uptimeConfEnvPrefix is the prefix the uptime pump is configured with through the
// top level configuration, i.e. TYK_PMP_UPTIMEPUMPCONFIG_MONGOURL.
const uptimeConfEnvPrefix = "TYK_PMP_UPTIMEPUMPCONFIG"

var kvResolver resolver.Resolver

// SetKVResolver installs the resolver used for pump specific env var overrides. Pass
// nil to drop it once every pump is up and its stores are closed.
func SetKVResolver(r resolver.Resolver) {
	kvResolver = r
}

type Pump interface {
	GetName() string
	New() Pump
	Init(interface{}) error
	WriteData(context.Context, []interface{}) error
	SetFilters(analytics.AnalyticsFilters)
	GetFilters() analytics.AnalyticsFilters
	SetTimeout(timeout int)
	GetTimeout() int
	SetOmitDetailedRecording(bool)
	GetOmitDetailedRecording() bool
	GetEnvPrefix() string
	Shutdown() error
	SetMaxRecordSize(size int)
	GetMaxRecordSize() int
	SetLogLevel(logrus.Level)
	SetIgnoreFields([]string)
	GetIgnoreFields() []string
	SetDecodingResponse(bool)
	GetDecodedResponse() bool
	SetDecodingRequest(bool)
	GetDecodedRequest() bool
}

type UptimePump interface {
	GetName() string
	Init(interface{}) error
	WriteUptimeData(data []interface{})
}

func GetPumpByName(name string) (Pump, error) {

	if pump, ok := AvailablePumps[strings.ToLower(name)]; ok && pump != nil {
		return pump, nil
	}

	return nil, errors.New(name + " Not found")
}

func processPumpEnvVars(pump Pump, log *logrus.Entry, cfg any, defaultEnv string) {
	prefix := effectiveEnvPrefix(pump, defaultEnv)

	log.Debug(fmt.Sprintf("Checking %s env variables with prefix %s", pump.GetName(), prefix))

	if overrideErr := envconfig.Process(prefix, cfg); overrideErr != nil {
		log.Error(fmt.Sprintf("Failed to process environment variables for %s pump %s with err:%v ", prefix, pump.GetName(), overrideErr))
	}

	if err := resolveKVReferences(context.Background(), cfg); err != nil {
		log.Fatalf("%s: %v", pump.GetName(), err)
	}
}

// resolveKVReferences dereferences the KV references cfg holds, if any, against the
// stores declared in the configuration. cfg is only rewritten when it actually holds a
// reference, so a configuration that uses none is left exactly as envconfig left it.
func resolveKVReferences(ctx context.Context, cfg any) error {
	raw, err := json.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("encode config: %w", err)
	}

	if !resolver.ContainsReferences(raw) {
		return nil
	}

	if kvResolver == nil {
		return errors.New("a KV reference was set via a pump specific env var but no KV stores are configured")
	}

	resolved, err := kvResolver.ResolveAll(ctx, raw)
	if err != nil {
		return fmt.Errorf("resolve KV references set via pump specific env vars: %w", err)
	}

	if err := json.Unmarshal(resolved, cfg); err != nil {
		return fmt.Errorf("decode resolved config: %w", err)
	}

	return nil
}

// processLegacyPumpEnvVars applies the deprecated, pump specific env var overrides
// identified by legacyPrefix - the naming that pre-dates the TYK_PMP_* coverage of
// the whole configuration. Every variable found is reported as deprecated, pointing
// at its replacementPrefix equivalent.
//
// These variables do not support KV references: they are applied at pump init, after
// the configuration has already been resolved, so a reference set here would reach the
// pump as a literal kv:// string. We fail hard rather than let the pump start with a
// value that only looks like a secret.
func processLegacyPumpEnvVars(pump Pump, log *logrus.Entry, cfg interface{}, legacyPrefix, replacementPrefix string) {
	if names := envVarNamesWithPrefix(legacyPrefix); len(names) > 0 {
		log.Warnf("%s: the %s_* environment variables are deprecated. "+
			"Use their %s_* equivalent instead: %s",
			pump.GetName(), legacyPrefix, replacementPrefix,
			strings.Join(renamedEnvVars(names, legacyPrefix, replacementPrefix), ", "))

		if refs := envVarsWithKVReference(names); len(refs) > 0 {
			log.Fatalf("%s: the deprecated %s_* environment variables do not support KV references, but one was set",
				pump.GetName(), legacyPrefix)
		}
	}

	if err := envconfig.Process(legacyPrefix, cfg); err != nil {
		log.Errorf("Failed to process environment variables for %s pump %s with err: %v", legacyPrefix, pump.GetName(), err)
	}
}

// effectiveEnvPrefix returns the prefix processPumpEnvVars overrides this pump with:
// the meta_env_prefix set in its configuration when present, its default otherwise.
func effectiveEnvPrefix(pump Pump, defaultEnv string) string {
	if prefix := pump.GetEnvPrefix(); prefix != "" {
		return prefix
	}

	return defaultEnv
}

// envVarNamesWithPrefix returns the names of the environment variables set under
// prefix. The separator is part of the match so that, say, PMP_MONGO does not pick
// up the PMP_MONGOAGG variables.
func envVarNamesWithPrefix(prefix string) []string {
	names := make([]string, 0)

	for _, env := range os.Environ() {
		name, _, found := strings.Cut(env, "=")
		if found && strings.HasPrefix(name, prefix+"_") {
			names = append(names, name)
		}
	}

	return names
}

func envVarsWithKVReference(names []string) []string {
	refs := make([]string, 0)

	for _, name := range names {
		if resolver.ContainsReferencesString(os.Getenv(name)) {
			refs = append(refs, name)
		}
	}

	return refs
}

// renamedEnvVars maps each name onto its replacement, e.g. PMP_MONGO_MONGOURL onto
// TYK_PMP_PUMPS_MONGO_META_MONGOURL, to make the deprecation warning actionable.
func renamedEnvVars(names []string, legacyPrefix, replacementPrefix string) []string {
	renamed := make([]string, 0, len(names))

	for _, name := range names {
		renamed = append(renamed, name+" -> "+replacementPrefix+strings.TrimPrefix(name, legacyPrefix))
	}

	return renamed
}
