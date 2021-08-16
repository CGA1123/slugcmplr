ARG STACK

FROM golang:1.16-alpine AS builder

COPY . /app
WORKDIR /app

RUN go mod download
RUN GOOS=linux GOARCH=amd64 go build -o bin/ .

FROM heroku/heroku:${STACK}-build

COPY --from=builder /app/bin/slugcmplr /bin/slugcmplr

CMD /bin/slugcmplr compile --build-dir /tmp/build --cache-dir /tmp/cache --local
