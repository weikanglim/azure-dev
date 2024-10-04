input: _

output: [{
	name:       input.tags.name
	kind:       "Microsoft.CognitiveServices/accounts"
	apiVersion: "2023-05-01"
	spec: {
		kind:     "OpenAI"
		location: input.location
		tags: input.tags
		properties: {
			customSubDomainName: input.tags.name
			publicNetworkAccess: "Enabled"
			disableLocalAuth:    true
		}
		sku: name: "S0"
	}
}, {
	name: input.name
	// parent: ${} -- defaults to the matching parent in the current module
	kind:       "Microsoft.CognitiveServices/accounts/deployments"
	apiVersion: "2023-05-01"
	spec: {
		location: input.location
		tags: input.tags
		properties: {
			model: {
				format:  "OpenAI"
				name:    input.props.model.name
				version: input.props.model.version
			}
		}
		sku: {
			name:     "Standard"
			capacity: 20
		}
	}
}]
