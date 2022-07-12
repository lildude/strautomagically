include .env

.PHONY: app_url
app_url:
	$(eval export URL=https://$$(shell heroku domains --app=${HEROKU_APP} | tail -1))

.PHONY: heroku
heroku: app_url
	$(if ${STRAVA_CLIENT_ID},,$(error must set STRAVA_CLIENT_ID in .env))
	$(if ${STRAVA_CLIENT_SECRET},,$(error must set STRAVA_CLIENT_SECRET in .env))
	heroku config:set STRAVA_CLIENT_ID=${STRAVA_CLIENT_ID} \
	STRAVA_CLIENT_SECRET=${STRAVA_CLIENT_SECRET} \
	STRAVA_REDIRECT_URI=${URL}/auth \
	STRAVA_CALLBACK_URI=${URL}/webhook \
	STRAVA_VERIFY_TOKEN=${STRAVA_VERIFY_TOKEN} \
	STATE_TOKEN=${STATE_TOKEN} \
	OWM_API_KEY=${OWM_API_KEY} \
	OWM_LAT=${OWM_LAT} \
	OWM_LON=${OWM_LON}

.PHONY: heroku-local
heroku-local:
	go build -o bin/strautomagically -v && heroku local --port 8080

.PHONY: reset-auth-token
reset-auth-token:
	echo DEL strava_auth_token | heroku redis:cli -a ${HEROKU_APP} -c ${HEROKU_APP}

.PHONY: reset-last-activity
reset-last-activity:
	echo DEL strava_activity | heroku redis:cli -a ${HEROKU_APP} -c ${HEROKU_APP}