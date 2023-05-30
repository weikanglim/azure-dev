package recording

import (
	"fmt"
	"net/http"
	"os"
)

var recordingId string

func init() {
	recordingId = os.Getenv("AZURE_RECORD_ID")
}

type NonTestRecordingOptions struct {
	UseHTTPS bool
}

func (r NonTestRecordingOptions) ReplaceAuthority(rawReq *http.Request) *http.Request {
	if GetRecordMode() != LiveMode {
		originalURLHost := rawReq.URL.Host

		// don't modify the original request
		cp := *rawReq
		cpURL := *cp.URL
		cp.URL = &cpURL
		cp.Header = rawReq.Header.Clone()

		cp.URL.Scheme = r.scheme()
		cp.URL.Host = r.host()
		cp.Host = r.host()

		cp.Header.Set(UpstreamURIHeader, fmt.Sprintf("%v://%v", r.scheme(), originalURLHost))
		cp.Header.Set(ModeHeader, GetRecordMode())
		cp.Header.Set(IDHeader, recordingId)
		rawReq = &cp
	}
	return rawReq
}

func (r NonTestRecordingOptions) host() string {
	if r.UseHTTPS {
		return "localhost:5001"
	}
	return "localhost:5000"
}

func (r NonTestRecordingOptions) scheme() string {
	if r.UseHTTPS {
		return "https"
	}
	return "http"
}

func (r NonTestRecordingOptions) baseURL() string {
	return fmt.Sprintf("%s://%s", r.scheme(), r.host())
}
