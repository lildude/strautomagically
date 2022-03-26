include .env

.PHONY: strava_token
strava_token:
	$(if ${STRAVA_CLIENT_ID},,$(error must set STRAVA_CLIENT_ID in .env))
	$(if ${STRAVA_CLIENT_SECRET},,$(error must set STRAVA_CLIENT_SECRET in .env))
	@echo "Getting STRAVA_ACCESS_TOKEN"
	oauth2-cli \
  -scope activity:read_all,activity:write \
  -id ${STRAVA_CLIENT_ID} \
  -secret ${STRAVA_CLIENT_SECRET} \
  -auth https://www.strava.com/oauth/authorize \
  -token https://www.strava.com/oauth/token

.PHONY: app_url
app_url:
	$(eval export URL=https://$$(shell heroku domains | tail -1	)/)

.PHONY: heroku
heroku:
	$(if ${STRAVA_ACCESS_TOKEN},,$(error must set STRAVA_ACCESS_TOKEN in .env))
	heroku config:set STRAVA_ACCESS_TOKEN=${STRAVA_ACCESS_TOKEN}

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