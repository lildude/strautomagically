include .env

.PHONY: app_url
app_url:
	$(eval export URL=https://$$(shell heroku domains | tail -1	)/)

.PHONY: heroku
heroku: app_url
	$(if ${STRAVA_CLIENT_ID},,$(error must set STRAVA_CLIENT_ID in .env))
	$(if ${STRAVA_CLIENT_SECRET},,$(error must set STRAVA_CLIENT_SECRET in .env))
	heroku config:set STRAVA_CLIENT_ID=${STRAVA_CLIENT_ID}
	heroku config:set STRAVA_CLIENT_SECRET=${STRAVA_CLIENT_SECRET}
	heroku config:set STRAVA_REDIRECT_URI=https://${URL}.herokuapp.com/auth/callback

.PHONY: register
register: app_url
	$(if ${STRAVA_CLIENT_ID},,$(error must set STRAVA_CLIENT_ID in .env))
	$(if ${STRAVA_CLIENT_SECRET},,$(error must set STRAVA_CLIENT_SECRET in .env))
	@echo "Registering push subscription with Strava"
	@echo ${URL}

	# curl -XPOST \
	# 	-F "client_id=${STRAVA_CLIENT_ID}" \
	# 	-F "client_secret=${STRAVA_CLIENT_SECRET}" \
	# 	-F 'verify_token=STRAVA' \
	# 	-F "callback_url=${URL}" \
	# 	https://api.strava.com/api/v3/push_subscriptions

.PHONY: heroku-local
heroku-local:
	go build -o bin/strautomagically -v && heroku local --port 8080

.PHONY: update-swagger
update-swagger:
	swagger-codegen generate --input-spec internal/strava-swagger-fixed.json --lang go --output internal/generated