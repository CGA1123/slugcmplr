# slugcmplr

```bash
go get github.com/cga1123/slugcmplr
```

`slugcmplr` is a CLI tool that interfaces with the Heroku Platform API in order
to enable you and your team to detach the building of your Heroku application
from the releasing of it.

This enables your CI/CD pipeline to run tests and building of your application
in parallel, potentially reducing the merge to deploy time for your team!

There are two commands available, `build` and `release` each require a
`[target]` argument (the name or identifier for your production application)
and a `--compiler` flag (the name or identifier for a 'compile' application)

The `build` command will synchronise all buildpacks and environment variables
between your production and compile application, it also sets the compile app
into maintenance mode and escapes any `release` task you have in your
`Procfile` (if present).

This ensures that your compile application is not exposed publicly and does not
run any release task (such as any DB migrations) during the compilation phase.

The `release` task performs a 'slug promotion' (a slug is the name Heroku gives
the artifact of a build) which transfers your application from your compile app
to your production app and triggers a normal Heroku release phase.
