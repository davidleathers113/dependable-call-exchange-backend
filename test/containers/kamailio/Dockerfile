# Use official Kamailio image
FROM kamailio/kamailio:5.7-alpine

# Install additional tools for health checks
USER root
RUN apk add --no-cache curl netcat-openbsd

# Copy custom configuration
COPY kamailio.cfg /etc/kamailio/kamailio.cfg

# Create health check script
RUN echo '#!/bin/sh\nnc -z localhost 5060' > /usr/local/bin/healthcheck.sh && \
    chmod +x /usr/local/bin/healthcheck.sh

# Expose ports
EXPOSE 5060/udp 5060/tcp 8080/tcp

# Health check endpoint
HEALTHCHECK --interval=5s --timeout=3s --retries=5 \
    CMD /usr/local/bin/healthcheck.sh

# Run Kamailio
CMD ["kamailio", "-DD", "-E", "-e", "-m", "64", "-M", "8"]
