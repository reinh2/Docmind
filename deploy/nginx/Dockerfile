FROM node:22-alpine AS admin-build
WORKDIR /app
COPY web/admin/package.json web/admin/package-lock.json ./
RUN npm ci
COPY web/admin/ ./
RUN npm run build

FROM nginxinc/nginx-unprivileged:1.28-alpine
COPY deploy/nginx/default.conf /etc/nginx/conf.d/default.conf
COPY --from=admin-build /app/dist /usr/share/nginx/html
EXPOSE 8080
