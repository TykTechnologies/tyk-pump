package pumps

import (
	"context"
	"fmt"
	"log/syslog"
	"strings"

	"github.com/mitchellh/mapstructure"

	"github.com/TykTechnologies/tyk-pump/analytics"
)

type SyslogPump struct {
	syslogConf *SyslogConf
	writer     *syslog.Writer
	filters    analytics.AnalyticsFilters
	timeout    int
	CommonPumpConfig
}

var (
	syslogPrefix     = "syslog-pump"
	syslogDefaultENV = PUMPS_ENV_PREFIX + "_SYSLOG" + PUMPS_ENV_META_PREFIX
)

// @PumpConf Syslog
type SyslogConf struct {
	// The prefix for the environment variables that will be used to override the configuration.
	// Defaults to `TYK_PMP_PUMPS_SYSLOG_META`
	EnvPrefix string `json:"meta_env_prefix" mapstructure:"meta_env_prefix"`
	// Possible values are `udp, tcp, tls` in string form.
	Transport string `json:"transport" mapstructure:"transport"`
	// Host & Port combination of your syslog daemon ie: `"localhost:5140"`.
	NetworkAddr string `json:"network_addr" mapstructure:"network_addr"`
	// The severity level, an integer from 0-7, based off the Standard:
	// [Syslog Severity Levels](https://en.wikipedia.org/wiki/Syslog#Severity_level).
	LogLevel int `json:"log_level" mapstructure:"log_level"`
	// Prefix tag
	//
	// When working with FluentD, you should provide a
	// [FluentD Parser](https://docs.fluentd.org/input/syslog) based on the OS you are using so
	// that FluentD can correctly read the logs.
	//
	// ```{.json}
	// "syslog": {
	//   "name": "syslog",
	//   "meta": {
	//     "transport": "udp",
	//     "network_addr": "localhost:5140",
	//     "log_level": 6,
	//     "tag": "syslog-pump"
	//   }
	// ```
	Tag string `json:"tag" mapstructure:"tag"`
	// If set to `true`, includes the `tags` field from the analytics record in the output.
	// Defaults to `false`, so the emitted message is unchanged when upgrading.
	//
	// Tags are unbounded in length. Over the default `udp` transport a record with a large
	// tag set can exceed syslog datagram limits and truncate the fields that sort after
	// `tags` — prefer `tcp` or `tls` when enabling this.
	IncludeTags bool `json:"include_tags" mapstructure:"include_tags"`
}

func (s *SyslogPump) GetName() string {
	return "Syslog Pump"
}

func (s *SyslogPump) New() Pump {
	newPump := SyslogPump{}
	return &newPump
}

func (s *SyslogPump) GetEnvPrefix() string {
	return s.syslogConf.EnvPrefix
}

func (s *SyslogPump) Init(config interface{}) error {
	//Read configuration file
	s.syslogConf = &SyslogConf{}
	s.log = log.WithField("prefix", syslogPrefix)

	err := mapstructure.Decode(config, &s.syslogConf)
	if err != nil {
		s.log.Fatal("Failed to decode configuration: ", err)
	}

	processPumpEnvVars(s, s.log, s.syslogConf, syslogDefaultENV)
	// Init the configs
	s.initConfigs()

	// Init the Syslog writer
	s.initWriter()

	s.log.Info(s.GetName() + " Initialized")

	return nil
}

func (s *SyslogPump) initWriter() {
	tag := syslogPrefix
	if s.syslogConf.Tag != "" {
		tag = s.syslogConf.Tag
	}
	syslogWriter, err := syslog.Dial(
		s.syslogConf.Transport,
		s.syslogConf.NetworkAddr,
		syslog.Priority(s.syslogConf.LogLevel),
		tag)

	if err != nil {
		s.log.Fatal("failed to connect to Syslog Daemon: ", err)
	}

	s.writer = syslogWriter
}

// Set default values if they are not explicitly given
// And perform validation
func (s *SyslogPump) initConfigs() {
	if s.syslogConf.Transport == "" {
		s.syslogConf.Transport = "udp"
		s.log.Info("No Transport given, using 'udp'")
	}

	if s.syslogConf.Transport != "udp" &&
		s.syslogConf.Transport != "tcp" &&
		s.syslogConf.Transport != "tls" {
		s.log.Fatal("Chosen invalid Transport type.  Please use a supported Transport type for Syslog")
	}

	if s.syslogConf.NetworkAddr == "" {
		s.syslogConf.NetworkAddr = "localhost:5140"
		s.log.Info("No host given, using 'localhost:5140'")
	}

	if s.syslogConf.LogLevel == 0 {
		s.log.Warn("Using Log Level 0 (KERNEL) for Syslog pump")
	}

	// Tags are unbounded and sort before timestamp and user_agent, so on a datagram
	// transport a large tag set silently truncates fields the pump emits today.
	// Worth a line at startup: the failure is silent at the point it happens.
	if s.syslogConf.IncludeTags && s.syslogConf.Transport == "udp" {
		s.log.Warn("include_tags is enabled on the 'udp' transport: records with " +
			"large tag sets may exceed the syslog datagram limit and lose the " +
			"fields that sort after 'tags'. Consider 'tcp' or 'tls'.")
	}
}

/**
** Write the actual Data to Syslog Here
 */
func (s *SyslogPump) WriteData(ctx context.Context, data []interface{}) error {
	s.log.Debug("Attempting to write ", len(data), " records...")

	//Data is all the analytics being written
	for _, v := range data {
		select {
		case <-ctx.Done():
			return nil
		default:
			// Decode the raw analytics into Form
			decoded := v.(analytics.AnalyticsRecord)

			// Escape newlines in raw_request and raw_response to prevent log fragmentation
			// while maintaining the original map format for backward compatibility
			escapedRawRequest := strings.ReplaceAll(decoded.RawRequest, "\n", "\\n")
			escapedRawResponse := strings.ReplaceAll(decoded.RawResponse, "\n", "\\n")

			message := Json{
				"timestamp":       decoded.TimeStamp,
				"method":          decoded.Method,
				"path":            decoded.Path,
				"raw_path":        decoded.RawPath,
				"response_code":   decoded.ResponseCode,
				"alias":           decoded.Alias,
				"api_key":         decoded.APIKey,
				"api_version":     decoded.APIVersion,
				"api_name":        decoded.APIName,
				"api_id":          decoded.APIID,
				"org_id":          decoded.OrgID,
				"oauth_id":        decoded.OauthID,
				"raw_request":     escapedRawRequest,
				"request_time_ms": decoded.RequestTime,
				"raw_response":    escapedRawResponse,
				"ip_address":      decoded.IPAddress,
				"host":            decoded.Host,
				"content_length":  decoded.ContentLength,
				"user_agent":      decoded.UserAgent,
			}

			if s.syslogConf.IncludeTags {
				message["tags"] = escapeTags(decoded.Tags)
			}

			// Print to Syslog using original map format (maintains backward compatibility)
			_, _ = fmt.Fprintf(s.writer, "%s", message)
		}
	}
	s.log.Info("Purged ", len(data), " records...")

	return nil
}

const syslogHexDigits = "0123456789abcdef"

// hasControlChars reports whether s contains a C0 control character or DEL.
//
// Scanned byte-wise rather than rune-wise, which is safe here: every byte of a
// multi-byte UTF-8 sequence is >= 0x80, so no control byte can be part of one.
func hasControlChars(s string) bool {
	for i := 0; i < len(s); i++ {
		if s[i] < 0x20 || s[i] == 0x7f {
			return true
		}
	}

	return false
}

// appendEscapedTag appends s to dst with control characters replaced by their
// printable escape sequences.
func appendEscapedTag(dst []byte, s string) []byte {
	for i := 0; i < len(s); i++ {
		c := s[i]

		if c >= 0x20 && c != 0x7f {
			dst = append(dst, c)

			continue
		}

		switch c {
		case '\n':
			dst = append(dst, `\n`...)
		case '\r':
			dst = append(dst, `\r`...)
		case '\t':
			dst = append(dst, `\t`...)
		case '\b':
			dst = append(dst, `\b`...)
		case '\f':
			dst = append(dst, `\f`...)
		case '\v':
			dst = append(dst, `\v`...)
		default:
			dst = append(dst, `\x`...)
			dst = append(dst, syslogHexDigits[c>>4], syslogHexDigits[c&0x0f])
		}
	}

	return dst
}

// escapeTags replaces control characters in tag values with printable escape
// sequences, so that a tag cannot alter the log stream it is written into.
//
// Tags are user-supplied and reach the log verbatim, and the characters cause
// distinct problems. A newline splits one record across two syslog lines, which a
// collector reads as two records -- the second malformed. A carriage return or
// backspace does not split the record, but moves the cursor back in terminals and
// log viewers that act on it, so following text overwrites what came before: a tag
// can be crafted to hide the rest of the record from whoever is reading the log.
// An ESC begins an ANSI sequence, which can recolour or clear the reader's screen.
// A NUL truncates the record in consumers that treat it as a string terminator,
// which includes C-based daemons such as rsyslog.
//
// The pumps that already emit this field -- Elasticsearch, Kafka, Kinesis, Moesif
// -- encode the record as JSON, and Go's JSON encoder escapes every one of these
// characters. Escaping the whole C0 range plus DEL therefore brings Syslog into
// line with them rather than inventing a rule for this pump.
//
// This is stricter than the treatment raw_request and raw_response get, which
// escape \n only. Widening those would change bytes this pump already emits, so it
// is left to its own change; tags are new and opt-in, so nothing existing moves.
//
// The escaping is defensive, not reversible: a tag containing the literal two
// characters \n is indistinguishable from one containing a newline, exactly as it
// already was.
//
// Multi-byte Unicode separators such as U+2028 and U+0085 are left alone. Syslog
// framing is byte-oriented -- a record ends at an LF or at an octet count -- and the
// UTF-8 encodings of those runes contain no 0x0A, so they cannot end a record.
// A Unicode-aware viewer further downstream may still render them as a break.
//
// Returns the input slice unchanged when nothing needs escaping, so the common path
// allocates nothing.
func escapeTags(tags []string) []string {
	needsEscaping := false

	for _, tag := range tags {
		if hasControlChars(tag) {
			needsEscaping = true

			break
		}
	}

	if !needsEscaping {
		return tags
	}

	escaped := make([]string, len(tags))
	buf := make([]byte, 0, 64)

	for i, tag := range tags {
		// Deliberately re-scanned: the loop above stops at the first dirty tag, so
		// this is the only thing keeping clean tags in a dirty slice from being
		// copied. Not a leftover from the scan above.
		if !hasControlChars(tag) {
			escaped[i] = tag

			continue
		}

		buf = appendEscapedTag(buf[:0], tag)
		escaped[i] = string(buf)
	}

	return escaped
}

func (s *SyslogPump) SetTimeout(timeout int) {
	s.timeout = timeout
}

func (s *SyslogPump) GetTimeout() int {
	return s.timeout
}

func (s *SyslogPump) SetFilters(filters analytics.AnalyticsFilters) {
	s.filters = filters
}
func (s *SyslogPump) GetFilters() analytics.AnalyticsFilters {
	return s.filters
}
