FROM golang:1.18 AS builder

# Set the Current Working Directory inside the container
WORKDIR /app

COPY go.mod .
COPY go.sum .

RUN go mod download

COPY . .

RUN go test -v ./...

WORKDIR /app/cmd/iot-device-mgmt

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build

FROM registry.access.redhat.com/ubi8/ubi-minimal
WORKDIR /opt/diwise

COPY --from=builder --chown=1001 /app/cmd/iot-device-mgmt/iot-device-mgmt /opt/diwise
COPY --chown=1001 assets/config/devices.csv /opt/diwise/config/devices.csv
COPY --chown=1001 wwwroot /opt/diwise/wwwroot

RUN chown 1001 /opt/diwise
RUN chmod 700 /opt/diwise

ENV "GUI_WEB_ROOT" : "/opt/diwise/wwwroot"

EXPOSE 8080
USER 1001

ENTRYPOINT ["/opt/diwise/iot-device-mgmt"]
