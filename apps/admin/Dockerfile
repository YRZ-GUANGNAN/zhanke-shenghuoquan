FROM node:22-alpine AS build

WORKDIR /workspace
COPY package.json package-lock.json ./
COPY apps/admin/package.json apps/admin/package.json
RUN npm ci --workspace apps/admin
COPY apps/admin apps/admin
RUN npm --workspace apps/admin run build

FROM nginx:1.27-alpine
COPY --from=build /workspace/apps/admin/dist /usr/share/nginx/html
COPY apps/admin/nginx.conf /etc/nginx/conf.d/default.conf
EXPOSE 80

