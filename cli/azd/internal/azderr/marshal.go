package azderr

import (
	"encoding/json"

	"github.com/azure/azure-dev/cli/azd/internal/telemetry/fields"
)

func (s *ServiceDetails) MarshalJSON() ([]byte, error) {
	m := map[string]interface{}{
		string(fields.ServiceName):          s.Name,
		string(fields.ServiceMethod):        s.Method,
		string(fields.ServiceStatusCode):    s.StatusCode,
		string(fields.ServiceCorrelationId): s.CorrelationId,
		string(fields.ServiceResource):      s.Resource,
	}
	return json.Marshal(m)
}

func (t *ToolDetails) MarshalJSON() ([]byte, error) {
	m := map[string]interface{}{
		string(fields.ToolName):     t.Name,
		string(fields.ToolCommand):  t.CmdPath,
		string(fields.ToolExitCode): t.ExitCode,
		string(fields.ToolFlags):    t.Flags,
	}
	return json.Marshal(m)
}
