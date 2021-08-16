ARG STACK

FROM golang:1.16-buster AS builder

COPY . /app
WORKDIR /app

RUN go mod download
RUN CGO_ENABLED=0 go build -o bin/ .

FROM heroku/heroku:${STACK}-build

COPY --from=builder /app/bin/slugcmplr /usr/bin/slugcmplr

RUN rm -rf /app

ENTRYPOINT ["/usr/bin/slugcmplr"]
