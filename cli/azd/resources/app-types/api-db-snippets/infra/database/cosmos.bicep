param accountName string
param location string = resourceGroup().location
param tags object = {}

// By default, no collections are created.
// If you like to create collections as part of infrastructure provisioning,
// here's an example:
// {
//   name: 'TodoList'
//   id: 'TodoList'
//   shardKey: 'Hash'
//   indexKey: '_id'
// }
// {
//   name: 'TodoItem'
//   id: 'TodoItem'
//   shardKey: 'Hash'
//   indexKey: '_id'
// }
//
// The example defines a TodoList and TodoItem, uses hash(id) as the partition key, id as the default index.
param collections array = []

param databaseName string
param keyVaultName string

module cosmos '../core/database/cosmos/mongo/cosmos-mongo-db.bicep' = {
  name: 'cosmos-mongo'
  params: {
    accountName: accountName
    databaseName: databaseName
    location: location
    collections: collections
    keyVaultName: keyVaultName
    tags: tags
  }
}

output connectionStringKey string = cosmos.outputs.connectionStringKey
output databaseName string = cosmos.outputs.databaseName
output endpoint string = cosmos.outputs.endpoint
