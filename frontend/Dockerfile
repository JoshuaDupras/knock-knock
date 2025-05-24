FROM node:20-alpine AS build
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
