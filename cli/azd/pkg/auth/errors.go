package auth

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"

	msal "github.com/AzureAD/microsoft-authentication-library-for-go/apps/errors"
)

const authFailedPrefix string = "failed to authenticate"

// unwrapResponse retrieves the response carried by
// an AuthenticationFailedError or MSAL CallErr, if any
func unwrapResponse(err error) *http.Response {
	var a *AuthFailedError
	var c msal.CallErr
	var res *http.Response
	if errors.As(err, &c) {
		res = c.Resp
	} else if errors.As(err, &a) {
		res = a.rawResp
	}
	return res
}

// An error response from Azure Active Directory.
// See https://www.rfc-editor.org/rfc/rfc6749#section-5.2 for OAuth 2.0 spec
// See https://learn.microsoft.com/en-us/azure/active-directory/develop/reference-aadsts-error-codes#handling-error-codes-in-your-application for AAD error spec
type AadErrorResponse struct {
	Error            string `json:"error"`
	ErrorDescription string `json:"error_description"`
	ErrorCodes       []int  `json:"error_codes"`
	Timestamp        string `json:"timestamp"`
	TraceId          string `json:"trace_id"`
	CorrelationId    string `json:"correlation_id"`
	ErrorUri         string `json:"error_uri"`
}

// AuthFailedError indicates an authentication request has failed.
type AuthFailedError struct {
	// rawResp is the HTTP response motivating the error.
	rawResp     *http.Response
	respErrCode string
	// Underlying error, if applicable
	err error

	// The parsed error response
	adError *AadErrorResponse
}

func newAuthFailedErrorFromResp(resp *http.Response) error {
	if resp == nil {
		return errors.New(authFailedPrefix)
	}

	e := &AuthFailedError{rawResp: resp}
	e.parseResponse()
	return e
}

func newAuthFailedError(
	err error) error {
	res := unwrapResponse(err)
	if res == nil { // no response available, provide error wrapping
		return fmt.Errorf("%s: %w", authFailedPrefix, err)
	}

	return newAuthFailedErrorFromResp(res)
}

func (e *AuthFailedError) parseResponse() {
	body, err := io.ReadAll(e.rawResp.Body)
	e.rawResp.Body.Close()
	if err != nil {
		log.Printf("error reading aad response body: %v", err)
		return
	}
	e.rawResp.Body = io.NopCloser(bytes.NewReader(body))

	var er AadErrorResponse
	if err := json.Unmarshal(body, &er); err != nil {
		log.Printf("parsing aad response body: %v", err)
		return
	}

	e.adError = &er
}

func (e *AuthFailedError) Unwrap() error {
	return e.err
}

func (e *AuthFailedError) Error() string {
	// first, log the error to debug logs
	if e.rawResp != nil {
		e.logError()
	}

	// if no response was available, return with the inner wrapped error
	if e.rawResp == nil {
		if e.err != nil {
			return fmt.Sprintf("%s: %s", authFailedPrefix, e.err)
		}
		return authFailedPrefix
	}

	// http response could not be unmarshalled, include the HTTP status code
	if e.adError == nil {
		sb := strings.Builder{}
		sb.WriteString(authFailedPrefix)
		sb.WriteString("\n\nAuthentication error\n")
		sb.WriteString(fmt.Sprintf("%s %s://%s%s\n",
			e.rawResp.Request.Method, e.rawResp.Request.URL.Scheme, e.rawResp.Request.URL.Host, e.rawResp.Request.URL.Path))
		sb.WriteString(fmt.Sprintf("Response: %s", e.rawResp.Status))
		return sb.String()
	}

	sb := strings.Builder{}
	sb.WriteString(authFailedPrefix)
	sb.WriteString("\n\nAuthentication error\n")
	sb.WriteString(fmt.Sprintf("(%s) %s\n", e.adError.Error, e.adError.ErrorDescription))
	// sb.WriteString(fmt.Sprintf("TraceId: %s\n", e.adError.TraceId))
	// sb.WriteString(fmt.Sprintf("CorrelationId: %s\n", e.adError.CorrelationId))
	// sb.WriteString("Error codes: ")
	// for i, code := range e.adError.ErrorCodes {
	// 	sb.WriteString(fmt.Sprintf("%d", code))
	// 	switch i {
	// 	case len(e.adError.ErrorCodes) - 1:
	// 		sb.WriteString("\n")
	// 	default:
	// 		sb.WriteString(", ")
	// 	}
	// }
	return sb.String()
}

// Log error to debugging logs.
func (e *AuthFailedError) logError() {
	msg := &bytes.Buffer{}
	log.Printf("authentication failed\n")
	log.Printf("%s %s://%s%s\n",
		e.rawResp.Request.Method, e.rawResp.Request.URL.Scheme, e.rawResp.Request.URL.Host, e.rawResp.Request.URL.Path)
	log.Println("--------------------------------------------------------------------------------")
	log.Printf("RESPONSE %s\n", e.rawResp.Status)
	log.Println("--------------------------------------------------------------------------------")
	body, err := io.ReadAll(e.rawResp.Body)
	e.rawResp.Body.Close()
	if err != nil {
		log.Printf("Error reading response body: %v", err)
	} else if len(body) > 0 {
		e.rawResp.Body = io.NopCloser(bytes.NewReader(body))
		if err := json.Indent(msg, body, "", "  "); err != nil {
			// failed to pretty-print so just dump it verbatim
			fmt.Fprint(msg, string(body))
		}
	} else {
		fmt.Fprint(msg, "Response contained no body")
	}
	fmt.Fprintln(msg, "\n--------------------------------------------------------------------------------")
	log.Print(msg.String())
}
