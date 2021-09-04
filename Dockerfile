ARG STACK

FROM golang:1.16-buster AS builder

COPY . /app
WORKDIR /app

RUN go mod download

# TODO: use goreleaser? or pass in version, commit, and date as build-args?
RUN CGO_ENABLED=0 go build -o bin/ ./cmd/slugcmplr

FROM heroku/heroku:${STACK}-build

LABEL org.opencontainers.image.source="https://github.com/CGA1123/slugcmplr"

COPY --from=builder /app/bin/slugcmplr /usr/bin/slugcmplr

ENTRYPOINT ["/usr/bin/slugcmplr"]
