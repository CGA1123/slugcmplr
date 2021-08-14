#!/bin/bash

BUILD_DIR=${1}

docker run \
	-w /tmp/app \
	-v ${BUILD_DIR}:/tmp/app \
	-v $(pwd)/bin/slugcmplr:/tmp/slugcmplr \
	-v ${HOME}/.netrc:/root/.netrc \
	--entrypoint="" \
	heroku/heroku:20-build \
	/tmp/slugcmplr compile --build-dir /tmp/app --cache-dir $(mktemp -d) --verbose
