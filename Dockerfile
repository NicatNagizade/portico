FROM node:22-bookworm AS api
WORKDIR /app
COPY backend/package.json backend/package-lock.json ./
RUN npm ci
COPY backend/ .
EXPOSE 8080
CMD ["npm", "run", "api"]

FROM node:22-bookworm-slim AS frontend-build
WORKDIR /app
COPY frontend/ .
RUN npm ci && npm run build

FROM nginx:alpine AS frontend
COPY frontend/nginx.conf /etc/nginx/conf.d/default.conf
COPY --from=frontend-build /app/dist /usr/share/nginx/html
