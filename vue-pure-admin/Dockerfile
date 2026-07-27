FROM node:22.22.1-alpine AS build-stage

WORKDIR /app
RUN corepack enable
RUN corepack prepare pnpm@11.0.0 --activate

COPY package.json pnpm-lock.yaml ./
RUN pnpm install --frozen-lockfile

COPY . .
ARG VITE_MALL_FRONTEND_BASE=""
ENV VITE_MALL_FRONTEND_BASE=${VITE_MALL_FRONTEND_BASE}
RUN pnpm build

FROM nginxinc/nginx-unprivileged:1.27-alpine AS production-stage

COPY nginx.conf /etc/nginx/conf.d/default.conf
COPY --from=build-stage /app/dist /usr/share/nginx/html
EXPOSE 8080

HEALTHCHECK --interval=30s --timeout=3s --start-period=10s --retries=3 CMD wget -q -O /dev/null http://127.0.0.1:8080/healthz || exit 1
