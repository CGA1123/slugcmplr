ARG STACK

FROM heroku/heroku:${STACK}-build

COPY . /app
WORKDIR /app

RUN go mod download
RUN go build -o bin/ .

COPY /app/bin/slugcmplr /usr/bin/slugcmplr

RUN rm -rf /app

ENTRYPOINT ["/usr/bin/slugcmplr"]
