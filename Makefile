-include .env

SHA=`git show --quiet --format=format:%H`

build:
	go build -o strautomagically cmd/strautomagically/main.go

build_azure:
	GOOS=linux GOARCH=amd64 go build -ldflags "-s -w -X main.Version=$(SHA)" -o strautomagically cmd/strautomagically/main.go

lint:
	golangci-lint run

test:
	ENV=test go test -v ./...

coverage:
	ENV=test go test ./... -coverprofile=coverage.out
	go tool cover -func coverage.out

start: build
	ENV=dev func start --custom

get-auth-token:
	echo GET strava_auth_token | redis-cli -u ${REDIS_URL}

get-last-activity:
	echo GET strava_activity | redis-cli -u ${REDIS_URL}

reset-last-activity:
	echo DEL strava_activity | redis-cli -u ${REDIS_URL}

reset-auth-token:
	echo DEL strava_auth_token | redis-cli -u ${REDIS_URL}
