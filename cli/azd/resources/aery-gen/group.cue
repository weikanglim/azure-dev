input: _
ctx: [for _ in output {}]

output: [{
	alias:       ctx[0].alias
	type:       "Microsoft.Resources/resourceGroups"
	apiVersion: "2022-09-01"
	spec: {
		location: input.location
		tags:     ctx[0].tags
	}
}]
