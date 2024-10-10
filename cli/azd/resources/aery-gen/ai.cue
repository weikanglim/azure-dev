input: _ // the app resource present in azure.yaml
ctx: [for _ in output {}]

output: [{
	name:       ctx[0].name
	type:       "Microsoft.CognitiveServices/accounts"
	apiVersion: "2023-05-01"
	spec: {
		kind:     "OpenAI"
		location: input.location
		tags:     ctx[0].tags
		properties: {
			customSubDomainName: ctx[0].name
			publicNetworkAccess: "Enabled"
			disableLocalAuth:    true
		}
		sku: name: "S0"
	}
}, {
	name: input.name
	// parent: ${} -- defaults to the matching parent in the current module
	type:       "Microsoft.CognitiveServices/accounts/deployments"
	apiVersion: "2023-05-01"
	spec: {
		location: input.location
		tags:     ctx[1].tags
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
