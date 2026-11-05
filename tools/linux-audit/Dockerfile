FROM alpine:3.22
RUN apk add --no-cache bash python3 curl iproute2 iptables bind-tools
ENV GATEWAY_AUDIT_CONTAINER=1 PYTHONDONTWRITEBYTECODE=1
WORKDIR /audit
COPY *.py /audit/
