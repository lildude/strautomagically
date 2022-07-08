# Strautomagically

Webhook endpoint that runs on Heroku to do stuff automagically to my Strava activities as they appear.
Inspired by [Klimat](https://klimat.app/) and [Strautomator](https://strautomator.com).

## 🚧 WIP 🚧

This is very much WIP and it more of a tinker tool to help me learn Go with a purpose.
There's no guarantee, yet, that any of the info below is accurate of this even works.
As such I've disabled issues and I'm not taking PRs.
Feel free to fork and tinker for your own purposes.
If I spot a fork and find something I like that you've done, I will pinch it 😜.

## Usage

### Prerequisites

You will need to create an API application in [your settings on Strava](https://www.strava.com/settings/api) and take note of the client ID and secret.
If you are running this locally, you will need to set the callback domain to `localhost:8080`, or your ngrok URL if you want to use [ngrok](https://ngrok.com/).
You will also need a Redis database which is used to store the authentication and refresh tokens.
Optional: If you want to add weather information to your entries, you will need to register for a free [OpenWeather](https://openweathermap.org) account and obtain an API key.

### Running Locally

1. Create a `.env` file and set the following:
   - `STRAVA_CLIENT_ID` & `STRAVA_CLIENT_SECRET` to the values from Strava
   - `STRAVA_REDIRECT_URI` to the callback domain you registered followed by `/auth`, eg `http://localhost:8080/auth`
   - `STRAVA_CALLBACK_URI` to the same domain as you registered followed by `/webhook` eg `http://localhost:8080/webhook`
   - `STRAVA_VERIFY_TOKEN` to any random unique string
   - `STATE_TOKEN` to any random unique string
   - `REDIS_URL` to the database URL for your Redis database in the form `redis://<username>:<password>@<hostname>/<database>:<port>`.
     If you're using Heroku, you can use the URL Heroku uses.
   - Optional: `OWM_API_KEY` to the OpenWeather API key.
1. Configure your rules in the `update.go` file. I plan to move this out to a better place in future.
1. Run: `heroku-local` and then visit the `STRAVA_REDIRECT_URI` URL and authorize the application with Strava.
1. Go for a run.

### Deployment

1. Create a `.env` file as above
1. Configure the Heroku config vars: `make heroku`
  This will set `STRAVA_REDIRECT_URI` and `STRAVA_CALLBACK_URI` to use your Heroku application URL.
  It gets this from `heroku domains | tail -1`

1. Deploy using your preferred method.
1. Visit the `STRAVA_REDIRECT_URI` URL and authorize the application with Strava.
1. Go for a run.
