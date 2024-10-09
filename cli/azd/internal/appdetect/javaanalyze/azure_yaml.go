package javaanalyze

type AzureYaml struct {
	Service         *Service         `json:"service"`
	Resources       []IResource      `json:"resources"`
	ServiceBindings []ServiceBinding `json:"serviceBindings"`
}

type IResource interface {
	GetName() string
	GetType() string
	GetBicepParameters() []BicepParameter
	GetBicepProperties() []BicepProperty
}

type Resource struct {
	Name            string           `json:"name"`
	Type            string           `json:"type"`
	BicepParameters []BicepParameter `json:"bicepParameters"`
	BicepProperties []BicepProperty  `json:"bicepProperties"`
}

func (r *Resource) GetName() string {
	return r.Name
}

func (r *Resource) GetType() string {
	return r.Type
}

func (r *Resource) GetBicepParameters() []BicepParameter {
	return r.BicepParameters
}

func (r *Resource) GetBicepProperties() []BicepProperty {
	return r.BicepProperties
}

type ServiceBusResource struct {
	Resource
	Queues                []string `json:"queues"`
	TopicAndSubscriptions []string `json:"topicAndSubscriptions"`
}

type BicepParameter struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Type        string `json:"type"`
}

type BicepProperty struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Type        string `json:"type"`
}

type ResourceType int32

const (
	RESOURCE_TYPE_MYSQL         ResourceType = 0
	RESOURCE_TYPE_AZURE_STORAGE ResourceType = 1
)

// Service represents a specific service's configuration.
type Service struct {
	Name        string        `json:"name"`
	Path        string        `json:"path"`
	ResourceURI string        `json:"resourceUri"`
	Description string        `json:"description"`
	Environment []Environment `json:"environment"`
}

type Environment struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

type ServiceBinding struct {
	Name        string   `json:"name"`
	ResourceURI string   `json:"resourceUri"`
	AuthType    AuthType `json:"authType"`
}

type AuthType int32

const (
	// Authentication type not specified.
	AuthType_SYSTEM_MANAGED_IDENTITY AuthType = 0
	// Username and Password Authentication.
	AuthType_USER_PASSWORD AuthType = 1
)