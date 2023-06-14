package recording

import (
	"bufio"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"golang.org/x/exp/slog"
	"gopkg.in/dnaeon/go-vcr.v3/cassette"
	"gopkg.in/dnaeon/go-vcr.v3/recorder"
	"gopkg.in/yaml.v3"
)

type recordOptions struct {
	mode recorder.Mode
}

type Options interface {
	Apply(r recordOptions) recordOptions
}

func WithRecordMode(mode recorder.Mode) Options {
	return modeOption{mode: mode}
}

type modeOption struct {
	mode recorder.Mode
}

func (in modeOption) Apply(out recordOptions) recordOptions {
	out.mode = in.mode
	return out
}

const EnvNameKey = "env_name"
const TimeKey = "time"

type Session struct {
	// ProxyUrl is the URL of the proxy server that will be recording or replaying interactions.
	ProxyUrl string

	// If true, playing back from recording.
	Playback bool

	// Variables stored in the session.
	// These variables are automatically set as environment variables for the CLI process under test.
	// See [test/azdcli] for more details.
	Variables map[string]string

	// The recorder proxy server.
	ProxyClient *http.Client
}

// Start starts the recorder proxy, returning a [recording.Session] if recording or playback is enabled.
// In live mode, it returns nil.
//
// By default, the recorder proxy will log errors and info messages.
// The environment variable RECORDER_PROXY_DEBUG can be set to enable debug logging for the recorder proxy.
func Start(t *testing.T, opts ...Options) *Session {
	opt := recordOptions{}
	// for local dev, use recordOnce which will record once if no recording isn't available on disk.
	// if the recording is available, it will playback.
	if os.Getenv("CI") == "" {
		opt.mode = recorder.ModeRecordOnce
	}

	// Set defaults based on AZURE_RECORD_MODE
	if os.Getenv("AZURE_RECORD_MODE") != "" {
		switch strings.ToLower(os.Getenv("AZURE_RECORD_MODE")) {
		case "live":
			opt.mode = recorder.ModePassthrough
		case "playback":
			opt.mode = recorder.ModeReplayOnly
		case "record":
			opt.mode = recorder.ModeRecordOnly
		default:
			t.Fatalf(
				"unsupported AZURE_RECORD_MODE: %s , valid options are: record, live, playback",
				os.Getenv("AZURE_RECORD_MODE"))
		}
	}

	// Apply user-defined options
	for _, o := range opts {
		opt = o.Apply(opt)
	}

	// Return nil for live mode
	if opt.mode == recorder.ModePassthrough {
		return nil
	}

	dir := callingDir(1)
	name := filepath.Join(dir, "testdata", "recordings", t.Name())

	writer := &logWriter{t: t}
	level := slog.LevelInfo
	if os.Getenv("RECORDER_PROXY_DEBUG") != "" {
		level = slog.LevelDebug
	}
	log := slog.New(slog.NewJSONHandler(writer, &slog.HandlerOptions{
		Level: level,
	}))

	session := &Session{}
	recorderOptions := &recorder.Options{
		CassetteName:       name,
		Mode:               opt.mode,
		SkipRequestLatency: true,
	}

	// This also automatically loads the recording.
	vcr, err := recorder.NewWithOptions(recorderOptions)
	if err != nil {
		t.Fatalf("failed to load recordings: %v", err)
	}
	err = initVariables(name+".yaml", &session.Variables)
	if err != nil {
		t.Fatalf("failed to load variables: %v", err)
	}

	if opt.mode == recorder.ModeReplayOnly {
		session.Playback = true
	} else if opt.mode == recorder.ModeRecordOnce && !vcr.IsNewCassette() {
		session.Playback = true
	}

	transport := http.DefaultTransport.(*http.Transport).Clone()
	vcr.SetRealTransport(&gzip2HttpRoundTripper{
		transport: transport,
	})

	vcr.AddHook(func(i *cassette.Interaction) error {
		i.Request.Headers.Set("Authorization", "SANITIZED")
		return nil
	}, recorder.BeforeSaveHook)

	// Fast-forward polling operations
	discarder := httpPollDiscarder{}
	vcr.AddHook(discarder.BeforeSave, recorder.BeforeSaveHook)

	// Trim GET subscriptions-level deployment responses
	vcr.AddHook(func(i *cassette.Interaction) error {
		return TrimSubscriptionsDeployment(i, session.Variables)
	}, recorder.BeforeSaveHook)

	vcr.AddHook(func(i *cassette.Interaction) error {
		if i.DiscardOnSave {
			log.Debug("recorderProxy: discarded response", "url", i.Request.URL, "status", i.Response.Code)
		}
		return nil
	}, recorder.BeforeSaveHook)

	vcr.AddHook(func(i *cassette.Interaction) error {
		if vcr.IsRecording() {
			log.Debug("recorderProxy: recording response", "url", i.Request.URL, "status", i.Response.Code)
		} else {
			log.Debug("recorderProxy: replaying response", "url", i.Request.URL, "status", i.Response.Code)
		}
		return nil
	}, recorder.BeforeResponseReplayHook)

	vcr.AddPassthrough(func(req *http.Request) bool {
		return strings.Contains(req.URL.Host, "login.microsoftonline.com") ||
			strings.Contains(req.URL.Host, "graph.microsoft.com")
	})

	proxy := &connectHandler{
		Log: log,
		HttpHandler: &recorderProxy{
			Log: log,
			Panic: func(msg string) {
				t.Fatal("recorderProxy: " + msg)
			},
			Recorder: vcr,
		},
	}

	server := httptest.NewTLSServer(proxy)
	proxy.TLS = server.TLS
	t.Logf("recorderProxy started with mode %v at %s", displayMode(vcr), server.URL)
	session.ProxyUrl = server.URL

	client, err := proxyClient(server.URL)
	if err != nil {
		t.Fatalf("failed to create proxy client: %v", err)
	}
	session.ProxyClient = client

	t.Cleanup(func() {
		server.Close()
		if !t.Failed() {
			shouldSave := vcr.IsRecording()
			err = vcr.Stop()
			if err != nil {
				t.Fatalf("failed to save recording: %v", err)
			}

			if shouldSave {
				err = saveVariables(recorderOptions.CassetteName+".yaml", session.Variables)
				if err != nil {
					t.Fatalf("failed to save variables: %v", err)
				}
			}
		}
	})

	return session
}

func proxyClient(proxyUrl string) (*http.Client, error) {
	proxyAddr, err := url.Parse(proxyUrl)
	if err != nil {
		return nil, err
	}

	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.Proxy = func(req *http.Request) (*url.URL, error) {
		return proxyAddr, nil
	}
	transport.TLSClientConfig = &tls.Config{InsecureSkipVerify: true}
	client := &http.Client{Transport: transport}
	return client, nil
}

var modeStrMap = map[recorder.Mode]string{
	recorder.ModeRecordOnly: "record",
	recorder.ModeRecordOnce: "recordOnce",

	recorder.ModeReplayOnly:  "replay",
	recorder.ModePassthrough: "live",
}

func displayMode(vcr *recorder.Recorder) string {
	mode := vcr.Mode()
	if mode == recorder.ModeRecordOnce {
		actualMode := "playback"
		if vcr.IsNewCassette() {
			actualMode = "record"
		}
		return fmt.Sprintf("%s (%s)", modeStrMap[mode], actualMode)
	}

	return modeStrMap[mode]
}

// Loads variables from disk, or by initializing default variables if not available.
// When loading from disk, the variables are expected to be the second document in the provided yaml file.
func initVariables(name string, variables *map[string]string) error {
	f, err := os.Open(name)
	if errors.Is(err, os.ErrNotExist) {
		initVars := map[string]string{}
		initVars[TimeKey] = fmt.Sprintf("%d", time.Now().Unix())
		*variables = initVars
		return nil
	}

	if err != nil {
		return fmt.Errorf("failed to load cassette file: %w", err)
	}

	// This implementation uses a buf reader to scan for the second document delimiter for performance.
	// A more robust implementation would use the YAML decoder to scan for the second document.
	r := bufio.NewReader(f)
	docIndex := 0
	for {
		text, err := r.ReadString('\n')
		if text == "---\n" {
			docIndex++
		}

		if docIndex == 2 { // found the second document containing variables
			break
		}

		// EOF
		if err != nil {
			break
		}
	}

	if docIndex != 2 { // no variables
		return nil
	}

	bytes, err := io.ReadAll(r)
	if err != nil {
		return fmt.Errorf("failed to read recording file: %w", err)
	}

	err = yaml.Unmarshal(bytes, &variables)
	if err != nil {
		return fmt.Errorf("failed to parse recording file: %w", err)
	}

	return nil
}

// Saves variables into the named file. The variables are appended as a separate YAML document to the file.
func saveVariables(name string, variables map[string]string) error {
	f, err := os.OpenFile(name, os.O_APPEND|os.O_RDWR, 0755)
	if err != nil {
		return err
	}

	defer f.Close()
	bytes, err := yaml.Marshal(variables)
	if err != nil {
		return err
	}

	// YAML document separator, see http://www.yaml.org/spec/1.2/spec.html#id2760395
	_, err = f.WriteString("---\n")
	if err != nil {
		return err
	}

	_, err = f.Write(bytes)
	if err != nil {
		return fmt.Errorf("failed to write variables: %v", err)
	}

	err = f.Close()
	if err != nil {
		return fmt.Errorf("failed to close file: %v", err)
	}

	return nil
}

func callingDir(skip int) string {
	_, b, _, _ := runtime.Caller(skip + 1)
	return filepath.Dir(b)
}

type logWriter struct {
	t  *testing.T
	sb strings.Builder
}

func (l *logWriter) Write(bytes []byte) (n int, err error) {
	for i, b := range bytes {
		err = l.sb.WriteByte(b)
		if err != nil {
			return i, err
		}

		if b == '\n' {
			l.t.Logf(l.sb.String())
			l.sb.Reset()
		}
	}
	return len(bytes), nil
}
