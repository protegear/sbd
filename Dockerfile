FROM registry.gitlab.com/ulrichschreiner/base/debian:buster-slim

RUN apt update && apt -y install ca-certificates libcap2 && mkdir /conf

RUN addgroup --system --gid 1001 directip \
    && adduser --uid 1001 --disabled-password --system --shell /sbin/nologin --ingroup directip directip

COPY cmd/directipserver/bin/directipserver /directipserver

ENTRYPOINT [ "/directipserver" ]
USER directip

