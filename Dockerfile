FROM golang:1.22-alpine3.19 as builder
RUN apk add --no-cache --virtual build-deps make git
# Build Elvish
COPY . /go/src/github.com/jpillora/elvish
RUN make -C /go/src/github.com/jpillora/elvish get

FROM alpine:3.19
COPY --from=builder /go/bin/elvish /bin/elvish
CMD ["/bin/elvish"]
