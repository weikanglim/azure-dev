// Copyright (c) Microsoft Corporation. All rights reserved.
// Licensed under the MIT License.

package tracing

import (
	"testing"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel/attribute"
)

func Test_sanitize(t *testing.T) {
	tests := []struct {
		name string
		arg  []attribute.KeyValue
		want []attribute.KeyValue
	}{
		{
			"non secret",
			[]attribute.KeyValue{attribute.String("k", "tokenizer: blue")},
			[]attribute.KeyValue{attribute.String("k", "tokenizer: blue")},
		},
		{
			"non secret slice",
			[]attribute.KeyValue{attribute.StringSlice("k", []string{"notASecret", "aSecretIsNotHere"})},
			[]attribute.KeyValue{attribute.StringSlice("k", []string{"notASecret", "aSecretIsNotHere"})},
		},
		{
			"secret",
			[]attribute.KeyValue{attribute.String("k", "clientSecret:mySecret")},
			[]attribute.KeyValue{attribute.String("k", "<REDACTED: secret>")},
		},
		{
			"secret slice",
			[]attribute.KeyValue{attribute.StringSlice("k", []string{
				"clientSecret:\"mySecret\"",
				"refreshToken=myToken",
				"accessToken=myToken",
				"password : 'myPassword'",
			})},
			[]attribute.KeyValue{attribute.StringSlice("k", []string{
				"<REDACTED: secret>",
				"<REDACTED: secret>",
				"<REDACTED: secret>",
				"<REDACTED: secret>",
			})},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sanitize(tt.arg)
			require.Equal(t, tt.want, tt.arg)
		})
	}
}
