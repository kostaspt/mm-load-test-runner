# LoadTest Runner

This is a runner or a helper to use along with [Mattermost LoadTest](https://github.com/mattermost/mattermost-load-test-ng).

### How to use

Login to AWS
```shell
aws configure sso --profile mm-loadtest
```

Copy .env.sample to .env, and fill in the values.

```shell
go run ./cmd/runner
```