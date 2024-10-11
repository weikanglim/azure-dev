input: _
ctx: [for _ in output {}]

output: [{
	name:       ctx[0].name
	type:       "Microsoft.Resources/resourceGroups"
	apiVersion: "2022-09-01"
	spec: {
		location: input.location
		tags:     ctx[0].tags
	}
}]
