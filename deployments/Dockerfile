FROM golang:1.18 AS builder

# Set the Current Working Directory inside the container
WORKDIR /app

COPY go.mod .
COPY go.sum .

RUN go mod download

COPY . .

RUN go test -v ./...

WORKDIR /app/cmd/iot-device-management

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build

FROM registry.access.redhat.com/ubi8/ubi-minimal
WORKDIR /opt/diwise

COPY --from=builder --chown=1001 /app/cmd/iot-device-management/iot-device-management /opt/diwise
COPY --chown=1001 assets/config/devices.csv /opt/diwise/config/devices.csv


RUN chown 1001 /opt/diwise
RUN chmod 700 /opt/diwise

EXPOSE 8080
USER 1001

ENTRYPOINT ["/opt/diwise/iot-device-management"]
