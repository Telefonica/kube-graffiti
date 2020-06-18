FROM golang:1.14.4-alpine3.12 as build

WORKDIR /root
COPY . .

ENV CGO_ENABLED 0
RUN go test ./...
RUN go build -trimpath -ldflags '-s -w' -o kube-graffiti .

FROM alpine:3.12
LABEL maintainer="javier.provechofernandez@telefonica.com"

RUN addgroup -g 10001 app && adduser -D -g '' -G app -s /bin/false -h /app -u 10001 app
USER 10001
COPY --chown=app:app --from=build /root/kube-graffiti /bin/kube-graffiti

ENTRYPOINT ["kube-graffiti"]
