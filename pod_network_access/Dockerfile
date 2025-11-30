FROM alpine:latest

RUN apk --no-cache add ca-certificates

WORKDIR /root/

# Copy the pre-built binary
COPY pod_network_access ./pod-network-monitor

CMD ["./pod-network-monitor"]
