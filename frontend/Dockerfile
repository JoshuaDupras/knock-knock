FROM node:20-alpine AS build
ARG API_BASE=/api
ENV REACT_APP_API_BASE=${API_BASE}
WORKDIR /web

# 1) Install dependencies (including the local Expo CLI)
COPY package*.json ./
RUN npm ci

# 2) Copy all source
COPY . .

# 3) Export a static web build into ./dist
RUN npx expo export --platform web

# ─── runtime stage ────────────────────────────────────────────
FROM nginx:alpine
COPY --from=build /web/dist /usr/share/nginx/html
EXPOSE 80
