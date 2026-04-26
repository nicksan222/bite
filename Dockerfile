# Used by GoReleaser. The `bite` binary is built outside Docker by GoReleaser
# and copied into the build context, so this image is just a runtime wrapper.
#
# To build locally without GoReleaser:
#   go build -o bite ./cmd/bite && docker build -t bite .
FROM gcr.io/distroless/static-debian12:nonroot

COPY bite /usr/local/bin/bite

ENV BITE_DB=/data/bite.db
VOLUME ["/data"]

USER nonroot:nonroot
ENTRYPOINT ["/usr/local/bin/bite"]
