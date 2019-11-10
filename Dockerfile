FROM alpine:3.10

RUN apk --no-cache add ca-certificates libcap && mkdir /conf

RUN addgroup -S -g 1001 directip \
    && adduser -u 1001 -D -S -s /sbin/nologin -G directip directip

COPY cmd/directipserver/bin/directipserver /directipserver

ENTRYPOINT [ "/directipserver" ]
USER directip

