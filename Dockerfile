FROM golang:1.27-bookworm AS api
WORKDIR /src
COPY backend/ .
RUN CGO_ENABLED=1 go build -o /usr/local/bin/api ./cmd/api
EXPOSE 8080
CMD ["api"]

FROM node:20-bookworm-slim AS frontend-build
WORKDIR /app
COPY frontend/ .
RUN npm ci && npm run build

FROM nginx:alpine AS frontend
COPY frontend/nginx.conf /etc/nginx/conf.d/default.conf
COPY --from=frontend-build /app/dist /usr/share/nginx/html
