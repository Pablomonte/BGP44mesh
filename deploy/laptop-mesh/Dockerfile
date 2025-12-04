FROM gravitl/netclient:v0.24.2

# Add custom CA certificate
COPY netmaker-ca.crt /usr/local/share/ca-certificates/netmaker-ca.crt

# Install ca-certificates and update trust store
RUN apk add --no-cache ca-certificates && \
    update-ca-certificates
