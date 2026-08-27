FROM golang:1.24-alpine AS build

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/7review ./cmd/7review
RUN mkdir -p /out/data/7review /out/agent /out/profiles && \
    cp agent/instructions.md /out/agent/instructions.md && \
    cp -R agent/skills /out/agent/skills && \
    cp profiles/default.input-profile.json /out/profiles/default.input-profile.json && \
    chmod -R a+rX /out/agent /out/profiles

FROM gcr.io/distroless/static-debian12:nonroot

WORKDIR /app
COPY --from=build --chown=nonroot:nonroot --chmod=0755 /out/7review /app/7review
COPY --from=build --chown=nonroot:nonroot /out/data /data
COPY --from=build --chown=nonroot:nonroot /out/agent /app/agent
COPY --from=build --chown=nonroot:nonroot /out/profiles /app/profiles
COPY --chown=nonroot:nonroot --chmod=0644 orchestrator.yaml /app/orchestrator.yaml

ENV LISTEN_ADDR=:8080
ENV ORCHESTRATOR_CONFIG=/app/orchestrator.yaml
ENV INPUT_PROFILE=/app/profiles/default.input-profile.json
ENV SKILLS_DIR=/app/agent/skills
ENV INSTRUCTIONS_PATH=/app/agent/instructions.md

EXPOSE 8080
USER nonroot:nonroot
HEALTHCHECK --interval=30s --timeout=5s --start-period=20s --retries=3 CMD ["/app/7review", "healthcheck"]
ENTRYPOINT ["/app/7review"]
