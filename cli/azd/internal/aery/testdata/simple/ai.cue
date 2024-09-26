// place the json input here with "-l"
input: _

input: #Schema
#Schema: {
	name:     string
	id:       string
	location: string
	tags: {}
	model: {
		name:    string
		version: string
	}
}

output: [{
	name:       input.name
	id:         input.id
	kind:       "Microsoft.CognitiveServices/accounts"
	apiVersion: "2023-05-01"
	spec: {
		kind:     "OpenAI"
		location: input.location
		tags:     input.tags
		properties: {
			customSubDomainName: input.name
			publicNetworkAccess: "Enabled"
			disableLocalAuth:    true
		}
		sku: name: "S0"
	}
}, {
	name: input.model.name
	// parent: ${} -- defaults to the matching parent in the current module
	kind:       "Microsoft.CognitiveServices/accounts/deployments"
	apiVersion: "2023-05-01"
	spec: {
		location: input.location
		tags:     input.tags
		properties: {
			model: {
				format:  "OpenAI"
				name:    input.model.name
				version: input.model.version
			}
		}
		sku: {
			name:     "Standard"
			capacity: 20
		}
	}
}]
