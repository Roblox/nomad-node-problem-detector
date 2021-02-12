FROM golang:1.13-alpine3.10 AS build
RUN apk --no-cache add git
WORKDIR /go/src/github.com/roblox/nomad-node-problem-detector
COPY . .
ENV GO111MODULE=on
RUN go mod download
RUN GOOS=linux go build -ldflags="-s -w" -o ./bin/npd .

FROM alpine:3.10
WORKDIR /usr/local/bin
COPY --from=build /go/src/github.com/roblox/nomad-node-problem-detector/bin .
EXPOSE 8083
ENTRYPOINT ["npd", "aggregator"]
