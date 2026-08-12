FROM openfga/cli:v0.7.20 AS cli

FROM alpine:3.22.1
RUN addgroup -S bootstrap && adduser -S -G bootstrap bootstrap && \
    mkdir /state && chown bootstrap:bootstrap /state
COPY --from=cli /fga /usr/local/bin/fga
COPY --chmod=0555 bootstrap.sh /usr/local/bin/bootstrap-openfga
USER bootstrap
VOLUME ["/state"]
ENTRYPOINT ["/usr/local/bin/bootstrap-openfga"]
