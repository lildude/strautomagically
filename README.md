# Strautomagically

Webhook endpoint that runs on Heroku to do stuff automagically to my Strava activities as they appear.

## Usage

### Prerequisites

1. Create an API application from [your settings on Strava][] with a callback domain of `localhost:8080`.
2. Create an API token that has `activity:read_all` and `activity:write` scopes for your account using [dcarley/oauth2-cli][]:

    oauth2-cli \
      -scope activity:read_all,activity:write \
      -id $STRAVA_CLIENT_ID \
      -secret $STRAVA_CLIENT_SECRET \
      -auth https://www.strava.com/oauth/authorize \
      -token https://www.strava.com/oauth/token

  Or use `make strava_token`

[your settings on Strava]: https://www.strava.com/settings/api
[dcarley/oauth2-cli]: https://github.com/dcarley/oauth2-cli

### Deployment

1. Create a `.env` file and set `STRAVA_CLIENT_ID`, `STRAVA_CLIENT_SECRET` and `STRAVA_API_TOKEN`.
2. Configure the Heroku config var for `STRAVA_API_TOKEN`
    
    make heroku

3. Build, package, and deploy:

    make

4. Create a push subscription if you're deploying for the first time or the URL has changed:

    make register
