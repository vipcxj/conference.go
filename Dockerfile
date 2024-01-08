FROM golang:1.21 AS Build

RUN mkdir /app
WORKDIR /app
COPY . .
RUN go env -w  GOPROXY=https://goproxy.cn,direct
RUN go mod download
RUN CGO_ENABLED=0 go build ./cmd/server

FROM scratch
COPY --from=Build /app/server /app/server
ENTRYPOINT [ "/app/server" ]