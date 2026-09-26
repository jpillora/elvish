FROM golang:alpine AS build
RUN apk add --no-cache git
WORKDIR /src
COPY . .
ENV CGO_ENABLED=0
RUN go build -trimpath -ldflags="-s -w" -o /tmp/elvish ./cmd/elvish

FROM scratch
COPY --from=build /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=build /tmp/elvish /bin/elvish
ENTRYPOINT ["/bin/elvish"]
