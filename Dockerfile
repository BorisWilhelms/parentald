FROM golang:1.25-alpine AS builder

RUN apk add --no-cache make
WORKDIR /src
COPY go.mod ./
RUN go mod download
COPY . .

ARG VERSION=dev
RUN make build-all VERSION=${VERSION}

FROM alpine:3.21

RUN apk add --no-cache ca-certificates
COPY --from=builder /src/parentald-server /usr/local/bin/
COPY --from=builder /src/dist/ /dist/

EXPOSE 8080
ENTRYPOINT ["parentald-server"]
CMD ["--bin-dir", "/dist"]
