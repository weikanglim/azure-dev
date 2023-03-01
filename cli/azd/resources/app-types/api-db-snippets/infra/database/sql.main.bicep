//inputs
param sqlServerName string = ''
param sqlDatabaseName string = ''

@secure()
@description('SQL Server administrator password')
param sqlAdminPassword string

@secure()
@description('Application user password')
param appUserPassword string
//inputs

//resources
// The application database
module sqlServer '../../../../../common/infra/bicep/app/sqlserver.bicep' = {
  name: 'sql'
  scope: rg
  params: {
    name: !empty(sqlServerName) ? sqlServerName : '${abbrs.sqlServers}${resourceToken}'
    databaseName: sqlDatabaseName
    location: location
    tags: tags
    sqlAdminPassword: sqlAdminPassword
    appUserPassword: appUserPassword
    keyVaultName: keyVault.outputs.name
  }
}
//resources

//outputs
output AZURE_SQL_CONNECTION_STRING_KEY string = sqlServer.outputs.connectionStringKey
//outputs
