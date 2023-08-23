package scaffold

import (
	"fmt"
	"strings"
)

type InfraSpec struct {
	Parameters []Parameter
	Services   []ServiceSpec

	// Databases to create
	DbPostgres *DatabasePostgres
	DbCosmos   *DatabaseCosmos
}

type Parameter struct {
	Name   string
	Value  string
	Type   string
	Secret bool
}

type DatabasePostgres struct {
	DatabaseUser string
	DatabaseName string
}

type DatabaseCosmos struct {
	DatabaseName string
}

type ServiceSpec struct {
	Name string
	Port int

	// Front-end properties.
	Frontend *Frontend

	// Back-end properties
	Backend *Backend

	// Connection to a database. Only one should be set.
	DbPostgres *DatabasePostgres
	DbCosmos   *DatabaseCosmos
}

type Frontend struct {
	Backends []ServiceReference
}

type Backend struct {
	Frontends []ServiceReference
}

type ServiceReference struct {
	Name string
}

func NewContainerAppServiceExistsParameter(serviceName string) Parameter {
	return Parameter{
		Name: BicepName(serviceName) + "Exists",
		Value: fmt.Sprintf("${SERVICE_%s_RESOURCE_EXISTS=false}",
			strings.ReplaceAll(strings.ToUpper(serviceName), "-", "_")),
		Type: "bool",
	}
}
