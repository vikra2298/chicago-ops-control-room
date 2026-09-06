FROM node:22-alpine AS frontend
WORKDIR /fe
COPY frontend/package.json frontend/package-lock.json ./
RUN npm ci
COPY frontend/ ./
RUN npm run build

FROM golang:1.23-alpine AS backend
WORKDIR /src
COPY backend/ ./
RUN CGO_ENABLED=0 go build -o /server ./cmd/server

FROM alpine:3.20
RUN apk add --no-cache ca-certificates tzdata
WORKDIR /app
COPY --from=backend /server /app/server
COPY --from=backend /src/data /app/data
COPY --from=frontend /fe/dist /app/frontend/dist
EXPOSE 8080
ENV ADDR=:8080
ENV DB_PATH=/app/data/ops.db
CMD ["/app/server"]
