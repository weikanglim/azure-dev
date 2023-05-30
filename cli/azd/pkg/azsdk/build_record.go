//go:build record

package azsdk

import (
	"net/http"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/policy"
	"github.com/azure/azure-dev/cli/azd/test/recording"
)

type recordingPolicy struct {
	options recording.NonTestRecordingOptions
}

func (r recordingPolicy) Host() string {
	if r.options.UseHTTPS {
		return "localhost:5001"
	}
	return "localhost:5000"
}

func (r recordingPolicy) Scheme() string {
	if r.options.UseHTTPS {
		return "https"
	}
	return "http"
}

func NewRecordingPolicy(o *recording.NonTestRecordingOptions) policy.Policy {
	if o == nil {
		o = &recording.NonTestRecordingOptions{UseHTTPS: true}
	}
	p := &recordingPolicy{options: *o}
	return p
}

func (p *recordingPolicy) Do(req *policy.Request) (resp *http.Response, err error) {
	if recording.GetRecordMode() != "live" {
		p.options.ReplaceAuthority(req.Raw())
	}
	return req.Next()
}

func NewClientOptionsBuilder() *ClientOptionsBuilder {
	builder := &ClientOptionsBuilder{}
	builder.perCallPolicies = append(builder.perCallPolicies, NewRecordingPolicy(nil))
	return builder
}
