#!/bin/bash

APP=${1}

git pull

go build -o bin/ .

docker run \
  -w /tmp/app \
  -v $(pwd)/fixtures/go-simple:/tmp/app \
  -v $(pwd)/bin/slugcmplr:/tmp/slugcmplr \
  -v ${HOME}/.netrc:/root/.netrc \
  --entrypoint="" \
  heroku/heroku:20-build \
  /tmp/slugcmplr compile ${APP} --verbose
