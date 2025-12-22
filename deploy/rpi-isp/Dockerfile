FROM debian:12-slim

RUN apt-get update && apt-get install -y \
    bird2 \
    iproute2 \
    gettext-base \
    && rm -rf /var/lib/apt/lists/*

COPY entrypoint.sh /entrypoint.sh
RUN chmod +x /entrypoint.sh

EXPOSE 179

HEALTHCHECK --interval=30s --timeout=10s --start-period=10s \
    CMD birdc show status || exit 1

ENTRYPOINT ["/entrypoint.sh"]

