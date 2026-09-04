FROM node:20-alpine AS frontend
WORKDIR /app/frontend
COPY frontend/package.json ./
RUN npm install || true
COPY frontend ./
RUN npm run build || echo "no build"

FROM golang:1.24-alpine AS backend
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN go build -o /server ./cmd/server

FROM alpine
WORKDIR /app
COPY --from=backend /server /server
COPY --from=frontend /app/frontend/dist ./static
EXPOSE 7001
CMD ["/server"]
