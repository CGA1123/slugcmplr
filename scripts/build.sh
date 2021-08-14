#!/bin/bash

BUILD_DIR=${1}

docker run \
	-w /tmp/build \
	-v ${BUILD_DIR}:/tmp/build \
	-v $(pwd)/bin/slugcmplr:/tmp/slugcmplr \
	-v ${HOME}/.netrc:/root/.netrc \
	--entrypoint="" \
	heroku/heroku:20-build \
	/tmp/slugcmplr compile --build-dir /tmp/build --cache-dir $(mktemp -d) --verbose
