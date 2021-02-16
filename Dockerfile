FROM golang:latest

LABEL maintainer="Shishir Mahajan <smahajan@roblox.com>"

RUN mkdir -p /go/src/github.com/roblox/nomad-node-problem-detector

WORKDIR /go/src/github.com/roblox/nomad-node-problem-detector

COPY . .

RUN make install

EXPOSE 8083

CMD ["npd", "aggregator"]
