FROM prom/prometheus:v2.53.1 AS prom

FROM grafana/grafana:11.2.0

# Copy prometheus binary from prometheus image
COPY --from=prom /bin/prometheus /usr/local/bin/prometheus
COPY --from=prom /etc/prometheus/prometheus.yml /etc/prometheus/prometheus.yml.default

# Install supervisor to run both services
USER root
RUN apk add --no-cache supervisor curl

# Copy entrypoint script
COPY entrypoint.sh /usr/local/bin/entrypoint.sh
RUN chmod +x /usr/local/bin/entrypoint.sh

# Create supervisor config
RUN mkdir -p /etc/supervisor/conf.d

# Expose ports
EXPOSE 9090 3000

# Health check
HEALTHCHECK --interval=30s --timeout=10s --start-period=40s --retries=3 \
    CMD curl -f http://localhost:9090/-/healthy && curl -f http://localhost:3000/api/health || exit 1

ENTRYPOINT ["/usr/local/bin/entrypoint.sh"]
