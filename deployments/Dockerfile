FROM --platform=$BUILDPLATFORM docker.io/golang:1.22 AS builder

# Set the Current Working Directory inside the container
WORKDIR /app

COPY go.mod .
COPY go.sum .

RUN go mod download

COPY . .

WORKDIR /app/cmd/iot-device-mgmt

ARG TARGETARCH
ARG TARGETOS
RUN GOOS=$TARGETOS GOARCH=$TARGETARCH go build


FROM registry.access.redhat.com/ubi9/ubi-minimal
WORKDIR /opt/diwise

COPY --from=builder --chown=1001 /app/cmd/iot-device-mgmt/iot-device-mgmt /opt/diwise
COPY --chown=1001 assets/config/authz.rego /opt/diwise/config/authz.rego
COPY --chown=1001 assets/config/devices.csv /opt/diwise/config/devices.csv
COPY --chown=1001 assets/config/alarms.csv /opt/diwise/config/alarms.csv

RUN chown 1001 /opt/diwise

EXPOSE 8080
USER 1001

ENTRYPOINT ["/opt/diwise/iot-device-mgmt"]
