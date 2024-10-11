input: _
ctx: [for _ in output {}]

output: [{
	alias: ctx[0].alias
	type:       "Microsoft.OperationalInsights/workspaces"
	apiVersion: "2021-12-01-preview"
	properties: {
		location:        input.location
		tags:            ctx[0].tags
		retentionInDays: 30
		features: searchVersion: 1
		sku: name:               "PerGB2018"
	}
}, {
	alias:       ctx[1].alias
	type:       "Microsoft.Insights/components"
	apiVersion: "2020-02-02"
	kind:       "web"
	properties: {
		location:            input.location
		tags:                ctx[0].tags
		Application_Type:    "web"
		WorkspaceResourceId: "${\(ctx[0].alias).id}"
	}
}]
