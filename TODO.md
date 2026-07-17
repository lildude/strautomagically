# TODOs

- [x] refactor to use a more standard layout
- [ ] if zwift and trainerroad workout close to each other, "merge" them
      - maybe take the images from Zwift and add it to TR and delete Zwift?
- [ ] generate verify token rather than using static config
- [ ] refactor subscription as its a hacky mess
- [x] add tests for subscription
- [x] move from Heroku to Azure function
- [x] Deploy via actions
- [ ] Add config to read from local.settings.json rather than .env - Viper might do the trick here https://github.com/spf13/viper though might be overkill.
- [ ] Move away from Redis to an sqlite DB.
- [ ] Move to Flex Consumption plan and use the functions Go SDK - https://learn.microsoft.com/en-us/azure/azure-functions/functions-reference-go


# Hosting on Azure Functions

Docs: https://docs.microsoft.com/en-us/azure/azure-functions/create-first-function-vs-code-other?tabs=go%2Cmacos

- I'm cheap so I've gone for the free RedisDB from Redis themselves. Add creds to .env file