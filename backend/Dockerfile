FROM alpine:3.21

WORKDIR /app

# Docker buildx sets these variables during the build.
ARG TARGETOS
ARG TARGETARCH

RUN apk add --no-cache ca-certificates curl tzdata

COPY --chmod=755 komari-${TARGETOS}-${TARGETARCH} /app/komari

ENV GIN_MODE=release
ENV KOMARI_LISTEN=0.0.0.0:25774
ENV KOMARI_DEPLOYMENT=docker

EXPOSE 25774 35938

CMD ["/app/komari", "server"]
