# slugcmplr

TODO

## Install

With the `go` toolchain installed:
```bash
go get github.com/cga1123/slugcmplr
```

There are also precompiled binaries and their associated checksums are
available attached to tagged [releases].

## Info

TODO


## Authentication

The `slugcmplr` CLI looks for credentials to `api.heroku.com` in your `.netrc`
file, this is the same technique used by [`heroku/cli`] and so if you are
currently making use of the `heroku` command during CI, you should already be
logging in somehow and have no issues.

Otherwise populating your `.netrc` should be a case of adding something
equivalent to the following script (assuming the `HEROKU_EMAIL` and
`HEROKU_API_KEY` are correctly populated environment variables):

```bash
cat << EOF >> ${HOME}/.netrc
machine api.heroku.com
  login ${HEROKU_EMAIL}
  password ${HEROKU_API_KEY}
EOF
```

By default, `slugcmplr` will look in `${HOME}/.netrc` for the credentials,
however it will respect the `${NETRC}` environment variable if set and
non-empty.

## Testing

The majority of tests for this project are acceptance tests that will create
and release live Heroku applications. These require the correct credentials to
be set in the environment as well as setting a sentinel value to execute the
acceptance tests:

```bash
SLUGCMPLR_ACC=true \
  SLUGCMPLR_ACC_HEROKU_PASS=<HEROKU-API-KEY> \
  SLUGCMPLR_ACC_HEROKU_EMAIL=<HEROKU-EMAIL> \
  go test -v
```

---

For more background on this you might find [this] Medium article helpful.

[this]: https://medium.com/carwow-product-engineering/speeding-up-our-heroku-deploys-by-35-percent-f9fa6f6cf404
[`heroku/cli`]: https://github.com/heroku/cli
[releases]: https://github.com/cga1123/slugcmplr/releases
