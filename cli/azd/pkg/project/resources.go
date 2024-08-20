package project

import (
	"fmt"

	"gopkg.in/yaml.v3"
)

type Resources map[string]*ResourceConfig

type ResourceType string

const (
	ResourceTypeDbRedis          ResourceType = "db.redis"
	ResourceTypeDbPostgres       ResourceType = "db.postgres"
	ResourceTypeDbMongo          ResourceType = "db.mongo"
	ResourceTypeHostContainerApp ResourceType = "host.containerapp"
)

func (r ResourceType) String() string {
	switch r {
	case ResourceTypeDbRedis:
		return "Redis"
	case ResourceTypeDbPostgres:
		return "PostgreSQL"
	case ResourceTypeDbMongo:
		return "MongoDB"
	case ResourceTypeHostContainerApp:
		return "Container App"
	}

	return ""
}

func AllResources() []ResourceType {
	return []ResourceType{
		ResourceTypeDbRedis,
		ResourceTypeDbPostgres,
		ResourceTypeDbMongo,
		ResourceTypeHostContainerApp,
	}
}

type ResourceConfig struct {
	// Reference to the parent project configuration
	Project *ProjectConfig `yaml:"-"`
	// Type of service
	Type ResourceType `yaml:"type"`
	// The friendly name/key of the project from the azure.yaml file
	Name string `yaml:"-"`
	// The properties for the resource
	RawProps map[string]yaml.Node `yaml:",inline"`
	Props    interface{}          `yaml:"-"`
	// The optional bicep module override for the resource
	Module string `yaml:"module,omitempty"`
	// Relationships to other resources
	Uses []string `yaml:"uses,omitempty"`
}

func (r *ResourceConfig) MarshalYAML() (interface{}, error) {
	type rawResourceConfig ResourceConfig
	raw := rawResourceConfig(*r)

	var marshalRawProps = func(in interface{}) error {
		marshaled, err := yaml.Marshal(in)
		if err != nil {
			return fmt.Errorf("marshaling props: %w", err)
		}

		props := map[string]yaml.Node{}
		if err := yaml.Unmarshal(marshaled, &props); err != nil {
			return err
		}
		raw.RawProps = props
		return nil
	}

	switch raw.Type {
	case ResourceTypeHostContainerApp:
		err := marshalRawProps(raw.Props.(ContainerAppProps))
		if err != nil {
			return nil, err
		}
	}

	return raw, nil
}

func (r *ResourceConfig) UnmarshalYAML(value *yaml.Node) error {
	type rawResourceConfig ResourceConfig
	raw := rawResourceConfig{}
	if err := value.Decode(&raw); err != nil {
		return err
	}

	var unmarshalProps = func(v interface{}) error {
		value, err := yaml.Marshal(raw.RawProps)
		if err != nil {
			panic("failed to marshal raw props")
		}

		if err := yaml.Unmarshal(value, v); err != nil {
			return err
		}

		return nil
	}

	// Unmarshal props based on type
	switch raw.Type {
	case ResourceTypeHostContainerApp:
		cap := ContainerAppProps{}
		if err := unmarshalProps(&cap); err != nil {
			return err
		}
		raw.Props = cap
	}

	*r = ResourceConfig(raw)
	return nil
}

func (r *ResourceConfig) DefaultModule() (bicepModule string, bicepVersion string) {
	switch r.Type {
	case ResourceTypeDbMongo:
		bicepModule = "avm/res/document-db/database-account"
		bicepVersion = "0.4.0"
	case ResourceTypeDbPostgres:
		bicepModule = "avm/res/db-for-postgre-sql/flexible-server"
		bicepVersion = "0.1.6"
	case ResourceTypeDbRedis:
		bicepModule = "avm/res/cache/redis"
		bicepVersion = "0.3.2"
	case ResourceTypeHostContainerApp:
		bicepModule = "avm/res/app/container-app"
		bicepVersion = "0.8.0"
	default:
		panic(fmt.Sprintf("unsupported resource type %s", r.Type))
	}
	return
}

type ContainerAppResource struct {
	ResourceConfig
}

// TODO(weilim): We can probably allow for a container app override here.
type ContainerAppProps struct {
	Port int             `yaml:"port,omitempty"`
	Env  []ServiceEnvVar `yaml:"env,omitempty"`
}
