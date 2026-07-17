-include .env

SHA=`git show --quiet --format=format:%H`

build:
	CGO_ENABLED=0 go build -o strautomagically cmd/strautomagically/main.go

build_azure:
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags "-s -w -X main.Version=$(SHA)" -o strautomagically cmd/strautomagically/main.go

lint:
	golangci-lint run

test:
	ENV=test go test -p 8 ./...

coverage:
	ENV=test go test ./... -coverprofile=coverage.out
	go tool cover -func coverage.out

start: build
	ENV=dev func start --custom

get-auth-token:
	sqlite3 $(or $(DATABASE_PATH),database.db) "SELECT strava_auth_token FROM athletes LIMIT 1;" | jq

get-last-activity:
	sqlite3 $(or $(DATABASE_PATH),database.db) "SELECT last_activity_id FROM athletes LIMIT 1;"

reset-last-activity:
	sqlite3 $(or $(DATABASE_PATH),database.db) "UPDATE athletes SET last_activity_id = 0;"

reset-auth-token:
	sqlite3 $(or $(DATABASE_PATH),database.db) "UPDATE athletes SET strava_auth_token = '';"

# Really not sure which of these get things working, but it should produce something like:
# {
#   "clientId": "...",
#   "clientSecret": "...",
#   "subscriptionId": "...",
#   "tenantId": "...",
#   "activeDirectoryEndpointUrl": "https://login.microsoftonline.com",
#   "resourceManagerEndpointUrl": "https://management.azure.com/",
#   "activeDirectoryGraphResourceId": "https://graph.windows.net/",
#   "sqlManagementEndpointUrl": "https://management.core.windows.net:8443/",
#   "galleryEndpointUrl": "https://gallery.azure.com/",
#   "managementEndpointUrl": "https://management.core.windows.net/"
# }
# Set this in AZURE_RBAC_CREDENTIALS in GitHub Actions secrets - we only need the first 4 fields.
new-azure-creds:
	az ad sp create-for-rbac --name "Strautomagically" --role contributor \
    --scopes /subscriptions/${AZURE_SUBSCRIPTION_ID}/resourceGroups/strautomagically/providers/Microsoft.Web/sites/strautomagically \
		--years 5 --json-auth \
		| jq '{clientId, clientSecret, subscriptionId, tenantId}'

# az ad sp create-for-rbac --name "Strautomagically" --role contributor \
#   --scopes /subscriptions/${AZURE_SUBSCRIPTION_ID}/resourceGroups/strautomagically

