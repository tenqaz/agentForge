# Stage 1: Build
FROM node:22-alpine AS builder

WORKDIR /build

COPY package.json package-lock.json* ./
RUN npm config set registry https://registry.npmmirror.com && npm install

COPY . .

ARG AGENTFORGE_API_BASE_URL=http://api:8080
ENV AGENTFORGE_API_BASE_URL=$AGENTFORGE_API_BASE_URL
ENV NEXT_TELEMETRY_DISABLED=1
RUN npm run build

# Stage 2: Runtime
FROM node:22-alpine

WORKDIR /app

RUN addgroup --system --gid 1001 nodejs && \
    adduser --system --uid 1001 nextjs

COPY --from=builder /build/public ./public
COPY --from=builder --chown=nextjs:nodejs /build/.next/standalone ./
COPY --from=builder --chown=nextjs:nodejs /build/.next/static ./.next/static

USER nextjs

EXPOSE 3000

ENV NODE_ENV=production
ENV NEXT_TELEMETRY_DISABLED=1
ENV HOSTNAME=0.0.0.0

CMD ["node", "server.js"]
